package sub

import (
	"encoding/json"
	"fmt"
	"net/http"

	platformhttp "rabbitmq-example/internal/platform/http"

	"github.com/labstack/echo/v5"
)

type Handler struct {
	hub *hub
}

func NewHandler() *Handler {
	return &Handler{hub: newHub()}
}

// Broadcast sends a message to every connected SSE client.
func (h *Handler) Broadcast(message string) {
	h.hub.broadcast(message)
}

// Stream keeps the HTTP connection open and sends every message as an SSE event.
func (h *Handler) Stream(c *echo.Context) error {
	flusher, ok := c.Response().(http.Flusher)
	if !ok {
		return platformhttp.Error(c, http.StatusInternalServerError, "Streaming is not supported")
	}

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	client := make(chan string, 16)
	history := h.hub.join(client)
	defer h.hub.leave(client)

	for _, message := range history {
		if err := writeEvent(w, message); err != nil {
			return nil
		}
	}
	flusher.Flush()

	for {
		select {
		case <-c.Request().Context().Done():
			return nil
		case message := <-client:
			if err := writeEvent(w, message); err != nil {
				return nil
			}
			flusher.Flush()
		}
	}
}

// Page renders a small page that joins the SSE stream.
func (h *Handler) Page(c *echo.Context) error {
	return c.HTML(http.StatusOK, `<!doctype html>
<html>
<head><meta charset="utf-8"><title>Subscriber</title></head>
<body>
<h1>Subscriber</h1>
<p id="status">connecting...</p>
<ol id="messages"></ol>
<script>
const MAX_MESSAGES = `+fmt.Sprint(historySize)+`;
const status = document.getElementById("status");
const messages = document.getElementById("messages");
const source = new EventSource("/events");

source.onopen = function () {
  status.textContent = "joined";
};

source.onerror = function () {
  status.textContent = "disconnected, reconnecting...";
};

source.onmessage = function (event) {
  const data = JSON.parse(event.data);
  const item = document.createElement("li");
  item.textContent = data.message;
  messages.appendChild(item);

  while (messages.children.length > MAX_MESSAGES) {
    messages.removeChild(messages.firstElementChild);
  }
};
</script>
</body>
</html>`)
}

func writeEvent(w http.ResponseWriter, message string) error {
	payload, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "data: %s\n\n", payload)
	return err
}
