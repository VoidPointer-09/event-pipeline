# Event Pipeline - Quick Test Reference

## 🚀 Quick Start

```bash
# 1. Start all services
docker compose up -d

# 2. Wait for initialization (15-30 seconds)
sleep 20

# 3. Run automated tests
./test.sh
```

## 📋 Test Categories (10 Total)

| # | Category | Tests | What It Checks |
|---|----------|-------|----------------|
| 1 | **Service Health** | 3 | All containers running, API & metrics accessible |
| 2 | **Database Schema** | 4 | Tables: users, orders, payments, inventory |
| 3 | **Event Production** | 1 | Events flowing through pipeline |
| 4 | **API Endpoints** | 3 | Valid/invalid requests, JSON responses |
| 5 | **Payment Workflow** | 3 | Auto-creation, status transitions |
| 6 | **Dead Letter Queue** | 2 | Error capture, invalid JSON handling |
| 7 | **Idempotency** | 1 | MERGE prevents duplicates |
| 8 | **Metrics** | 3 | Counters, latency tracking |
| 9 | **High Load** | 1 | 50+ events/sec (optional, interactive) |
| 10 | **Edge Cases** | 3 | SQL injection, HTTP methods, null handling |

**Total:** ~24 automated tests

## 🎨 Output Colors

- 🔵 **BLUE** `[INFO]` - Information
- 🟡 **YELLOW** `[TEST]` - Test running
- 🟢 **GREEN** `[PASS]` - Test passed ✅
- 🔴 **RED** `[FAIL]` - Test failed ❌

## ⚡ One-Line Commands

```bash
# Full test with high load (auto-accepts high load test)
echo "y" | ./test.sh

# Test only (skip high load)
echo "n" | ./test.sh

# Run and save output
./test.sh | tee test-results-$(date +%Y%m%d-%H%M%S).log

# Quick service check
docker compose ps && curl -s http://localhost:8080/users/test

# Quick metrics check
curl -s http://localhost:2112/metrics | grep "events_processed_total\|dlq_messages_total"
```

## 🔧 Manual Quick Tests

### API Health
```bash
# Test API response
curl http://localhost:8080/users/test-id

# Should return JSON like:
# {"error":"not_found","message":"User not found"}
```

### Metrics Check
```bash
# View consumer metrics
curl http://localhost:2112/metrics | grep "^events_processed_total"

# View API metrics
curl http://localhost:2113/metrics | grep "^http_requests_total"
```

### Database Quick Check
```bash
# Count all records
docker compose exec mssql /opt/mssql-tools18/bin/sqlcmd -S localhost \
  -U sa -P 'YourStrong!Passw0rd' -d events -C -Q \
  "SELECT 
    (SELECT COUNT(*) FROM dbo.users) as users,
    (SELECT COUNT(*) FROM dbo.orders) as orders,
    (SELECT COUNT(*) FROM dbo.payments) as payments,
    (SELECT COUNT(*) FROM dbo.inventory) as inventory"
```

### DLQ Check
```bash
# Check DLQ size
docker compose exec redis redis-cli LLEN dlq

# View latest DLQ message
docker compose exec redis redis-cli LINDEX dlq 0
```

### Logs Quick View
```bash
# Last 20 lines from consumer
docker compose logs --tail=20 consumer

# Last 20 lines from API
docker compose logs --tail=20 api

# Follow producer logs
docker compose logs -f producer
```

## 🐛 Troubleshooting

### Test Script Fails Immediately

**Symptom:** Tests fail with "Service not ready" or connection errors

**Solution:**
```bash
# Check service status
docker compose ps

# Restart all services
docker compose restart

# Wait longer before testing
sleep 30 && ./test.sh
```

### High Load Test Causes Issues

**Symptom:** System becomes unresponsive during high load test

**Solution:**
```bash
# Stop high-rate producer
docker compose stop producer

# Remove temporary containers
docker compose rm -f

# Restart with normal rate
docker compose up -d producer
```

### Database Tests Fail

**Symptom:** "Cannot connect to database" errors

**Solution:**
```bash
# Check database is running
docker compose ps mssql

# Test database connection
docker compose exec mssql /opt/mssql-tools18/bin/sqlcmd \
  -S localhost -U sa -P 'YourStrong!Passw0rd' -C -Q "SELECT 1"

# If fails, restart database
docker compose restart mssql
sleep 30  # Wait for initialization
```

### Metrics Not Available

**Symptom:** `curl: (7) Failed to connect`

**Solution:**
```bash
# Check if ports are exposed
docker compose ps consumer api

# Restart services
docker compose restart consumer api

# Verify ports in docker-compose.yml
grep -A5 "ports:" docker-compose.yml
```

## 📊 Expected Results

### Normal Operation
- **Event Rate:** 5-10 events/sec
- **DB Latency:** 2-5ms average
- **DLQ Size:** 0-2 messages
- **Success Rate:** > 99.9%

### High Load (Optional Test)
- **Event Rate:** 50-100+ events/sec
- **DB Latency:** 3-8ms average
- **Success Rate:** > 99%

### API Performance
- **Response Time:** < 100ms
- **HTTP 200:** Valid requests
- **HTTP 404:** Invalid IDs (with JSON error)
- **HTTP 405:** Invalid methods

## 📁 Related Documentation

- `TEST_SCRIPT_README.md` - Complete test script documentation
- `E2E_TEST_REPORT.md` - Manual end-to-end testing results
- `BEST_WORST_CASE_TEST_REPORT.md` - Stress testing results
- `TESTING_SUMMARY.md` - Executive summary
- `PAYMENT_WORKFLOW.md` - Payment lifecycle documentation

## ⏱️ Test Duration

- **Standard Tests:** ~2-3 minutes
- **With High Load:** ~4-5 minutes
- **Manual Mode:** Variable

## 🎯 Success Criteria

All tests should pass with:
- ✅ All services running
- ✅ Events being processed
- ✅ API returning proper responses
- ✅ Payments auto-created
- ✅ DLQ capturing failures
- ✅ Idempotency working
- ✅ Metrics accurate

## 💡 Tips

1. **Run after changes:** Always run `./test.sh` after code modifications
2. **CI/CD integration:** Script designed for automated pipelines
3. **Save results:** Pipe output to file for record keeping
4. **Monitor during test:** Watch `docker compose logs -f` in another terminal
5. **Clean state:** For fresh start: `docker compose down -v && docker compose up -d`

## 🔗 Quick Links

```bash
# API Endpoints
http://localhost:8080/users/{id}
http://localhost:8080/orders/{id}

# Metrics
http://localhost:2112/metrics  # Consumer
http://localhost:2113/metrics  # API

# Service Management
docker compose ps              # Status
docker compose logs <service>  # Logs
docker compose restart         # Restart all
```

---

**Need help?** Check `TEST_SCRIPT_README.md` for detailed documentation.
