package metrics

import (
	"errors"
	"log"
	"os"
	"syscall"
)

func InitFifo(pipeFile string) (*os.File, error) {
	if _, err := os.Stat(pipeFile); err == nil {
		log.Println("fifo already exists")
	} else if os.IsNotExist(err) {
		err = syscall.Mkfifo(pipeFile, 0775)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("Schroedinger file case, file exists and do not exist at the same time")
	}

	f, err := os.OpenFile(pipeFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		return nil, err
	}
	return f, nil
}
