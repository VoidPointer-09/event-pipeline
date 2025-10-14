package main

import (
	"context"
	"encoding/json"
	"strconv"
	"math/rand"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	k "github.com/example/event-pipeline/internal/kafka"
	m "github.com/example/event-pipeline/internal/models"
	imetrics "github.com/example/event-pipeline/internal/metrics"
)

func main() {
	imetrics.Serve()
	cfg := k.LoadConfigFromEnv()
	producer, err := k.NewProducer(cfg)
	if err != nil { panic(err) }
	defer producer.Close()

	rate := 5
	if v := os.Getenv("EVENT_RATE"); v != "" { if n, err := strconv.Atoi(v); err==nil { rate = n } }
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()
	ctx := context.Background()
	for range ticker.C {
		env, key := randomEvent()
		b, _ := json.Marshal(env)
		if err := k.Publish(ctx, producer, cfg.Topic, key, b); err != nil {
			log.Error().Err(err).Str("eventId", env.EventID).Msg("publish failed")
			continue
		}
		imetrics.Processed.Inc()
	}
}

func randomEvent() (m.Envelope, string) {
	now := time.Now().UTC()
	eID := uuid.New().String()
	switch rand.Intn(4) {
	case 0:
		userID := uuid.New().String()
		payload, _ := json.Marshal(m.UserCreated{UserID: userID, Name: "Alice", Email: "alice@example.com"})
		return m.Envelope{EventID: eID, Type: m.TypeUserCreated, OccurredAt: now, Key: userID, Payload: payload}, userID
	case 1:
		orderID := uuid.New().String(); userID := uuid.New().String()
		payload, _ := json.Marshal(m.OrderPlaced{OrderID: orderID, UserID: userID, Amount: 42.5})
		return m.Envelope{EventID: eID, Type: m.TypeOrderPlaced, OccurredAt: now, Key: orderID, Payload: payload}, orderID
	case 2:
		orderID := uuid.New().String(); paymentID := uuid.New().String()
		payload, _ := json.Marshal(m.PaymentSettled{PaymentID: paymentID, OrderID: orderID, Status: "settled", Amount: 42.5})
		return m.Envelope{EventID: eID, Type: m.TypePaymentSettled, OccurredAt: now, Key: orderID, Payload: payload}, orderID
	default:
		sku := "SKU-" + uuid.New().String()[:8]
		payload, _ := json.Marshal(m.InventoryAdjusted{SKU: sku, Delta: 1, Reason: "replenish"})
		return m.Envelope{EventID: eID, Type: m.TypeInventoryAdjusted, OccurredAt: now, Key: sku, Payload: payload}, sku
	}
}
