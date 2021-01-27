package trafficlights

import (
	worker "contargo.net/gatecontrol/gatecontrol-agent/pkg/agent"
	"github.com/gobuffalo/packr/v2"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
)

type Status struct {
	FsmState     string
	ErrorMessage string
}

type StatusOnline struct {
	IsOnline bool
}

type Webserver struct {
	fsmDataChan     chan interface{}
	manualDataChan  chan bool
	ShutdownChannel chan struct{}
	onlineChannel   chan bool
	upgrader        websocket.Upgrader
	connections     []*websocket.Conn
	mu              sync.Mutex
}

func NewWebserver(fsmDataChan chan interface{}, manualDataChan chan bool, onlineChannel chan bool, shutdownChannel chan struct{}) *Webserver {
	return &Webserver{
		fsmDataChan:     fsmDataChan,
		manualDataChan:  manualDataChan,
		ShutdownChannel: shutdownChannel,
		onlineChannel:   onlineChannel,
		upgrader:        websocket.Upgrader{},
		connections:     []*websocket.Conn{},
	}
}

func (ws *Webserver) closeConnections() {
	for _, connection := range ws.connections {
		if err := connection.Close(); err != nil {
			log.Println("error on closing", err)
		}
	}
}

func getErrorFromScan(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (ws *Webserver) removeConnection(i int) {
	ws.connections[i] = ws.connections[len(ws.connections)-1] // Copy last element to index i.
	ws.connections[len(ws.connections)-1] = nil               // Erase last element (write zero value).
	ws.connections = ws.connections[:len(ws.connections)-1]   // Truncate slice.

}

func (ws *Webserver) informManualOpen() {
	fsmData := worker.FsmScanRequest{
		State:       worker.StateGating,
		ScanRequest: worker.ScanRequest{},
	}
	ws.inform(fsmData)
}

func (ws *Webserver) inform(fsmScanRequest worker.FsmScanRequest) {
	ws.mu.Lock()
	status := Status{
		FsmState:     fsmScanRequest.State,
		ErrorMessage: getErrorFromScan(fsmScanRequest.ScanRequest.Error()),
	}

	for i, conn := range ws.connections {
		if err := conn.WriteJSON(status); err != nil {
			log.Println("can't write", err)
			ws.removeConnection(i)
		}
	}
	ws.mu.Unlock()
}

func (ws *Webserver) informIsOnline(isOnline bool) {
	ws.mu.Lock()
	statusOnline := StatusOnline{
		IsOnline: isOnline,
	}

	for i, conn := range ws.connections {
		if err := conn.WriteJSON(statusOnline); err != nil {
			log.Println("can't write", err)
			ws.removeConnection(i)
		}
	}
	ws.mu.Unlock()
}

func (ws *Webserver) echo(w http.ResponseWriter, r *http.Request) {
	c, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	ws.connections = append(ws.connections, c)
	indexOfConnection := len(ws.connections) - 1
	defer c.Close()
	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			ws.removeConnection(indexOfConnection)
			break
		}
	}
}

func (ws *Webserver) Start(listenAddress string) error {
	log.Println("starting traffic lights on", listenAddress)
	box := packr.New("staticAssets", "./staticAssets")

	http.Handle("/", http.FileServer(box))

	http.HandleFunc("/echo", ws.echo)
	return http.ListenAndServe(listenAddress, nil)
}

func (ws *Webserver) Listen() {
	for {

		select {
		case fsmData := <-ws.fsmDataChan:
			fsmDataCasted := fsmData.(worker.FsmScanRequest)
			log.Println("Received fsm state", fsmDataCasted.State)
			ws.inform(fsmDataCasted)
			break
		case <-ws.manualDataChan:
			ws.informManualOpen()
			break
		case isOnline := <-ws.onlineChannel:
			ws.informIsOnline(isOnline)
			break
		case <-ws.ShutdownChannel:
			close(ws.fsmDataChan)
			return
		}
	}
}
