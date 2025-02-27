package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type ConsumerHandler interface {
	Process(ctx context.Context, message *sarama.ConsumerMessage) error
}

type consumer struct {
	consumerGroup sarama.ConsumerGroup
}

func NewConsumer(addrs []string, groupId string) (*consumer, error) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest

	consumerGroup, err := sarama.NewConsumerGroup(addrs, groupId, saramaConfig)
	if err != nil {
		return nil, err
	}
	return &consumer{consumerGroup}, nil
}

type consumerGroupHandler struct {
	handler ConsumerHandler
}

func (c *consumer) Consume(ctx context.Context, topics []string, handler ConsumerHandler) error {
	defer c.consumerGroup.Close()
	consumerGroupHandler := &consumerGroupHandler{handler}
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill)
	defer cancel()
	var (
		errConsume error
		wg         sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := c.consumerGroup.Consume(ctx, topics, consumerGroupHandler); err != nil {
				if errors.Is(err, sarama.ErrClosedConsumerGroup) {
					errConsume = nil
					return
				}
				errConsume = err
				return
			}
			if ctx.Err() != nil {
				errConsume = nil
				return
			}
		}
	}()
	wg.Wait()

	if errConsume != nil {
		return errConsume
	}
	return nil
}

func (c *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	assigneds := []any{}
	for topic, partitions := range session.Claims() {
		assigneds = append(assigneds, slog.String(topic, fmt.Sprint(partitions)))
	}
	slog.Info("Assigning topics",
		slog.Group("assigned_topics", assigneds...),
	)
	return nil
}

func (c *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	slog.Info("Cleaning up consumer")
	return nil
}

func (c *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	ctx := session.Context()
	for message := range claim.Messages() {
		c.ProcessMessage(ctx, message)
		session.MarkMessage(message, "")
	}
	return nil
}

func (c *consumerGroupHandler) ProcessMessage(ctx context.Context, message *sarama.ConsumerMessage) error {
	ctx, span := startConsumerSpan(ctx, message)
	defer span.End()
	defer func() {
		if r := recover(); r != nil {
			span.RecordError(fmt.Errorf("panic: %v", r))
			span.SetStatus(codes.Error, "panic")
		}
	}()
	err := c.handler.Process(ctx, message)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	span.SetStatus(codes.Ok, "")
	return nil
}

func startConsumerSpan(ctx context.Context, message *sarama.ConsumerMessage) (context.Context, trace.Span) {
	carrier := propagation.MapCarrier{}
	for _, header := range message.Headers {
		carrier[string(header.Key)] = string(header.Value)
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)

	tracer := otel.Tracer(tracerName)
	return tracer.Start(ctx, consumeMessageSpanName,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("kafka.topic", message.Topic),
			attribute.Int("kafka.partition", int(message.Partition)),
			attribute.Int64("kafka.offset", message.Offset),
		),
	)
}
