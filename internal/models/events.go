package models

import (
	"encoding/json"
	"time"
)

// Envelope wraps every event with a type and metadata.
// EventID is used for correlation across logs.
// OccurredAt represents the event time.
type Envelope struct {
	EventID    string          `json:"eventId"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurredAt"`
	Key        string          `json:"key"` // userId/orderId/sku used for Kafka keying
	Payload    json.RawMessage `json:"payload"`
}

type UserCreated struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

type OrderPlaced struct {
	OrderID string  `json:"orderId"`
	UserID  string  `json:"userId"`
	Amount  float64 `json:"amount"`
}

type PaymentSettled struct {
	PaymentID string  `json:"paymentId"`
	OrderID   string  `json:"orderId"`
	Status    string  `json:"status"` // e.g. settled, failed
	Amount    float64 `json:"amount"`
}

type InventoryAdjusted struct {
	SKU     string `json:"sku"`
	Delta   int    `json:"delta"`
	Reason  string `json:"reason"`
}

const (
	TypeUserCreated      = "UserCreated"
	TypeOrderPlaced      = "OrderPlaced"
	TypePaymentSettled   = "PaymentSettled"
	TypeInventoryAdjusted = "InventoryAdjusted"
)
