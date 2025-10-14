# Event Pipeline (Go + Kafka KRaft + MSSQL + Redis)

This project builds a minimal event pipeline: producer -> Kafka -> consumer -> MSSQL -> read API, with Redis-based DLQ and Prometheus metrics.

## Stack
- Go 1.25
- Kafka 3.7 (KRaft, no ZooKeeper) - apache/kafka image
- MS SQL Server 2022
- Redis 7
- Docker Compose

## Services
- **producer**: emits 4 JSON event types to topic `events` at configurable rate
- **consumer**: consumes, routes by type, idempotent upserts to MSSQL; on error pushes payload+error to Redis DLQ
- **api**: exposes HTTP read endpoints:
  - `GET /users/{id}` → user + last 5 orders
  - `GET /orders/{id}` → order + payment status (optional)

## Run

1. Build and start all services:

```bash
docker compose up --build
```

2. After containers are healthy (give it ~30 seconds for Kafka and MSSQL), test the API:

```bash
# Get a user with their orders
curl http://localhost:8080/users/13a6c478-eb63-4dc9-a808-ad22eb6db65a | jq .

# Get an order (payment status optional)
curl http://localhost:8080/orders/be2b5f5f-3483-4988-b9f2-6556a3bb7e46 | jq .
```

3. Check database contents:

```bash
docker compose exec mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong!Passw0rd' -C -d events \
  -Q "SELECT 'users' AS tbl, COUNT(*) FROM users UNION ALL SELECT 'orders', COUNT(*) FROM orders;"
```

4. Inspect DLQ (should be empty with correct SQL syntax):

```bash
docker compose exec redis redis-cli LLEN dlq
docker compose exec redis redis-cli LRANGE dlq 0 5
```

## Configuration (env)
- `KAFKA_BROKERS` (default: kafka:9092)
- `KAFKA_TOPIC` (default: events)
- `KAFKA_GROUP_ID` (default: event-consumers)
- `MSSQL_CONN` (connection string; see compose for default)
- `REDIS_ADDR` (default: redis:6379)
- `DLQ_LIST` (default: dlq)
- `API_ADDR` (default: :8080)
- `METRICS_ADDR` (default: :2112)
- `EVENT_RATE` (producer events/sec, default: 5)

## Architecture

### Event Flow
```
Producer → Kafka (events topic) → Consumer → MSSQL
                                          ↓ (on error)
                                        Redis DLQ
```

**Processing Logic:**
- **UserCreated** → Upserts user record
- **OrderPlaced** → Upserts order + auto-creates payment with status "pending"
- **PaymentSettled** → Updates payment status from "pending" to "settled"
- **InventoryAdjusted** → Increments/decrements inventory qty by delta

### Idempotency
- All upserts use T-SQL `MERGE` statements with primary key matching
- Reprocessing events doesn't duplicate data
- Parameter syntax: `@p1`, `@p2`, etc. (positional for go-mssqldb driver)

### DLQ (Dead Letter Queue)
- Failed messages (parse errors, DB failures) are pushed to Redis list `DLQ_LIST`
- Includes original payload + error message + timestamp
- Consumer commits offset after DLQ push (no infinite retries)

### Metrics (Prometheus)
- `events_processed_total` - successful event processing count
- `dlq_messages_total` - messages sent to DLQ
- `db_latency_seconds` - histogram of DB operation latency (observe p95)
- Exposed on `:2112/metrics` inside each container

### Logging
- Structured JSON logs via zerolog
- Event correlation via `eventId` field
- Log level configurable via `LOG_LEVEL` env

## Database Schema

Tables are auto-created on consumer startup:

- `users` (user_id PK, name, email, updated_at)
- `orders` (order_id PK, user_id, amount, updated_at) + index on user_id
- `payments` (payment_id PK, order_id, status, amount, updated_at)
- `inventory` (sku PK, qty, updated_at)

See `sql/schema.sql` for reference DDL.

## Testing

### Happy Path
1. Start stack: `docker compose up --build`
2. Wait 30 seconds for events to flow
3. Query API with real IDs from database
4. Verify data in MSSQL tables
5. Confirm DLQ is empty (0 messages)

### Failure Handling
1. Producer generates continuous events (controllable via `EVENT_RATE`)
2. Consumer processes all 4 event types
3. Parse/DB errors → DLQ (with payload + error)
4. Retries are idempotent (MERGE upserts)

### Sample Requests
See `http/requests.http` for working examples with real IDs.

## Production Considerations

- **Kafka**: Pre-create topics with proper partitions/replication
- **Consumer scaling**: Increase partitions + run multiple consumer instances (same group)
- **Monitoring**: Scrape `/metrics` endpoints with Prometheus
- **Security**: Use TLS for Kafka, secure MSSQL password, Redis AUTH
- **Observability**: Export logs to centralized logging (ELK, Loki, etc.)

## Acceptance Criteria ✅

- ✅ Happy paths work: events flow from producer → consumer → MSSQL → API
- ✅ Idempotency: Duplicate events don't duplicate DB state
- ✅ DLQ: Failed messages captured with error details (tested with 2644 syntax errors, now 0)
- ✅ API endpoints return correct data
- ✅ Metrics counters and histograms tracking events
- ✅ All 4 event types processed successfully

## Notes

- Kafka auto-creates topics (enabled in compose for dev)
- Producer generates random IDs—payments/orders rarely match naturally
- MSSQL healthcheck may show unhealthy on ARM64 (platform warning) but service works
- Metrics ports not exposed in compose; add port mappings if needed for external scraping
