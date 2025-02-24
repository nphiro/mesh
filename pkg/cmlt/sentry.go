package cmlt

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/nphiro/mesh/pkg/env"
	"go.opentelemetry.io/otel/trace"
)

func initSentry(dsn, service, release string) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: env.DeploymentEnv(),
		Release:     release,
		Tags: map[string]string{
			"service": service,
		},
		Debug:            true,
		EnableTracing:    true,
		TracesSampleRate: 1.0,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			for _, exception := range event.Exception {
				frames := exception.Stacktrace.Frames
				for i := range frames {
					if frames[i].Module == "log/slog" {
						exception.Stacktrace.Frames = frames[:i]
						break
					}
				}
			}
			if hint == nil {
				return event
			}
			spanContext := trace.SpanContextFromContext(hint.Context)
			if spanContext.IsValid() {
				event.Contexts["trace"] = sentry.Context{
					"trace_id": spanContext.TraceID().String(),
					"span_id":  spanContext.SpanID().String(),
				}
			}
			event.Fingerprint = []string{"{{ default }}", event.Message}
			return event
		},
	})
	if err != nil {
		slog.Error("Failed to init sentry")
		os.Exit(1)
	}

	addShutdownFunc("sentry", func(ctx context.Context) error {
		sentry.Flush(3 * time.Second)
		return nil
	})
}
