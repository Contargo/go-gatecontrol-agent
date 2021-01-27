package main

import (
	"log"
	"strconv"
	"strings"

	"contargo.net/gatecontrol/gatecontrol-agent/pkg/agent"
	"github.com/vaughan0/go-ini"
)

type Config struct {
	Application ApplicationConfig
	Terminal    TerminalConfig
	Gate        GateConfig
	RabbitMQ    RabbitMQConfig
	Scanners    []ScannerConfig
}

type ApplicationConfig struct {
	Name            string
	Instance        int64
	ShutdownTimeout int64
	PrintTimeout    int64
	InfluxUrl       string
}

type TerminalConfig struct {
	Location     string
	LoadingPlace int64
}

type GateConfig struct {
	Name           string
	Purpose        agent.GatePurpose
	Cmd            string
	ReEntryTimeOut int
}

type RabbitMQConfig struct {
	URL string
}

type ScannerConfig struct {
	Name   string
	Driver string
	Path   string
	Prefix string
}

func ReadConfig(path string) Config {
	inifile, err := ini.LoadFile(path)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	return Config{
		Application: readApplicationConfig(inifile),
		Terminal:    readTerminalConfig(inifile),
		Gate:        readGateConfig(inifile),
		RabbitMQ:    readRabbitMQConfig(inifile),
		Scanners:    readScannerConfig(inifile),
	}
}

func readApplicationConfig(config ini.File) ApplicationConfig {
	name := conf(config, "application", "name")
	instance, err := strconv.ParseInt(conf(config, "application", "instance"), 10, 32)
	if err != nil {
		log.Fatalf("[application]instance is not an integer!")
	}
	shutdownTimeout, err := strconv.ParseInt(conf(config, "application", "shutdownTimeout"), 10, 32)
	if err != nil {
		log.Fatalf("[application]shutdownTimeout is not an integer!")
	}
	printTimeout, err := strconv.ParseInt(conf(config, "application", "printTimeout"), 10, 32)
	if err != nil {
		log.Fatalf("[application]printTimeout is not an integer!")
	}
	influxUrl := confOptional(config, "application", "influxURL")
	if influxUrl == nil {
		log.Fatalf("No influxURL provided")
	}

	return ApplicationConfig{name, instance, shutdownTimeout, printTimeout, *influxUrl}
}

func readTerminalConfig(config ini.File) TerminalConfig {
	location := conf(config, "terminal", "location")
	loadingplace, err := strconv.ParseInt(conf(config, "terminal", "loadingplace"), 10, 64)
	if err != nil {
		log.Fatalf("[terminal]loadingplace is not an integer!")
	}
	return TerminalConfig{location, loadingplace}
}

func readGateConfig(config ini.File) GateConfig {
	purpose, err := agent.NewGatePurpose(conf(config, "gate", "purpose"))
	if err != nil {
		log.Fatalf("[gate]purpose is not valid! %v", err)
	}
	reentryTimeoutStr := confOptional(config, "gate", "reEntryTimeout")
	reEntryTimeout := 5
	if reentryTimeoutStr != nil {
		log.Println("Not assuming default", reentryTimeoutStr)
		reEntryTimeout, err = strconv.Atoi(*reentryTimeoutStr)
		if err != nil {
			log.Fatalln("Cannot parse", reentryTimeoutStr)
		}
	}
	log.Println("ReEntry timeout is", reEntryTimeout, "min")
	gate := GateConfig{
		Name:           conf(config, "gate", "name"),
		Purpose:        purpose,
		Cmd:            conf(config, "gate", "command"),
		ReEntryTimeOut: reEntryTimeout,
	}
	return gate
}

func readRabbitMQConfig(config ini.File) RabbitMQConfig {
	rabbitMQ := RabbitMQConfig{
		URL: conf(config, "rabbitmq", "url"),
	}
	return rabbitMQ
}

func readScannerConfig(config ini.File) []ScannerConfig {
	var scanners []ScannerConfig

	for section := range config {
		if strings.HasPrefix(section, "scanner ") {
			name := strings.TrimPrefix(section, "scanner ")
			prefix := conf(config, section, "prefix")
			driver := conf(config, section, "driver")
			path := conf(config, section, "path")
			scanners = append(scanners, ScannerConfig{name, driver, path, prefix})
		}
	}

	return scanners
}

func confOptional(config ini.File, section, key string) *string {
	value, ok := config.Get(section, key)
	if !ok {
		return nil
	}
	return &value
}

func conf(config ini.File, section string, key string) string {
	value, ok := config.Get(section, key)
	if !ok {
		log.Fatalf("Expected config option [%s]%s", section, key)
	}
	return value
}

func (c *Config) ScannerNames() []string {
	var names []string
	for _, s := range c.Scanners {
		names = append(names, s.Name)
	}
	return names
}
