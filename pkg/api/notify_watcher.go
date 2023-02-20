package api

import (
	"fmt"
	"sync"

	"github.com/evanw/esbuild/internal/fs"
	"github.com/evanw/esbuild/internal/helpers"
	"github.com/evanw/esbuild/internal/logger"
	"github.com/fsnotify/fsnotify"
)

type watcherInterface interface {
	setWatchData(data fs.WatchData)
	start(logLevel logger.LogLevel, useColor logger.UseColor)
	stop()
	tryToFindDirtyPath() string
}

type notifyWatcher struct {
	data             fs.WatchData
	dirtyPath        string
	fs               fs.FS
	rebuild          func() fs.WatchData
	mutex            sync.Mutex
	internaleWatcher *fsnotify.Watcher
}

func (n *notifyWatcher) setWatchData(data fs.WatchData) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.stop()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	n.internaleWatcher = watcher
	for key := range data.Paths {
		if _, ok := n.data.Paths[key]; !ok {
			if !helpers.IsInsideNodeModules(key) && !helpers.IsInternalModule(key) {
				err := n.internaleWatcher.Add(key)
				if err != nil {
					fmt.Println(err.Error())
				}
			}
		}
	}
	n.data = data

	go func() {
		for {
			select {
			case event, ok := <-n.internaleWatcher.Events:

				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) {
					n.mutex.Lock()
					n.dirtyPath = event.Name
					n.mutex.Unlock()
					n.rebuild()
				}
			case err, ok := <-n.internaleWatcher.Errors:
				if !ok {
					return
				}
				fmt.Println("error:", err)
			}
		}
	}()
}

func (n *notifyWatcher) start(logLevel logger.LogLevel, useColor logger.UseColor) {}

func (n *notifyWatcher) stop() {
	if n.internaleWatcher != nil {
		n.internaleWatcher.Close()
	}
}

func (n *notifyWatcher) tryToFindDirtyPath() string {
	return n.dirtyPath
}
