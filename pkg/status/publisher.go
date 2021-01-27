package status

import (
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/buildinfo"
	"encoding/json"
	"log"

	"github.com/streadway/amqp"
)

type Channel interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

type Publisher struct {
	name         string
	instance     int64
	location     string
	loadingPlace int64

	ch Channel

	gateStatus    map[string]string
	scannerStatus map[string]string
}

func NewPublisher(name string, instance int64, location string, loadingPlace int64, ch Channel) *Publisher {
	return &Publisher{
		name:          name,
		instance:      instance,
		location:      location,
		loadingPlace:  loadingPlace,
		ch:            ch,
		gateStatus:    make(map[string]string),
		scannerStatus: make(map[string]string),
	}
}

func (p *Publisher) UpdateGate(name, status string) {
	p.gateStatus[name] = status
}

func (p *Publisher) UpdateScanner(name, status string) {
	p.scannerStatus[name] = status
}

func (p *Publisher) Publish() error {
	status := p.status()

	log.Printf("Publishing status update: %s", status.String())

	payload, err := json.Marshal(status)
	if err != nil {
		return err
	}

	err = p.ch.Publish(
		"gatecontrol.event",
		"gatecontrol.agent.status",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
		},
	)

	return err
}

func (p *Publisher) status() Status {
	gates := []GateStatus{}
	scanners := []ScannerStatus{}

	for name, status := range p.gateStatus {
		gates = append(gates, GateStatus{name, status})
	}

	for name, status := range p.scannerStatus {
		scanners = append(scanners, ScannerStatus{name, status})
	}

	return Status{
		Application: Application{p.name, p.instance, buildinfo.GitSHA},
		Terminal:    Terminal{p.location, p.loadingPlace},
		Gates:       gates,
		Scanners:    scanners,
	}
}
