package server

import "sync"

// Hub manages SSE subscribers for one watched folder.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[chan []byte]struct{})}
}

func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 16)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	_, exists := h.subscribers[ch]
	delete(h.subscribers, ch)
	h.mu.Unlock()
	if exists {
		close(ch)
	}
}

func (h *Hub) Broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		cp := make([]byte, len(data))
		copy(cp, data)
		select {
		case ch <- cp:
		default:
		}
	}
}
