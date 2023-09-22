package bundler

import (
	"fmt"
	"strings"
	"sync"

	"github.com/evanw/esbuild/internal/ast"
	"github.com/evanw/esbuild/internal/cache"
	"github.com/evanw/esbuild/internal/config"
	"github.com/evanw/esbuild/internal/fs"
	"github.com/evanw/esbuild/internal/graph"
	"github.com/evanw/esbuild/internal/helpers"
	"github.com/evanw/esbuild/internal/hmr"
	"github.com/evanw/esbuild/internal/js_parser"
	"github.com/evanw/esbuild/internal/logger"
	"github.com/evanw/esbuild/internal/resolver"
	streamlinker "github.com/evanw/esbuild/internal/stream_linker"
)

type StreamBundle struct {
	uniqueKeyPrefix string
	fs              fs.FS
	res             *resolver.Resolver
	entryPoints     []graph.EntryPoint
	options         config.Options
	fileChannel     chan streamlinker.StremInputFile
	doneChannel     chan bool
}

var lastBundle *StreamBundle
var lastScanner *scanner

func resolveRuntime(log logger.Log, s *scanner, options *config.Options) []parseResult {
	s.results = append(s.results, make([]parseResult, 4)...)
	runtimeResults := make([]parseResult, 4)
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		source, ast, ok := globalRuntimeCache.parseRuntime(options)
		runtimeResults[0] = parseResult{
			file: scannerFile{
				inputFile: graph.InputFile{
					Source: source,
					Repr:   &graph.JSRepr{AST: ast},
				},
			},
			ok: ok,
		}
		wg.Done()
	}()
	parseRuntime := func(index int, source logger.Source) {
		runtimeAST, ok := js_parser.Parse(log, source, js_parser.OptionsFromConfig(&config.Options{
			UnsupportedJSFeatures: options.UnsupportedJSFeatureOverrides,
			MinifySyntax:          options.MinifySyntax,
			MinifyIdentifiers:     options.MinifyIdentifiers,
		}))
		runtimeResults[index] = parseResult{
			file: scannerFile{
				inputFile: graph.InputFile{
					Source: source,
					Repr:   &graph.JSRepr{AST: runtimeAST},
				},
			},
			ok: ok,
		}
		wg.Done()
	}

	go parseRuntime(1, hmr.GenerateDevClient(s.allocateSourceIndex(hmr.DevClientPath, cache.SourceIndexNormal)))
	go parseRuntime(2, hmr.GenerateStyleRuntime(s.allocateSourceIndex(hmr.StyleRuntimePath, cache.SourceIndexNormal)))
	go parseRuntime(3, hmr.GenerateReactRefershRuntime(s.allocateSourceIndex(hmr.ReactRefreshRuntimePath, cache.SourceIndexNormal)))

	wg.Wait()
	return runtimeResults
}

func ScanBundleByStream(
	log logger.Log,
	fs fs.FS,
	caches *cache.CacheSet,
	entryPoints []EntryPoint,
	options config.Options,
	timer *helpers.Timer,
) StreamBundle {
	timer.Begin("Scan Stream phase")
	defer timer.End("Scan Stream phase")
	applyOptionDefaults(&options)

	timer.Begin("On-start callbacks")
	onStartWaitGroup := sync.WaitGroup{}
	for _, plugin := range options.Plugins {
		for _, onStart := range plugin.OnStart {
			onStartWaitGroup.Add(1)
			go func(plugin config.Plugin, onStart config.OnStart) {
				result := onStart.Callback()
				logPluginMessages(fs, log, plugin.Name, result.Msgs, result.ThrownError, nil, logger.Range{})
				onStartWaitGroup.Done()
			}(plugin, onStart)
		}
	}
	// Each bundling operation gets a separate unique key
	uniqueKeyPrefix, err := generateUniqueKeyPrefix()
	if err != nil {
		log.AddError(nil, logger.Range{}, fmt.Sprintf("Failed to read from randomness source: %s", err.Error()))
	}

	s := scanner{
		log:             log,
		fs:              fs,
		res:             resolver.NewResolver(fs, log, caches, options),
		caches:          caches,
		options:         options,
		timer:           timer,
		results:         make([]parseResult, 0, caches.SourceIndexCache.LenHint()),
		visited:         make(map[logger.Path]visitedFile),
		parseArgsMap:    make(map[logger.Path]parseArgs),
		resultChannel:   make(chan parseResult),
		uniqueKeyPrefix: uniqueKeyPrefix,
	}

	runtimeResults := resolveRuntime(log, &s, &options)

	onStartWaitGroup.Wait()
	timer.End("On-start callbacks")

	fileChannel := make(chan streamlinker.StremInputFile)
	doneChannel := make(chan bool)

	s.preprocessInjectedFiles()
	entryPointMeta := s.addEntryPoints(entryPoints)

	go s.scanAllDependenciesByStream(runtimeResults, fileChannel, doneChannel)

	bundle := StreamBundle{
		fs:              fs,
		res:             s.res,
		fileChannel:     fileChannel,
		doneChannel:     doneChannel,
		entryPoints:     entryPointMeta,
		uniqueKeyPrefix: uniqueKeyPrefix,
		options:         s.options,
	}
	lastScanner = &s
	lastBundle = &bundle
	return bundle
}

func (s *scanner) scanAllDependenciesByStream(runtimeResults []parseResult, fileChannel chan streamlinker.StremInputFile, doneChannel chan bool) {
	var emitFile = func(result parseResult) {
		if !result.ok {
			return
		}
		// Don't try to resolve paths if we're not bundling
		if recordsPtr := result.file.inputFile.Repr.ImportRecords(); s.options.Mode == config.ModeBundle && recordsPtr != nil {
			records := *recordsPtr
			for importRecordIndex := range records {
				record := &records[importRecordIndex]

				// Skip this import record if the previous resolver call failed
				resolveResult := result.resolveResults[importRecordIndex]
				if resolveResult == nil {
					continue
				}

				path := resolveResult.PathPair.Primary
				if !resolveResult.IsExternal {
					// Handle a path within the bundle
					sourceIndex := s.maybeParseFile(*resolveResult, resolver.PrettyPath(s.fs, path),
						&result.file.inputFile.Source, record.Range, resolveResult.PluginData, inputKindNormal, nil)
					record.SourceIndex = ast.MakeIndex32(sourceIndex)
				} else {
					// Allow this import statement to be removed if something marked it as "sideEffects: false"
					if resolveResult.PrimarySideEffectsData != nil {
						record.Flags |= ast.IsExternalWithoutSideEffects
					}

					// If the path to the external module is relative to the source
					// file, rewrite the path to be relative to the working directory
					if path.Namespace == "file" {
						if relPath, ok := s.fs.Rel(s.options.AbsOutputDir, path.Text); ok {
							// Prevent issues with path separators being different on Windows
							relPath = strings.ReplaceAll(relPath, "\\", "/")
							if resolver.IsPackagePath(relPath) {
								relPath = "./" + relPath
							}
							record.Path.Text = relPath
						} else {
							record.Path = path
						}
					} else {
						record.Path = path
					}
				}
			}
		}

		s.caches.JSCache.Clear(result.file.inputFile.Source.KeyPath)
		s.results[result.file.inputFile.Source.Index] = result
		fileChannel <- streamlinker.StremInputFile{
			InputFile:      result.file.inputFile,
			ResolveResults: result.resolveResults,
		}
	}
	for _, result := range runtimeResults {
		emitFile(result)
	}
	for s.remaining > 0 {
		s.remaining--
		result := <-s.resultChannel
		emitFile(result)
	}
	doneChannel <- true
}

func (b *StreamBundle) Compile(log logger.Log, timer *helpers.Timer, mangleCache map[string]interface{}) ([]graph.OutputFile, string, fs.WatchData) {
	timer.Begin("Compile phase")
	defer timer.End("Compile phase")

	options := b.options

	// In most cases we don't need synchronized access to the mangle cache
	// options.ExclusiveMangleCacheUpdate = func(cb func(mangleCache map[string]interface{})) {
	// 	cb(mangleCache)
	// }
	results, watchData := streamlinker.StreamLinker(&options, timer, log, b.entryPoints, b.fileChannel, b.doneChannel)
	return results, "", watchData
}

func IncrementalBuildBundle(path string) StreamBundle {
	var sourceIndex uint32
	var lp logger.Path
	for _, f := range lastScanner.results {
		if f.file.inputFile.Source.KeyPath.Text == path {
			sourceIndex = f.file.inputFile.Source.Index
			lp = f.file.inputFile.Source.KeyPath
			break
		}
	}

	if sourceIndex != 0 {
		lastBundle.fs.ClearPath(path)
		lastScanner.remaining++

		go parseFile(lastScanner.parseArgsMap[lp])

		go lastScanner.scanAllDependenciesByStream(make([]parseResult, 0), lastBundle.fileChannel, lastBundle.doneChannel)

		bundle := StreamBundle{
			fs:              lastBundle.fs,
			res:             lastScanner.res,
			entryPoints:     lastBundle.entryPoints,
			uniqueKeyPrefix: lastBundle.uniqueKeyPrefix,
			options:         lastScanner.options,
			fileChannel:     lastBundle.fileChannel,
			doneChannel:     lastBundle.doneChannel,
		}
		lastBundle = &bundle
		return bundle
	}
	return *lastBundle
}
