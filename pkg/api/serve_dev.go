package api

import (
	"net/http"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}
var timeout = 30 * time.Second

var connectionsMutex = sync.Mutex{}
var connections = make([]*websocket.Conn, 0, 64)
var fileCache = make(map[string][]byte)

type PackMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

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

func runBuilder(buildOptions BuildOptions) error {
	go func() {
		watch := buildOptions.Watch
		// 必须开启watch mode
		buildOptions.Watch = &WatchMode{
			OnRebuild: func(br BuildResult) {
				if watch != nil && watch.OnRebuild != nil {
					watch.OnRebuild(br)
				}
				onBuild(br)
			},
		}
		buildResult := buildImpl(buildOptions)
		onBuild(buildResult.result)
	}()
	return nil
}

// 1. 实现内存服务
// 2. 实现websoket通知
func devServeImpl(serveOptions ServeOptions, buildOptions BuildOptions) (result ServeResult, err error) {

	var port uint16 = 8081
	if serveOptions.Port != 0 {
		port = serveOptions.Port
	}
	host := "0.0.0.0"
	if serveOptions.Host != "" {
		host = serveOptions.Host
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

	result.Wait = func() error {
		serveWaitGroup.Wait()
		return serveError
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		runBuilder(buildOptions)
	}()
	result.Stop = func() {
		server.Close()
		serveWaitGroup.Wait()
	}
	result.Port = port
	result.Host = host
	return
}
