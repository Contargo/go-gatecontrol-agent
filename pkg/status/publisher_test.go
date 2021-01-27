package status

import (
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/buildinfo"
	"fmt"
	"testing"

	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

type DummyChannelCalls struct {
	exchange  string
	key       string
	mandatory bool
	immediate bool
	msg       amqp.Publishing
}

type DummyChannel struct {
	calls []DummyChannelCalls
	err   error
}

func (c *DummyChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	c.calls = append(c.calls, DummyChannelCalls{
		exchange,
		key,
		mandatory,
		immediate,
		msg,
	})
	return c.err
}

func TestPublisherPublish(t *testing.T) {
	t.Run("publishs status", func(t *testing.T) {
		buildinfo.GitSHA = "TerminalOperationsVersion"
		ch := DummyChannel{}
		publisher := NewPublisher("test", 23, "terminal", 42, &ch)

		publisher.UpdateGate("gate-1", "UP")
		publisher.UpdateScanner("scanner-1", "UP")

		err := publisher.Publish()

		assert.NoError(t, err)
		assert.Equal(t, 1, len(ch.calls))
		assert.Equal(t, "gatecontrol.event", ch.calls[0].exchange)
		assert.Equal(t, "gatecontrol.agent.status", ch.calls[0].key)
		assert.Equal(t, false, ch.calls[0].mandatory)
		assert.Equal(t, false, ch.calls[0].immediate)
		assert.Equal(t, "application/json", ch.calls[0].msg.ContentType)
		assert.Equal(t, "{"+
			"\"application\":{\"name\":\"test\",\"instance\":23,\"commitSha\":\"TerminalOperationsVersion\"},"+
			"\"terminal\":{\"locationCode\":\"terminal\",\"loadingPlaceId\":42},"+
			"\"gates\":[{\"name\":\"gate-1\",\"status\":\"UP\"}],"+
			"\"scanners\":[{\"name\":\"scanner-1\",\"status\":\"UP\"}]"+
			"}", string(ch.calls[0].msg.Body))
	})
	t.Run("returns errors", func(t *testing.T) {
		ch := DummyChannel{}
		ch.err = fmt.Errorf("error")
		publisher := NewPublisher("test", 23, "terminal", 42, &ch)

		publisher.UpdateGate("gate-1", "UP")
		publisher.UpdateScanner("scanner-1", "UP")

		err := publisher.Publish()

		assert.Equal(t, err, ch.err)
	})
}

func TestPublisherStatus(t *testing.T) {
	t.Run("creates status representation", func(t *testing.T) {
		publisher := NewPublisher("test", 23, "terminal", 42, nil)

		gate1 := GateStatus{"gate-1", "UP"}
		gate2 := GateStatus{"gate-2", "DOWN"}
		scanner1 := ScannerStatus{"scanner-1", "UP"}
		scanner2 := ScannerStatus{"scanner-2", "DOWN"}

		publisher.UpdateGate(gate1.Name, gate1.Status)
		publisher.UpdateGate(gate2.Name, gate2.Status)
		publisher.UpdateScanner(scanner1.Name, scanner1.Status)
		publisher.UpdateScanner(scanner2.Name, scanner2.Status)

		status := publisher.status()

		assert.Equal(t, "test", status.Application.Name)
		assert.Equal(t, int64(23), status.Application.Instance)
		assert.Equal(t, "terminal", status.Terminal.Location)
		assert.Equal(t, int64(42), status.Terminal.Loadingplace)
		assert.ElementsMatch(t, []GateStatus{gate1, gate2}, status.Gates)
		assert.ElementsMatch(t, []ScannerStatus{scanner1, scanner2}, status.Scanners)
	})
}
