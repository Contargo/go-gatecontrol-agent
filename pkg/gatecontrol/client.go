package gatecontrol

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Contargo/chamqp"
	"log"
	"time"

	"github.com/streadway/amqp"
)

const (
	timeout = 5 * time.Second
)

var (
	errTimedOut = errors.New("timed out waiting for reply")
)

// A PermissionValidator validates a token.
type PermissionValidator interface {
	ValidateEntry(location string, loadingplace int64, token string) (bool, error)
	ValidateExit(location string, loadingplace int64, token string) (bool, error)
}

// A ProcessNotifier notifies about the process of a vehicle for token.
type ProcessNotifier interface {
	GatedIn(location string, loadingplace int64, token string) error
	GatedOut(location string, loadingplace int64, token string) error
}

// A Client acts as a command sender and receiver for Gate-Control.
type Client struct {
	ch         *chamqp.Channel
	replyQueue <-chan amqp.Delivery
}

// NewClient returns a new Gate-Control client.
func NewClient(conn *chamqp.Connection) *Client {
	ch := conn.Channel()
	replyQueue := make(chan amqp.Delivery)
	ch.Consume("amq.rabbitmq.reply-to", "", true, false, false, false, nil, replyQueue, nil)

	return &Client{ch, replyQueue}
}

// ValidateEntry implements the PermissionValidator interface. It sends a
// validate entry permission command to Gate-Control and returns the result.
func (c *Client) ValidateEntry(location string, loadingplace int64, token string) (bool, error) {
	return c.validate(validateEntry, permissionRequest{location, loadingplace, token})
}

// ValidateExit implements the PermissionValidator interface. It sends a
// validate exit permission command to Gate-Control and returns the result.
func (c *Client) ValidateExit(location string, loadingplace int64, token string) (bool, error) {
	return c.validate(validateExit, permissionRequest{location, loadingplace, token})
}

// GatedIn implements the ProcessNotifier interface. It sends an use entry
// permission command to Gate-Control that actually triggers a Gate In.
func (c *Client) GatedIn(location string, loadingplace int64, token string) error {
	_, err := c.use(useEntry, permissionRequest{location, loadingplace, token})
	return err
}

// GatedOut implements the ProcessNotifier interface. It sends an use exit
// permission command to Gate-Control that actually triggers a Gate Out.
func (c *Client) GatedOut(location string, loadingplace int64, token string) error {
	_, err := c.use(useExit, permissionRequest{location, loadingplace, token})
	return err
}

func (c *Client) validate(purpose validatePurpose, req permissionRequest) (bool, error) {
	log.Printf("gatecontrol: Send validate permission command for token %s (%s)",
		req.Token, purpose.Rk())

	payload, err := json.Marshal(req)
	if err != nil {
		return false, fmt.Errorf("failed to marshal json: %v", err)
	}

	msg := amqp.Publishing{
		Headers: amqp.Table{
			"type":    purpose.Type(),
			"version": purpose.Version(),
		},
		ContentType:   "application/json",
		ReplyTo:       "amq.rabbitmq.reply-to",
		CorrelationId: req.Token,
		Body:          payload,
	}

	err = c.ch.Publish("gatecontrol.terminalpermission.command", purpose.Rk(), false, false, msg)
	if err != nil {
		return false, err
	}

	timer := time.NewTimer(timeout)
	for {
		select {
		case reply := <-c.replyQueue:
			var response permissionResponse
			err := json.Unmarshal(reply.Body, &response)
			if err != nil {
				log.Printf("failed to unmarshal json: %v", err)
				continue
			}
			if reply.CorrelationId != req.Token {
				log.Printf("ignoring reply for different token: %s", reply.CorrelationId)
				continue
			}
			if response.Message != nil {
				err = errors.New(response.Message.MessageCode)
			}
			return response.Permitted, err
		case <-timer.C:
			return false, errTimedOut
		}
	}
}

func (c *Client) use(purpose usePurpose, req permissionRequest) (bool, error) {
	log.Printf("gatecontrol: Send use permission command for token %s (%s)",
		req.Token, purpose.Rk())

	payload, err := json.Marshal(req)
	if err != nil {
		return false, fmt.Errorf("failed to marshal json: %v", err)
	}

	msg := amqp.Publishing{
		Headers: amqp.Table{
			"type":    purpose.Type(),
			"version": purpose.Version(),
		},
		ContentType:   "application/json",
		ReplyTo:       "amq.rabbitmq.reply-to",
		CorrelationId: req.Token,
		Body:          payload,
	}

	err = c.ch.Publish("gatecontrol.terminalpermission.command", purpose.Rk(), false, false, msg)
	if err != nil {
		return false, err
	}

	timer := time.NewTimer(timeout)
	for {
		select {
		case reply := <-c.replyQueue:
			var response permissionResponse
			err := json.Unmarshal(reply.Body, &response)
			if err != nil {
				log.Printf("failed to unmarshal json: %v", err)
				continue
			}
			if reply.CorrelationId != req.Token {
				log.Printf("ignoring reply for different token: %s", reply.CorrelationId)
				continue
			}
			if response.Message != nil {
				err = errors.New(response.Message.MessageCode)
			}
			return response.Permitted, err
		case <-timer.C:
			return false, errTimedOut
		}
	}
}
