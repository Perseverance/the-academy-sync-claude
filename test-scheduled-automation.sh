#!/bin/bash

# End-to-end test script for scheduled automation feature
# This script demonstrates how to test the complete flow

echo "==================================="
echo "Scheduled Automation Test Script"
echo "==================================="
echo ""

# Check if user ID is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <user_id>"
    echo "Example: $0 1"
    exit 1
fi

USER_ID=$1

echo "Testing scheduled automation for user ID: $USER_ID"
echo ""

# Step 1: Test the scheduler endpoint
echo "Step 1: Testing scheduler endpoint"
echo "---------------------------------"
echo "Making POST request to http://localhost:8080/tasks/invoke-scheduler"
echo ""

RESPONSE=$(curl -s -X POST http://localhost:8080/tasks/invoke-scheduler \
  -H "Content-Type: application/json" \
  -w "\nHTTP_STATUS:%{http_code}")

HTTP_STATUS=$(echo "$RESPONSE" | grep -o "HTTP_STATUS:[0-9]*" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')

echo "Response Status: $HTTP_STATUS"
echo "Response Body: $BODY"
echo ""

if [ "$HTTP_STATUS" != "202" ]; then
    echo "ERROR: Expected HTTP 202, got $HTTP_STATUS"
    exit 1
fi

# Extract jobs_enqueued from response
JOBS_ENQUEUED=$(echo "$BODY" | grep -o '"jobs_enqueued":[0-9]*' | cut -d: -f2)
echo "Jobs enqueued: $JOBS_ENQUEUED"
echo ""

# Step 2: Check Redis queue (if Redis CLI is available)
if command -v redis-cli &> /dev/null; then
    echo "Step 2: Checking Redis queue"
    echo "----------------------------"
    QUEUE_LENGTH=$(redis-cli -u "${REDIS_URL:-redis://localhost:6379}" LLEN jobs_queue)
    echo "Current queue length: $QUEUE_LENGTH"
    echo ""
fi

# Step 3: Monitor logs (if the services are running)
echo "Step 3: Instructions for monitoring"
echo "----------------------------------"
echo "To monitor the processing, run these commands in separate terminals:"
echo ""
echo "1. Backend API logs:"
echo "   docker compose logs -f backend-api | grep -E '(scheduler|invoke-scheduler|ProcessScheduledRun)'"
echo ""
echo "2. Automation Engine logs:"
echo "   docker compose logs -f automation-engine | grep -E '(scheduled|ProcessUser|RunScheduledCycle)'"
echo ""
echo "3. Database activity logs (after processing):"
echo "   docker compose exec postgres psql -U postgres -d academy_sync -c \"SELECT * FROM activity_logs WHERE user_id = $USER_ID AND processing_type = 'daily' ORDER BY created_at DESC LIMIT 5;\""
echo ""

# Step 4: Manual time zone test
echo "Step 4: Testing time zone calculation"
echo "------------------------------------"
echo "To test if a specific user would be in the processing window:"
echo ""
echo "docker compose exec postgres psql -U postgres -d academy_sync -c \""
echo "SELECT id, email, timezone,"
echo "       NOW() AT TIME ZONE timezone as local_time,"
echo "       EXTRACT(HOUR FROM (NOW() AT TIME ZONE timezone)) as local_hour,"
echo "       CASE WHEN EXTRACT(HOUR FROM (NOW() AT TIME ZONE timezone)) >= 3"
echo "            AND EXTRACT(HOUR FROM (NOW() AT TIME ZONE timezone)) < 5"
echo "       THEN 'IN WINDOW' ELSE 'NOT IN WINDOW' END as status"
echo "FROM users WHERE id = $USER_ID;\""
echo ""

echo "Test script completed. Check the logs for processing details."