package service

import (
	"sync"
)

type Notification struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt string         `json:"created_at"`
}

// NotificationBus is an in-memory pub/sub broker for real-time SSE delivery.
// One global instance is shared across handlers. Each user can have multiple
// concurrent SSE connections (e.g. multiple browser tabs).
type NotificationBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan Notification
}

var GlobalBus = &NotificationBus{
	subscribers: make(map[string][]chan Notification),
}

// Subscribe registers a new channel for the given userID.
// The returned cancel function must be deferred by the caller.
func (b *NotificationBus) Subscribe(userID string) (chan Notification, func()) {
	ch := make(chan Notification, 32)
	b.mu.Lock()
	b.subscribers[userID] = append(b.subscribers[userID], ch)
	b.mu.Unlock()
	return ch, func() { b.unsubscribe(userID, ch) }
}

// Publish delivers a notification to all active connections for the user.
// Non-blocking: slow consumers are skipped (channel buffer absorbs bursts).
func (b *NotificationBus) Publish(userID string, n Notification) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers[userID] {
		select {
		case ch <- n:
		default:
		}
	}
}

func (b *NotificationBus) unsubscribe(userID string, ch chan Notification) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[userID]
	for i, c := range subs {
		if c == ch {
			b.subscribers[userID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}
