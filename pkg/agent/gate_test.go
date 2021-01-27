package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGatePurpose(t *testing.T) {
	t.Run("returns purpose for named purpose", func(t *testing.T) {
		purpose, err := NewGatePurpose("entry")
		assert.NoError(t, err)
		assert.Equal(t, PurposeEntry, purpose)
	})
	t.Run("returns error for invalid purpose", func(t *testing.T) {
		_, err := NewGatePurpose("x")
		assert.Error(t, err)
	})
}

func TestGate_Open(t *testing.T) {
	t.Run("open runs configured command", func(t *testing.T) {
		gate := Gate{Cmd: "/bin/true"}
		err := gate.Open()
		assert.NoError(t, err)
	})
	t.Run("returns errors for failed commands", func(t *testing.T) {
		gate := Gate{Cmd: "/bin/false"}
		err := gate.Open()
		assert.Error(t, err)
	})
}
