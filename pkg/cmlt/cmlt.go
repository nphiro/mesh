package cmlt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	sentryotel "github.com/getsentry/sentry-go/otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	appTracer     = otel.Tracer("app")
	shutdownFuncs []shutdownFunc
)

type shutdownFunc struct {
	name  string
	apply func(context.Context) error
}

func addShutdownFunc(name string, f func(context.Context) error) {
	shutdownFuncs = append(shutdownFuncs, shutdownFunc{name, f})
}

type Config struct {
	SentryDsn string

	Release string
}

func defaultConfig() *Config {
	return &Config{}
}

func Setup(serviceName string, config *Config) {
	if config == nil {
		config = defaultConfig()
	}

	resources := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(config.Release),
	)

	tracerProviderOptions := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resources),
	}

	textMapPropagators := []propagation.TextMapPropagator{
		propagation.TraceContext{},
		propagation.Baggage{},
	}

	if config.SentryDsn != "" {
		initSentry(config.SentryDsn, serviceName, config.Release)
		tracerProviderOptions = append(tracerProviderOptions, sdktrace.WithSpanProcessor(sentryotel.NewSentrySpanProcessor()))
		textMapPropagators = append(textMapPropagators, sentryotel.NewSentryPropagator())
	}

	tracerProvider := sdktrace.NewTracerProvider(tracerProviderOptions...)
	addShutdownFunc("tracer provider", tracerProvider.Shutdown)

	appTracer = tracerProvider.Tracer(serviceName)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(textMapPropagators...))
}

func Flush() error {
	var errs []error
	for _, shutdownFunc := range shutdownFuncs {
		slog.Info(fmt.Sprintf("Flushing buffered %s data", shutdownFunc.name))
		if err := shutdownFunc.apply(context.Background()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func AppTracer() trace.Tracer {
	return appTracer
}
