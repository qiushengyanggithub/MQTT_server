// SPDX-License-Identifier: MIT

package api

import (
	"encoding/base64"
	"time"
	"unicode/utf8"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// Message is a published MQTT message forwarded to the web dashboard.
type Message struct {
	At            int64  `json:"at"`
	Topic         string `json:"topic"`
	ClientID      string `json:"client_id"`
	Payload       string `json:"payload,omitempty"`
	PayloadBase64 string `json:"payload_base64,omitempty"`
	Qos           byte   `json:"qos"`
	Retain        bool   `json:"retain"`
}

// BrokerHook bridges broker publish events into the web API without
// blocking the MQTT packet processing path.
type BrokerHook struct {
	mqtt.HookBase
	messages chan Message
}

// NewBrokerHook creates a hook with a buffered event channel.
func NewBrokerHook(buffer int) *BrokerHook {
	if buffer <= 0 {
		buffer = 256
	}
	return &BrokerHook{messages: make(chan Message, buffer)}
}

func (h *BrokerHook) ID() string {
	return "web-api"
}

func (h *BrokerHook) Provides(b byte) bool {
	return b == mqtt.OnPublished
}

// Messages exposes a read-only stream of published messages.
func (h *BrokerHook) Messages() <-chan Message {
	return h.messages
}

func (h *BrokerHook) OnPublished(cl *mqtt.Client, pk packets.Packet) {
	msg := Message{
		At:     time.Now().UnixMilli(),
		Topic:  pk.TopicName,
		Qos:    pk.FixedHeader.Qos,
		Retain: pk.FixedHeader.Retain,
	}
	if cl != nil {
		msg.ClientID = cl.ID
	}
	if utf8.Valid(pk.Payload) {
		msg.Payload = string(pk.Payload)
	} else {
		msg.PayloadBase64 = base64.StdEncoding.EncodeToString(pk.Payload)
	}

	select {
	case h.messages <- msg:
	default:
	}
}
