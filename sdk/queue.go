package sdk

import (
	"fmt"
	"strings"
)

type FailedMessage struct {
	Task    *NitricTask
	Message string
}

type SendBatchResponse struct {
	FailedMessages []*FailedMessage
}

// QueueService - The Nitric plugin interface for cloud native queue adapters
type QueueService interface {
	// Send - Send a single task to a queue
	Send(queue string, task NitricTask) error
	// SendBatch - sends multiple tasks to a queue
	SendBatch(queue string, tasks []NitricTask) (*SendBatchResponse, error)
	// Receive - Receives one or more tasks(s) off a queue
	Receive(options ReceiveOptions) ([]NitricTask, error)
	// Complete - Marks a received task as completed
	Complete(queue string, leaseId string) error
}

type ReceiveOptions struct {
	// Nitric name for the queue.
	//
	// The Nitric name will match the AWS SQS Queue name.
	//
	// queueName is a required field
	QueueName string `type:"string" required:"true"`

	// Max depth of queue messages to pop.
	//
	// If nil or 0, defaults to depth 1.
	Depth *uint32 `type:"int" required:"false"`
}

func (p *ReceiveOptions) Validate() error {
	// Validation
	var invalidParams []string
	if p.QueueName == "" {
		invalidParams = append(invalidParams, fmt.Errorf("queueName param must not be blank").Error())
	}
	if len(invalidParams) > 0 {
		return fmt.Errorf("invalid params: %s", strings.Join(invalidParams, "\n"))
	}

	// Defaults
	// Set depth to 1 by default
	if p.Depth == nil {
		p.Depth = new(uint32)
		*p.Depth = 1
	} else if *p.Depth < 1 {
		*p.Depth = uint32(1)
	}
	return nil
}

// UnimplementedQueuePlugin - A Default interface, that provide implementations of QueueService methods that
// Flag the method as unimplemented
type UnimplementedQueuePlugin struct {
	QueueService
}

// Ensure UnimplementedQueuePlugin conforms to QueueService interface
var _ QueueService = (*UnimplementedQueuePlugin)(nil)

// Push - Unimplemented Stub for the UnimplementedQueuePlugin
func (*UnimplementedQueuePlugin) Send(queue string, task NitricTask) error {
	return fmt.Errorf("UNIMPLEMENTED")
}

func (*UnimplementedQueuePlugin) SendBatch(queue string, tasks []NitricTask) (*SendBatchResponse, error) {
	return nil, fmt.Errorf("UNIMPLEMENTED")
}

func (*UnimplementedQueuePlugin) Receive(options ReceiveOptions) ([]NitricTask, error) {
	return nil, fmt.Errorf("UNIMPLEMENTED")
}

func (*UnimplementedQueuePlugin) Complete(queue string, leaseId string) error {
	return fmt.Errorf("UNIMPLEMENTED")
}
