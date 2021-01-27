package main

import (
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/metrics_amqp"
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/rescan"
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/trafficlights"
	"context"
	"flag"
	"github.com/Contargo/chamqp"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"contargo.net/gatecontrol/gatecontrol-agent/pkg/agent"
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/buildinfo"
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/gatecontrol"
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/metrics"
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/scanner"
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/status"
	"github.com/google/uuid"
)

var (
	config        Config
	configPath    string
	logTimestamps bool
	wg            sync.WaitGroup
)

func main() {
	flag.StringVar(&configPath, "config", "./config.ini", "configure location of config file")
	flag.BoolVar(&logTimestamps, "timestamps", false, "prepend timestamps when logging")
	flag.Parse()

	if !logTimestamps {
		log.SetFlags(0)
	}

	config = ReadConfig(configPath)

	printTimeout := time.Duration(config.Application.PrintTimeout) * time.Second
	shutdownTimeout := time.Duration(config.Application.ShutdownTimeout) * time.Second

	log.Printf("Starting gatecontrol-agent %s",
		buildinfo.GitSHA)
	log.Printf("application : %s:%d",
		config.Application.Name,
		config.Application.Instance)
	log.Printf("              waiting %v to print", printTimeout)
	log.Printf("              waiting %v to shutdown", shutdownTimeout)
	log.Printf("terminal    : %s (%d)",
		config.Terminal.Location,
		config.Terminal.LoadingPlace)
	log.Printf("gate        : %s (%s)",
		config.Gate.Name,
		config.Gate.Purpose)
	log.Printf("scanner(s)  : %s", strings.Join(config.ScannerNames(), ", "))

	gate := &agent.Gate{
		Name:    config.Gate.Name,
		Purpose: config.Gate.Purpose,
		Cmd:     config.Gate.Cmd,
	}

	// Global shutdown channel to notify go routines to shutdown.
	shutdownChan := make(chan struct{})

	// Start amqp error logger.
	isOnlineChan := make(chan bool)
	amqpErrorChan := make(chan error)
	go errorLogger(&wg, "amqp", amqpErrorChan, isOnlineChan)

	// Start amqp connection manager.
	conn := chamqp.Dial(config.RabbitMQ.URL)
	conn.NotifyError(amqpErrorChan)

	// Start gate-control amqp client.
	manualDataChan := make(chan bool)
	gc := gatecontrol.NewClient(conn)
	go openGateRequestListener(&wg, conn.Channel(), gate, manualDataChan, shutdownChan)

	// Start gate-control agent.
	a := agent.Agent{
		ValidateHandler: agent.ValidateHandler(gc),
		PrintHandler:    agent.PrintHandler(printTimeout),
		GateHandler:     agent.GateHandler(gc, gate),
		ErrorHandler:    agent.ErrorHandler(),
	}
	metricsChannel := make(chan interface{})
	a.Subscribe(metricsChannel)

	influxClient := metrics.NewInfluxClient(config.Application.InfluxUrl)
	go metrics.Listen(influxClient, metricsChannel, shutdownChan)

	rescanChan := make(chan interface{})
	rescanHandler := rescan.NewRescanHandler(rescanChan, shutdownChan, manualDataChan, gate, config.Gate.ReEntryTimeOut)
	go rescanHandler.Listen()
	a.Subscribe(rescanChan)

	traffcLightsChan := make(chan interface{})

	trafficlightsWebserver := trafficlights.NewWebserver(traffcLightsChan, manualDataChan, isOnlineChan, shutdownChan)
	go func() {
		if err := trafficlightsWebserver.Start("localhost:8080"); err != nil {
			log.Println("Cannot start traffic lights", err)
		}
	}()
	go trafficlightsWebserver.Listen()
	a.Subscribe(traffcLightsChan)

	go a.Listen()

	metricsAmqpChannel := make(chan interface{})
	metricsClient := metrics_amqp.NewMetricsPublisher(conn.Channel(), config.Terminal.Location, config.Gate.Purpose.String(), metricsAmqpChannel, shutdownChan)
	go metricsClient.Listen()
	a.Subscribe(metricsAmqpChannel)

	// Start scanned token dispatcher.
	tokenChan := make(chan scanner.Token)
	go tokenDispatcher(&wg, &a, rescanHandler, tokenChan, shutdownChan)

	// Start status publisher.
	statusPublisher := status.NewPublisher(
		config.Application.Name,
		config.Application.Instance,
		config.Terminal.Location,
		config.Terminal.LoadingPlace,
		conn.Channel(),
	)
	statusPublisher.UpdateGate(config.Gate.Name, "UP")

	scannerStatusChan := make(chan scanner.Status, 5)

	go statusUpdater(&wg, statusPublisher, isOnlineChan, scannerStatusChan, shutdownChan)

	// Start reopening scanners.
	scanners := []*scanner.ReopeningScanner{}
	for _, config := range config.Scanners {
		var s *scanner.ReopeningScanner

		switch config.Driver {
		case "keyboard":
			builder := fileScannerOpener(config.Name, config.Prefix, config.Path)
			s = scanner.NewReopeningScanner(config.Name, builder)
		case "usbcom":
			builder := usbComScannerOpener(config.Name, config.Prefix, config.Path, 115200)
			s = scanner.NewReopeningScanner(config.Name, builder)
		}
		s.NotifyStatus(scannerStatusChan)
		s.NotifyTokens(tokenChan)
		scanners = append(scanners, s)
		go s.Listen()
	}

	log.Println("Ready.")

	waitForInterrupt()

	log.Printf("Shutting down... (will timeout in %v)", shutdownTimeout)

	close(shutdownChan)

	ctx := context.Background()
	ctxWithTimeout, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	exitCode := 0

	// Shutting down reopening scanners.
	for _, scanner := range scanners {
		if err := scanner.Shutdown(ctxWithTimeout); err != nil {
			log.Printf("Scanner %s failed to shutdown: %v", scanner.Name(), err)
			exitCode = 1
		}
	}

	// Shutting down gate-control agent.
	if err := a.Shutdown(ctxWithTimeout); err != nil {
		log.Printf("Agent failed to shutdown: %v", err)
		exitCode = 1
	}

	// Closing amqp connection manager.
	if err := conn.Close(); err != nil {
		log.Printf("Connection failed to close: %v", err)
		exitCode = 1
	}

	// Close open channels.
	close(amqpErrorChan)
	close(scannerStatusChan)
	close(tokenChan)

	// Make sure go-routines have stopped.
	wg.Wait()

	if exitCode == 0 {
		log.Println("Agent gracefully shutdown.")
	} else {
		log.Println("Agent shutdown with failures.")
	}

	os.Exit(exitCode)
}

// Periodically publishs the current state.
func statusUpdater(wg *sync.WaitGroup,
	publisher *status.Publisher,
	isOnlineChannel chan bool,
	scannerStatusChan chan scanner.Status,
	shutdownChan chan struct{}) {

	wg.Add(1)
	defer wg.Done()

	publish := func() {
		err := publisher.Publish()
		if err != nil {
			isOnlineChannel <- false
			log.Printf("Failed to publish status: %v", err)
		} else {
			isOnlineChannel <- true
		}
	}

	// Publish first status update after 5 seconds
	c1 := time.After(5 * time.Second)
	// Publish status update every 60 seconds
	c2 := time.Tick(60 * time.Second)

	for {
		select {
		case status := <-scannerStatusChan:
			// Publish status update on state change
			publisher.UpdateScanner(status.Name, status.State)
			publish()
		case <-c1:
			publish()
		case <-c2:
			publish()
		case <-shutdownChan:
			return
		}
	}
}

func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}
func tokenDispatcher(wg *sync.WaitGroup, a *agent.Agent, rescanHandler *rescan.RescanHandler, tokenChan chan scanner.Token, shutdownChan chan struct{}) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case token := <-tokenChan:
			if !IsValidUUID(token.Content) {
				log.Println("[", token, "]", "Ignoring scan request, seems to be a ghost scan")
				continue
			}
			if token.Content != "" {
				if !rescanHandler.HandleReentry(token.Content) {
					req := agent.NewScanRequest(config.Terminal.Location, config.Terminal.LoadingPlace, config.Gate.Purpose, token)
					a.HandleScanRequest(req)
				}
			}
		case <-shutdownChan:
			return
		}
	}
}

func errorLogger(wg *sync.WaitGroup, prefix string, errorChan <-chan error, isOnlineChan chan bool) {
	wg.Add(1)
	defer wg.Done()

	for err := range errorChan {
		log.Printf("%s: %v", prefix, err)
		isOnlineChan <- false
	}
}

func waitForInterrupt() {
	trap := make(chan os.Signal, 1)
	signal.Notify(trap, os.Interrupt)
	<-trap
}
