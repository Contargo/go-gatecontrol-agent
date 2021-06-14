package scanner

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func DummyOpener(name, prefix string) Opener {
	return Opener(func() (*Scanner, error) {
		return Listen(name, prefix, &DummyReadCloser{value: "test-token\n"})
	})
}

func FailingOpener(name string) Opener {
	return Opener(func() (*Scanner, error) {
		return nil, errors.New("failed")
	})
}

func TestReopeningScanner(t *testing.T) {
	t.Run("returns its name", func(t *testing.T) {
		scanner := NewReopeningScanner("scanner", DummyOpener("", ""))
		assert.Equal(t, "scanner", scanner.Name())
	})
}

func TestReopeningScanner_Listen(t *testing.T) {
	t.Run("notifies on tokens", func(t *testing.T) {
		scanner := NewReopeningScanner("scanner", DummyOpener("", ""))
		go scanner.Listen()
		defer scanner.Shutdown(context.Background())

		ch := make(chan Token)
		scanner.NotifyTokens(ch)
		assert.Equal(t, <-ch, *NewToken("test-token", ""))
	})
	t.Run("notifies on up status", func(t *testing.T) {
		scanner := NewReopeningScanner("scanner", DummyOpener("", ""))
		go scanner.Listen()
		defer scanner.Shutdown(context.Background())

		ch := make(chan Status)
		scanner.NotifyStatus(ch)
		status := <-ch
		assert.Equal(t, "scanner", status.Name)
		assert.Equal(t, StateUp, status.State)
		assert.NoError(t, status.Error)
	})
	t.Run("notifies on down status", func(t *testing.T) {
		scanner := NewReopeningScanner("scanner", FailingOpener(""))
		go scanner.Listen()
		defer scanner.Shutdown(context.Background())

		ch := make(chan Status)
		scanner.NotifyStatus(ch)
		status := <-ch
		assert.Equal(t, "scanner", status.Name)
		assert.Equal(t, StateDown, status.State)
		assert.Error(t, status.Error)
	})
}

func TestReopeningScanner_Shutdown(t *testing.T) {
	t.Run("terminates reopening scanner", func(t *testing.T) {
		scanner := NewReopeningScanner("scanner", DummyOpener("", ""))
		go scanner.Listen()

		err := scanner.Shutdown(context.Background())
		assert.NoError(t, err)

		_, ok := <-scanner.shutdownChan
		assert.False(t, ok)

		_, ok = <-scanner.doneChan
		assert.False(t, ok)
	})
	t.Run("respects context", func(t *testing.T) {
		scanner := NewReopeningScanner("scanner", DummyOpener("", ""))
		go scanner.Listen()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := scanner.Shutdown(ctx)

		assert.EqualError(t, err, "context canceled")
	})
}
