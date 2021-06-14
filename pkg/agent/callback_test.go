package agent

import (
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/scanner"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallbackFunc(t *testing.T) {
	t.Run("Call calls func", func(t *testing.T) {
		called := false
		request := ScanRequest{token: *scanner.NewToken("test-token", "scanner 1")}

		fn := func(r ScanRequest) error {
			called = true
			assert.Equal(t, request, r)
			return nil
		}

		CallbackFunc(fn).Call(request)
		assert.True(t, called)
	})
}
