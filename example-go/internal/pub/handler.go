package pub

import (
	"net/http"

	platformhttp "rabbitmq-example/internal/platform/http"
	"rabbitmq-example/internal/platform/logger"
	internalrmq "rabbitmq-example/internal/rabbitmq"

	"github.com/labstack/echo/v5"
	rmq "github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

type Handler struct {
	publisher *rmq.Publisher
}

func NewHandler(publisher *rmq.Publisher) *Handler {
	return &Handler{publisher: publisher}
}

// Send publishes the msg query parameter to the queue.
func (h *Handler) Send(c *echo.Context) error {
	message := c.QueryParam("msg")
	if message == "" {
		return platformhttp.Error(c, http.StatusBadRequest, "msg is required")
	}

	if err := internalrmq.Publish(c.Request().Context(), h.publisher, message); err != nil {
		logger.L().Error().Stack().Err(err).Msg("Failed to publish message")
		return platformhttp.Error(c, http.StatusInternalServerError, "Failed to publish message")
	}

	return platformhttp.OK(c, "message sent", map[string]string{"message": message})
}

// Page renders a small form for sending a message.
func (h *Handler) Page(c *echo.Context) error {
	return c.HTML(http.StatusOK, `<!doctype html>
<html>
<head><meta charset="utf-8"><title>Publisher</title></head>
<body>
<h1>Publisher</h1>
<form id="form">
  <input id="msg" name="msg" placeholder="your message" required>
  <button type="submit">Send</button>
</form>
<pre id="result"></pre>
<script>
document.getElementById("form").addEventListener("submit", async function (event) {
  event.preventDefault();
  const message = document.getElementById("msg").value;
  const response = await fetch("/send-msg?msg=" + encodeURIComponent(message));
  document.getElementById("result").textContent = await response.text();
});
</script>
</body>
</html>`)
}
