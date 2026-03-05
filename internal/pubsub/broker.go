package pubsub

import (
	"sync"
)

// subscriber represents one connected client subscribed to one or more channels.
// Each message delivered to it is sent via its ch channel.
// bufferSize = 64: if a slow client can't keep up, we drop rather than block the publisher.
const bufferSize = 64

type subscriber struct {
	ch chan Message
}

// Message is what gets delivered to a subscriber.
type Message struct {
	Channel string
	Payload string
}

// Broker manages all channel subscriptions.
// Designed for high publish throughput — RWMutex lets multiple goroutines
// read the subscriber list concurrently; only subscribe/unsubscribe need a write lock.
type Broker struct {
	mu   sync.RWMutex
	subs map[string]map[*subscriber]struct{} // channel → set of subscribers
}

func NewBroker() *Broker {
	return &Broker{
		subs: make(map[string]map[*subscriber]struct{}),
	}
}

// Subscribe registers a new subscriber on the given channels.
// Returns the subscriber (caller reads from sub.ch) and a cleanup function.
func (b *Broker) Subscribe(channels ...string) (*subscriber, func()) {
	sub := &subscriber{
		ch: make(chan Message, bufferSize),
	}

	b.mu.Lock()
	for _, ch := range channels {
		if b.subs[ch] == nil {
			b.subs[ch] = make(map[*subscriber]struct{})
		}
		b.subs[ch][sub] = struct{}{}
	}
	b.mu.Unlock()

	cleanup := func() {
		b.mu.Lock()
		for _, ch := range channels {
			delete(b.subs[ch], sub)
			if len(b.subs[ch]) == 0 {
				delete(b.subs, ch)
			}
		}
		b.mu.Unlock()
		close(sub.ch)
	}

	return sub, cleanup
}

// Publish sends a message to all subscribers on a channel.
// Returns the number of clients that received the message.
// Non-blocking: slow clients get their message dropped (buffered channel).
func (b *Broker) Publish(channel, payload string) int {
	b.mu.RLock()
	targets := b.subs[channel]
	// snapshot the set under read lock — then release immediately
	// so we don't hold the lock while sending
	snapshot := make([]*subscriber, 0, len(targets))
	for sub := range targets {
		snapshot = append(snapshot, sub)
	}
	b.mu.RUnlock()

	msg := Message{Channel: channel, Payload: payload}
	var delivered int
	for _, sub := range snapshot {
		select {
		case sub.ch <- msg:
			delivered++
		default:
			// subscriber is too slow — drop the message, never block
		}
	}
	return delivered
}

// NumSubscribers returns total active subscriber count across all channels.
func (b *Broker) NumSubscribers() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	total := 0
	for _, subs := range b.subs {
		total += len(subs)
	}
	return total
}

// Ch exposes the subscriber's receive channel.
func (s *subscriber) Ch() <-chan Message {
	return s.ch
}