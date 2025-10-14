# Payment Workflow

## Overview

This document explains the automatic payment lifecycle in the event pipeline.

## Payment States

```
OrderPlaced Event → Payment (status: "pending")
                           ↓
                    PaymentSettled Event
                           ↓
                    Payment (status: "settled")
```

## Detailed Flow

### 1. Order Creation
When a **OrderPlaced** event is published:

```json
{
  "eventId": "123e4567-e89b-12d3-a456-426614174000",
  "type": "OrderPlaced",
  "occurredAt": "2025-10-15T10:30:00Z",
  "key": "order-abc123",
  "payload": {
    "orderId": "order-abc123",
    "userId": "user-xyz789",
    "amount": 99.99
  }
}
```

**Consumer Action:**
1. Upserts order to `dbo.orders` table
2. **Automatically creates payment** with:
   - `payment_id` = `order_id` (same as order)
   - `order_id` = from event
   - `status` = "pending"
   - `amount` = from order

**SQL Result:**
```sql
-- dbo.orders
order_id      | user_id       | amount | updated_at
order-abc123  | user-xyz789   | 99.99  | 2025-10-15 10:30:00

-- dbo.payments (auto-created)
payment_id    | order_id      | status   | amount | updated_at
order-abc123  | order-abc123  | pending  | 99.99  | 2025-10-15 10:30:00
```

### 2. Payment Settlement
When a **PaymentSettled** event is published (could be minutes, hours, or days later):

```json
{
  "eventId": "234f5678-f90c-23e4-b567-537725285111",
  "type": "PaymentSettled",
  "occurredAt": "2025-10-15T10:35:00Z",
  "key": "order-abc123",
  "payload": {
    "paymentId": "order-abc123",
    "orderId": "order-abc123",
    "status": "settled",
    "amount": 99.99
  }
}
```

**Consumer Action:**
1. Upserts payment (MERGE on `payment_id`)
2. Updates `status` from "pending" → "settled"

**SQL Result:**
```sql
-- dbo.payments (updated)
payment_id    | order_id      | status   | amount | updated_at
order-abc123  | order-abc123  | settled  | 99.99  | 2025-10-15 10:35:00
```

## Implementation Details

### Consumer Code (`cmd/consumer/main.go`)

```go
case im.TypeOrderPlaced:
    var e im.OrderPlaced
    if err := json.Unmarshal(env.Payload, &e); err != nil {
        dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err)
        return nil
    }
    // 1. Create/update the order
    if err := db.UpsertOrder(ctx, e.OrderID, e.UserID, e.Amount); err != nil {
        dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err)
        return nil
    }
    // 2. Automatically create pending payment
    if err := db.UpsertPayment(ctx, e.OrderID, e.OrderID, "pending", e.Amount); err != nil {
        dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err)
        return nil
    }

case im.TypePaymentSettled:
    var e im.PaymentSettled
    if err := json.Unmarshal(env.Payload, &e); err != nil {
        dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err)
        return nil
    }
    // Update payment status to settled
    if err := db.UpsertPayment(ctx, e.PaymentID, e.OrderID, e.Status, e.Amount); err != nil {
        dlq.Push(ctx, os.Getenv("DLQ_LIST"), m.Value, err)
        return nil
    }
```

### Database MERGE Statement (`internal/storage/mssql.go`)

```go
func (d *DB) UpsertPayment(ctx context.Context, id, orderID, status string, amount float64) error {
    q := `MERGE dbo.payments AS t 
          USING (SELECT @p1 AS payment_id, @p2 AS order_id, @p3 AS status, @p4 AS amount) AS s
          ON (t.payment_id=s.payment_id)
          WHEN MATCHED THEN 
              UPDATE SET order_id=s.order_id, status=s.status, amount=s.amount, updated_at=SYSUTCDATETIME()
          WHEN NOT MATCHED THEN 
              INSERT(payment_id,order_id,status,amount,updated_at) 
              VALUES(s.payment_id,s.order_id,s.status,s.amount,SYSUTCDATETIME());`
    _, err := d.SQL.ExecContext(ctx, q, id, orderID, status, amount)
    return err
}
```

## Benefits

### 1. Automatic Payment Tracking
- Every order immediately has an associated payment record
- No orphaned orders without payment status
- API can always return payment status (never null/empty)

### 2. Event-Driven Status Updates
- Payment status reflects business reality
- `PaymentSettled` events update status asynchronously
- Supports eventual consistency patterns

### 3. Idempotency
- MERGE statements ensure reprocessing is safe
- Duplicate `OrderPlaced` events don't create duplicate payments
- Duplicate `PaymentSettled` events simply update status again (no harm)

### 4. Query Simplicity
- API queries can always include payment status
- No need for LEFT JOIN logic complexity
- Frontend gets consistent response structure

## API Examples

### Query Order with Pending Payment
```bash
curl http://localhost:8080/orders/order-abc123 | jq
```

**Response:**
```json
{
  "OrderID": "order-abc123",
  "UserID": "user-xyz789",
  "Amount": 99.99,
  "PaymentStatus": "pending"
}
```

### Query Order with Settled Payment
After `PaymentSettled` event is processed:

```bash
curl http://localhost:8080/orders/order-abc123 | jq
```

**Response:**
```json
{
  "OrderID": "order-abc123",
  "UserID": "user-xyz789",
  "Amount": 99.99,
  "PaymentStatus": "settled"
}
```

## Testing

### Verify Automatic Payment Creation

1. Check latest orders with their payments:
```bash
docker compose exec mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong!Passw0rd' -d events -C \
  -Q "SELECT TOP 10 o.order_id, o.amount, p.status, o.updated_at 
      FROM dbo.orders o 
      LEFT JOIN dbo.payments p ON o.order_id = p.payment_id 
      ORDER BY o.updated_at DESC"
```

2. Count payments by status:
```bash
docker compose exec mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong!Passw0rd' -d events -C \
  -Q "SELECT status, COUNT(*) as count FROM dbo.payments GROUP BY status"
```

Expected output:
```
status      count
pending     XX
settled     XXXX
```

### Verify Status Transitions

1. Find an order with pending payment:
```bash
docker compose exec mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong!Passw0rd' -d events -C \
  -Q "SELECT TOP 1 order_id FROM dbo.payments WHERE status='pending'"
```

2. Check via API:
```bash
curl http://localhost:8080/orders/{order_id}
# Should show "PaymentStatus": "pending"
```

3. Wait for `PaymentSettled` event to be generated (producer randomly generates all event types)

4. Check again:
```bash
curl http://localhost:8080/orders/{order_id}
# Should show "PaymentStatus": "settled"
```

## Troubleshooting

### No Payments Created
**Issue:** Orders exist but payments table is empty

**Check:**
1. Consumer logs for errors:
   ```bash
   docker compose logs consumer | grep -i error
   ```
2. DLQ for failed messages:
   ```bash
   docker compose exec redis redis-cli LLEN dlq
   docker compose exec redis redis-cli LRANGE dlq 0 5
   ```

### Payments Stuck in Pending
**Issue:** All payments remain "pending", never transition to "settled"

**Check:**
1. Verify producer is generating `PaymentSettled` events:
   ```bash
   docker compose logs producer | grep PaymentSettled
   ```
2. Check if consumer is processing them:
   ```bash
   docker compose logs consumer | grep PaymentSettled
   ```
3. Verify payment_id matches between order and payment:
   ```sql
   SELECT o.order_id, p.payment_id, p.status 
   FROM orders o 
   LEFT JOIN payments p ON o.order_id = p.order_id
   WHERE p.status = 'pending'
   ```

## Future Enhancements

Potential improvements to the payment workflow:

1. **Payment Failed Status**
   - Add "failed" status for declined payments
   - Emit `PaymentFailed` event type

2. **Partial Payments**
   - Support multiple payment records per order
   - Track payment installments

3. **Payment Methods**
   - Add `payment_method` field (credit_card, paypal, etc.)
   - Track payment provider transaction IDs

4. **Refunds**
   - Add "refunded" status
   - Emit `PaymentRefunded` event type

5. **Payment Expiry**
   - Auto-expire pending payments after timeout
   - Background job to mark expired payments

## Summary

The automatic payment creation on `OrderPlaced` events provides:

✅ **Consistency** - Every order has a payment record  
✅ **Traceability** - Payment status tracked from creation  
✅ **Simplicity** - No complex JOIN logic or null handling  
✅ **Idempotency** - Safe to reprocess events  
✅ **Event-Driven** - Status updates via domain events  

This pattern supports eventual consistency and provides clear audit trails for payment lifecycle management.
