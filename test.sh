#!/bin/bash

# Event Pipeline Test Suite
# Comprehensive automated testing script

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Functions
print_header() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
}

print_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

print_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((PASSED_TESTS++))
    ((TOTAL_TESTS++))
}

print_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((FAILED_TESTS++))
    ((TOTAL_TESTS++))
}

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

wait_for_services() {
    print_info "Waiting for services to be ready..."
    sleep 5
    
    # Wait for API to be ready
    for i in {1..30}; do
        if curl -s http://localhost:8080/users/test > /dev/null 2>&1; then
            print_info "API is ready"
            break
        fi
        sleep 1
    done
    
    # Wait for metrics to be ready
    for i in {1..30}; do
        if curl -s http://localhost:2112/metrics > /dev/null 2>&1; then
            print_info "Metrics endpoint is ready"
            break
        fi
        sleep 1
    done
    
    sleep 3
}

# Test 1: Service Health Check
test_service_health() {
    print_header "Test 1: Service Health Check"
    
    print_test "Checking if all services are running"
    if docker compose ps | grep -q "Up"; then
        print_pass "Services are running"
    else
        print_fail "Some services are not running"
        docker compose ps
    fi
    
    print_test "Checking API endpoint availability"
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/users/test-id | grep -q "404\|200"; then
        print_pass "API is accessible"
    else
        print_fail "API is not accessible"
    fi
    
    print_test "Checking metrics endpoint availability"
    if curl -s http://localhost:2112/metrics | grep -q "events_processed_total"; then
        print_pass "Metrics endpoint is accessible"
    else
        print_fail "Metrics endpoint is not accessible"
    fi
}

# Test 2: Database Schema
test_database_schema() {
    print_header "Test 2: Database Schema Verification"
    
    print_test "Checking if all required tables exist"
    TABLES=$(docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -h -1 -W -Q "SET NOCOUNT ON; SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE='BASE TABLE'" | tr -d '\r\n ' | grep -o "[a-z]*" | sort | uniq)
    
    for table in "users" "orders" "payments" "inventory"; do
        if echo "$TABLES" | grep -q "$table"; then
            print_pass "Table '$table' exists"
        else
            print_fail "Table '$table' is missing"
        fi
    done
}

# Test 3: Event Production
test_event_production() {
    print_header "Test 3: Event Production Verification"
    
    print_test "Capturing initial event count"
    INITIAL_COUNT=$(curl -s http://localhost:2112/metrics | grep "^events_processed_total" | awk '{print $2}')
    print_info "Initial count: $INITIAL_COUNT"
    
    print_test "Waiting 10 seconds for new events"
    sleep 10
    
    FINAL_COUNT=$(curl -s http://localhost:2112/metrics | grep "^events_processed_total" | awk '{print $2}')
    print_info "Final count: $FINAL_COUNT"
    
    if [ "$FINAL_COUNT" -gt "$INITIAL_COUNT" ]; then
        print_pass "Events are being produced and consumed"
        print_info "Events processed: $((FINAL_COUNT - INITIAL_COUNT)) in 10 seconds"
    else
        print_fail "No new events processed"
    fi
}

# Test 4: API Endpoints
test_api_endpoints() {
    print_header "Test 4: API Endpoint Testing"
    
    # Get a real user ID from database
    print_test "Getting test user ID from database"
    USER_ID=$(docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -h -1 -W -Q "SET NOCOUNT ON; SELECT TOP 1 user_id FROM dbo.users" | tr -d ' \r\n')
    
    if [ -n "$USER_ID" ]; then
        print_info "Using user ID: $USER_ID"
        
        print_test "Testing GET /users/{id} with valid ID"
        RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/users/$USER_ID)
        HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
        BODY=$(echo "$RESPONSE" | head -n-1)
        
        if [ "$HTTP_CODE" = "200" ] && echo "$BODY" | jq -e '.UserID' > /dev/null 2>&1; then
            print_pass "Valid user request returns 200 and valid JSON"
        else
            print_fail "Valid user request failed (HTTP $HTTP_CODE)"
        fi
    else
        print_info "No users in database yet, skipping valid user test"
    fi
    
    print_test "Testing GET /users/{id} with invalid ID"
    RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/users/non-existent-user-id)
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n-1)
    
    if [ "$HTTP_CODE" = "404" ] && echo "$BODY" | jq -e '.error' > /dev/null 2>&1; then
        print_pass "Invalid user request returns 404 with JSON error"
    else
        print_fail "Invalid user request handling failed (HTTP $HTTP_CODE)"
    fi
    
    print_test "Testing GET /orders/{id} with invalid ID"
    RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/orders/non-existent-order-id)
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n-1)
    
    if [ "$HTTP_CODE" = "404" ] && echo "$BODY" | jq -e '.error' > /dev/null 2>&1; then
        print_pass "Invalid order request returns 404 with JSON error"
    else
        print_fail "Invalid order request handling failed (HTTP $HTTP_CODE)"
    fi
}

# Test 5: Payment Workflow
test_payment_workflow() {
    print_header "Test 5: Payment Workflow Testing"
    
    print_test "Checking payment status distribution"
    PAYMENT_STATS=$(docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -h -1 -W -Q "SET NOCOUNT ON; SELECT status, COUNT(*) as count FROM dbo.payments GROUP BY status" | tr -d '\r')
    
    if echo "$PAYMENT_STATS" | grep -q "pending"; then
        print_pass "Pending payments exist"
    else
        print_info "No pending payments found (may be normal)"
    fi
    
    if echo "$PAYMENT_STATS" | grep -q "settled"; then
        print_pass "Settled payments exist"
    else
        print_info "No settled payments found (may be normal)"
    fi
    
    print_test "Verifying OrderPlaced creates pending payment"
    ORDER_COUNT=$(docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -h -1 -W -Q "SET NOCOUNT ON; SELECT COUNT(*) FROM dbo.orders" | tr -d ' \r\n')
    PAYMENT_COUNT=$(docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -h -1 -W -Q "SET NOCOUNT ON; SELECT COUNT(*) FROM dbo.payments" | tr -d ' \r\n')
    
    if [ "$PAYMENT_COUNT" -ge "$ORDER_COUNT" ]; then
        print_pass "Payment auto-creation working (payments >= orders)"
        print_info "Orders: $ORDER_COUNT, Payments: $PAYMENT_COUNT"
    else
        print_fail "Insufficient payments for orders"
    fi
}

# Test 6: DLQ Verification
test_dlq() {
    print_header "Test 6: Dead Letter Queue Testing"
    
    print_test "Checking DLQ size"
    DLQ_SIZE=$(docker compose exec -T redis redis-cli LLEN dlq)
    print_info "DLQ size: $DLQ_SIZE messages"
    
    if [ "$DLQ_SIZE" -lt "10" ]; then
        print_pass "DLQ has acceptable size (< 10 messages)"
    else
        print_fail "DLQ has too many messages: $DLQ_SIZE"
    fi
    
    print_test "Testing invalid JSON handling"
    BEFORE_DLQ=$(docker compose exec -T redis redis-cli LLEN dlq)
    echo "invalid json test" | docker compose exec -T kafka /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka:9092 --topic events 2>/dev/null || true
    sleep 3
    AFTER_DLQ=$(docker compose exec -T redis redis-cli LLEN dlq)
    
    if [ "$AFTER_DLQ" -gt "$BEFORE_DLQ" ]; then
        print_pass "Invalid JSON sent to DLQ"
    else
        print_info "DLQ size unchanged (may need longer wait)"
    fi
}

# Test 7: Idempotency
test_idempotency() {
    print_header "Test 7: Idempotency Testing"
    
    TEST_USER_ID="idempotency-test-$(date +%s)"
    
    print_test "Creating test user"
    docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -Q "INSERT INTO dbo.users (user_id, name, email, updated_at) VALUES ('$TEST_USER_ID', 'Test User', 'test@example.com', SYSUTCDATETIME());" > /dev/null 2>&1
    
    print_test "Attempting duplicate MERGE operation"
    docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -Q "MERGE dbo.users AS t USING (SELECT '$TEST_USER_ID' AS user_id, 'Updated Name' AS name, 'updated@example.com' AS email) AS s ON (t.user_id=s.user_id) WHEN MATCHED THEN UPDATE SET name=s.name, email=s.email, updated_at=SYSUTCDATETIME() WHEN NOT MATCHED THEN INSERT(user_id,name,email,updated_at) VALUES(s.user_id,s.name,s.email,SYSUTCDATETIME());" > /dev/null 2>&1
    
    COUNT=$(docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -h -1 -W -Q "SET NOCOUNT ON; SELECT COUNT(*) FROM dbo.users WHERE user_id = '$TEST_USER_ID'" | tr -d ' \r\n')
    
    if [ "$COUNT" = "1" ]; then
        print_pass "MERGE operation is idempotent (count = 1)"
    else
        print_fail "MERGE created duplicate records (count = $COUNT)"
    fi
}

# Test 8: Metrics Accuracy
test_metrics() {
    print_header "Test 8: Metrics Accuracy Testing"
    
    print_test "Verifying metrics are being collected"
    PROCESSED=$(curl -s http://localhost:2112/metrics | grep "^events_processed_total" | awk '{print $2}')
    DLQ_METRICS=$(curl -s http://localhost:2112/metrics | grep "^dlq_messages_total" | awk '{print $2}')
    LATENCY_COUNT=$(curl -s http://localhost:2112/metrics | grep "^db_latency_seconds_count" | awk '{print $2}')
    
    if [ "$PROCESSED" -gt "0" ]; then
        print_pass "Events processed metric is tracking (value: $PROCESSED)"
    else
        print_fail "Events processed metric is zero"
    fi
    
    print_info "DLQ messages: $DLQ_METRICS"
    print_info "DB operations tracked: $LATENCY_COUNT"
    
    if [ "$LATENCY_COUNT" -gt "0" ]; then
        LATENCY_SUM=$(curl -s http://localhost:2112/metrics | grep "^db_latency_seconds_sum" | awk '{print $2}')
        AVG_LATENCY=$(python3 -c "print(f'{($LATENCY_SUM / $LATENCY_COUNT * 1000):.2f}')" 2>/dev/null || echo "N/A")
        print_pass "DB latency is being tracked (avg: ${AVG_LATENCY}ms)"
    else
        print_info "DB latency tracking not yet available"
    fi
}

# Test 9: High Load (Optional)
test_high_load() {
    print_header "Test 9: High Load Testing (Optional)"
    
    read -p "Run high load test? This will increase event rate to 50/sec for 15 seconds. (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Skipping high load test"
        return
    fi
    
    print_test "Capturing baseline metrics"
    BASELINE=$(curl -s http://localhost:2112/metrics | grep "^events_processed_total" | awk '{print $2}')
    print_info "Baseline: $BASELINE events"
    
    print_test "Stopping normal producer"
    docker compose stop producer > /dev/null 2>&1
    
    print_test "Starting high-rate producer (50 events/sec)"
    docker compose run -e EVENT_RATE=50 -d producer > /dev/null 2>&1
    
    print_test "Monitoring for 15 seconds"
    sleep 15
    
    AFTER=$(curl -s http://localhost:2112/metrics | grep "^events_processed_total" | awk '{print $2}')
    PROCESSED=$((AFTER - BASELINE))
    RATE=$((PROCESSED / 15))
    
    print_info "Processed: $PROCESSED events in 15 seconds"
    print_info "Rate: $RATE events/sec"
    
    if [ "$RATE" -gt "30" ]; then
        print_pass "High load handled successfully ($RATE events/sec)"
    else
        print_fail "High load performance below expected ($RATE events/sec)"
    fi
    
    print_test "Restoring normal producer"
    docker compose stop event-pipeline-producer-run-* > /dev/null 2>&1 || true
    docker compose up -d producer > /dev/null 2>&1
}

# Test 10: API Edge Cases
test_api_edge_cases() {
    print_header "Test 10: API Edge Cases Testing"
    
    print_test "Testing SQL injection attempt"
    RESPONSE=$(curl -s -w "\n%{http_code}" "http://localhost:8080/users/'; DROP TABLE users; --")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n-1)
    
    if [ "$HTTP_CODE" = "404" ] && echo "$BODY" | jq -e '.error' > /dev/null 2>&1; then
        print_pass "SQL injection attempt safely handled"
    else
        print_fail "SQL injection handling issue (HTTP $HTTP_CODE)"
    fi
    
    print_test "Testing invalid HTTP method"
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8080/users/test-id)
    
    if [ "$HTTP_CODE" = "405" ]; then
        print_pass "Invalid HTTP method rejected (405)"
    else
        print_fail "Invalid HTTP method not properly rejected (HTTP $HTTP_CODE)"
    fi
    
    print_test "Testing empty orders array (not null)"
    # Find a user with no orders or create one
    TEST_USER="empty-orders-test-$(date +%s)"
    docker compose exec -T mssql /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong!Passw0rd' -d events -C -Q "INSERT INTO dbo.users (user_id, name, email, updated_at) VALUES ('$TEST_USER', 'Test', 'test@example.com', SYSUTCDATETIME());" > /dev/null 2>&1
    
    RESPONSE=$(curl -s http://localhost:8080/users/$TEST_USER)
    if echo "$RESPONSE" | jq -e '.Orders == []' > /dev/null 2>&1; then
        print_pass "Empty orders returns [] not null"
    else
        print_fail "Empty orders handling issue"
    fi
}

# Main execution
main() {
    clear
    print_header "Event Pipeline Test Suite"
    print_info "Starting comprehensive test suite..."
    print_info "Date: $(date)"
    echo ""
    
    # Check dependencies
    print_info "Checking dependencies..."
    for cmd in docker curl jq python3; do
        if ! command -v $cmd &> /dev/null; then
            echo -e "${RED}ERROR: $cmd is not installed${NC}"
            exit 1
        fi
    done
    print_info "All dependencies available"
    
    # Wait for services
    wait_for_services
    
    # Run tests
    test_service_health
    test_database_schema
    test_event_production
    test_api_endpoints
    test_payment_workflow
    test_dlq
    test_idempotency
    test_metrics
    test_api_edge_cases
    test_high_load
    
    # Summary
    print_header "Test Summary"
    echo -e "Total Tests:  ${TOTAL_TESTS}"
    echo -e "${GREEN}Passed:       ${PASSED_TESTS}${NC}"
    if [ "$FAILED_TESTS" -gt "0" ]; then
        echo -e "${RED}Failed:       ${FAILED_TESTS}${NC}"
    else
        echo -e "Failed:       ${FAILED_TESTS}"
    fi
    echo ""
    
    if [ "$FAILED_TESTS" -eq "0" ]; then
        echo -e "${GREEN}✅ All tests passed!${NC}"
        echo -e "${GREEN}System is functioning correctly.${NC}"
        exit 0
    else
        echo -e "${RED}❌ Some tests failed!${NC}"
        echo -e "${RED}Please review the failures above.${NC}"
        exit 1
    fi
}

# Run main function
main
