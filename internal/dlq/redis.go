package dlq

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"

	imetrics "github.com/example/event-pipeline/internal/metrics"
)

type DLQMessage struct {
	At       time.Time       `json:"at"`
	Error    string          `json:"error"`
	Payload  json.RawMessage `json:"payload"`
}

type Client struct {
	cli *redis.Client
}

func New() *Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "redis:6379"
	}
	cli := redis.NewClient(&redis.Options{Addr: addr})
	return &Client{cli: cli}
}

func (c *Client) Push(ctx context.Context, key string, payload []byte, err error) {
	msg := DLQMessage{At: time.Now(), Error: err.Error(), Payload: json.RawMessage(payload)}
	b, _ := json.Marshal(msg)
	if key == "" { key = "dlq" }
	if _, rerr := c.cli.LPush(ctx, key, b).Result(); rerr != nil {
		log.Ctx(ctx).Error().Err(rerr).Msg("redis DLQ push failed")
		return
	}
	imetrics.DLQCount.Inc()
}
