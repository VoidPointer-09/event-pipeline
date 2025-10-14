package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IBM/sarama"
)

// Config holds runtime configuration for Kafka.
type Config struct {
	Brokers     []string
	Topic       string
	GroupID     string
	TLS         bool
}

func NewSaramaConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V3_7_0_0 // Kafka 3.7 (KRaft)
	cfg.Producer.Return.Successes = true
	cfg.Producer.Idempotent = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Net.MaxOpenRequests = 1
	cfg.Consumer.Return.Errors = true
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	cfg.Metadata.Retry.Max = 5
	cfg.Metadata.Retry.Backoff = 2 * time.Second
	return cfg
}

func MustEnv(name string, def string) string {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	return v
}

func LoadConfigFromEnv() Config {
	brokers := MustEnv("KAFKA_BROKERS", "kafka:9092")
	return Config{
		Brokers: splitAndTrim(brokers),
		Topic:   MustEnv("KAFKA_TOPIC", "events"),
		GroupID: MustEnv("KAFKA_GROUP_ID", "event-consumers"),
		TLS:     MustEnv("KAFKA_TLS", "false") == "true",
	}
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			res = append(res, p)
		}
	}
	return res
}

// NewProducer creates a Sarama SyncProducer.
func NewProducer(cfg Config) (sarama.SyncProducer, error) {
	scfg := NewSaramaConfig()
	if cfg.TLS {
		scfg.Net.TLS.Enable = true
		scfg.Net.TLS.Config = &tls.Config{InsecureSkipVerify: true}
	}
	return sarama.NewSyncProducer(cfg.Brokers, scfg)
}

// NewConsumerGroup creates a Sarama ConsumerGroup.
func NewConsumerGroup(cfg Config) (sarama.ConsumerGroup, error) {
	scfg := NewSaramaConfig()
	if cfg.TLS {
		scfg.Net.TLS.Enable = true
		scfg.Net.TLS.Config = &tls.Config{InsecureSkipVerify: true}
	}
	return sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, scfg)
}

// Publish sends a message with key and value bytes.
func Publish(ctx context.Context, p sarama.SyncProducer, topic, key string, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
	_, _, err := p.SendMessage(msg)
	return err
}

// ConsumerHandler routes messages to a callback.
type ConsumerHandler struct {
	Handle func(ctx context.Context, m *sarama.ConsumerMessage) error
}

func (h *ConsumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *ConsumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *ConsumerHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := h.Handle(sess.Context(), msg); err != nil {
			fmt.Printf("consumer handler error: %v\n", err)
		} else {
			sess.MarkMessage(msg, "")
		}
	}
	return nil
}
