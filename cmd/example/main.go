package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nphiro/mesh/pkg/cmlt"
	"github.com/nphiro/mesh/pkg/env"
	"github.com/nphiro/mesh/pkg/xerrors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var cfg struct {
	SentryDsn string `env:"SENTRY_DSN" envDefault:""`
}

func init() {
	env.Parse(&cfg)

	cmlt.Setup("example application", &cmlt.Config{
		SentryDsn: cfg.SentryDsn,
	})
}

func main() {
	defer cmlt.Flush()

	tracer := otel.Tracer("main")
	ctx, span := tracer.Start(context.Background(), "main_test")
	defer span.End()

	_, span2 := tracer.Start(ctx, "sub_test")
	time.Sleep(200 * time.Millisecond)
	span2.End()
	time.Sleep(600 * time.Millisecond)

	span.AddEvent("Starting application", trace.WithAttributes(attribute.String("mode", os.Getenv("ENV"))))
	err := test()
	span.SetStatus(codes.Error, "boom")
	slog.ErrorContext(ctx, "Starting application", slog.String("env", os.Getenv("ENV")), slog.Group("app", slog.Any("error", err), slog.String("message", "test error"), slog.Int("code", 500)))
}

func test() error {
	return xerrors.Wrap(fmt.Errorf("test error"))
}
