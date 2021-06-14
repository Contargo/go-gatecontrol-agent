package metrics

import (
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/agent"
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/scanner"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"regexp"
	"strconv"
	"testing"
)

type InfluxClientMock struct {
	receivedValue chan string
}

func (i *InfluxClientMock) Write(line string) error {
	i.receivedValue <- line
	return nil
}

func (i *InfluxClientMock) Receive() string {
	fmt.Println("receive")
	receivedValue := <-i.receivedValue
	fmt.Println("got value", receivedValue)
	return receivedValue
}

func assertStateError(t *testing.T, expectedNumericalState int, actualData string, err string) {
	hostname, _ := os.Hostname()
	assert.Regexp(t, regexp.MustCompile("go-gateagent host=\""+hostname+"\",scanner=\"scanner 1\",state="+strconv.FormatInt(int64(expectedNumericalState), 10)+",error=\""+err+"\" \\d+\n"), actualData)
}

func assertState(t *testing.T, expectedNumericalState int, actualData string) {
	assertStateError(t, expectedNumericalState, actualData, "no error")
}

func TestMetrics(t *testing.T) {
	t.Run("should reflect state changes in influx", func(t *testing.T) {
		scanRequest := agent.NewScanRequest("", 123, agent.PurposeEntry, *scanner.NewToken("token", "scanner 1"))
		shutdownChan := make(chan struct{})
		metricsChannel := make(chan interface{})

		influxClientMock := InfluxClientMock{make(chan string, 4)}
		go Listen(&influxClientMock, metricsChannel, shutdownChan)
		metricsChannel <- agent.FsmScanRequest{State: agent.StateIdle, ScanRequest: scanRequest}
		assertState(t, 0, influxClientMock.Receive())
		metricsChannel <- agent.FsmScanRequest{State: agent.StateValidating, ScanRequest: scanRequest}
		assertState(t, 1, influxClientMock.Receive())
		metricsChannel <- agent.FsmScanRequest{State: agent.StatePrinting, ScanRequest: scanRequest}
		assertState(t, 2, influxClientMock.Receive())
		metricsChannel <- agent.FsmScanRequest{State: agent.StateGating, ScanRequest: scanRequest}
		assertState(t, 3, influxClientMock.Receive())
		shutdownChan <- struct{}{}
	})

	t.Run("should also reflect errors in influx", func(t *testing.T) {
		scanRequest := agent.NewScanRequest("", 123, agent.PurposeEntry, *scanner.NewToken("token", "scanner 1"))
		scanRequest.Fail(errors.New("sample error"))
		influxClientMock := InfluxClientMock{make(chan string, 4)}
		shutdownChan := make(chan struct{})
		metricsChannel := make(chan interface{})

		go Listen(&influxClientMock, metricsChannel, shutdownChan)
		metricsChannel <- agent.FsmScanRequest{State: agent.StateError, ScanRequest: scanRequest}
		assertStateError(t, 4, influxClientMock.Receive(), "sample error")

		shutdownChan <- struct{}{}
	})

	t.Run("should terminate when shutdown is triggered", func(t *testing.T) {
		var a struct{}
		shutdownChannel := make(chan struct{})
		metricsChannel := make(chan interface{})

		influxClientMock := InfluxClientMock{make(chan string, 4)}
		go func() {
			shutdownChannel <- a
		}()
		Listen(&influxClientMock, metricsChannel, shutdownChannel)

		assert.Equal(t, true, true)
	})
}
