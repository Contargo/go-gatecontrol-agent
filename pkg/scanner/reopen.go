package scanner

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// StateUnknown represents the unknown state
	StateUnknown string = "UNKNOWN"
	// StateUp represents the up state, aka the scanner is healthy.
	StateUp string = "UP"
	// StateDown represents the down state, aka the scanner is unhealthy.
	StateDown string = "DOWN"
)

// A Opener can be used to (re-)open an instance of a scanner.
type Opener func() (*Scanner, error)

// A Status represents the current state of an instance of a scanner.
type Status struct {
	Name  string
	State string
	Error error
}

type ScanSource string

const (
	Invalid       ScanSource = "INVALID"
	Paper         ScanSource = "PAPER"
	TruckersTrust ScanSource = "TRUCKERS_TRUST"
)

type Token struct {
	Content    string
	Scanner    string
	ScanSource ScanSource
}

func getScanSource(rawContent string) ScanSource {
	if strings.HasPrefix(rawContent, "tt-") {
		return TruckersTrust
	}
	return Paper
}

func NewToken(rawContent, scanner string) *Token {
	content := strings.Replace(rawContent, "tt-", "", 1)
	return &Token{
		content,
		scanner,
		getScanSource(rawContent),
	}
}

func (t *Token) IsValidUUID() bool {
	if strings.HasPrefix(t.Content, "tt-") {
		replacedToken := strings.Replace(t.Content, "tt-", "", 1)
		_, err := uuid.Parse(replacedToken)
		return err == nil
	}
	_, err := uuid.Parse(t.Content)
	return err == nil
}

// A ReopeningScanner is a Scanner that tries to reopen whenever it gets closed.
type ReopeningScanner struct {
	name                   string
	opener                 Opener
	scanner                *Scanner
	tokenChans             []chan Token
	statusChans            []chan Status
	shutdownChan, doneChan chan struct{}
	mu                     sync.Mutex
}

// NewReopeningScanner creates a new reopening scanner.
func NewReopeningScanner(name string, opener Opener) *ReopeningScanner {
	return &ReopeningScanner{
		name:         name,
		opener:       opener,
		shutdownChan: make(chan struct{}),
		doneChan:     make(chan struct{}),
	}
}

// ReadAndStartListen listens on the configured scanner and notifies subscribers about
// tokens and status updates.
func (s *ReopeningScanner) Listen() {
	defer close(s.doneChan)

	status := StateUnknown

	updateStatus := func(err error) {
		// Going from DOWN to UP
		if status != StateUp && err == nil {
			status = StateUp
			s.dispatchStatus(Status{s.name, status, nil})
			return
		}

		// Going from UP to DOWN
		if status != StateDown && err != nil {
			status = StateDown
			s.dispatchStatus(Status{s.name, status, err})
			return
		}
	}

	for {
		select {
		case <-s.shutdownChan:
			return
		default:
		}

		if status == StateDown {
			select {
			case <-time.After(time.Second):
			case <-s.shutdownChan:
				return
			}
		}

		scanner, err := s.opener()
		updateStatus(err)
		if err != nil {
			s.scanner = nil
			continue
		} else {
			s.scanner = scanner
		}

		for status == StateUp {
			token, err := s.scanner.Accept()
			updateStatus(err)
			if err != nil {
				s.scanner = nil
				continue
			} else {
				if token != "" {
					s.dispatchToken(token)
				}
			}
		}
	}
}

// Shutdown gracefully shuts down the reopening scanner. Shutdown works by
// closing the scanner device and wait for the listening routine to stop. If the
// provided context expires before the shutdown is complete, Shutdown returns
// the context's error, otherwise nil.
func (s *ReopeningScanner) Shutdown(ctx context.Context) error {
	close(s.shutdownChan)

	if s.scanner != nil {
		s.scanner.Close()
	}

	select {
	case <-s.doneChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Name returns the name of the scanner.
func (s *ReopeningScanner) Name() string {
	return s.name
}

// NotifyTokens adds `ch` to the list of subscribers for tokens.
func (s *ReopeningScanner) NotifyTokens(ch chan Token) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokenChans = append(s.tokenChans, ch)
}

// NotifyStatus adds `ch` to the list of subscribers for status updates.
func (s *ReopeningScanner) NotifyStatus(ch chan Status) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.statusChans = append(s.statusChans, ch)
}

func (s *ReopeningScanner) dispatchToken(token string) {
	tokenContainer := NewToken(token, s.scanner.name)
	for _, ch := range s.tokenChans {
		select {
		case ch <- *tokenContainer:
		default:
		}
	}
}

func (s *ReopeningScanner) dispatchStatus(status Status) {
	for _, ch := range s.statusChans {
		select {
		case ch <- status:
		default:
		}
	}
}
