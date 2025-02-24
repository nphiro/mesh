package kafka

import (
	"context"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func startConsumerSpan(ctx context.Context, message *sarama.ConsumerMessage) (context.Context, trace.Span) {
	carrier := propagation.MapCarrier{}
	for _, header := range message.Headers {
		carrier[string(header.Key)] = string(header.Value)
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, consumeMessageSpanName, trace.WithSpanKind(trace.SpanKindConsumer))
	span.SetAttributes(
		attribute.String("kafka.topic", message.Topic),
		attribute.Int("kafka.partition", int(message.Partition)),
		attribute.Int64("kafka.offset", message.Offset),
	)
	return ctx, span
}
