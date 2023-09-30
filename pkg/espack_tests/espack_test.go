package espacktests

import (
	"errors"
	"os"
	"path"
	"testing"

	"github.com/evanw/esbuild/internal/test"
	api "github.com/evanw/esbuild/pkg/api"
	plugin "github.com/evanw/esbuild/pkg/plugin"
)

func TestModuleImport(t *testing.T) {
	execUserCase(t, "module_import")
	compare(t, "module_import")
}

func TestSassImport(t *testing.T) {
	execUserCase(t, "sass_import")
	compare(t, "sass_import")
}

func TestSourcemap(t *testing.T) {
	execUserCase(t, "sourcemap")
	compare(t, "sourcemap")
}

func TestHTMLTemplate(t *testing.T) {
	context, err := api.Context(api.BuildOptions{
		EntryPointsAdvanced: []api.EntryPoint{
			{
				InputPath: "./html_template/src/index.tsx",
			},
		},
		Outdir:            "./html_template/build",
		Write:             true,
		Bundle:            true,
		TreeShaking:       api.TreeShakingFalse,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Sourcemap:         api.SourceMapExternal,
		Target:            api.ES2015, // 这里会影响语法树结构
		JSX:               api.JSXTransform,
		LogLevel:          api.LogLevelInfo,
		LogOverride:       make(map[string]api.LogLevel),
		Loader: map[string]api.Loader{
			".js":   api.LoaderJSX,
			".jsx":  api.LoaderJSX,
			".ttf":  api.LoaderDataURL,
			".png":  api.LoaderDataURL,
			".jpg":  api.LoaderDataURL,
			".jpeg": api.LoaderDataURL,
			".svg":  api.LoaderDataURL,
		},
		Plugins: []api.Plugin{
			plugin.DartSassLoaderPlugin(),
			plugin.HTMLTemplatePlugin(plugin.HTMLTemplatePluginOptions{
				Entrires: []plugin.TemplateEntry{
					{
						Template: "./html_template/src/index.html",
						Chunks:   []string{"main"},
					},
				},
				Data: map[string]interface{}{
					"Key":  "Value",
					"Arr":  []string{"Apple", "Orange", "Banana"},
					"Cond": true,
				},
			}),
		},
		Format:         api.FormatCommonJS, //非常重要 如果是esm模式输出会require重写__require
		AllowOverwrite: true,
		HMR:            true,
	})
	if err != nil {
		panic(err)
	}

	server, e := context.DevServe(api.DevServeOptions{
		Host: "127.0.0.1",
		Port: 8081,
	})
	if e != nil {
		panic(e)
	}
	server.Run(false)
}

func compareDir(t *testing.T, sourceDir, targetDir string) {
	sources, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Error(err)
		return
	}

	targets, err := os.ReadDir(targetDir)
	if err != nil {
		t.Error(err)
		return
	}

	if len(sources) != len(targets) {
		t.Error(errors.New("builds not equal snapshot"))
		return
	}
	for _, entry := range sources {
		entryPath := path.Join(sourceDir, entry.Name())
		if entry.IsDir() {
			compareDir(t, path.Join(sourceDir, entryPath), path.Join(targetDir, entryPath))
		} else {
			sourceContent, err := os.ReadFile(entryPath)
			if err != nil {
				t.Error(err)
				return
			}
			targetContent, err := os.ReadFile(path.Join(targetDir, entry.Name()))
			if err != nil {
				t.Error(err)
				return
			}
			test.AssertEqualWithDiff(t, string(sourceContent), string(targetContent))
		}
	}
}

func compare(t *testing.T, userCase string) {
	compareDir(t, "./"+userCase+"/build", "./"+userCase+"/snapshot")
}

func execUserCase(t *testing.T, userCase string) {
	context, err := api.Context(api.BuildOptions{
		EntryPointsAdvanced: []api.EntryPoint{
			{
				InputPath: "./" + userCase + "/src/index.tsx",
			},
		},
		Outdir:            "./" + userCase + "/build",
		Write:             true,
		Bundle:            true,
		TreeShaking:       api.TreeShakingFalse,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Sourcemap:         api.SourceMapExternal,
		Target:            api.ES2015, // 这里会影响语法树结构
		JSX:               api.JSXTransform,
		LogLevel:          api.LogLevelInfo,
		LogOverride:       make(map[string]api.LogLevel),
		Loader: map[string]api.Loader{
			".js":   api.LoaderJSX,
			".jsx":  api.LoaderJSX,
			".ttf":  api.LoaderDataURL,
			".png":  api.LoaderDataURL,
			".jpg":  api.LoaderDataURL,
			".jpeg": api.LoaderDataURL,
			".svg":  api.LoaderDataURL,
		},
		Plugins: []api.Plugin{
			plugin.DartSassLoaderPlugin(),
		},
		Format:         api.FormatCommonJS, //非常重要 如果是esm模式输出会require重写__require
		AllowOverwrite: true,
		HMR:            true,
	})
	if err != nil {
		panic(err)
	}

	server, e := context.DevServe(api.DevServeOptions{
		Host: "127.0.0.1",
		Port: 8081,
	})
	if e != nil {
		panic(e)
	}
	server.Run(false)
}
