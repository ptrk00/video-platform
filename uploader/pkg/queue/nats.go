package queue

import (
	"context"
	"encoding/json"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
	"log"
	"time"
)

func PublishMessage(ctx context.Context, bucketname, filename string) {
	tracer := otel.Tracer("uploader")
	ctx, span := tracer.Start(ctx, "publishMessage")
	defer span.End()

	// Connect to a NATS server
	nc, err := nats.Connect("nats://admin:admin@nats:4222")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to connect to NATS")
		zap.Error(err)
		return
	}
	defer nc.Close()

	// Get JetStream context
	js, err := nc.JetStream()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get JetStream context")
		zap.Error(err)
		return
	}

	// Create the message
	message := Message{
		Bucket:   bucketname,
		Filename: filename,
	}
	data, err := json.Marshal(message)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to marshal message")
		zap.Error(err)
		return
	}

	// Publish the message
	msg := nats.NewMsg("videos.uploaded")
	msg.Data = data
	msg.Header.Add("time", time.Now().String())

	// Send the message
	ack, err := js.PublishMsg(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to publish message")
		zap.Error(err)
		return
	}

	span.SetAttributes(attribute.Int64("nats.sequence", int64(ack.Sequence)))
	log.Printf("Published message on subject %s with sequence %d\n", msg.Subject, ack.Sequence)
}
