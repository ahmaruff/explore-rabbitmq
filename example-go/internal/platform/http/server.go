package http

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"rabbitmq-example/internal/platform/logger"
)

type Server struct {
	echo *echo.Echo
	addr string
}

func (s *Server) Echo() *echo.Echo {
	return s.echo
}

func (s *Server) Address() string {
	return s.addr
}

// Static mounts a plain net/http handler under pattern and strips prefix before
// calling it. Feature packages stay router agnostic; only this package needs to
// know which router the app uses.
func (s *Server) Static(pattern string, prefix string, handler http.Handler) {
	wrapped := echo.WrapHandler(http.StripPrefix(prefix, handler))

	s.echo.GET(pattern, wrapped)
	s.echo.HEAD(pattern, wrapped)
}

func (s *Server) Start(ctx context.Context) error {
	sc := echo.StartConfig{
		Address:         s.addr,
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: 10 * time.Second,
		BeforeServeFunc: func(server *http.Server) error {
			server.ReadHeaderTimeout = 5 * time.Second
			server.ReadTimeout = 60 * time.Second
			server.WriteTimeout = 5 * time.Minute
			server.IdleTimeout = 60 * time.Second

			return nil
		},
	}

	err := sc.Start(ctx, s.echo)

	// sc.Start returns http.ErrServerClosed on successful graceful exits,
	// which shouldn't be bubbled up as an actual application crash error.
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func NewServer(port string) *Server {
	customSlog := logger.Slog()

	e := echo.NewWithConfig(echo.Config{
		Logger: customSlog,
	})

	e.Validator = NewRequestValidator()
	e.HTTPErrorHandler = errorHandler

	e.Use(
		middleware.RequestID(),
		withRequestLogger,
		middleware.Recover(),
		middleware.BodyLimit(2_097_152),
		middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			LogLatency:  true,
			LogProtocol: true,
			LogRemoteIP: true,
			LogMethod:   true,
			LogURI:      true,
			LogStatus:   true,
			HandleError: true,

			LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
				lvl := slog.LevelInfo

				if v.Status >= 500 {
					lvl = slog.LevelError
				} else if v.Status >= 400 {
					lvl = slog.LevelWarn
				}

				customSlog.LogAttrs(c.Request().Context(), lvl, "HTTP Request",
					slog.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
					slog.String("ip", v.RemoteIP),
					slog.String("method", v.Method),
					slog.String("uri", redactURI(v.URI)),
					slog.Int("status", v.Status),
					slog.Duration("latency", v.Latency),
					slog.String("protocol", v.Protocol),
				)

				return nil
			},
		}),
	)

	return &Server{
		echo: e,
		addr: net.JoinHostPort("0.0.0.0", port),
	}
}

// redactURI keeps request logs useful without storing bearer-like query
// parameters such as file signatures or access tokens.
func redactURI(raw string) string {
	parsed, err := url.ParseRequestURI(raw)

	if err != nil {
		return raw
	}

	query := parsed.Query()

	for _, key := range []string{"signature", "token", "access_token", "api_key"} {
		if query.Has(key) {
			query.Set(key, "REDACTED")
		}
	}

	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func withRequestLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		requestID := c.Response().Header().Get(echo.HeaderXRequestID)

		log := logger.L().With().
			Str("request_id", requestID).
			Logger()

		ctx := logger.WithContext(c.Request().Context(), log)
		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
	}
}

func errorHandler(c *echo.Context, err error) {
	if res, _ := echo.UnwrapResponse(c.Response()); res != nil && res.Committed {
		return
	}

	status := echo.StatusCode(err)

	if status == 0 {
		status = http.StatusInternalServerError
	}

	message := http.StatusText(status)

	var httpErr *echo.HTTPError

	if errors.As(err, &httpErr) && httpErr.Message != "" {
		message = httpErr.Message
	}

	if message == "" {
		message = "Something went wrong"
	}

	if status >= http.StatusInternalServerError {
		logger.Err(c.Request().Context(), err).Msg("Unhandled request error")
	}

	if writeErr := Error(c, status, message); writeErr != nil {
		logger.Err(c.Request().Context(), writeErr).Msg("Failed to write error response")
	}
}
