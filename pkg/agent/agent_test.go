package agent

import (
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/scanner"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type DummyHandler struct {
	timesCalled int
	lastCalled  time.Time
}

func (h *DummyHandler) Handle(ScanRequest) error {
	h.timesCalled++
	h.lastCalled = time.Now()
	return nil
}

func TestAgent_ScanRequest(t *testing.T) {
	t.Run("makes attributes available", func(t *testing.T) {
		req := NewScanRequest("location", 42, PurposeEntry, *scanner.NewToken("token", "scanner 1"))
		assert.Equal(t, "location", req.Location())
		assert.Equal(t, int64(42), req.LoadingPlace())
		assert.Equal(t, PurposeEntry, req.Purpose())
		assert.Equal(t, "token", req.Token())
		assert.Nil(t, req.Error())
	})
	t.Run("can have errors", func(t *testing.T) {
		req := NewScanRequest("location", 42, PurposeEntry, *scanner.NewToken("token", "scanner 1"))
		err := fmt.Errorf("fail")
		req.Fail(err)
		assert.Equal(t, err, req.Error())
	})
}

func TestAgent_Listen(t *testing.T) {
	t.Run("handles scan requests", func(t *testing.T) {
		agent := &Agent{
			ValidateHandler: &NopCallback,
			PrintHandler:    &NopCallback,
			GateHandler:     &NopCallback,
			ErrorHandler:    &NopCallback,
		}
		go agent.Listen()
		defer agent.Shutdown(context.Background())

		ch := make(chan interface{}, 10)
		agent.Subscribe(ch)
		scanRequest := ScanRequest{token: *scanner.NewToken("test-token", "scanner 1")}
		agent.getScanChan() <- scanRequest

		assert.Equal(t, FsmScanRequest{ScanRequest: scanRequest, State: StateValidating}, <-ch)
	})
	t.Run("ignores operator requests", func(t *testing.T) {
		agent := &Agent{}
		go agent.Listen()
		defer agent.Shutdown(context.Background())

		agent.getOperatorChan() <- "test-token"

		// TODO: how to assert?
	})
}

func TestAgent_Shutdown(t *testing.T) {
	t.Run("terminates agent", func(t *testing.T) {
		agent := &Agent{}
		go agent.Listen()

		err := agent.Shutdown(context.Background())
		assert.NoError(t, err)

		_, ok := <-agent.shutdownChan
		assert.False(t, ok)

		_, ok = <-agent.doneChan
		assert.False(t, ok)
	})
	t.Run("respects context", func(t *testing.T) {
		agent := &Agent{}
		go agent.Listen()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := agent.Shutdown(ctx)

		assert.EqualError(t, err, "context canceled")
	})
}

func TestAgent_Subscribe(t *testing.T) {
	t.Run("can subscribe and unsubscribe channels", func(t *testing.T) {
		agent := &Agent{
			ValidateHandler: &NopCallback,
			PrintHandler:    &NopCallback,
			GateHandler:     &NopCallback,
			ErrorHandler:    &NopCallback,
		}
		go agent.Listen()
		defer agent.Shutdown(context.Background())

		ch := make(chan interface{}, 10)
		agent.Subscribe(ch)
		scanRequest := ScanRequest{token: *scanner.NewToken("test-token", "scanner 1")}
		agent.getScanChan() <- scanRequest

		assert.Equal(t, FsmScanRequest{ScanRequest: scanRequest, State: StateValidating}, <-ch)
		assert.Equal(t, FsmScanRequest{ScanRequest: scanRequest, State: StatePrinting}, <-ch)
		assert.Equal(t, FsmScanRequest{ScanRequest: scanRequest, State: StateGating}, <-ch)
		assert.Equal(t, FsmScanRequest{ScanRequest: scanRequest, State: StateIdle}, <-ch)

		agent.Unsubscribe(ch)
		agent.getScanChan() <- ScanRequest{token: *scanner.NewToken("test-token", "scanner 1")}

		select {
		case state := <-ch:
			assert.Fail(t, "Received state from channel: %s", state)
		default:
		}
	})
}

func TestAgent_Validate(t *testing.T) {
	t.Run("calls configured validate handler", func(t *testing.T) {
		called := false
		agent := &Agent{
			ValidateHandler: CallbackFunc(func(ScanRequest) error {
				called = true
				return nil
			}),
		}
		err := agent.Validate(ScanRequest{})
		assert.NoError(t, err)

		assert.True(t, called)
	})
	t.Run("can handle missing validate handler", func(t *testing.T) {
		agent := &Agent{}
		err := agent.Validate(ScanRequest{})
		assert.NoError(t, err)
	})
}

func TestAgent_Print(t *testing.T) {
	t.Run("calls configured print handler", func(t *testing.T) {
		called := false
		agent := &Agent{
			PrintHandler: CallbackFunc(func(ScanRequest) error {
				called = true
				return nil
			}),
		}
		err := agent.Print(ScanRequest{})
		assert.NoError(t, err)

		assert.True(t, called)
	})
	t.Run("can handle missing print handler", func(t *testing.T) {
		agent := &Agent{}
		err := agent.Print(ScanRequest{})
		assert.NoError(t, err)
	})
}

func TestAgent_Gate(t *testing.T) {
	t.Run("calls configured gate handler", func(t *testing.T) {
		called := false
		agent := &Agent{
			GateHandler: CallbackFunc(func(ScanRequest) error {
				called = true
				return nil
			}),
		}
		err := agent.Gate(ScanRequest{})
		assert.NoError(t, err)

		assert.True(t, called)
	})
	t.Run("can handle missing gate handler", func(t *testing.T) {
		agent := &Agent{}
		err := agent.Gate(ScanRequest{})
		assert.NoError(t, err)
	})
}

func TestAgent_Error(t *testing.T) {
	t.Run("calls configured error handler", func(t *testing.T) {
		called := false
		agent := &Agent{
			ErrorHandler: CallbackFunc(func(ScanRequest) error {
				called = true
				return nil
			}),
		}
		err := agent.Error(ScanRequest{})
		assert.NoError(t, err)

		assert.True(t, called)
	})
	t.Run("can handle missing error handler", func(t *testing.T) {
		agent := &Agent{}
		err := agent.Error(ScanRequest{})
		assert.NoError(t, err)
	})
}
