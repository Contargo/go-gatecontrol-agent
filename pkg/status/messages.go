package status

import (
	"contargo.net/gatecontrol/gatecontrol-agent/pkg/buildinfo"
	"fmt"
)

type Application struct {
	Name      string `json:"name"`
	Instance  int64  `json:"instance"`
	CommitSha string `json:"commitSha"`
}

type Terminal struct {
	Location     string `json:"locationCode"`
	Loadingplace int64  `json:"loadingPlaceId"`
}

type EntityStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type GateStatus EntityStatus
type ScannerStatus EntityStatus

type Status struct {
	Hostname    string          `json:"hostname"`
	Application Application     `json:"application"`
	Terminal    Terminal        `json:"terminal"`
	Gates       []GateStatus    `json:"gates"`
	Scanners    []ScannerStatus `json:"scanners"`
}

func (s *Status) String() string {
	return fmt.Sprintf("%s:%d, %s (%d), Gates: %v, Scanners: %v, version: %s",
		s.Application.Name,
		s.Application.Instance,
		s.Terminal.Location,
		s.Terminal.Loadingplace,
		s.Gates,
		s.Scanners,
		buildinfo.GitSHA)
}
