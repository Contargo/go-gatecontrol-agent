package agent

import (
	"fmt"
	"log"
	"os/exec"
)

// A GatePurpose represents the purpose of a gate.
type GatePurpose int

const (
	// PurposeEntry represents an entry gate
	PurposeEntry GatePurpose = iota
	// PurposeExit represents an exit gate
	PurposeExit
)

var purposeName = map[GatePurpose]string{
	PurposeEntry: "entry",
	PurposeExit:  "exit",
}

func (p GatePurpose) String() string {
	return purposeName[p]
}

// NewGatePurpose returns the purpose identified by name.
func NewGatePurpose(name string) (GatePurpose, error) {
	for k, v := range purposeName {
		if name == v {
			return k, nil
		}
	}

	return PurposeEntry, fmt.Errorf("Undefined gate purpose: %s", name)
}

// A Gate represents a physical gate.
type Gate struct {
	Name    string
	Purpose GatePurpose
	Cmd     string
}

// Open opens the gate.
func (g *Gate) Open() error {
	log.Printf("Open gate: %v", g)
	return runCmd(g.Cmd)
}

func runCmd(args string) error {
	cmd := exec.Command("sh", "-c", args)
	return cmd.Run()
}
