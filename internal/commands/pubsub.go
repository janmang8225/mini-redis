package commands

import (
	"fmt"

	"github.com/janmang8225/mini-redis/internal/pubsub"
	"github.com/janmang8225/mini-redis/internal/resp"
)

// handleSubscribe puts the connection into subscribe mode.
// It BLOCKS until the client disconnects or unsubscribes from all channels.
// This is intentional — a subscribed Redis client does nothing but receive messages.
func handleSubscribe(broker *pubsub.Broker, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) < 2 {
		_ = w.WriteError("wrong number of arguments for 'SUBSCRIBE'")
		return
	}

	channels := cmd.Args[1:]
	sub, cleanup := broker.Subscribe(channels...)
	defer cleanup()

	// confirm subscription for each channel — Redis protocol requires this
	for i, ch := range channels {
		_ = writeSubscribeConfirm(w, "subscribe", ch, i+1)
	}

	// block and forward messages until the channel is closed (client disconnects)
	for msg := range sub.Ch() {
		_ = writeMessage(w, msg.Channel, msg.Payload)
	}
}

// handlePublish sends a message to a channel.
// Returns the number of subscribers that received it.
func handlePublish(broker *pubsub.Broker, cmd *resp.Command, w *resp.Writer) {
	if len(cmd.Args) != 3 {
		_ = w.WriteError("wrong number of arguments for 'PUBLISH'")
		return
	}
	n := broker.Publish(cmd.Args[1], cmd.Args[2])
	_ = w.WriteInteger(int64(n))
}

// writeSubscribeConfirm writes the 3-element array Redis sends on subscribe:
// *3\r\n$9\r\nsubscribe\r\n$<len>\r\n<channel>\r\n:<count>\r\n
func writeSubscribeConfirm(w *resp.Writer, kind, channel string, count int) error {
	_ = w.WriteArrayHeader(3)
	_ = w.WriteBulkString(kind)
	_ = w.WriteBulkString(channel)
	return w.WriteInteger(int64(count))
}

// writeMessage writes the 3-element array Redis sends for each pub/sub message:
// *3\r\n$7\r\nmessage\r\n$<len>\r\n<channel>\r\n$<len>\r\n<payload>\r\n
func writeMessage(w *resp.Writer, channel, payload string) error {
	_ = w.WriteArrayHeader(3)
	_ = w.WriteBulkString("message")
	_ = w.WriteBulkString(channel)
	return w.WriteBulkString(fmt.Sprintf("%s", payload))
}