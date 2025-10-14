package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog/log"

	ik "github.com/example/event-pipeline/internal/kafka"
	im "github.com/example/event-pipeline/internal/models"
	idlq "github.com/example/event-pipeline/internal/dlq"
	istore "github.com/example/event-pipeline/internal/storage"
	imetrics "github.com/example/event-pipeline/internal/metrics"
)

func main() {
	imetrics.Serve()
	cfg := ik.LoadConfigFromEnv()
	group, err := ik.NewConsumerGroup(cfg)
	if err != nil { panic(err) }
	defer group.Close()
	db, err := istore.Connect()
	if err != nil { panic(err) }
	ctx := context.Background()
	if err := db.Init(ctx); err != nil { panic(err) }
	dlq := idlq.New()

	h := &ik.ConsumerHandler{Handle: func(ctx context.Context, m *sarama.ConsumerMessage) error {
		start := time.Now()
		defer func(){ imetrics.DBLatency.Observe(time.Since(start).Seconds()) }()
		var env im.Envelope
		if err := json.Unmarshal(m.Value, &env); err != nil {
			dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err)
			// Commit offset by returning nil; we've handled to DLQ
			return nil
		}
		log := log.With().Str("eventId", env.EventID).Logger()
		switch env.Type {
		case im.TypeUserCreated:
			var e im.UserCreated; if err := json.Unmarshal(env.Payload, &e); err != nil { dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err); return nil }
			if err := db.UpsertUser(ctx, e.UserID, e.Name, e.Email); err != nil { dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err); return nil }
		case im.TypeOrderPlaced:
			var e im.OrderPlaced; if err := json.Unmarshal(env.Payload, &e); err != nil { dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err); return nil }
			if err := db.UpsertOrder(ctx, e.OrderID, e.UserID, e.Amount); err != nil { dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err); return nil }
			// Automatically create a pending payment for this order
			if err := db.UpsertPayment(ctx, e.OrderID, e.OrderID, "pending", e.Amount); err != nil { dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err); return nil }
		case im.TypePaymentSettled:
			var e im.PaymentSettled; if err := json.Unmarshal(env.Payload, &e); err != nil { dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err); return nil }
			if err := db.UpsertPayment(ctx, e.PaymentID, e.OrderID, e.Status, e.Amount); err != nil { dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err); return nil }
		case im.TypeInventoryAdjusted:
			var e im.InventoryAdjusted; if err := json.Unmarshal(env.Payload, &e); err != nil { dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err); return nil }
			if err := db.UpsertInventory(ctx, e.SKU, e.Delta); err != nil { dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err); return nil }
		default:
			log.Warn().Str("type", env.Type).Msg("unknown event type")
		}
		imetrics.Processed.Inc()
		return nil
	}}
	
	topics := []string{cfg.Topic}
	for {
		if err := group.Consume(ctx, topics, h); err != nil {
			log.Error().Err(err).Msg("consume error")
			time.Sleep(time.Second)
		}
	}
}
