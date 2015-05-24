package sweet

import (
	"bytes"
	"fmt"
	"github.com/goji/httpauth"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"html/template"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

//// webserver
func RunWebserver(Opts *SweetOptions) {
	if Opts.HttpEnabled == true {
		Opts.LogInfo(fmt.Sprintf("Starting web server on %s", Opts.HttpListen))
		Opts.Hub = newHub()
		router := mux.NewRouter()
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			webIndexHandler(w, r, Opts)
		})
		router.HandleFunc("/ws", Opts.wsMiddleware(Opts.websocketHandler))
		router.HandleFunc("/indices", func(w http.ResponseWriter, r *http.Request) {
			deviceIndexHandler(w, r, Opts)
		})
		router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			webStaticHandler(w, r, Opts)
		})
		if len(Opts.HttpUser) > 0 && len(Opts.HttpPass) > 0 {
			http.Handle("/", httpauth.SimpleBasicAuth(Opts.HttpUser, Opts.HttpPass)(handlers.CombinedLoggingHandler(os.Stderr, router)))
		} else {
			http.Handle("/", handlers.CombinedLoggingHandler(os.Stderr, router))
		}

		go func() {
			err := http.ListenAndServe(Opts.HttpListen, nil)
			if err != nil {
				Opts.LogFatal(fmt.Sprintf("Error starting HTTP server: %s", err.Error()))
			}
		}()

		go func() {
			Opts.Hub.run()
		}()

		go func() {
			for {
				time.Sleep(time.Second)
				metrics := map[string]string{"goroutines": fmt.Sprintf("%d", runtime.NumGoroutine())}
				metrics["devices"] = "123" // TODO
				Opts.Hub.broadcast <- event{MessageType: "metric", Device: "", Metrics: metrics}
			}
		}()

		Opts.LogInfo(fmt.Sprintf("Web server started on %s", Opts.HttpListen))
	}
}

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type WsHandlerFunc func(*http.Request, *websocket.Conn)

type event struct {
	MessageType string            `json:"messageType"`
	Message     string            `json:"messageData"`
	Device      string            `json:"device"`
	Status      DeviceStatus      `json:"status"`
	Metrics     map[string]string `json:"metrics"`
}
type connection struct {
	ws   *websocket.Conn
	send chan event // buffered channel of outbound messages.
}
type Hub struct {
	connections map[*connection]bool // Registered connections.
	broadcast   chan event           // Inbound messages from the connections.
	register    chan *connection     // Register requests from the connections.
	unregister  chan *connection     // Unregister requests from connections.
	lock        sync.Mutex
}

func (Opts *SweetOptions) websocketHandler(r *http.Request, ws *websocket.Conn) {
	vars := mux.Vars(r)
	c := &connection{send: make(chan event, 256), ws: ws}
	Opts.Hub.register <- c
	defer func() {
		Opts.Hub.unregister <- c
	}()
	// reader
	go func() {
		for {
			_, message, err := c.ws.ReadMessage()
			if err != nil {
				Opts.LogErr(fmt.Sprintf("ReadMessage error %v", err.Error()))
				break
			}
			switch string(message) {
			case "":
			default:
				Opts.LogErr(fmt.Sprintf("ReadMessage unrecognized command %v", message))
			}
		}
	}()
	// init messages
	// ongoing messages
	for message := range c.send {
		err := c.ws.WriteJSON(message)
		if err != nil {
			Opts.LogErr(fmt.Sprintf("WS send error %v", err.Error()))
			break
		}
	}
	Opts.LogInfo(fmt.Sprintf("End WS connection %s: %s", vars["object"], ws.RemoteAddr()))
}

func (Opts *SweetOptions) wsMiddleware(fn WsHandlerFunc) http.HandlerFunc {
	f := func(w http.ResponseWriter, r *http.Request) {
		ws, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				Opts.LogErr(fmt.Sprintf("WS middleware error %v", err.Error()))
			}
			Opts.LogErr(fmt.Sprintf("WS upgrade error %v", err.Error()))
			return
		}
		defer func() {
			ws.Close()
		}()
		fn(r, ws)
	}
	return f
}

func newHub() *Hub {
	return &Hub{
		broadcast:   make(chan event),
		register:    make(chan *connection),
		unregister:  make(chan *connection),
		connections: make(map[*connection]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.connections[c] = true
		case c := <-h.unregister:
			if _, ok := h.connections[c]; ok {
				delete(h.connections, c)
				close(c.send)
			}
		case m := <-h.broadcast:
			for c := range h.connections {
				select {
				case c.send <- m:
				default:
					delete(h.connections, c)
					close(c.send)
				}
			}
		}
	}
}

func webIndexHandler(w http.ResponseWriter, r *http.Request, Opts *SweetOptions) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	pageTemplate, err := Asset("frontend/dist/index.html")
	//pageTemplate, err := ioutil.ReadFile("../frontend/dist/index.html")
	if err != nil {
		Opts.LogFatal(fmt.Sprintf("Error with HTTP server template asset: %s", err.Error()))
	}
	t, err := template.New("index").Parse(string(pageTemplate))
	if err != nil {
		Opts.LogFatal(fmt.Sprintf("Error parsing HTTP server template: %s", err.Error()))
	}
	hostname, err := os.Hostname()
	if err != nil {
		Opts.LogFatal(fmt.Sprintf("Error fetching my hostname: %s", err.Error()))
	}
	data := map[string]interface{}{
		"MyHostname": hostname,
		"WsUrl":      fmt.Sprintf("ws://%s/ws", r.Host),
	}
	err = t.Execute(w, data)
	if err != nil {
		Opts.LogFatal(fmt.Sprintf("Error executing HTTP template: %s", err.Error()))
	}
}

// webStaticHandler serves embedded static web files (js&css)
func webStaticHandler(w http.ResponseWriter, r *http.Request, Opts *SweetOptions) {
	assetPath := "frontend/dist/" + r.URL.Path[1:]
	staticAsset, err := Asset(assetPath)
	if err != nil {
		Opts.LogErr(err.Error())
		http.NotFound(w, r)
		return
	}
	headers := w.Header()
	if strings.HasSuffix(assetPath, ".js") {
		headers["Content-Type"] = []string{"application/javascript"}
	} else if strings.HasSuffix(assetPath, ".css") {
		headers["Content-Type"] = []string{"text/css"}
	}
	io.Copy(w, bytes.NewReader(staticAsset))
}

func deviceIndexHandler(w http.ResponseWriter, r *http.Request, Opts *SweetOptions) {
	fmt.Fprintf(w, `{"indices": []}`)
}
