package cmlt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/nphiro/mesh/pkg/env"
	"go.opentelemetry.io/otel/trace"
)

type slogHandler struct {
	handler slog.Handler
}

func init() {
	var (
		writer       io.Writer = os.Stdout
		minimumLevel           = slog.LevelInfo
	)
	if env.InLocalMachine() {
		writer = &prettierStdout{}
		minimumLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(
		&slogHandler{
			handler: slog.NewJSONHandler(writer, &slog.HandlerOptions{
				AddSource: false,
				Level:     minimumLevel,
			}),
		},
	))
}

func handleAttrs(attr slog.Attr) (any, error) {
	if attr.Value.Kind() == slog.KindGroup {
		groupMap := map[string]any{}
		var exception error
		for _, gattr := range attr.Value.Group() {
			val, err := handleAttrs(gattr)
			if err != nil {
				exception = err
			}
			groupMap[gattr.Key] = val
		}
		return groupMap, exception
	}

	val := attr.Value.Any()
	if err, ok := val.(error); ok {
		return err.Error(), err
	}
	return val, nil

}

func (h *slogHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.handler.Enabled(ctx, lvl)
}

func (h *slogHandler) Handle(ctx context.Context, rec slog.Record) error {
	if rec.Level >= slog.LevelWarn {
		if sentryClient := sentry.CurrentHub().Client(); sentryClient != nil {
			event := sentryClient.EventFromMessage(rec.Message, sentry.LevelError)
			if rec.Level == slog.LevelWarn {
				event.Level = sentry.LevelWarning
			}
			event.Timestamp = rec.Time
			event.Extra = map[string]any{}
			var exception error
			rec.Attrs(func(attr slog.Attr) bool {
				val, err := handleAttrs(attr)
				if err != nil {
					exception = err
				}
				event.Extra[attr.Key] = val
				return true
			})
			event.SetException(exception, 10)
			sentryClient.CaptureEvent(event, &sentry.EventHint{Context: ctx, OriginalException: exception}, nil)
		}
	}
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		rec.AddAttrs(slog.String("trace_id", spanContext.TraceID().String()))
		rec.AddAttrs(slog.String("span_id", spanContext.SpanID().String()))
	}
	return h.handler.Handle(ctx, rec)
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &slogHandler{
		handler: h.handler.WithAttrs(attrs),
	}
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	return &slogHandler{
		handler: h.handler.WithGroup(name),
	}
}

type prettierStdout struct{}

func (s *prettierStdout) Write(b []byte) (int, error) {
	var fields map[string]any
	_ = json.Unmarshal(b, &fields)

	t, _ := time.Parse(time.RFC3339Nano, fields["time"].(string))

	level := fields["level"].(string)
	switch level {
	case slog.LevelDebug.String():
		level = "\033[0;36m" + level + "\033[0m"
	case slog.LevelInfo.String():
		level = "\033[0;32m" + level + "\033[0m"
	case slog.LevelWarn.String():
		level = "\033[0;33m" + level + "\033[0m"
	case slog.LevelError.String():
		level = "\033[0;31m" + level + "\033[0m"
	default:
		level = "\033[0;37m" + level + "\033[0m"
	}

	b, _ = json.MarshalIndent(fields, "", "  ")
	fmt.Println(t.Format("[2006-01-02 15:04:05]"), level, fields["msg"], string(b))

	return len(b), nil
}
