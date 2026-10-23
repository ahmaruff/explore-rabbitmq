package sub

import "sync"

// historySize is how many recent messages are replayed to a new client.
const historySize = 50

// hub keeps the channels of connected SSE clients and a short history.
type hub struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
	history []string
}

func newHub() *hub {
	return &hub{
		clients: make(map[chan string]struct{}),
	}
}

func (h *hub) broadcast(message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.history = append(h.history, message)
	if len(h.history) > historySize {
		h.history = h.history[len(h.history)-historySize:]
	}

	for client := range h.clients {
		select {
		case client <- message:
		default:
			// The client is not reading fast enough, drop this message for it.
		}
	}
}

func (h *hub) join(client chan string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = struct{}{}

	history := make([]string, len(h.history))
	copy(history, h.history)

	return history
}

func (h *hub) leave(client chan string) {
	h.mu.Lock()
	delete(h.clients, client)
	h.mu.Unlock()
}
