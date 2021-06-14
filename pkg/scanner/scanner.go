package scanner

import (
	"bytes"
	"io"
	"log"
	"strings"
)

// A Scanner is a generic device that provides string values from optical,
// machine-readable representations of data.
type Scanner struct {
	name   string
	prefix string
	rc     io.ReadCloser
}

// Name returns the configured name of the scanner.
func (s *Scanner) Name() string {
	return s.name
}

// Prefix returns the configured prefix of the scanner.
func (s *Scanner) Prefix() string {
	return s.prefix
}

// Accept waits for and returns the next value from the scanner.
func (s *Scanner) Accept() (string, error) {
	buf := make([]byte, 64)

	n, err := s.rc.Read(buf)
	if err != nil {
		defer s.Close()
		return "", err
	}
	input := string(bytes.TrimSpace(buf[:n]))
	input = strings.TrimPrefix(input, s.prefix)
	input = strings.TrimPrefix(input, "QR")
	log.Println("Received token", input, "on scanner", s.name)
	return input, nil
}

// Close closes the scanner.
// Any blocked Accept operations will be unblocked and return errors.
func (s *Scanner) Close() error {
	return s.rc.Close()
}

// ReadAndStartListen creates a scanner that reads from rc.
func Listen(name, prefix string, rc io.ReadCloser) (*Scanner, error) {
	scanner := &Scanner{
		name:   name,
		prefix: prefix,
		rc:     rc,
	}
	return scanner, nil
}
