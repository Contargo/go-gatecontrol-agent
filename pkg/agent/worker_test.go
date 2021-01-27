package agent

import (
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/scanner"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type DummyAgent struct {
	errors chan error
}

func (a *DummyAgent) Step()  { a.errors <- nil }
func (a *DummyAgent) Fail()  { a.errors <- errors.New("failed") }
func (a *DummyAgent) Close() { close(a.errors) }

func (a *DummyAgent) Validate(ScanRequest) error { return <-a.errors }
func (a *DummyAgent) Print(ScanRequest) error    { return <-a.errors }
func (a *DummyAgent) Gate(ScanRequest) error     { return <-a.errors }
func (a *DummyAgent) Error(ScanRequest) error    { return <-a.errors }

func fsmScanValidating(scanRequest ScanRequest) FsmScanRequest {
	return FsmScanRequest{ScanRequest: scanRequest, State: StateValidating}
}
func fsmScanError(scanRequest ScanRequest, err error) FsmScanRequest {
	fsmScanRequest := FsmScanRequest{ScanRequest: scanRequest, State: StateError}
	fsmScanRequest.ScanRequest.error = err
	return fsmScanRequest
}
func fsmScanIdle(scanRequest ScanRequest) FsmScanRequest {
	return FsmScanRequest{ScanRequest: scanRequest, State: StateIdle}
}

func fsmScanIdleError(scanRequest ScanRequest, err error) FsmScanRequest {
	fsmScanRequest := fsmScanIdle(scanRequest)
	fsmScanRequest.ScanRequest.error = err
	return fsmScanRequest
}
func fsmScanPrinting(scanRequest ScanRequest) FsmScanRequest {
	return FsmScanRequest{ScanRequest: scanRequest, State: StatePrinting}
}
func fsmScanGating(scanRequest ScanRequest) FsmScanRequest {
	return FsmScanRequest{ScanRequest: scanRequest, State: StateGating}
}

func TestWorker_FSM(t *testing.T) {
	t.Run("defaults to state idle", func(t *testing.T) {
		w := newWorker(&DummyAgent{})
		assert.Equal(t, StateIdle, w.fsm.Current())
	})
	t.Run("scan returns error when busy", func(t *testing.T) {
		w := newWorker(&DummyAgent{})
		w.fsm.SetState(StateValidating)

		err := w.Scan(ScanRequest{token: scanner.Token{"token", "scanner 1"}})
		assert.EqualError(t, err, ErrBusy.Error())
	})
	t.Run("scan returns error when shutting down", func(t *testing.T) {
		w := newWorker(&DummyAgent{})
		w.Shutdown(context.Background())

		err := w.Scan(ScanRequest{token: scanner.Token{"token", "scanner 1"}})
		assert.EqualError(t, err, ErrShutdown.Error())
	})
	t.Run("scan without errors leads to idle", func(t *testing.T) {
		a := DummyAgent{make(chan error)}
		defer a.Close()

		ch := make(chan interface{}, 1)
		w := newWorker(&a)
		w.Subscribe(ch)

		scanRequest := ScanRequest{token: scanner.Token{"token", "scanner 1"}}
		err := w.Scan(scanRequest)
		assert.NoError(t, err)

		go a.Step()
		assert.Equal(t, fsmScanValidating(scanRequest), <-ch)

		go a.Step()
		assert.Equal(t, fsmScanPrinting(scanRequest), <-ch)

		go a.Step()
		assert.Equal(t, fsmScanGating(scanRequest), <-ch)

		assert.Equal(t, fsmScanIdle(scanRequest), <-ch)
		assert.Equal(t, StateIdle, w.fsm.Current())
	})
	t.Run("invalid scan leads to error", func(t *testing.T) {
		a := DummyAgent{make(chan error)}
		defer a.Close()

		ch := make(chan interface{}, 1)
		w := newWorker(&a)
		w.Subscribe(ch)
		scanRequest := ScanRequest{token: scanner.Token{"token", "scanner 1"}}
		err := w.Scan(scanRequest)
		assert.NoError(t, err)

		go a.Fail()
		assert.Equal(t, fsmScanValidating(scanRequest), <-ch)

		go a.Step()
		assert.Equal(t, fsmScanError(scanRequest, errors.New("failed")), <-ch)

		assert.Equal(t, fsmScanIdleError(scanRequest, errors.New("failed")), <-ch)
		assert.Equal(t, StateIdle, w.fsm.Current())
	})
	t.Run("invalid print leads to error", func(t *testing.T) {
		a := DummyAgent{make(chan error)}
		defer a.Close()

		ch := make(chan interface{}, 1)
		w := newWorker(&a)
		w.Subscribe(ch)
		scanRequest := ScanRequest{token: scanner.Token{"token", "scanner 1"}}
		err := w.Scan(scanRequest)
		assert.NoError(t, err)

		go a.Step()
		assert.Equal(t, fsmScanValidating(scanRequest), <-ch)

		go a.Fail()
		assert.Equal(t, fsmScanPrinting(scanRequest), <-ch)

		go a.Step()
		assert.Equal(t, fsmScanError(scanRequest, errors.New("failed")), <-ch)

		assert.Equal(t, fsmScanIdleError(scanRequest, errors.New("failed")), <-ch)
		assert.Equal(t, StateIdle, w.fsm.Current())
	})
	t.Run("invalid gate leads to error", func(t *testing.T) {
		a := DummyAgent{make(chan error)}
		defer a.Close()

		ch := make(chan interface{}, 1)
		w := newWorker(&a)
		w.Subscribe(ch)
		scanRequest := ScanRequest{token: scanner.Token{"token", "scanner 1"}}
		err := w.Scan(scanRequest)
		assert.NoError(t, err)

		go a.Step()
		assert.Equal(t, fsmScanValidating(scanRequest), <-ch)

		go a.Step()
		assert.Equal(t, fsmScanPrinting(scanRequest), <-ch)

		go a.Fail()
		assert.Equal(t, fsmScanGating(scanRequest), <-ch)

		go a.Step()
		assert.Equal(t, fsmScanError(scanRequest, errors.New("failed")), <-ch)

		assert.Equal(t, fsmScanIdleError(scanRequest, errors.New("failed")), <-ch)
		assert.Equal(t, StateIdle, w.fsm.Current())
	})
	t.Run("can handle errors in error handler", func(t *testing.T) {
		a := DummyAgent{make(chan error)}
		defer a.Close()

		ch := make(chan interface{}, 1)
		w := newWorker(&a)
		w.Subscribe(ch)

		scanRequest := ScanRequest{token: scanner.Token{"token", "scanner 1"}}
		err := w.Scan(scanRequest)
		assert.NoError(t, err)

		go a.Step()
		assert.Equal(t, fsmScanValidating(scanRequest), <-ch)

		go a.Step()
		assert.Equal(t, fsmScanPrinting(scanRequest), <-ch)

		go a.Fail()
		assert.Equal(t, fsmScanGating(scanRequest), <-ch)

		go a.Fail()
		assert.Equal(t, fsmScanError(scanRequest, errors.New("failed")), <-ch)

		assert.Equal(t, fsmScanIdleError(scanRequest, errors.New("failed")), <-ch)
		assert.Equal(t, StateIdle, w.fsm.Current())
	})
}

func TestWorker_Shutdown(t *testing.T) {
	t.Run("terminates worker", func(t *testing.T) {
		w := newWorker(nil)

		err := w.Shutdown(context.Background())
		assert.NoError(t, err)

		_, ok := <-w.shutdownChan
		assert.False(t, ok)

		_, ok = <-w.doneChan
		assert.False(t, ok)
	})
	t.Run("respects context", func(t *testing.T) {
		w := newWorker(nil)
		w.fsm.SetState(StateValidating)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := w.Shutdown(ctx)

		assert.EqualError(t, err, "context canceled")
	})
	t.Run("terminates on idle", func(t *testing.T) {
		w := newWorker(nil)
		w.fsm.SetState(StateGating)

		go func() {
			time.Sleep(100 * time.Millisecond)
			w.fsm.Event(EventFinished, ScanRequest{})
		}()

		err := w.Shutdown(context.Background())
		assert.NoError(t, err)

		_, ok := <-w.shutdownChan
		assert.False(t, ok)

		_, ok = <-w.doneChan
		assert.False(t, ok)
	})
}

func TestWorker_Subscribe(t *testing.T) {
	w := newWorker(nil)

	assert.Equal(t, 0, len(w.subs))

	ch := make(chan interface{}, 1)
	w.Subscribe(ch)
	assert.Equal(t, 1, len(w.subs))
	w.Unsubscribe(ch)
	assert.Equal(t, 0, len(w.subs))
}
