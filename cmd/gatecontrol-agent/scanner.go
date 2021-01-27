package main

import (
	"os"

	"contargo.net/gatecontrol/gatecontrol-agent/pkg/scanner"
	"github.com/jacobsa/go-serial/serial"
)

func fileScannerOpener(name, prefix, path string) scanner.Opener {
	return scanner.Opener(func() (*scanner.Scanner, error) {
		f, err := os.OpenFile(path, os.O_RDWR, os.ModeNamedPipe)
		if err != nil {
			return nil, err
		}
		return scanner.Listen(name, prefix, f)
	})
}

func usbComScannerOpener(name, prefix, device string, baud uint) scanner.Opener {
	return scanner.Opener(func() (*scanner.Scanner, error) {
		port, err := serial.Open(serial.OpenOptions{
			PortName:        device,
			BaudRate:        baud,
			DataBits:        8,
			StopBits:        1,
			MinimumReadSize: 4,
		})
		if err != nil {
			return nil, err
		}

		return scanner.Listen(name, prefix, port)
	})
}
