package logger

import (
	"context"

	"github.com/rs/zerolog"
)

type ctxKey struct{}

// WithContext returns ctx carrying l. The HTTP middleware does this once per
// request so every downstream log line shares the request fields.
func WithContext(ctx context.Context, l zerolog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// Ctx returns the logger stored in ctx, or the process-wide logger when the
// context carries none. A nil context is tolerated (falls back to the global
// logger) so logging never panics.
func Ctx(ctx context.Context) *zerolog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(zerolog.Logger); ok {
			return &l
		}
	}
	return &base
}

// Err starts an error-level event with err AND its stack attached, in the order
// zerolog requires (Stack must be called before Err, an easy trap).
//
// Use it once, where you decide what the error means:
//	logger.Err(ctx, err).Str("dependency", "mysql").Msg("health check failed")

func Err(ctx context.Context, err error) *zerolog.Event {
	return Ctx(ctx).Error().Stack().Err(err)
}
