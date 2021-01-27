package metrics

import (
	worker "contargo.net/gatecontrol/gatecontrol-agent/pkg/agent"
	"fmt"
	"log"
	"os"
	"time"
)

var stateToNumerical = map[string]int{
	worker.StateIdle:       0,
	worker.StateValidating: 1,
	worker.StatePrinting:   2,
	worker.StateGating:     3,
	worker.StateError:      4,
}

func convertToLineProtocol(stateName, usedScanner string, err error) string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("go-gateagent host=\"%s\",scanner=\"%s\",state=%d,error=\"%s\" %d\n", hostname, usedScanner, stateToNumerical[stateName], getErrorFromScan(err), time.Now().UnixNano())
}

func Listen(influxClient InfluxClient, metricsChannel chan interface{}, shutdownChannel chan struct{}) {
	for {
		select {
		case fsmData := <-metricsChannel:
			fsmDataCasted := fsmData.(worker.FsmScanRequest)
			log.Println(convertToLineProtocol(fsmDataCasted.State, fsmDataCasted.ScanRequest.ScannerName(), fsmDataCasted.ScanRequest.Error()))
			err := influxClient.Write(convertToLineProtocol(fsmDataCasted.State, fsmDataCasted.ScanRequest.ScannerName(), fsmDataCasted.ScanRequest.Error()))
			if err != nil {
				log.Println("got err response", err)
			}
			break
		case <-shutdownChannel:
			close(metricsChannel)
			return
		}
	}
}

func getErrorFromScan(err error) string {
	if err == nil {
		return "no error"
	}
	return err.Error()
}
