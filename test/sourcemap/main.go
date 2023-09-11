package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	libsass "github.com/wellington/go-libsass"
)

type ResolveModulesOption struct {
	Module string
	Path   string
}

func ResolveModulePathPlugin(modules []ResolveModulesOption) esbuild.Plugin {
	return esbuild.Plugin{
		Name: "ResolveModulePathPlugin",
		Setup: func(pb esbuild.PluginBuild) {
			for _, module := range modules {
				path := module.Path
				pb.OnResolve(esbuild.OnResolveOptions{
					Filter: module.Module,
				}, func(ora esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					return esbuild.OnResolveResult{
						Path: path,
					}, nil
				})
			}
		},
	}
}

type SassImportResolver struct {
	Prev            string
	RootNodeModules string
}

type SassImportScheme int

const (
	None SassImportScheme = iota
	Local
	Remote
)

func newSassImportResolver(prev string) *SassImportResolver {
	cmd := exec.Command("npm", "root", "--location=global")
	out, err := cmd.CombinedOutput()
	rootNodeModules := string(out)
	if err != nil {
		panic(err)
	}
	return &SassImportResolver{
		Prev:            prev,
		RootNodeModules: rootNodeModules,
	}
}

func (r *SassImportResolver) localFileExists(p string) bool {
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

var matchPatternWithUnderscore = [...]string{"{{file}}.css", "{{file}}.scss", "{{file}}.sass", "{{file}}/index.scss", "{{file}}/index.sass", "{{file}}/_index.scss", "{{file}}/_index.sass"}
var matchPatternNormal = [...]string{"{{file}}.css", "{{file}}.scss", "{{file}}.sass", "_{{file}}.scss", "_{{file}}.sass", "{{file}}/index.scss", "{{file}}/index.sass", "{{file}}/_index.scss", "{{file}}/_index.sass"}

func (r *SassImportResolver) tryReloveLocalFile(base string, filename string) string {

	hasSuffix := strings.HasSuffix(filename, ".css") || strings.HasSuffix(filename, ".scss") || strings.HasSuffix(filename, ".sass")
	if hasSuffix {
		resolved := path.Join(base, filename)
		if r.localFileExists(resolved) {
			return resolved
		}
	} else {
		if strings.HasPrefix(filename, "_") {
			for _, pattern := range matchPatternWithUnderscore {
				maybePath := path.Join(base, strings.ReplaceAll(pattern, "{{file}}", filename[1:]))
				if r.localFileExists(maybePath) {
					return maybePath
				}
			}
		} else {
			for _, pattern := range matchPatternNormal {
				maybePath := path.Join(base, strings.ReplaceAll(pattern, "{{file}}", filename))
				if r.localFileExists(maybePath) {
					return maybePath
				}
			}
		}
	}
	return ""
}

func (r *SassImportResolver) lookupFormNodeModules(url string, searchPaths []string) string {
	dir := r.Prev
	filename := path.Base(url)
	for {
		dir = path.Dir(dir)
		resolvedFile := r.tryReloveLocalFile(path.Join(dir, "node_modules", path.Dir(url)), filename)
		if resolvedFile != "" {
			return resolvedFile
		}
		if dir == "/" {
			for _, p := range searchPaths {
				resolvedFile := r.tryReloveLocalFile(path.Join(p, path.Dir(url)), filename)
				if resolvedFile != "" {
					return resolvedFile
				}
			}
			return ""
		}
	}
}

func (r *SassImportResolver) normalizePath(path string) string {
	segments := strings.Split(path, "/")
	normalizeSegments := make([]string, 0, len(segments))
	cur := 0
	for _, s := range segments {
		if s == ".." {
			if cur != 0 {
				cur = cur - 1
				normalizeSegments[cur] = ""
			}
		}
		if s == "." {
			continue
		}
		normalizeSegments[cur] = s
		cur = cur + 1
	}
	return strings.Join(normalizeSegments, "/")
}

func (r *SassImportResolver) resolveSchemeWithURL(url string) (SassImportScheme, string) {
	if strings.HasPrefix(url, "file://") {
		return Local, url
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "//") {
		return Remote, url
	}

	if strings.HasPrefix(url, "~") {
		return Local, r.lookupFormNodeModules(url[1:], []string{r.RootNodeModules})
	}
	if strings.HasPrefix(url, ".") {
		return Local, r.normalizePath(path.Join(r.Prev, url))
	}
	if strings.HasPrefix(url, "/") {
		return Local, url
	}

	resolvedURL := r.lookupFormNodeModules(url, []string{r.RootNodeModules})
	if resolvedURL != "" {
		return Local, resolvedURL
	}

	return None, ""
}

func (r *SassImportResolver) resolveLocal(url string) (string, error) {
	content, err := ioutil.ReadFile(url)
	return string(content), err
}

func (r *SassImportResolver) resolveRemote(url string) (string, error) {
	return "", errors.New("no implment")
}

func (r *SassImportResolver) resolve(url string) (string, string, error) {
	scheme, resolvedURL := r.resolveSchemeWithURL(url)
	if resolvedURL == "" {
		return "", "", fmt.Errorf("cannot resolve url: %s, import from: %s", url, r.Prev)
	}
	if scheme == Remote {
		content, err := r.resolveRemote(resolvedURL)
		return content, resolvedURL, err
	} else {
		content, err := r.resolveLocal(resolvedURL)
		return content, resolvedURL, err
	}
}

// TODO 返回所有以来的文件路径
func SassLoaderPlugin() esbuild.Plugin {
	return esbuild.Plugin{
		Name: "SassLoaderPlugin",
		Setup: func(pb esbuild.PluginBuild) {
			// cache要加锁
			var sassContentCache = make(map[string]string)
			var sassCompileCache = make(map[string]string)

			getFormCache := func(path string, content string) (string, bool) {
				if c, ok := sassContentCache[path]; ok {
					key := path + "|" + c
					if content == c {
						return sassCompileCache[key], true
					} else {
						delete(sassCompileCache, key)
						return "", false
					}
				}
				return "", false
			}

			setIntoCache := func(path string, content string, compileContent string) {
				oldContent, ok := sassContentCache[path]
				if ok {
					delete(sassCompileCache, path+"|"+oldContent)
				}
				sassContentCache[path] = content
				sassCompileCache[path+"|"+content] = compileContent
			}

			pb.OnLoad(esbuild.OnLoadOptions{
				Filter: ".scss",
			}, func(ola esbuild.OnLoadArgs) (esbuild.OnLoadResult, error) {
				start := time.Now().UnixMilli()
				output := new(bytes.Buffer)
				data, err := ioutil.ReadFile(ola.Path)
				content := string(data)
				if err != nil {
					panic(err)
				}

				if outputContent, ok := getFormCache(ola.Path, content); ok {
					return esbuild.OnLoadResult{
						Contents: &outputContent,
						Loader:   esbuild.LoaderCSS,
					}, nil
				}

				input := new(bytes.Buffer)
				input.WriteString(content)

				var imports *libsass.Imports
				imports = libsass.NewImportsWithResolver(func(url, prev string) (newURL string, body string, resolved bool) {
					resolver := newSassImportResolver(prev)

					content, resolvedURL, err := resolver.resolve(url)
					if err != nil {
						panic(err)
					}

					if output, ok := getFormCache(ola.Path, content); ok {
						return url, output, true
					}

					input := new(bytes.Buffer)

					input.WriteString(content)

					output := new(bytes.Buffer)
					comp, err := libsass.New(output, input, libsass.Path(resolvedURL), libsass.ImportsOption(imports))
					if err != nil {
						panic(err)
					}

					if err := comp.Run(); err != nil {
						panic(err)
					}
					outputContent := output.String()
					setIntoCache(ola.Path, content, outputContent)

					return url, outputContent, true
				})

				comp, err := libsass.New(output, input, libsass.Path(ola.Path), libsass.ImportsOption(imports))
				if err != nil {
					return esbuild.OnLoadResult{}, err
				}

				if err := comp.Run(); err != nil {
					return esbuild.OnLoadResult{}, err
				}

				outputContent := output.String()

				// setIntoCache(ola.Path, content, outputContent)

				fmt.Println("load scss consume: ", time.Now().UnixMilli()-start, "ms")
				return esbuild.OnLoadResult{
					Contents: &outputContent,
					Loader:   esbuild.LoaderCSS,
				}, nil
			})
		},
	}
}

func main() {
	context, err := esbuild.Context(esbuild.BuildOptions{
		EntryPointsAdvanced: []esbuild.EntryPoint{
			{
				InputPath: "./app/index.tsx",
			},
		},
		Outdir:      "build",
		Write:       true,
		Bundle:      true,
		TreeShaking: esbuild.TreeShakingFalse,

		Define: map[string]string{
			"process.env.REACT_APP_ENV": "\"test\"",
			"process.env.NODE_ENV":      "\"development\"",
			"VERSION":                   "\"v1.0.0\"",
			"COMMIT_ID":                 "\"espack_test\"",
		},
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Sourcemap:         esbuild.SourceMapExternal,
		Target:            esbuild.ES2015, // 这里会影响语法树结构
		JSX:               esbuild.JSXTransform,
		LogLevel:          esbuild.LogLevelInfo,
		LogOverride:       make(map[string]esbuild.LogLevel),
		Tsconfig:          "/Users/yanbo.wu/Documents/shopee/infrasec_fe/packages/hids/tsconfig.json",
		Loader: map[string]esbuild.Loader{
			".js":   esbuild.LoaderJSX,
			".jsx":  esbuild.LoaderJSX,
			".ttf":  esbuild.LoaderBase64,
			".png":  esbuild.LoaderBase64,
			".jpg":  esbuild.LoaderBase64,
			".jpeg": esbuild.LoaderBase64,
			".svg":  esbuild.LoaderBase64,
		},
		// Plugins: []esbuild.Plugin{
		// 	SassLoaderPlugin(),
		// },
		// LogLevel:       esbuild.LogLevelError,
		Format:         esbuild.FormatCommonJS, //非常重要 如果是esm模式输出会require重写__require
		AllowOverwrite: true,
		HMR:            true,
	})
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	result, e := context.DevServe(esbuild.DevServeOptions{
		Host: "127.0.0.1",
		Port: 8081,
	})
	if e != nil {
		panic(e)
	}
	fmt.Printf("espack running at %s:%d\n", result.Host, result.Port)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-c
}
