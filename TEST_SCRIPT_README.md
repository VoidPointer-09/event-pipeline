# Test Script Documentation

## Overview

The `test.sh` script provides comprehensive automated testing for the entire event pipeline system. It validates functionality, performance, and edge cases across all components.

## Prerequisites

The script requires the following tools to be installed:
- **docker** - For running services
- **curl** - For HTTP requests
- **jq** - For JSON parsing
- **python3** - For calculations

Install missing tools:
```bash
# macOS
brew install jq

# Linux (Ubuntu/Debian)
sudo apt-get install jq
```

## Usage

### Basic Usage

```bash
./test.sh
```

The script will automatically:
1. Check all dependencies
2. Wait for services to be ready
3. Run comprehensive tests
4. Display results with color-coded output

### Requirements

Services must be running before executing the test:
```bash
docker compose up -d
```

Wait 10-15 seconds for all services to initialize before running tests.

## Test Categories

### 1. Service Health Check
- Verifies all Docker containers are running
- Tests API endpoint availability (port 8080)
- Tests metrics endpoint availability (port 2112)

**Expected Result:** All services accessible

### 2. Database Schema Verification
- Checks for existence of required tables: `users`, `orders`, `payments`, `inventory`
- Validates schema initialization

**Expected Result:** All 4 tables exist

### 3. Event Production Verification
- Monitors event processing over 10 seconds
- Validates that events are being produced and consumed
- Reports processing rate

**Expected Result:** Events being processed continuously

### 4. API Endpoint Testing
- Tests valid user retrieval (200 OK)
- Tests invalid user ID (404 with JSON error)
- Tests invalid order ID (404 with JSON error)
- Validates JSON response format

**Expected Result:** Proper HTTP codes and JSON responses

### 5. Payment Workflow Testing
- Checks payment status distribution (pending/settled)
- Verifies OrderPlaced creates pending payments
- Validates payment count >= order count

**Expected Result:** Payments auto-created for orders

### 6. Dead Letter Queue Testing
- Verifies DLQ size is reasonable (< 10 messages)
- Tests invalid JSON handling
- Confirms failed messages sent to DLQ

**Expected Result:** DLQ capturing failures properly

### 7. Idempotency Testing
- Creates test user
- Attempts duplicate MERGE operations
- Validates only 1 record exists

**Expected Result:** No duplicate records created

### 8. Metrics Accuracy Testing
- Validates events_processed_total counter
- Checks DLQ metrics
- Calculates average database latency
- Reports metrics accuracy

**Expected Result:** All metrics tracking correctly

### 9. High Load Testing (Optional)
- **Interactive:** Prompts user before running
- Increases event rate to 50/sec for 15 seconds
- Measures throughput and performance
- Restores normal producer afterwards

**Expected Result:** System handles 30+ events/sec

**Note:** This test is optional and requires user confirmation.

### 10. API Edge Cases Testing
- SQL injection attempt (safely handled)
- Invalid HTTP method (405 response)
- Empty arrays instead of null

**Expected Result:** All edge cases handled properly

## Output Format

The script uses color-coded output:
- 🔵 **BLUE** - Headers and informational messages
- 🟡 **YELLOW** - Test execution notices
- 🟢 **GREEN** - Passed tests
- 🔴 **RED** - Failed tests

### Example Output

```
========================================
Test 1: Service Health Check
========================================

[TEST] Checking if all services are running
[PASS] Services are running
[TEST] Checking API endpoint availability
[PASS] API is accessible
[TEST] Checking metrics endpoint availability
[PASS] Metrics endpoint is accessible
```

## Exit Codes

- **0** - All tests passed
- **1** - One or more tests failed or dependencies missing

## Test Results

At the end of execution, the script displays:
- Total number of tests run
- Number of passed tests (green)
- Number of failed tests (red)
- Overall status (✅ or ❌)

### Example Summary

```
========================================
Test Summary
========================================
Total Tests:  25
Passed:       25
Failed:       0

✅ All tests passed!
System is functioning correctly.
```

## Troubleshooting

### Services Not Ready

If tests fail immediately:
```bash
# Check service status
docker compose ps

# View logs
docker compose logs consumer
docker compose logs api

# Restart services
docker compose restart
```

### High Load Test Skipped

The high load test requires user confirmation. To skip:
- Press `N` when prompted
- Test will continue with remaining tests

To run high load test:
- Press `Y` when prompted
- Wait 15 seconds for completion
- Producer automatically restored

### DLQ Tests Fail

DLQ tests may show "unchanged" if:
- Invalid message already consumed
- Need longer wait time (adjust sleep 3 to sleep 5)

This is informational, not critical.

### Database Connection Errors

If database tests fail:
```bash
# Check database is running
docker compose exec mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -Q "SELECT 1"

# Restart database
docker compose restart mssql

# Wait 30 seconds for initialization
sleep 30
```

## Integration with CI/CD

The script is designed for CI/CD integration:

### GitHub Actions Example

```yaml
name: Test Event Pipeline

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Install dependencies
        run: |
          sudo apt-get update
          sudo apt-get install -y jq
      
      - name: Start services
        run: docker compose up -d
      
      - name: Wait for services
        run: sleep 30
      
      - name: Run tests
        run: ./test.sh
      
      - name: Cleanup
        if: always()
        run: docker compose down -v
```

### Jenkins Pipeline Example

```groovy
pipeline {
    agent any
    stages {
        stage('Setup') {
            steps {
                sh 'docker compose up -d'
                sh 'sleep 30'
            }
        }
        stage('Test') {
            steps {
                sh './test.sh'
            }
        }
        stage('Cleanup') {
            steps {
                sh 'docker compose down -v'
            }
        }
    }
}
```

## Customization

### Adjusting Timeouts

Edit wait times in `test.sh`:
```bash
# Line 53: Service readiness timeout (default: 30 seconds)
for i in {1..30}; do

# Line 118: Event production monitoring (default: 10 seconds)
sleep 10

# Line 326: High load test duration (default: 15 seconds)
sleep 15
```

### Modifying Test Thresholds

Edit validation criteria:
```bash
# Line 250: DLQ acceptable size (default: < 10)
if [ "$DLQ_SIZE" -lt "10" ]; then

# Line 342: High load minimum rate (default: 30 events/sec)
if [ "$RATE" -gt "30" ]; then
```

### Adding Custom Tests

Add new test functions following the pattern:
```bash
test_custom_feature() {
    print_header "Test X: Custom Feature Testing"
    
    print_test "Describe what you're testing"
    # Your test logic here
    
    if [ condition ]; then
        print_pass "Test passed"
    else
        print_fail "Test failed"
    fi
}
```

Then add to `main()` function:
```bash
test_custom_feature
```

## Manual Testing Alternative

If you prefer manual testing, refer to:
- `E2E_TEST_REPORT.md` - End-to-end manual tests
- `BEST_WORST_CASE_TEST_REPORT.md` - Stress testing scenarios
- `TESTING_SUMMARY.md` - Executive summary

## Support

For issues or questions:
1. Check Docker logs: `docker compose logs`
2. Review test output carefully
3. Ensure all services are running: `docker compose ps`
4. Verify database connectivity
5. Check metrics endpoints manually

## Performance Benchmarks

Expected performance (based on testing):
- **Event Processing:** 5-10 events/sec (normal)
- **High Load:** 50-100+ events/sec sustained
- **API Response Time:** < 100ms
- **DB Latency:** 2-5ms average
- **Success Rate:** > 99.9%
- **DLQ Size:** < 10 messages

## License

This test script is part of the event-pipeline project.
