package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	zmq "github.com/pebbe/zmq4"
)

type JSONZMQAdapter struct {
	chain    string
	endpoint string
	topic    string
}

func NewJSONZMQAdapter(chain, endpoint, topic string) *JSONZMQAdapter {
	normalizedTopic := strings.TrimSpace(topic)
	if normalizedTopic == "" {
		normalizedTopic = "rawtx"
	}
	return &JSONZMQAdapter{
		chain:    strings.ToLower(strings.TrimSpace(chain)),
		endpoint: strings.TrimSpace(endpoint),
		topic:    normalizedTopic,
	}
}

func (a *JSONZMQAdapter) Name() string {
	return a.chain
}

func (a *JSONZMQAdapter) Run(ctx context.Context, out chan<- MempoolMessage) error {
	if strings.TrimSpace(a.endpoint) == "" {
		return fmt.Errorf("chain=%s endpoint is empty", a.chain)
	}
	if out == nil {
		return errors.New("output channel is nil")
	}

	sub, err := zmq.NewSocket(zmq.SUB)
	if err != nil {
		return fmt.Errorf("chain=%s create zmq socket failed: %w", a.chain, err)
	}
	defer sub.Close()

	if err := sub.Connect(a.endpoint); err != nil {
		return fmt.Errorf("chain=%s connect zmq endpoint failed: %w", a.chain, err)
	}
	if err := sub.SetSubscribe(a.topic); err != nil {
		return fmt.Errorf("chain=%s subscribe topic failed: %w", a.chain, err)
	}
	if err := sub.SetRcvtimeo(1 * time.Second); err != nil {
		return fmt.Errorf("chain=%s set rcv timeout failed: %w", a.chain, err)
	}

	log.Printf("[ZMQ] adapter started: chain=%s endpoint=%s topic=%s", a.chain, a.endpoint, a.topic)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		parts, err := sub.RecvMessageBytes(0)
		if err != nil {
			// Timeout is expected to allow context cancellation checks.
			continue
		}
		if len(parts) == 0 {
			continue
		}

		payload := parts[len(parts)-1]
		msg, ok := a.decodeMessage(payload)
		if !ok {
			continue
		}

		select {
		case <-ctx.Done():
			return nil
		case out <- msg:
		}
	}
}

func (a *JSONZMQAdapter) decodeMessage(payload []byte) (MempoolMessage, bool) {
	var msg MempoolMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		log.Printf("[ZMQ] chain=%s skipped non-json payload: %v", a.chain, err)
		return MempoolMessage{}, false
	}
	if msg.Chain == "" {
		msg.Chain = a.chain
	}
	if msg.PinList == nil {
		msg.PinList = make([]*PinRecord, 0)
	}
	return msg, true
}
