package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
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

type DevServer struct {
	Host         string
	Port         uint32
	ctx          *internalContext
	server       *http.Server
	wg           *sync.WaitGroup
	clientsMutex *sync.Mutex
	clients      []*ClientConnection
	fileCache    map[string][]byte
	closeChannel chan os.Signal
	timeout      time.Duration
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var fileUnits = []string{"KB", "MB", "GB"}

func formatFileSize(size int) (float64, string) {
	var i = 0
	var fsize = float64(size)
	for {
		fsize = fsize / 1000
		if fsize > 1000 && i < len(fileUnits)-1 {
			i++
			continue
		}
		break
	}
	return fsize, fileUnits[i]
}

func NewDevServer(ctx *internalContext, host string, port uint32, timeout time.Duration) *DevServer {
	return &DevServer{
		Host:         host,
		Port:         port,
		ctx:          ctx,
		wg:           &sync.WaitGroup{},
		clientsMutex: &sync.Mutex{},
		clients:      make([]*ClientConnection, 0, 64),
		fileCache:    make(map[string][]byte),
		closeChannel: make(chan os.Signal, 1),
		timeout:      timeout,
	}
}

func (d *DevServer) Run() {
	d.server = &http.Server{
		Addr: fmt.Sprintf("%s:%d", d.Host, d.Port),
	}
	http.HandleFunc("/espack-socket", d.socketHandler)
	http.HandleFunc("/", d.resourceHandler)
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
	d.fileCache["index.html"] = []byte(html)
	go func() {
		logger.Infof("Espack dev server working on %s", d.server.Addr)
		err := d.server.ListenAndServe()
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				logger.Infof("Espack dev server closed")
			} else {
				logger.PrintMessageToStderr(os.Args, logger.Msg{
					Kind: logger.Error,
				})
			}
		}
	}()

	d.runBuilder()

	signal.Notify(d.closeChannel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-d.closeChannel
	d.disposeInternal()
}

func (d DevServer) Dispose() {
	close(d.closeChannel)
}

func (d *DevServer) disposeInternal() {
	if !d.ctx.didDispose {
		d.server.Close()
		d.ctx.Dispose()
	}
}

func (d *DevServer) removeConnFromSet(conn *websocket.Conn) {
	d.clientsMutex.Lock()
	defer d.clientsMutex.Unlock()
	index := 1
	for i, client := range d.clients {
		if client.conn == conn {
			index = i
		}
	}
	if index != -1 {
		slices.Delete(d.clients, index, index)
	}
}

func (d *DevServer) addConnToSet(conn *websocket.Conn) *ClientConnection {
	d.clientsMutex.Lock()
	defer d.clientsMutex.Unlock()
	client := &ClientConnection{
		conn:  conn,
		mutex: &sync.Mutex{},
	}
	d.clients = append(d.clients, client)
	return client
}

func (d *DevServer) sendMessageToAllConn(message PackMessage) {
	d.clientsMutex.Lock()
	defer d.clientsMutex.Unlock()
	for _, client := range d.clients {
		client.mutex.Lock()
		client.conn.WriteJSON(message)
		client.mutex.Unlock()
	}
}

func (d *DevServer) serveClient(client *ClientConnection) {
	defer client.conn.Close()
	client.conn.SetReadLimit(1024 * 1024) // 1MB
	for {
		client.conn.SetReadDeadline(time.Now().Add(d.timeout))
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			break
		}
		if string(message) == "ping" {
			client.conn.SetWriteDeadline(time.Now().Add(d.timeout))
			client.mutex.Lock()
			client.conn.WriteJSON(PackMessage{
				Type: "pong",
			})
			client.mutex.Unlock()
		}
	}
}

func (d *DevServer) socketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}

	conn.SetCloseHandler(func(code int, text string) error {
		d.removeConnFromSet(conn)
		return nil
	})

	client := d.addConnToSet(conn)

	d.serveClient(client)
}

func (d *DevServer) resourceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if content, ok := d.fileCache[path.Base(r.URL.Path)]; ok {
			if strings.HasSuffix(r.URL.Path, ".js") {
				w.Header().Set("Content-Type", "text/javascript")
			} else if strings.HasSuffix(r.URL.Path, ".css") {
				w.Header().Set("Content-Type", "text/css")
			}
			w.WriteHeader(http.StatusOK)
			w.Write(content)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write(d.fileCache["index.html"])
		}
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (d *DevServer) onBuild(br BuildResult) {
	if len(br.Errors) > 0 {
		errorMessage := ""
		for _, error := range br.Errors {
			errorMessage = errorMessage + error.Text + "\n"
			errorMessage = errorMessage + error.Location.File + "\n"
			errorMessage = errorMessage + error.Location.LineText + "\n"
		}
		d.sendMessageToAllConn(PackMessage{
			Type: "errors",
			Data: errorMessage,
		})
	} else {
		if len(br.OutputFiles) == 0 {
			panic(errors.New("no output files"))
		} else {
			hash := br.OutputFiles[len(br.OutputFiles)-1].Contents
			var outputMsg = ""

			for _, file := range br.OutputFiles {
				size, unit := formatFileSize(len(file.Contents))
				outputMsg += fmt.Sprintf("\t%s %.2f%s\n", path.Base(file.Path), size, unit)

				err := os.WriteFile(file.Path, file.Contents, 0644)
				if err != nil {
					logger.Errorf("err: %s", err.Error())
				}
				d.fileCache[path.Base(file.Path)] = file.Contents
			}
			logger.Infof("Output files: \n%s", outputMsg[:len(outputMsg)-1])
			d.sendMessageToAllConn(PackMessage{
				Type: "hash",
				Data: string(hash),
			})
			d.sendMessageToAllConn(PackMessage{
				Type: "ok",
				Data: string(hash),
			})
		}
	}
}

func (d *DevServer) runBuilder() error {
	d.ctx.mutex.Lock()
	d.ctx.watcher = &notifyWatcher{
		fs: d.ctx.realFS,
		rebuild: func() fs.WatchData {
			logger.Clear()
			dirtyPath := d.ctx.watcher.tryToFindDirtyPath()
			logger.Infof("dirty path: %s", dirtyPath)
			timer := &helpers.Timer{}
			log := logger.NewStderrLog(d.ctx.args.logOptions)
			timer.Begin("Hot rebuild")
			state := d.ctx.incrementalBuild(dirtyPath)
			timer.End("Hot rebuild")
			timer.Log(log)
			log.Done()
			d.onBuild(state.result)
			return state.watchData
		},
	}
	// 必须开启watch mode
	d.ctx.args.options.WatchMode = true
	d.ctx.mutex.Unlock()
	timer := &helpers.Timer{}

	logger.Clear()
	log := logger.NewStderrLog(d.ctx.args.logOptions)
	timer.Begin("First build")
	buildResult := d.ctx.rebuild().result
	timer.End("First build")
	timer.Log(log)
	log.Done()

	d.onBuild(buildResult)
	return nil
}

// 1. 实现内存服务
// 2. 实现websoket通知
func (ctx *internalContext) DevServe(opts DevServeOptions) (*DevServer, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	if ctx.didDispose {
		return nil, errors.New("cannot run espack dev serve on a disposed context")
	}

	var port uint16 = 8081
	if opts.Port != 0 {
		port = opts.Port
	}
	host := "0.0.0.0"
	if opts.Host != "" {
		host = opts.Host
	}

	var server = NewDevServer(ctx, host, uint32(port), 30*time.Second)

	ctx.devHandler = &devApiHandler{
		stop: func() {
			server.Dispose()
		},
	}

	return server, nil
}
