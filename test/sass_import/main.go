package main

import (
	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/evanw/esbuild/pkg/plugin"
)

func main() {
	context, err := esbuild.Context(esbuild.BuildOptions{
		EntryPointsAdvanced: []esbuild.EntryPoint{
			{
				InputPath: "./src/index.tsx",
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
		Plugins: []esbuild.Plugin{
			plugin.DartSassLoaderPlugin(),
		},
		// LogLevel:       esbuild.LogLevelError,
		Format:         esbuild.FormatCommonJS, //非常重要 如果是esm模式输出会require重写__require
		AllowOverwrite: true,
		HMR:            true,
	})
	if err != nil {
		panic(err)
	}

	server, e := context.DevServe(esbuild.DevServeOptions{
		Host: "127.0.0.1",
		Port: 8081,
	})
	if e != nil {
		panic(e)
	}
	server.Run(true)
}
