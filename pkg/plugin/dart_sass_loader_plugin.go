package plugin

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	godartsass "github.com/bep/godartsass/v2"
	"github.com/evanw/esbuild/pkg/api"
)

//go:embed resources/*
var f embed.FS

func copyEmbedDir(embedPath string, targetPath string) error {
	entries, err := f.ReadDir(embedPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			err := copyEmbedDir(embedPath+"/"+e.Name(), filepath.Join(targetPath, e.Name()))
			if err != nil {
				return err
			}
		} else {
			bytes, err := f.ReadFile(embedPath + "/" + e.Name())
			if err != nil {
				return err
			}
			err = os.MkdirAll(targetPath, 0777)
			if err != nil {
				return err
			}

			info, err := e.Info()
			if err != nil {
				return err
			}
			mode := info.Mode()
			if e.Name() == "dart" || e.Name() == "sass" {
				mode = 0555
			}
			err = os.WriteFile(filepath.Join(targetPath, e.Name()), bytes, mode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func initDartSass(cwd string) error {
	if _, err := os.Stat(filepath.Join(cwd, ".espack", "plugin")); !errors.Is(err, os.ErrNotExist) {
		return nil
	}
	err := copyEmbedDir("resources", filepath.Join(cwd, ".espack", "plugin"))
	return err
}

func getAllNodeModuleDirs(cwd string) []string {
	var result []string
	var dir = cwd

	for {
		result = append(result, filepath.Join(dir, "node_modules"))
		dir = filepath.Dir(dir)
		if dir == "/" {
			break
		}
	}
	cmd := exec.Command("npm", "root", "--location=global")
	out, _ := cmd.CombinedOutput()
	rootNodeModules := string(out)
	result = append(result, strings.TrimSpace(rootNodeModules))
	return result
}

type SourceMap struct {
	Sources []string `json:"sources"`
}

func DartSassLoaderPlugin() api.Plugin {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	err = initDartSass(cwd)
	if err != nil {
		panic(err)
	}
	t, err := godartsass.Start(godartsass.Options{
		DartSassEmbeddedFilename: filepath.Join(cwd, ".espack", "plugin", runtime.GOOS, "dart-sass", "sass"),
		Timeout:                  60 * time.Second,
		LogEventHandler: func(e godartsass.LogEvent) {
			fmt.Println(e)
		},
	})
	if err != nil {
		panic(err)
	}
	allNodeModules := getAllNodeModuleDirs(cwd)
	return api.Plugin{
		Name: "DartSassLoaderPlugin",
		Setup: func(pb api.PluginBuild) {
			pb.OnLoad(api.OnLoadOptions{
				Filter: "\\.scss$",
			}, func(ola api.OnLoadArgs) (api.OnLoadResult, error) {
				start := time.Now().UnixMilli()
				data, err := ioutil.ReadFile(ola.Path)
				if err != nil {
					panic(err)
				}
				content := string(data)
				result, err := t.Execute(godartsass.Args{
					URL:             "file://" + ola.Path,
					Source:          content,
					EnableSourceMap: true,
					IncludePaths:    allNodeModules,
				})
				if err != nil {
					return api.OnLoadResult{}, err
				}
				var sourcemap SourceMap
				json.Unmarshal([]byte(result.SourceMap), &sourcemap)
				var watchFiles []string
				for _, s := range sourcemap.Sources {
					watchFiles = append(watchFiles, strings.TrimPrefix(s, "file://"))
				}
				fmt.Println("load scss consume: ", time.Now().UnixMilli()-start, "ms")
				return api.OnLoadResult{
					Contents:   &result.CSS,
					Loader:     api.LoaderCSS,
					WatchFiles: watchFiles,
				}, nil
			})
		},
	}
}
