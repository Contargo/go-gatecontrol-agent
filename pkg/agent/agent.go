package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"contargo.net/gatecontrol/gatecontrol-agent/pkg/scanner"
)

const (
	logStatusInterval time.Duration = 5 * time.Second
)

// A Handler knows how to validate, print and gate a ScanRequest. Also it knows
// how to handle errors.
type Handler interface {
	Validate(ScanRequest) error
	Print(ScanRequest) error
	Gate(ScanRequest) error
	Error(ScanRequest) error
}

// A ScanRequest represents an scan request received by an agent.
type ScanRequest struct {
	location     string
	loadingPlace int64
	purpose      GatePurpose
	token        scanner.Token
	error        error
}

func (s *ScanRequest) Source() string {
	return string(s.token.ScanSource)
}

// NewScanRequest creates a new scan request.
func NewScanRequest(location string, loadingPlace int64, purpose GatePurpose, token scanner.Token) ScanRequest {
	return ScanRequest{
		location:     location,
		loadingPlace: loadingPlace,
		purpose:      purpose,
		token:        token,
	}
}

func (r *ScanRequest) ScannerName() string {
	return r.token.Scanner
}

// Location returns the location of the scan request.
func (r *ScanRequest) Location() string {
	return r.location
}

// LoadingPlace returns the loading place of the scan request.
func (r *ScanRequest) LoadingPlace() int64 {
	return r.loadingPlace
}

// Purpose returns the purpose of the scan request.
func (r *ScanRequest) Purpose() GatePurpose {
	return r.purpose
}

// Token returns the scanned token of the scan request.
func (r *ScanRequest) Token() string {
	return r.token.Content
}

// Error returns the error of the scan request.
func (r *ScanRequest) Error() error {
	return r.error
}

// Fail marks the scan request as failed.
func (r *ScanRequest) Fail(err error) {
	r.error = err
}

// An Agent defines parameters for running a Gate-Control Agent.
// The zero value for Agent is a valid configuration.
type Agent struct {
	ValidateHandler Callback
	PrintHandler    Callback
	GateHandler     Callback
	ErrorHandler    Callback

	worker                 *worker
	scanChan               chan ScanRequest
	operatorChan           chan string
	shutdownChan, doneChan chan struct{}
	mu                     sync.Mutex
}

// ReadAndStartListen listens on configured input channels and handles incoming requests.
func (a *Agent) Listen() {
	defer a.closeDoneChan()

	for {
		select {
		case req := <-a.getScanChan():
			a.HandleScanRequest(req)
		case <-a.getOperatorChan():
			log.Println("Operator requests are not supported yet.")
		case <-a.getShutdownChan():
			return
		}
	}
}

// Shutdown gracefully shuts down the server without interrupting any active
// processes. Shutdown works by first closing all open input channels and then
// waiting indefinitely to return to idle and then shutdown. If the provided
// context expires before the shutdown is complete, Shutdown returns the
// context's error, otherwise nil.
func (a *Agent) Shutdown(ctx context.Context) error {
	err := a.getWorker().Shutdown(ctx)

	a.closeShutdownChan()

	select {
	case <-a.getDoneChan():
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// HandleScanRequest processes a scanned token.
func (a *Agent) HandleScanRequest(req ScanRequest) {
	err := a.getWorker().Scan(req)
	if err != nil {
		log.Printf("Failed to handle scan request for token %s: %v", req.Token(), err)
	}
}

// Subscribe subscribes msgCh to receive published messages in the future.
func (a *Agent) Subscribe(msgCh chan interface{}) {
	a.getWorker().Subscribe(msgCh)
}

// Unsubscribe unsubscribes msgCh to receive published messages in the future.
func (a *Agent) Unsubscribe(msgCh chan interface{}) {
	a.getWorker().Unsubscribe(msgCh)
}

// Validate implements the Handler interface.
func (a *Agent) Validate(r ScanRequest) error {
	if a.ValidateHandler == nil {
		log.Printf("No handler for \"Validate\". Skipping.")
		return nil
	}
	return a.ValidateHandler.Call(r)
}

// Print implements the Handler interface.
func (a *Agent) Print(r ScanRequest) error {
	if a.PrintHandler == nil {
		log.Printf("No handler for \"Print\". Skipping.")
		return nil
	}
	return a.PrintHandler.Call(r)
}

// Gate implements the Handler interface.
func (a *Agent) Gate(r ScanRequest) error {
	if a.GateHandler == nil {
		log.Printf("No handler for \"Gate\". Skipping.")
		return nil
	}
	return a.GateHandler.Call(r)
}

// Error implements the Handler interface.
func (a *Agent) Error(r ScanRequest) error {
	if a.ErrorHandler == nil {
		log.Printf("No handler for \"Error\". Skipping.")
		return nil
	}
	return a.ErrorHandler.Call(r)
}

func (a *Agent) getWorker() *worker {
	if a.worker == nil {
		a.worker = newWorker(a)
	}
	return a.worker
}

func (a *Agent) getScanChan() chan ScanRequest {
	if a.scanChan == nil {
		a.scanChan = make(chan ScanRequest)
	}
	return a.scanChan
}

func (a *Agent) getOperatorChan() chan string {
	if a.operatorChan == nil {
		a.operatorChan = make(chan string)
	}
	return a.operatorChan
}

func (a *Agent) getShutdownChan() chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.getShutdownChanLocked()
}

func (a *Agent) getShutdownChanLocked() chan struct{} {
	if a.shutdownChan == nil {
		a.shutdownChan = make(chan struct{})
	}
	return a.shutdownChan
}

func (a *Agent) closeShutdownChan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closeShutdownChanLocked()
}

func (a *Agent) closeShutdownChanLocked() {
	ch := a.getShutdownChanLocked()
	select {
	case <-ch:
		// Already closed. Nothing to do.
	default:
		close(ch)
	}
}

func (a *Agent) getDoneChan() chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.getDoneChanLocked()
}

func (a *Agent) getDoneChanLocked() chan struct{} {
	if a.doneChan == nil {
		a.doneChan = make(chan struct{})
	}
	return a.doneChan
}

func (a *Agent) closeDoneChan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closeDoneChanLocked()
}

func (a *Agent) closeDoneChanLocked() {
	ch := a.getDoneChanLocked()
	select {
	case <-ch:
		// Already closed. Nothing to do.
	default:
		close(ch)
	}
}
