package rescan

import (
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/agent"
	worker "contargo.net/gatecontrol/gatecontrol-agent/pkg/agent"
	"log"
	"sync"
	"time"
)

/*
Idea is:
- First listen for fsm events
- if gate is successfully - remember token and time

Provide method handleReentry(tokne) -> bool
-> if token is equal to last scanned token and is in time range
-> open gate means

*/

type lastTokenScan struct {
	token       string
	scannedTime time.Time
}

func (l *lastTokenScan) isStillValid(timeoutInMinute int) bool {

	nowTime := time.Now()
	nowTimeBeofre := nowTime.Add(-time.Duration(timeoutInMinute) * time.Minute)
	return l.scannedTime.After(nowTimeBeofre) && l.scannedTime.Before(nowTime)
}

type RescanHandler struct {
	lastToken       *lastTokenScan
	dataChan        chan interface{}
	shutdownChannel chan struct{}
	manualDataChan  chan bool
	gate            *agent.Gate
	timeoutInMin    int
	mutex           sync.Mutex
}

func NewRescanHandler(dataChan chan interface{}, shutdownChannel chan struct{}, manualDataChan chan bool, gate *agent.Gate, timeoutInMin int) *RescanHandler {
	return &RescanHandler{
		nil,
		dataChan,
		shutdownChannel,
		manualDataChan,
		gate,
		timeoutInMin,
		sync.Mutex{},
	}
}

func (r *RescanHandler) HandleReentry(token string) bool {
	log.Println("Check if token was last one and still valid")
	if r.lastToken != nil && r.lastToken.isStillValid(r.timeoutInMin) {
		if r.lastToken.token != token {
			log.Println("Not the same token - NOT opening the gate again")
			return false
		}
		log.Println("Token still valid, (re)opening gate")
		if err := r.gate.Open(); err != nil {
			log.Println("Opening gate returned err", err)
		}
		r.manualDataChan <- true
		return true
	}
	log.Println("Not opening gate - either expired or no token saved")
	return false
}

func (r *RescanHandler) Listen() {
	for {

		select {
		case fsmData := <-r.dataChan:
			r.mutex.Lock()
			fsmDataCasted := fsmData.(worker.FsmScanRequest)
			if fsmDataCasted.State == worker.StateGating {
				token := fsmDataCasted.ScanRequest.Token()
				r.lastToken = &lastTokenScan{
					token,
					time.Now(),
				}
			}
			r.mutex.Unlock()
			break
		case <-r.shutdownChannel:
			close(r.dataChan)
			return
		}
	}
}
