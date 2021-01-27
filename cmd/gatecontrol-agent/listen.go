package main

import (
	"encoding/json"
	"fmt"
	"github.com/Contargo/chamqp"
	"log"
	"os"
	"sync"

	"contargo.net/gatecontrol/gatecontrol-agent/pkg/agent"
	"github.com/streadway/amqp"
)

// Terminal references an operational terminal by location and loading place.
type Terminal struct {
	Location     string `json:"locationCode"`
	LoadingPlace int64  `json:"loadingPlaceId"`
}

// NamedGate references a gate by name
type NamedGate struct {
	Name string `json:"name"`
}

// OpenGate describes a open gate request for gates on a terminal.
type OpenGate struct {
	Terminal Terminal    `json:"terminal"`
	Gates    []NamedGate `json:"gates"`
}

func openGateRequestListener(wg *sync.WaitGroup, ch *chamqp.Channel, gate *agent.Gate, manualDataChan chan bool, shutdownChan chan struct{}) {
	wg.Add(1)
	defer wg.Done()

	queue := fmt.Sprintf("%s.%s.%s.%d", config.Application.Name, config.Terminal.Location, config.Gate.Name, os.Getpid())

	gateOpenChan := make(chan amqp.Delivery)
	errChan := make(chan error)

	ch.ExchangeDeclare("gatecontrol.event", "topic", true, false, false, false, nil, errChan)
	ch.QueueDeclare(queue, false, true, false, false, nil, nil, nil)
	ch.QueueBind(queue, "gates.open", "gatecontrol.event", false, nil, nil)
	ch.Consume(queue, "", false, false, false, false, nil, gateOpenChan, nil)

	for {
		select {
		case msg := <-gateOpenChan:
			var req OpenGate
			err := json.Unmarshal(msg.Body, &req)
			if err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				msg.Nack(false, false)
			}
			if req.Terminal.Location != config.Terminal.Location ||
				req.Terminal.LoadingPlace != config.Terminal.LoadingPlace {
				log.Printf("Skipping request for different terminal: %v", req.Terminal)
				msg.Ack(false)

			} else {
				for _, v := range req.Gates {
					if config.Gate.Name == v.Name {
						log.Printf("Open gate manually.")
						gate.Open()
						manualDataChan <- true
					}
				}
				msg.Ack(false)
			}
		case err := <-errChan:
			log.Printf("Failed to listen for gate open requests: %v", err)
			return
		case <-shutdownChan:
			return
		}
	}
}
