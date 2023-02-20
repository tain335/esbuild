package api

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/evanw/esbuild/internal/fs"
	"github.com/evanw/esbuild/internal/helpers"
	"github.com/evanw/esbuild/internal/logger"
	"github.com/gorilla/websocket"
	"golang.org/x/exp/slices"
)

type PackMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type ClientConnection struct {
	conn  *websocket.Conn
	mutex *sync.Mutex
}

type devApiHandler struct {
	stop func()
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
var timeout = 30 * time.Second

var clientsMutex = sync.Mutex{}
var clients = make([]*ClientConnection, 0, 64)
var fileCache = make(map[string][]byte)

func removeConnFromSet(conn *websocket.Conn) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	index := 1
	for i, client := range clients {
		if client.conn == conn {
			index = i
		}
	}
	if index != -1 {
		slices.Delete(clients, index, index)
	}
}

func addConnToSet(conn *websocket.Conn) *ClientConnection {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	client := &ClientConnection{
		conn:  conn,
		mutex: &sync.Mutex{},
	}
	clients = append(clients, client)
	return client
}

func sendMessageToAllConn(message PackMessage) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	for _, client := range clients {
		client.mutex.Lock()
		client.conn.WriteJSON(message)
		client.mutex.Unlock()
	}
}

func serveClient(client *ClientConnection) {
	defer client.conn.Close()
	client.conn.SetReadLimit(1024 * 1024) // 1MB
	for {
		client.conn.SetReadDeadline(time.Now().Add(timeout))
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			// error
			break
		}
		if string(message) == "ping" {
			client.conn.SetWriteDeadline(time.Now().Add(timeout))
			client.mutex.Lock()
			client.conn.WriteJSON(PackMessage{
				Type: "pong",
			})
			client.mutex.Unlock()
		}
	}
}

func socketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}

	conn.SetCloseHandler(func(code int, text string) error {
		removeConnFromSet(conn)
		return nil
	})

	client := addConnToSet(conn)

	serveClient(client)
}

func resourceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if content, ok := fileCache[path.Base(r.URL.Path)]; ok {
			if strings.HasSuffix(r.URL.Path, ".js") {
				w.Header().Set("Content-Type", "text/javascript")
			} else if strings.HasSuffix(r.URL.Path, ".css") {

				w.Header().Set("Content-Type", "text/css")
			}
			w.WriteHeader(http.StatusOK)
			w.Write(content)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write(fileCache["index.html"])
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func onBuild(br BuildResult) {
	if len(br.Errors) > 0 {
		errorMessage := ""
		for _, error := range br.Errors {
			errorMessage = errorMessage + error.Text + "\n"
			errorMessage = errorMessage + error.Location.File + "\n"
			errorMessage = errorMessage + error.Location.LineText + "\n"
		}
		sendMessageToAllConn(PackMessage{
			Type: "errors",
			Data: errorMessage,
		})
	} else {
		if len(br.OutputFiles) == 0 {
			panic(errors.New("no output files"))
		} else {
			hash := br.OutputFiles[len(br.OutputFiles)-1].Contents

			for _, file := range br.OutputFiles {
				println(file.Path, path.Base(file.Path))
				fileCache[path.Base(file.Path)] = file.Contents
			}
			sendMessageToAllConn(PackMessage{
				Type: "hash",
				Data: string(hash),
			})
			sendMessageToAllConn(PackMessage{
				Type: "ok",
				Data: string(hash),
			})
		}
	}

}

func runBuilder(ctx *internalContext) error {
	ctx.mutex.Lock()
	ctx.watcher = &notifyWatcher{
		fs: ctx.realFS,
		rebuild: func() fs.WatchData {
			dirtyPath := ctx.watcher.tryToFindDirtyPath()
			fmt.Println("dirtyPath: ", dirtyPath)

			timer := &helpers.Timer{}
			log := logger.NewStderrLog(ctx.args.logOptions)
			timer.Begin("Hot rebuild")

			state := ctx.incrementalBuild(dirtyPath)
			go onBuild(state.result)

			timer.End("Hot rebuild")
			timer.Log(log)
			log.Done()
			return state.watchData
		},
	}
	// 必须开启watch mode
	ctx.args.options.WatchMode = true
	ctx.mutex.Unlock()
	timer := &helpers.Timer{}
	log := logger.NewStderrLog(ctx.args.logOptions)
	timer.Begin("First build")
	buildResult := ctx.rebuild().result
	onBuild(buildResult)
	runtime.GC()
	timer.End("First build")
	timer.Log(log)
	log.Done()
	return nil
}

// 1. 实现内存服务
// 2. 实现websoket通知
func (ctx *internalContext) DevServe(opts DevServeOptions) (ServeResult, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	if ctx.didDispose {
		return ServeResult{}, errors.New("cannot run dev serve on a disposed context")
	}

	var port uint16 = 8081
	if opts.Port != 0 {
		port = opts.Port
	}
	host := "0.0.0.0"
	if opts.Host != "" {
		host = opts.Host
	}
	server := http.Server{
		Addr: host + ":" + strconv.Itoa(int(port)),
	}
	http.HandleFunc("/espack-socket", socketHandler)
	http.HandleFunc("/", resourceHandler)
	// TEST
	html := `<!DOCTYPE html>
<html>
	<head></head>
	<body>
	<div id="root"></div>
	<script src="/index.js"></script>
	</body>
</html>
	`
	fileCache["index.html"] = []byte(html)
	serveWaitGroup := sync.WaitGroup{}
	serveWaitGroup.Add(1)
	var serveError error

	go func() {
		serveError = server.ListenAndServe()
		serveWaitGroup.Done()
	}()

	handler := &devApiHandler{
		stop: func() {
			serveWaitGroup.Wait()
			server.Close()
		},
	}

	ctx.devHandler = handler

	go func() {
		runBuilder(ctx)
	}()

	go func() {
		http.ListenAndServe("0.0.0.0:8080", nil)
	}()

	var result ServeResult
	result.Port = port
	result.Host = host
	return result, serveError
}
