package streamlinker

import (
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/evanw/esbuild/internal/ast"
	codespliting "github.com/evanw/esbuild/internal/code_spliting"
	"github.com/evanw/esbuild/internal/config"
	"github.com/evanw/esbuild/internal/css_ast"
	"github.com/evanw/esbuild/internal/css_lexer"
	"github.com/evanw/esbuild/internal/css_printer"
	"github.com/evanw/esbuild/internal/fs"
	"github.com/evanw/esbuild/internal/graph"
	"github.com/evanw/esbuild/internal/helpers"
	"github.com/evanw/esbuild/internal/hmr"
	"github.com/evanw/esbuild/internal/js_ast"
	"github.com/evanw/esbuild/internal/js_parser"
	"github.com/evanw/esbuild/internal/js_template"
	"github.com/evanw/esbuild/internal/logger"
	"github.com/evanw/esbuild/internal/resolver"
	"github.com/evanw/esbuild/internal/runtime"
	"github.com/evanw/esbuild/internal/sourcemap"
)

type StremInputFile struct {
	InputFile      graph.InputFile
	ResolveResults []*resolver.ResolveResult
}

type StreamInputFileCacheEntry struct {
	InputFile      graph.InputFile
	Output         string
	ResolveRsults  []*resolver.ResolveResult
	SourceMapChunk sourcemap.Chunk
	// genereate css file
	CSSMedia    string
	CSSEncoding string
	CSSClones   []uint32
}

func isASCIIOnly(text string) bool {
	for _, c := range text {
		if c < 0x20 || c > 0x7E {
			return false
		}
	}
	return true
}

func generatePackCode(chunkId string, module *LinkModule, execArray []string) (string, sourcemap.LineColumnOffset) {
	var execCode string
	if len(execArray) > 0 {
		var execModuleIds []string
		for _, execModule := range execArray {
			execModuleIds = append(execModuleIds, fmt.Sprintf(`"%s"`, execModule))
		}
		execCode = "[" + strings.Join(execModuleIds, ", ") + "]"
	}
	j := helpers.Joiner{}
	packCodePrev := fmt.Sprintf("webpackJsonp.push([[\"%s\"], {\"%s\": ", chunkId, module.moduleId)
	j.AddString(packCodePrev)
	module.generatedOffset.AdvanceString(packCodePrev)
	j.AddString(module.content)
	packCodePost := fmt.Sprintf(" },[%s]]);\n", execCode)
	j.AddString(packCodePost)

	nextGenerateOffset := sourcemap.LineColumnOffset{}
	return string(j.Done()), nextGenerateOffset
}

type ModuleState int16

const (
	Add ModuleState = iota
	Replace
	Delete
)

type HotModule struct {
	state    ModuleState
	moduleId string
	output   string
}

func generateHotCode(chunkId string, newModules map[uint32]bool, oldModules map[uint32]bool) []byte {
	hotModules := []HotModule{}
	for sourceIndex := range newModules {
		if !oldModules[sourceIndex] {
			hotModules = append(hotModules, HotModule{
				state:    Add,
				moduleId: moduleFileCache[sourceIndex].InputFile.Source.KeyPath.Text,
				output:   moduleFileCache[sourceIndex].Output,
			})
		}
	}

	for sourceIndex := range oldModules {
		if !newModules[sourceIndex] {
			hotModules = append(hotModules, HotModule{
				moduleId: moduleFileCache[sourceIndex].InputFile.Source.KeyPath.Text,
				state:    Delete,
			})
		}
	}

	for _, sourceIndex := range needUpdateModules {
		if oldModules[sourceIndex] && newModules[sourceIndex] {
			hotModules = append(hotModules, HotModule{
				state:    Replace,
				moduleId: moduleFileCache[sourceIndex].InputFile.Source.KeyPath.Text,
				output:   moduleFileCache[sourceIndex].Output,
			})
		}
	}

	if len(hotModules) > 0 {
		output := helpers.Joiner{}
		output.AddString(fmt.Sprintf(`webpackHotUpdate("%s", {`, chunkId))
		for _, module := range hotModules {
			if module.state == Delete {
				output.AddString(fmt.Sprintf("\"%s\": false,\n", module.moduleId))
			} else {
				output.AddString(fmt.Sprintf("\"%s\": %s,\n", module.moduleId, module.output))
			}
		}
		output.AddString("})")
		return output.Done()
	}

	return nil
}

func generateNewSymbolMap(source logger.Source, ast *js_ast.AST) *js_ast.SymbolMap {
	symbolMap := js_ast.NewSymbolMap()
	fileSymbols := append([]js_ast.Symbol{}, ast.Symbols...)
	mutex.Lock()
	symbolMap.SymbolsForSource[0] = append([]js_ast.Symbol{}, moduleFileCache[0].InputFile.Repr.(*graph.JSRepr).AST.Symbols...)
	mutex.Unlock()
	symbolMap.SymbolsForSource[source.Index] = fileSymbols
	return &symbolMap
}

func generateInitialModuleSource(log logger.Log, entrySource logger.Source) logger.Source {
	entryModuleId := entrySource.KeyPath.Text
	source := logger.Source{
		Index: ^uint32(0) - entrySource.Index,
		KeyPath: logger.Path{
			Text: fmt.Sprintf("<initial_module_%d>", entrySource.Index),
		},
		Contents: `
			require("<dev_client>")
			require("<react_refresh_runtime>")
			require("` + entryModuleId + `")
		`,
	}
	moduleAST, _ := js_parser.Parse(log, source, js_parser.Options{})
	moduleAST.ImportRecords = append(moduleAST.ImportRecords,
		ast.ImportRecord{
			Kind:        ast.PackRequire,
			Path:        hmr.DevClientPath,
			SourceIndex: ast.MakeIndex32(runtimeSourceCache[hmr.DevClientPath.Text]),
		},
		ast.ImportRecord{
			Kind:        ast.PackRequire,
			Path:        hmr.ReactRefreshRuntimePath,
			SourceIndex: ast.MakeIndex32(runtimeSourceCache[hmr.ReactRefreshRuntimePath.Text]),
		},
		ast.ImportRecord{
			Kind:        ast.PackRequire,
			SourceIndex: ast.MakeIndex32(entrySource.Index),
			Path:        entrySource.KeyPath,
		})

	symbolMap := generateNewSymbolMap(source, &moduleAST)
	context := NewModuleTransformerContext(log, source, symbolMap, []*resolver.ResolveResult{}, nil, runtimeSourceCache, &moduleAST, buildOptions)
	result := context.transformESMToCJS()
	moduleFileCache[source.Index] = &StreamInputFileCacheEntry{
		InputFile: graph.InputFile{
			Source: source,
			Repr: &graph.JSRepr{
				AST: moduleAST,
			},
		},
		Output:         result.JS,
		SourceMapChunk: result.SourceMapChunk,
	}
	return source
}

func generateRuntimeModule(log logger.Log, source logger.Source) LinkModule {
	if moduleAST, ok := js_parser.Parse(log, source, js_parser.Options{}); ok {
		symbolMap := generateNewSymbolMap(source, &moduleAST)
		context := NewModuleTransformerContext(log, source, symbolMap, []*resolver.ResolveResult{}, nil, runtimeSourceCache, &moduleAST, buildOptions)
		result := context.output(false)
		moduleFileCache[source.Index] = &StreamInputFileCacheEntry{
			InputFile: graph.InputFile{
				Source: source,
				Repr: &graph.JSRepr{
					AST: moduleAST,
				},
			},
			Output:         result.JS,
			SourceMapChunk: result.SourceMapChunk,
		}
		return LinkModule{
			sourceIndex:    source.Index,
			moduleId:       source.KeyPath.Text,
			content:        result.JS,
			sourcemapChunk: result.SourceMapChunk,
		}
	} else {
		panic("cannot parse source: " + source.IdentifierName)
	}
}

func linkModules(index uint32, visited map[uint32]bool, modules []LinkModule) []LinkModule {
	if visited[index] {
		return modules
	}
	visited[index] = true
	entry, ok := moduleFileCache[index]
	if !ok {
		panic(fmt.Sprintf("module not found: %d", index))
	}
	switch repr := entry.InputFile.Repr.(type) {
	case *graph.JSRepr:
		for _, record := range *repr.ImportRecords() {
			if record.Kind == ast.ImportURL {
				continue
			}
			// Ignore records that the parser has discarded. This is used to remove
			// type-only imports in TypeScript files.
			if record.Flags.Has(ast.IsUnused) {
				continue
			}
			modules = linkModules(record.SourceIndex.GetIndex(), visited, modules)
		}
	case *graph.CSSRepr:
		panic(fmt.Sprintf("css module not process: %d", index))
	}
	modules = append(modules, LinkModule{
		sourceIndex:    entry.InputFile.Source.Index,
		moduleId:       entry.InputFile.Source.KeyPath.Text,
		content:        entry.Output,
		sourcemapChunk: entry.SourceMapChunk,
	})
	return modules
}

func transformCSSToJSModule(log logger.Log, source logger.Source, imports string, content string, media string, resolveRsults []*resolver.ResolveResult) ModuleTransformResult {
	code := fmt.Sprintf(`
	%s
	var content = $$content$$;
	if (typeof content === 'string') {
		content = [[module.i, content,  $$media$$]];
	}
	var options = {};
	options.insert = "head";
	options.singleton = false;
	var update = api(content, options);
	module.hot.accept();
	module.hot.dispose(function() {
		update();
	});`, imports)
	moduleAst := js_template.Template(log, source.Index, code, map[string]interface{}{
		"content": &js_ast.EString{Value: helpers.StringToUTF16(content)},
		"media":   &js_ast.EString{Value: helpers.StringToUTF16(media)},
	}, js_parser.Options{})

	moduleAst = combineParts(moduleAst)
	symbolMap := generateNewSymbolMap(source, moduleAst)
	context := NewModuleTransformerContext(log, source, symbolMap, resolveRsults, nil, runtimeSourceCache, moduleAst, buildOptions)
	return context.transformESMToCJS()
}

// 因为每次导入就要执行一次，不同与js只运行一次，这里会复制一份新的文件模块
func generateCSSModule(log logger.Log, source logger.Source, cssAst *css_ast.AST, resolveResults []*resolver.ResolveResult) ModuleTransformResult {
	entry := moduleFileCache[source.Index]
	var importStmts = []string{`import api from "<style_runtime>;"`}
	newResolveResults := append([]*resolver.ResolveResult{},
		&resolver.ResolveResult{
			PathPair: resolver.PathPair{
				Primary: logger.Path{
					Text: "<style_runtime>",
				},
			},
		})

	var rules = make([]css_ast.Rule, 0, len(cssAst.Rules))
	var ruleConditionMap = make(map[uint32]*css_ast.RAtImport)
	var charset *css_ast.RAtCharset
	var encoding = entry.CSSEncoding // may be a clone
	for i := range cssAst.Rules {
		if rule, ok := cssAst.Rules[i].Data.(*css_ast.RAtImport); ok {
			if len(rule.ImportConditions) > 0 {
				ruleConditionMap[rule.ImportRecordIndex] = rule
			}
			continue
		}
		if rule, ok := cssAst.Rules[i].Data.(*css_ast.RAtCharset); ok {
			charset = rule
		}
		rules = append(rules, cssAst.Rules[i])
	}

	if charset == nil && entry.CSSEncoding != "" {
		charset = &css_ast.RAtCharset{
			Encoding: entry.CSSEncoding,
		}
		rules = append([]css_ast.Rule{
			{
				Data: charset,
			},
		}, rules...)
	}

	cssAst.Rules = rules

	var newCSSImportFromExistFile = func(importRecord *ast.ImportRecord, resolveResult *resolver.ResolveResult, media string) {
		// new clone file index
		index := js_ast.MakeDynamicIndex()

		entry := moduleFileCache[importRecord.SourceIndex.GetIndex()]

		importRecord.SourceIndex = ast.MakeIndex32(index)
		resolveResult.PathPair.Primary.Text = fmt.Sprintf("%s<%d>", resolveResult.PathPair.Primary.Text, index)

		clone := entry
		clone.CSSMedia = media
		if encoding != "" {
			clone.CSSEncoding = encoding
		}
		clone.InputFile.Source.KeyPath.Text = resolveResult.PathPair.Primary.Text
		moduleFileCache[index] = clone
		// add a copy
		entry.CSSClones = append(entry.CSSClones, index)

		newResolveResults = append(newResolveResults, resolveResult)
		importStmts = append(importStmts, fmt.Sprintf(`import "%s";`, importRecord.Path.Text))
	}

	var newCSSImportFromNewFile = func(importRecord *ast.ImportRecord, resolveResult *resolver.ResolveResult, media string) {
		index := js_ast.MakeDynamicIndex()
		importRecord.SourceIndex = ast.MakeIndex32(index)
		resolveResult.PathPair.Primary.Text = fmt.Sprintf("%s<%d>", resolveResult.PathPair.Primary.Text, index)
		var content string
		if encoding != "" {
			content = fmt.Sprintf(`@charset "%s";`, encoding)
		}
		content = content + fmt.Sprintf("@import url(%s);", resolveResult.PathPair.Primary.Text)
		output := transformCSSToJSModule(log,
			logger.Source{
				Index: index,
				KeyPath: logger.Path{
					Text:      resolveResult.PathPair.Primary.Text,
					Namespace: "file",
				},
			},
			`import api from "<style_runtime>";`,
			content,
			media,
			[]*resolver.ResolveResult{
				{
					PathPair: resolver.PathPair{
						Primary: logger.Path{
							Text: "<style_runtime>",
						},
					},
				},
			})

		moduleFileCache[index] = &StreamInputFileCacheEntry{
			InputFile: graph.InputFile{
				Repr: &graph.JSRepr{
					AST: js_ast.AST{
						ImportRecords: []ast.ImportRecord{
							{
								Kind:        ast.PackRequire,
								SourceIndex: ast.MakeIndex32(runtimeSourceCache[hmr.StyleRuntimePath.Text]),
								Path:        hmr.StyleRuntimePath,
							},
						},
					},
				},
				Source: logger.Source{
					KeyPath: logger.Path{
						Text:      resolveResult.PathPair.Primary.Text,
						Namespace: "file",
					},
					Index:    index,
					Contents: content,
				},
			},
			ResolveRsults:  []*resolver.ResolveResult{resolveResult},
			Output:         output.JS,
			SourceMapChunk: output.SourceMapChunk,
			CSSMedia:       media,
		}

		newResolveResults = append(newResolveResults, resolveResult)
		importStmts = append(importStmts, fmt.Sprintf(`import "%s";`, importRecord.Path.Text))
	}

	var getMediaText func(tokens *[]css_ast.Token, tokenText []string) []string
	getMediaText = func(tokens *[]css_ast.Token, tokenText []string) []string {
		for _, token := range *tokens {
			tokenText = append(tokenText, token.Text)

			if token.Children != nil {
				tokenText = getMediaText(token.Children, tokenText)
			}
			if token.Kind == css_lexer.TOpenParen {
				tokenText = append(tokenText, ")")
			}
		}
		return tokenText
	}
	cssAst.ImportRecords = append(cssAst.ImportRecords, ast.ImportRecord{
		Kind:        ast.PackRequire,
		SourceIndex: ast.MakeIndex32(runtimeSourceCache[hmr.StyleRuntimePath.Text]),
		Path:        hmr.StyleRuntimePath,
	})
	for i := range cssAst.ImportRecords {
		importRecord := &cssAst.ImportRecords[i]
		switch importRecord.Kind {
		case ast.ImportAt:
			if _, ok := resolver.ParseDataURL(importRecord.Path.Text); ok {
				continue
			}
			if resolveResults[i].IsExternal {
				newCSSImportFromNewFile(importRecord, resolveResults[i], "")
				continue
			}
			if charset != nil {
				moduleFileCache[importRecord.SourceIndex.GetIndex()].CSSEncoding = charset.Encoding
			}
			newResolveResults = append(newResolveResults, resolveResults[i])
			importStmts = append(importStmts, fmt.Sprintf(`import "%s";`, importRecord.Path.Text))
		case ast.ImportAtConditional:
			rule := ruleConditionMap[uint32(i)]
			mediaText := []string{}
			mediaText = getMediaText(&rule.ImportConditions, mediaText)

			media := strings.Join(mediaText, " ")

			if resolveResults[i].IsExternal {
				newCSSImportFromNewFile(importRecord, resolveResults[i], media)
			} else {
				newCSSImportFromExistFile(importRecord, resolveResults[i], media)
			}
		case ast.ImportURL:
			if resolveResults[i].IsExternal {
				continue
			} else if _, ok := resolver.ParseDataURL(importRecord.Path.Text); ok {
				continue
			}
			entry := moduleFileCache[importRecord.SourceIndex.GetIndex()]
			if repr, ok := entry.InputFile.Repr.(*graph.JSRepr); ok {
				if repr.AST.URLForCSS != "" {
					importRecord.Path.Text = repr.AST.URLForCSS
				}
			}
		}
	}
	originRules := cssAst.Rules
	cssAst.Rules = rules
	result := css_printer.Print(*cssAst, css_printer.Options{})
	cssAst.Rules = originRules //因为后面clone还会用来继续解析，所以要保留一份
	needReleaseModules = append(needReleaseModules, source.Index)
	return transformCSSToJSModule(log, source, strings.Join(importStmts, "\n"), string(result.CSS), entry.CSSMedia, newResolveResults)
}

// 必须等所有文件处理完，才可以处理css文件
func processCSSModules(log logger.Log, index uint32, visited map[uint32]bool) {
	if visited[index] {
		return
	}
	visited[index] = true
	entry, ok := moduleFileCache[index]
	if !ok {
		panic(fmt.Sprintf("cannot found file cache: %d", index))
	}
	switch repr := entry.InputFile.Repr.(type) {
	case *graph.CSSRepr:
		output := generateCSSModule(log, entry.InputFile.Source, &repr.AST, entry.ResolveRsults)
		entry.Output = output.JS
		entry.SourceMapChunk = output.SourceMapChunk
		for _, record := range *repr.ImportRecords() {
			if _, ok := resolver.ParseDataURL(record.Path.Text); !ok && record.SourceIndex.IsValid() {
				processCSSModules(log, record.SourceIndex.GetIndex(), visited)
			}
		}
		// rewrite
		entry.InputFile.Repr = &graph.JSRepr{
			AST: js_ast.AST{
				ImportRecords: *repr.ImportRecords(),
			},
		}
	case *graph.JSRepr:
		for _, record := range *repr.ImportRecords() {
			// Ignore records that the parser has discarded. This is used to remove
			// type-only imports in TypeScript files.
			if record.Flags.Has(ast.IsUnused) {
				continue
			}
			if _, ok := resolver.ParseDataURL(record.Path.Text); !ok && record.SourceIndex.IsValid() {
				processCSSModules(log, record.SourceIndex.GetIndex(), visited)
			}
		}
	}

}

func releaseAllModules() {
	for _, index := range needReleaseModules {
		switch repr := moduleFileCache[index].InputFile.Repr.(type) {
		case *graph.JSRepr:
			repr.AST.Parts = []js_ast.Part{}
		case *graph.CSSRepr:
			repr.AST.Rules = []css_ast.Rule{}
		}
	}
}

var mutex sync.Mutex

var moduleFileCache = make(map[uint32]*StreamInputFileCacheEntry)
var chunkModuleCache = make(map[string]map[uint32]bool)
var runtimeSourceCache = make(map[string]uint32)

var needReleaseModules = make([]uint32, 0)
var needUpdateModules = make([]uint32, 0)

var currentHash string

var buildOptions *config.Options

type Manifest struct {
	H string          `json:"h"`
	C map[string]bool `json:"c"`
}

type LinkModule struct {
	sourceIndex     uint32
	moduleId        string
	content         string
	generatedOffset sourcemap.LineColumnOffset
	sourcemapChunk  sourcemap.Chunk
}

func init() {
	runtimeSourceCache[hmr.DevClientPath.Text] = 0
	runtimeSourceCache[hmr.StyleRuntimePath.Text] = 0
	runtimeSourceCache[hmr.ReactRefreshRuntimePath.Text] = 0
}

type SourcemapItem struct {
	Path     string
	Contents []byte
}

type StreamLinkerResult struct {
	OutputFiles []graph.OutputFile
	Chunks      []codespliting.ChunkNode
	WatchData   fs.WatchData
}

func StreamLinker(
	options *config.Options,
	timer *helpers.Timer,
	log logger.Log,
	entryPoints []graph.EntryPoint,
	fileChannel chan StremInputFile,
	doneChannel chan bool,
) StreamLinkerResult {
	buildOptions = options
	var wg sync.WaitGroup
	timer.Begin("Stream link")
	var fileCount = 0
loop:
	for {
		select {
		case file := <-fileChannel:
			fileCount++
			wg.Add(1)
			if _, ok := runtimeSourceCache[file.InputFile.Source.KeyPath.Text]; ok {
				runtimeSourceCache[file.InputFile.Source.KeyPath.Text] = file.InputFile.Source.Index
			}
			handleFile := func() {
				switch repr := file.InputFile.Repr.(type) {
				case *graph.JSRepr:
					var ast *js_ast.AST = &repr.AST
					var symbolMap *js_ast.SymbolMap
					if file.InputFile.Source.Index == 0 {
						newSymbolMap := js_ast.NewSymbolMap()
						symbolMap = &newSymbolMap
						fileSymbols := append([]js_ast.Symbol{}, repr.AST.Symbols...)
						symbolMap.SymbolsForSource[file.InputFile.Source.Index] = fileSymbols
					} else {
						symbolMap = generateNewSymbolMap(file.InputFile.Source, ast)
					}

					context := NewModuleTransformerContext(log, file.InputFile.Source, symbolMap, file.ResolveResults, file.InputFile.InputSourceMap, runtimeSourceCache, ast, buildOptions)
					var result ModuleTransformResult
					if repr.AST.HasLazyExport {
						result = context.transformOtherToCJS()
					} else {
						result = context.transformESMToCJS()
					}
					mutex.Lock()
					repr.AST = js_ast.AST{
						ImportRecords: *repr.ImportRecords(),
					}

					moduleFileCache[file.InputFile.Source.Index] = &StreamInputFileCacheEntry{
						InputFile:      file.InputFile,
						Output:         result.JS,
						ResolveRsults:  file.ResolveResults,
						SourceMapChunk: result.SourceMapChunk,
					}

					if currentHash != "" {
						needUpdateModules = append(needUpdateModules, file.InputFile.Source.Index)
					}
					mutex.Unlock()

					wg.Done()
				case *graph.CSSRepr:
					mutex.Lock()
					moduleFileCache[file.InputFile.Source.Index] = &StreamInputFileCacheEntry{
						InputFile:     file.InputFile,
						ResolveRsults: file.ResolveResults,
					}
					if currentHash != "" {
						needUpdateModules = append(needUpdateModules, file.InputFile.Source.Index)
					}
					mutex.Unlock()
					wg.Done()
				case *graph.CopyRepr:
					wg.Done()
				}
			}
			if file.InputFile.Source.Index == 0 {
				handleFile()
			} else {
				go handleFile()
			}

		case <-doneChannel:
			break loop
		}
	}
	wg.Wait()

	log.AddMsg(logger.Msg{
		Kind: logger.Info,
		Data: logger.MsgData{
			Text: fmt.Sprintf("File Count: %d", fileCount),
		},
	})

	manifest := Manifest{
		H: RandHash(16),
		C: map[string]bool{},
	}

	if flag.Lookup("test.v") != nil {
		manifest.H = "espack_test"
	}

	var watchData = fs.WatchData{
		Paths: map[string]func() string{},
	}
	var outputFiles = []graph.OutputFile{}

	var chunks = []codespliting.ChunkNode{}

	// link
	// 异步chunk
	// 多页面entry chunk，如果不提取到公共模块模块也应该复制一份
	for _, entry := range entryPoints {
		var chunkId = entry.Name
		var modules = []LinkModule{}

		var visitedModules = make(map[uint32]bool)
		processCSSModules(log, entry.SourceIndex, visitedModules)

		visitedModules = make(map[uint32]bool)
		initialModuleSource := generateInitialModuleSource(log, moduleFileCache[entry.SourceIndex].InputFile.Source)

		runtimeSource := runtime.GeneratePackSource(js_ast.MakeDynamicIndex(), manifest.H)
		runtimeModule := generateRuntimeModule(log, runtimeSource)

		modules = append(modules, runtimeModule)
		modules = linkModules(initialModuleSource.Index, visitedModules, modules)

		output := helpers.Joiner{}
		sourcemapOutput := helpers.Joiner{}
		sourceIndexToSourcesIndex := make(map[uint32]int)
		prevOffset := sourcemap.LineColumnOffset{}

		sourmapItems := make([]SourcemapItem, 0, len(modules))

		nextSourcesIndex := 0
		for i := range modules {
			module := &modules[i]
			sourceIndexToSourcesIndex[module.sourceIndex] = nextSourcesIndex
			sm := moduleFileCache[module.sourceIndex].InputFile.InputSourceMap
			if sm != nil {
				for i := range sm.Sources {
					source := sm.Sources[i]
					value := sm.SourcesContent[i]
					// ASCIIOnly 内容只有ASCII字符
					if value.Quoted != "" && (!options.ASCIIOnly || !isASCIIOnly(value.Quoted)) {
						sourmapItems = append(sourmapItems, SourcemapItem{
							Path:     source,
							Contents: []byte(sm.SourcesContent[i].Quoted),
						})
					} else {
						sourmapItems = append(sourmapItems, SourcemapItem{
							Path:     source,
							Contents: helpers.QuoteForJSON(helpers.UTF16ToString(value.Value), options.ASCIIOnly),
						})
					}
				}
				nextSourcesIndex += len(sm.Sources)
			} else {
				contents := moduleFileCache[module.sourceIndex].InputFile.Source.Contents
				sourmapItems = append(sourmapItems, SourcemapItem{
					Path:     module.moduleId,
					Contents: helpers.QuoteForJSON(contents, options.ASCIIOnly),
				})
				nextSourcesIndex++
			}

			module.generatedOffset = prevOffset
			if module.sourceIndex == runtimeSource.Index {
				output.AddString(module.content)
				output.AddString(";\n")
				prevOffset = sourcemap.LineColumnOffset{}
				continue
			}

			var code string
			if i == len(modules)-1 {
				code, prevOffset = generatePackCode(chunkId, module, []string{module.moduleId})
				output.AddString(code)
			} else {
				code, prevOffset = generatePackCode(chunkId, module, []string{})
				output.AddString(code)
			}
		}

		sourcemapOutput.AddString("{\n  \"version\": 3")
		sourcemapOutput.AddString(",\n  \"sources\": [")

		for i, item := range sourmapItems {
			if i != 0 {
				sourcemapOutput.AddString(", ")
			}
			sourcemapOutput.AddBytes(helpers.QuoteForJSON(item.Path, options.ASCIIOnly))
		}

		sourcemapOutput.AddString("]")
		if options.SourceRoot != "" {
			sourcemapOutput.AddString(",\n  \"sourceRoot\": ")
			sourcemapOutput.AddBytes(helpers.QuoteForJSON(options.SourceRoot, options.ASCIIOnly))
		}

		if !options.ExcludeSourcesContent {
			sourcemapOutput.AddString(",\n  \"sourcesContent\": [")
			for i, item := range sourmapItems {
				if i != 0 {
					sourcemapOutput.AddString(", ")
				}
				sourcemapOutput.AddBytes(item.Contents)
			}
			sourcemapOutput.AddString("]")
		}

		sourcemapOutput.AddString(",\n  \"mappings\": \"")
		prevEndState := sourcemap.SourceMapState{}
		prevColumnOffset := 0
		totalQuotedNameLen := 0
		// 主要考虑生成代码是一行还是多行的
		for _, module := range modules {
			offset := module.generatedOffset
			startState := sourcemap.SourceMapState{
				SourceIndex:     sourceIndexToSourcesIndex[module.sourceIndex],
				GeneratedLine:   offset.Lines,
				GeneratedColumn: offset.Columns,
				OriginalName:    totalQuotedNameLen,
			}
			if offset.Lines == 0 {
				startState.GeneratedColumn += prevColumnOffset
			}
			sourcemap.AppendSourceMapChunk(&sourcemapOutput, prevEndState, startState, module.sourcemapChunk.Buffer)
			// 因为后面的sourcemap需要减去前面的数量，因为所有位置都是用相对位置来实现的
			prevEndState = module.sourcemapChunk.EndState
			prevEndState.SourceIndex += startState.SourceIndex
			// 就是生成代码的最后一行column偏移
			prevColumnOffset = module.sourcemapChunk.FinalGeneratedColumn

			totalQuotedNameLen += len(module.sourcemapChunk.QuotedNames)

			if prevEndState.GeneratedLine == 0 {
				prevEndState.GeneratedColumn += startState.GeneratedColumn
				prevColumnOffset += startState.GeneratedColumn
			}
		}

		sourcemapOutput.AddString("\",\n  \"names\": [")
		isFirstName := true
		for _, module := range modules {
			for _, quotedName := range module.sourcemapChunk.QuotedNames {
				if isFirstName {
					isFirstName = false
				} else {
					sourcemapOutput.AddString(", ")
				}
				sourcemapOutput.AddBytes(quotedName)
			}
		}
		sourcemapOutput.AddString("]")

		sourcemapOutput.AddString("\n}\n")

		if currentHash != "" {
			hotData := generateHotCode(chunkId, visitedModules, chunkModuleCache[chunkId])
			if hotData != nil {
				manifest.C[chunkId] = true
				outputFiles = append(outputFiles, graph.OutputFile{
					AbsPath:  filepath.Join(options.AbsOutputDir, chunkId+"."+currentHash+".hot-update.js"),
					Contents: hotData,
				})
			} else {
				manifest.C[chunkId] = false
			}
		}

		chunkModuleCache[chunkId] = visitedModules

		ext := filepath.Ext(entry.OutputPath)
		if ext != "" {
			ext = ""
		} else {
			ext = ".js"
		}
		output.AddString("//# sourceMappingURL=" + entry.OutputPath + ext + ".map")

		content := output.Done()

		outputFiles = append(outputFiles, graph.OutputFile{
			Chunks:   []string{entry.Name},
			AbsPath:  filepath.Join(options.AbsOutputDir, entry.OutputPath+ext),
			Contents: content,
		})

		chunks = append(chunks, codespliting.ChunkNode{
			Name:         entry.Name,
			IsEntryPoint: true,
			Kind:         codespliting.JSChunk,
			Async:        false,
			Content:      content,
			SourceIndex:  entry.SourceIndex,
		})

		outputFiles = append(outputFiles, graph.OutputFile{
			AbsPath:  filepath.Join(options.AbsOutputDir, entry.OutputPath+ext+".map"),
			Contents: sourcemapOutput.Done(),
		})

		var empty = func() string {
			return ""
		}
		for key := range visitedModules {
			entry := moduleFileCache[key]
			watchData.Paths[entry.InputFile.Source.KeyPath.Text] = empty
		}
	}

	if currentHash != "" {
		manifestContent, _ := json.Marshal(manifest)
		outputFiles = append(outputFiles, graph.OutputFile{
			AbsPath:  filepath.Join(options.AbsOutputDir, currentHash+".hot-update.json"),
			Contents: manifestContent,
		})
	}

	releaseAllModules()

	needReleaseModules = make([]uint32, 0)
	needUpdateModules = make([]uint32, 0)

	outputFiles = append(outputFiles, graph.OutputFile{
		AbsPath:  filepath.Join(options.AbsOutputDir, "versions"),
		Contents: []byte(manifest.H),
	})

	currentHash = manifest.H
	timer.End("Stream link")

	log.AddMsg(logger.Msg{
		Kind: logger.Info,
		Data: logger.MsgData{
			Text: "Current Hash: " + currentHash,
		},
	})

	return StreamLinkerResult{
		OutputFiles: outputFiles,
		Chunks:      chunks,
		WatchData:   watchData,
	}
}
