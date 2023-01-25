package api

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evanw/esbuild/internal/fs"
	"github.com/gorilla/websocket"
)

type PackMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type devApiHandler struct {
	stop func()
}

var upgrader = websocket.Upgrader{}
var timeout = 30 * time.Second

var connectionsMutex = sync.Mutex{}
var connections = make([]*websocket.Conn, 0, 64)
var fileCache = make(map[string][]byte)

func removeConnFromSet(conn *websocket.Conn) {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()
}

func addConnToSet(conn *websocket.Conn) {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()
	connections = append(connections, conn)
}

func sendMessageToAllConn(message PackMessage) {
	for _, conn := range connections {
		conn.WriteJSON(message)
	}
}

func serveClient(conn *websocket.Conn) {
	defer func() {
		conn.Close()
	}()
	conn.SetReadLimit(1024 * 1024) // 1MB
	for {
		conn.SetReadDeadline(time.Now().Add(timeout))
		_, message, err := conn.ReadMessage()
		if err != nil {
			// error
			break
		}
		if string(message) == "ping" {
			conn.SetWriteDeadline(time.Now().Add(timeout))
			conn.WriteJSON(PackMessage{
				Type: "pong",
			})
		}
	}
}

func socketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		return
	}

	conn.SetCloseHandler(func(code int, text string) error {
		removeConnFromSet(conn)
		return nil
	})

	addConnToSet(conn)

	go serveClient(conn)
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
			w.WriteHeader(http.StatusNotFound)
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

func runBuilder(ctx *internalContext) error {
	ctx.mutex.Lock()
	ctx.watcher = &watcher{
		fs: ctx.realFS,
		rebuild: func() fs.WatchData {
			start := time.Now().UnixMilli()
			state := ctx.rebuild()
			onBuild(state.result)
			fmt.Println("rebuild consume: ", time.Now().UnixMilli()-start, "ms")
			return state.watchData
		},
	}
	// 必须开启watch mode
	ctx.args.options.WatchMode = true
	ctx.mutex.Unlock()
	start := time.Now().UnixMilli()
	ctx.watcher.start(ctx.args.logOptions.LogLevel, ctx.args.logOptions.Color)
	buildResult := ctx.rebuild().result
	onBuild(buildResult)
	fmt.Println("first build consume: ", time.Now().UnixMilli()-start, "ms")
	return nil
}

// 1. 实现内存服务
// 2. 实现websoket通知
func (ctx *internalContext) DevServe(opts DevServeOptions) (ServeResult, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	if ctx.didDispose {
		return ServeResult{}, errors.New("Cannot run dev serve on a disposed context")
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
	<head>
	</head>
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
		time.Sleep(10 * time.Millisecond)
		runBuilder(ctx)
	}()
	var result ServeResult
	result.Port = port
	result.Host = host
	return result, serveError
}
