package agent

import (
	"fmt"
	"log"
	"time"

	"contargo.net/gatecontrol/gatecontrol-agent/pkg/gatecontrol"
)

// A Callback responds to an scan request.
type Callback interface {
	Call(ScanRequest) error
}

// The CallbackFunc type is an adapter to allow the use of ordinary functions
// as worker callbacks. If f is a function with the appropriate signature,
// CallbackFunc(f) is a Callback that calls f.
type CallbackFunc func(ScanRequest) error

// Call calls f(r).
func (f CallbackFunc) Call(r ScanRequest) error {
	return f(r)
}

// NopCallback is a Callback doing nothing.
var NopCallback = CallbackFunc(func(ScanRequest) error { return nil })

// ValidateHandler is a callback that validates the token from the scan request.
func ValidateHandler(validator gatecontrol.PermissionValidator) Callback {
	return CallbackFunc(func(r ScanRequest) error {
		var (
			permitted bool
			err       error
		)

		log.Printf("[%s] Validating permission for %s.", r.Token(), r.Purpose())

		switch r.Purpose() {
		case PurposeEntry:
			permitted, err = validator.ValidateEntry(r.Location(), r.LoadingPlace(), r.Token())
		case PurposeExit:
			permitted, err = validator.ValidateExit(r.Location(), r.LoadingPlace(), r.Token())
		default:
			return fmt.Errorf("unknown purpose: %s", r.Purpose())
		}

		if err != nil {
			return err
		}

		if !permitted {
			return fmt.Errorf("not permitted")
		}

		return nil
	})
}

// PrintHandler is a callback that does the printing for the request.
func PrintHandler(waitTime time.Duration) Callback {
	return CallbackFunc(func(r ScanRequest) error {
		log.Printf("[%s] Waiting %v for print job to be done.", r.Token(), waitTime)
		time.Sleep(waitTime)
		return nil
	})
}

// GateHandler is a callback that tries to perform the actual gate process.
//
// The process consists of the following steps:
// * Check if the request is valid
// * Open gate
// * Check the induction loop
// * Close gate
//
// Dependent of the state of the induction loop, Gate terminates successfully
// or with a timeout because the actual "gate" couldn't be detected.
func GateHandler(notifier gatecontrol.ProcessNotifier, gate *Gate) Callback {
	return CallbackFunc(func(r ScanRequest) error {
		var err error

		log.Printf("[%s] Notifying consumers about the %s.", r.Token(), r.Purpose())

		switch r.Purpose() {
		case PurposeEntry:
			err = notifier.GatedIn(r.Location(), r.LoadingPlace(), r.Token())
		case PurposeExit:
			err = notifier.GatedOut(r.Location(), r.LoadingPlace(), r.Token())
		default:
			return fmt.Errorf("unknown purpose: %s", r.Purpose())
		}

		if err != nil {
			return err
		}

		log.Printf("[%s] Open gate on Scan.", r.Token())
		return gate.Open()
	})
}

// ErrorHandler is a callback that handles errors during the process.
func ErrorHandler() Callback {
	return CallbackFunc(func(r ScanRequest) error {
		log.Printf("[%s] Error: %s", r.Token(), r.Error())
		return nil
	})
}
