package metrics_amqp

import (
	worker "contargo.net/gatecontrol/gatecontrol-agent/pkg/agent"
	"encoding/json"
	"github.com/Contargo/chamqp"
	"github.com/streadway/amqp"
	"log"
)

type Client struct {
	channel         *chamqp.Channel
	metricsChannel  chan interface{}
	shutdownChannel chan struct{}
	locode          string
	role            string
}

type FSMMessage struct {
	State  string
	Locode string
	Role   string
	Error  string
}

func NewMetricsPublisher(channel *chamqp.Channel, locode string, role string, metricsChannel chan interface{}, shutdownChannel chan struct{}) *Client {
	return &Client{
		channel,
		metricsChannel,
		shutdownChannel,
		locode,
		role,
	}
}

func getErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (m *Client) Listen() {
	errChan := make(chan error)
	m.channel.ExchangeDeclare("gateagent", "topic", false, false, false, false, nil, errChan)
	for {
		select {
		case fsmData := <-m.metricsChannel:
			fsmDataCasted := fsmData.(worker.FsmScanRequest)

			fsmMessage := &FSMMessage{
				fsmDataCasted.State,
				m.locode,
				m.role,
				getErrorString(fsmDataCasted.ScanRequest.Error()),
			}
			payload, err := json.Marshal(fsmMessage)
			if err != nil {
				log.Println("Cannot marshal msg", err)
				continue
			}
			_ = m.channel.Publish(
				"gateagent",
				"fsm.status",
				false, false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        payload,
				},
			)
			break
		case <-m.shutdownChannel:
			break
		}
	}
}
