package scanner

import (
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

type DummyReadCloser struct {
	value  string
	closed bool
}

func (rc *DummyReadCloser) Read(buf []byte) (int, error) {
	if rc.closed {
		return 0, errors.New("closed")
	}
	return copy(buf[:], rc.value), nil
}

func (rc *DummyReadCloser) Close() error {
	rc.closed = true
	return nil
}

type FailingReadCloser struct {
	closed bool
}

func (rc *FailingReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("always fails")
}

func (rc *FailingReadCloser) Close() error {
	rc.closed = true
	return errors.New("always fails")
}

type EOFReadCloser struct {
	closed bool
}

func (rc *EOFReadCloser) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (rc *EOFReadCloser) Close() error {
	rc.closed = true
	return nil
}

func TestKeyboardScanner(t *testing.T) {
	t.Run("returns its name", func(t *testing.T) {
		scanner, _ := Listen("test", "", &DummyReadCloser{})
		assert.Equal(t, "test", scanner.Name())
	})
	t.Run("returns its prefix", func(t *testing.T) {
		scanner, _ := Listen("test", "prefix", &DummyReadCloser{})
		assert.Equal(t, "prefix", scanner.Prefix())
	})
}

func TestKeyboardScanner_Accept(t *testing.T) {
	t.Run("returns tokens", func(t *testing.T) {
		scanner, _ := Listen("test", "", &DummyReadCloser{value: "test-token\n"})

		token, err := scanner.Accept()
		assert.NoError(t, err)
		assert.Equal(t, "test-token", token)
	})
	t.Run("strips configured prefix", func(t *testing.T) {
		scanner, _ := Listen("test", "test-", &DummyReadCloser{value: "test-token\n"})

		token, err := scanner.Accept()
		assert.NoError(t, err)
		assert.Equal(t, "token", token)
	})
	t.Run("strips static QR prefix", func(t *testing.T) {
		scanner, _ := Listen("test", "", &DummyReadCloser{value: "QRtoken\n"})

		token, err := scanner.Accept()
		assert.NoError(t, err)
		assert.Equal(t, "token", token)
	})
	t.Run("strips configured and static QR prefix", func(t *testing.T) {
		scanner, _ := Listen("test", "IN", &DummyReadCloser{value: "INQRtoken\n"})

		token, err := scanner.Accept()
		assert.NoError(t, err)
		assert.Equal(t, "token", token)
	})
	t.Run("closes scanner on error", func(t *testing.T) {
		reader := &FailingReadCloser{}
		scanner, _ := Listen("test", "", reader)

		_, err := scanner.Accept()
		assert.Error(t, err)
		assert.True(t, reader.closed)
	})
	t.Run("closes scanner on eof", func(t *testing.T) {
		reader := &EOFReadCloser{}
		scanner, _ := Listen("test", "", reader)

		_, err := scanner.Accept()
		assert.Error(t, err)
		assert.Equal(t, io.EOF, err)
		assert.True(t, reader.closed)
	})
}

func TestKeyboardScanner_Close(t *testing.T) {
	t.Run("returns close errors", func(t *testing.T) {
		reader := &FailingReadCloser{}
		scanner, _ := Listen("test", "", reader)

		assert.Error(t, scanner.Close())
	})
}
