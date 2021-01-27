package agent

import (
	"context"
	"errors"
	"sync"

	"github.com/looplab/fsm"
)

const (
	// EventScanned gets fired when a token was scanned.
	EventScanned string = "scanned"
	// EventValidated gets fired when a request has been validated successfully.
	EventValidated = "validated"
	// EventPrinted gets fired when the interchange for a request has been printed successfully.
	EventPrinted = "printed"
	// EventFinished gets fired when handling a request has finished.
	EventFinished = "finished"
	// EventFailed gets fired when handling a request has failed.
	EventFailed = "failed"
	// EventReset gets fired after a request failed and the error handling completed.
	EventReset = "reset"

	// StateIdle represents the idle state.
	StateIdle string = "idle"
	// StateValidating represents the validating state.
	StateValidating = "validating"
	// StatePrinting represents the printing state.
	StatePrinting = "printing"
	// StateGating represents the gating state.
	StateGating = "gating"
	// StateError represents the error state.
	StateError = "error"
)

var (
	// ErrBusy is returned when the worker is busy handling other requests.
	ErrBusy = errors.New("worker is busy")
	// ErrShutdown is returned when the worker is shutting down.
	ErrShutdown = errors.New("worker is shutting down")
)

type worker struct {
	handler                Handler
	fsm                    *fsm.FSM
	mu                     sync.Mutex
	subs                   map[chan interface{}]struct{}
	shutdownChan, doneChan chan struct{}
}

type FsmScanRequest struct {
	ScanRequest ScanRequest
	State       string
}

func newWorker(handler Handler) *worker {
	w := &worker{
		handler:      handler,
		subs:         map[chan interface{}]struct{}{},
		shutdownChan: make(chan struct{}),
		doneChan:     make(chan struct{}),
	}
	w.fsm = fsm.NewFSM(
		StateIdle,
		fsm.Events{
			{Name: EventScanned, Src: []string{StateIdle}, Dst: StateValidating},
			{Name: EventValidated, Src: []string{StateValidating}, Dst: StatePrinting},
			{Name: EventPrinted, Src: []string{StatePrinting}, Dst: StateGating},
			{Name: EventFinished, Src: []string{StateGating}, Dst: StateIdle},
			{Name: EventFailed, Src: []string{StateValidating, StatePrinting, StateGating}, Dst: StateError},
			{Name: EventReset, Src: []string{StateError}, Dst: StateIdle},
		},
		fsm.Callbacks{
			"enter_state":   w.enterState,
			StateIdle:       w.onIdle,
			StateValidating: w.onValidating,
			StatePrinting:   w.onPrinting,
			StateGating:     w.onGating,
			StateError:      w.onError,
		},
	)

	return w
}

// Shutdown gracefully shuts down the worker without interrupting any active
// processes. Shutdown works by waiting indefinitely to return to idle and then
// shutdown. If the provided context expires before the shutdown is complete,
// Shutdown returns the context's error, otherwise nil.
func (w *worker) Shutdown(ctx context.Context) error {
	close(w.shutdownChan)

	// Worker is idle, therefore we can shutdown immediatly.
	if w.fsm.Current() == StateIdle {
		close(w.doneChan)
	}

	select {
	case <-w.doneChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *worker) Scan(req ScanRequest) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.isShutdown() {
		return ErrShutdown
	}
	if w.fsm.Cannot(EventScanned) {
		return ErrBusy
	}
	go w.fsm.Event(EventScanned, req)
	return nil
}

// Subscribe subscribes msgCh to reveive published messages in the future.
func (w *worker) Subscribe(msgCh chan interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.subs[msgCh] = struct{}{}
}

// Unsubscribe unsubscribes msgCh to reveive published messages in the future.
func (w *worker) Unsubscribe(msgCh chan interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.subs, msgCh)
}

func (w *worker) isShutdown() bool {
	select {
	case <-w.shutdownChan:
		return true
	default:
		return false
	}
}

func (w *worker) publish(msg interface{}) {
	w.mu.Lock()
	for msgCh := range w.subs {
		msgCh <- msg
	}
	w.mu.Unlock()
}
func (w *worker) enterState(e *fsm.Event) {
	req := e.Args[0].(ScanRequest)

	toBePublished := FsmScanRequest{
		ScanRequest: req,
		State:       w.fsm.Current(),
	}
	w.publish(toBePublished)
}

func (w *worker) onIdle(e *fsm.Event) {
	if w.isShutdown() {
		close(w.doneChan)
	}
}

func (w *worker) onValidating(e *fsm.Event) {
	req := e.Args[0].(ScanRequest)
	if err := w.handler.Validate(req); err != nil {
		req.Fail(err)
		go w.fsm.Event(EventFailed, req)
	} else {
		go w.fsm.Event(EventValidated, req)
	}
}

func (w *worker) onPrinting(e *fsm.Event) {
	req := e.Args[0].(ScanRequest)
	if err := w.handler.Print(req); err != nil {
		req.Fail(err)
		go w.fsm.Event(EventFailed, req)
	} else {
		go w.fsm.Event(EventPrinted, req)
	}
}

func (w *worker) onGating(e *fsm.Event) {
	req := e.Args[0].(ScanRequest)
	if err := w.handler.Gate(req); err != nil {
		req.Fail(err)
		go w.fsm.Event(EventFailed, req)
	} else {
		go w.fsm.Event(EventFinished, req)
	}
}

func (w *worker) onError(e *fsm.Event) {
	req := e.Args[0].(ScanRequest)
	w.handler.Error(req)
	go w.fsm.Event(EventReset, req)
}
