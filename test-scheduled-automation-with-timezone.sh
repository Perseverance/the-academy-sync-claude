#!/bin/bash

# Enhanced test script for scheduled automation with timezone manipulation
# This script temporarily changes a user's timezone to test scheduled automation

echo "============================================="
echo "Scheduled Automation Test with Timezone Setup"
echo "============================================="
echo ""

# Check if user ID is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <user_id>"
    echo "Example: $0 1"
    exit 1
fi

USER_ID=$1
DB_NAME="${DB_NAME:-academy_sync}"
DB_USER="${DB_USER:-postgres}"
DB_HOST="${DB_HOST:-localhost}"

echo "Testing scheduled automation for user ID: $USER_ID"
echo ""

# Function to execute PostgreSQL commands
execute_sql() {
    docker compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" -t -c "$1"
}

# Step 1: Get and save the current timezone
echo "Step 1: Getting current user timezone"
echo "------------------------------------"
ORIGINAL_TIMEZONE=$(execute_sql "SELECT timezone FROM users WHERE id = $USER_ID;" | xargs)

if [ -z "$ORIGINAL_TIMEZONE" ]; then
    echo "ERROR: User $USER_ID not found"
    exit 1
fi

echo "Current timezone: $ORIGINAL_TIMEZONE"
echo ""

# Step 2: Calculate a timezone that's currently in 3-5 AM window
echo "Step 2: Finding a timezone in the 3-5 AM window"
echo "-----------------------------------------------"

# Get current UTC time
CURRENT_UTC=$(date -u +"%Y-%m-%d %H:%M:%S")
echo "Current UTC time: $CURRENT_UTC"

# Function to find a timezone offset that puts us in the 3-5 AM window
find_scheduler_timezone() {
    # Get current UTC hour
    UTC_HOUR=$(date -u +"%H")
    
    # Calculate how many hours to subtract from UTC to get 3-4 AM
    # We want hour 3 or 4 (3 AM - 4:59 AM)
    TARGET_HOUR=4
    OFFSET=$((TARGET_HOUR - UTC_HOUR))
    
    # Adjust for wrap-around (e.g., if UTC is 1 AM and we want 4 AM, offset is +3)
    if [ $OFFSET -lt -12 ]; then
        OFFSET=$((OFFSET + 24))
    elif [ $OFFSET -gt 12 ]; then
        OFFSET=$((OFFSET - 24))
    fi
    
    # Convert offset to timezone name
    if [ $OFFSET -eq 0 ]; then
        echo "UTC"
    elif [ $OFFSET -gt 0 ]; then
        # Positive offset means we need a timezone east of UTC
        # Use Etc/GMT notation (note: signs are reversed in Etc/GMT)
        echo "Etc/GMT-$OFFSET"
    else
        # Negative offset means we need a timezone west of UTC
        OFFSET=${OFFSET#-}  # Remove negative sign
        echo "Etc/GMT+$OFFSET"
    fi
}

SCHEDULER_TIMEZONE=$(find_scheduler_timezone)
echo "Calculated timezone for scheduler window: $SCHEDULER_TIMEZONE"

# Verify the timezone will put us in the window
VERIFY_SQL="SELECT 
    '$SCHEDULER_TIMEZONE' as test_timezone,
    NOW() AT TIME ZONE '$SCHEDULER_TIMEZONE' as local_time,
    EXTRACT(HOUR FROM (NOW() AT TIME ZONE '$SCHEDULER_TIMEZONE')) as local_hour,
    CASE WHEN EXTRACT(HOUR FROM (NOW() AT TIME ZONE '$SCHEDULER_TIMEZONE')) >= 3
         AND EXTRACT(HOUR FROM (NOW() AT TIME ZONE '$SCHEDULER_TIMEZONE')) < 5
    THEN 'IN WINDOW' ELSE 'NOT IN WINDOW' END as status;"

echo ""
echo "Verifying timezone calculation:"
execute_sql "$VERIFY_SQL"
echo ""

# Step 3: Update user timezone
echo "Step 3: Updating user timezone temporarily"
echo "-----------------------------------------"
execute_sql "UPDATE users SET timezone = '$SCHEDULER_TIMEZONE' WHERE id = $USER_ID;"
echo "User timezone updated to: $SCHEDULER_TIMEZONE"
echo ""

# Verify the update
echo "Verifying user is now in processing window:"
VERIFY_USER_SQL="SELECT id, email, timezone,
       NOW() AT TIME ZONE timezone as local_time,
       EXTRACT(HOUR FROM (NOW() AT TIME ZONE timezone)) as local_hour,
       automation_enabled,
       CASE WHEN EXTRACT(HOUR FROM (NOW() AT TIME ZONE timezone)) >= 3
            AND EXTRACT(HOUR FROM (NOW() AT TIME ZONE timezone)) < 5
       THEN 'IN WINDOW' ELSE 'NOT IN WINDOW' END as status
FROM users WHERE id = $USER_ID;"

execute_sql "$VERIFY_USER_SQL"
echo ""

# Step 4: Invoke the scheduler
echo "Step 4: Invoking the scheduler"
echo "------------------------------"
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
    echo "WARNING: Expected HTTP 202, got $HTTP_STATUS"
fi

# Extract jobs_enqueued from response
JOBS_ENQUEUED=$(echo "$BODY" | grep -o '"jobs_enqueued":[0-9]*' | cut -d: -f2)
echo "Jobs enqueued: $JOBS_ENQUEUED"
echo ""

# Step 5: Wait for processing to complete
echo "Step 5: Waiting for processing to complete"
echo "-----------------------------------------"
echo "Waiting 10 seconds for automation engine to process..."
sleep 10

# Check activity logs
echo ""
echo "Checking activity logs for scheduled processing:"
ACTIVITY_LOG_SQL="SELECT 
    processing_date,
    processing_type,
    status,
    activities_found,
    activities_processed,
    processing_duration_ms,
    error_message,
    created_at
FROM activity_logs 
WHERE user_id = $USER_ID 
    AND processing_type = 'daily'
    AND created_at > NOW() - INTERVAL '1 minute'
ORDER BY created_at DESC 
LIMIT 1;"

execute_sql "$ACTIVITY_LOG_SQL"
echo ""

# Step 6: Restore original timezone
echo "Step 6: Restoring original timezone"
echo "-----------------------------------"
execute_sql "UPDATE users SET timezone = '$ORIGINAL_TIMEZONE' WHERE id = $USER_ID;"
echo "User timezone restored to: $ORIGINAL_TIMEZONE"
echo ""

# Final verification
echo "Final verification - user settings restored:"
execute_sql "SELECT id, email, timezone, automation_enabled FROM users WHERE id = $USER_ID;"
echo ""

echo "============================================="
echo "Test completed!"
echo "============================================="
echo ""
echo "Summary:"
echo "- Original timezone: $ORIGINAL_TIMEZONE"
echo "- Test timezone: $SCHEDULER_TIMEZONE"
echo "- Jobs enqueued: ${JOBS_ENQUEUED:-0}"
echo ""
echo "To view detailed logs, run:"
echo "  docker compose logs -f automation-engine | grep -E 'user_id.*$USER_ID'"
echo ""
echo "To check the training plan spreadsheet updates:"
echo "  docker compose exec postgres psql -U $DB_USER -d $DB_NAME -c \"SELECT * FROM activity_logs WHERE user_id = $USER_ID ORDER BY created_at DESC LIMIT 5;\""