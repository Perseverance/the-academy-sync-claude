#!/bin/bash

# Script to check the status of the Cloud Scheduler in staging
# Usage: ./check-scheduler-status.sh

set -e

PROJECT_ID="the-academy-sync-sdlc-test"
REGION="europe-central2"
SCHEDULER_JOB_NAME="staging-automation-scheduler"

echo "========================================"
echo "Cloud Scheduler Status Check - Staging"
echo "========================================"
echo ""

# 1. Check if the scheduler job exists and its configuration
echo "1. Scheduler Job Details:"
echo "-------------------------"
gcloud scheduler jobs describe $SCHEDULER_JOB_NAME \
  --location=$REGION \
  --project=$PROJECT_ID \
  --format="yaml(name,schedule,timeZone,state,lastAttemptTime,nextRunTime)" || {
    echo "ERROR: Scheduler job not found!"
    exit 1
  }

echo ""
echo "2. Last Execution Status:"
echo "------------------------"
# Get the last attempt details
LAST_ATTEMPT=$(gcloud scheduler jobs describe $SCHEDULER_JOB_NAME \
  --location=$REGION \
  --project=$PROJECT_ID \
  --format="value(lastAttemptTime)")

if [ -n "$LAST_ATTEMPT" ]; then
    echo "Last attempt: $LAST_ATTEMPT"
    
    # Check recent scheduler logs
    echo ""
    echo "Recent scheduler execution logs:"
    gcloud logging read "resource.type=cloud_scheduler_job AND resource.labels.job_id=$SCHEDULER_JOB_NAME AND timestamp>=\"$(date -u -d '1 hour ago' '+%Y-%m-%dT%H:%M:%S')Z\"" \
      --limit=5 \
      --project=$PROJECT_ID \
      --format="table(timestamp,severity,textPayload,jsonPayload.statusCode)"
else
    echo "No execution attempts yet"
fi

echo ""
echo "3. Backend API Scheduler Endpoint Calls (Last Hour):"
echo "---------------------------------------------------"
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=staging-backend-api AND httpRequest.requestUrl=~\"invoke-scheduler\" AND timestamp>=\"$(date -u -d '1 hour ago' '+%Y-%m-%dT%H:%M:%S')Z\"" \
  --limit=10 \
  --project=$PROJECT_ID \
  --format="table(timestamp,httpRequest.status,jsonPayload.message,jsonPayload.jobs_enqueued)"

echo ""
echo "4. Automation Engine Scheduled Processing (Last Hour):"
echo "----------------------------------------------------"
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=staging-automation-engine AND jsonPayload.msg=~\"scheduled\" AND timestamp>=\"$(date -u -d '1 hour ago' '+%Y-%m-%dT%H:%M:%S')Z\"" \
  --limit=10 \
  --project=$PROJECT_ID \
  --format="table(timestamp,jsonPayload.msg,jsonPayload.user_id,jsonPayload.trigger_type)"

echo ""
echo "5. Database Activity Logs (Recent Scheduled Processing):"
echo "------------------------------------------------------"
echo "Run this SQL query in Cloud SQL:"
echo ""
cat << 'EOF'
SELECT 
    user_id,
    processing_date,
    processing_type,
    status,
    activities_found,
    activities_processed,
    processing_duration_ms,
    error_message,
    created_at
FROM activity_logs 
WHERE processing_type = 'daily'
  AND created_at > NOW() - INTERVAL '2 hours'
ORDER BY created_at DESC
LIMIT 10;
EOF

echo ""
echo "6. Quick Status Summary:"
echo "-----------------------"
# Get scheduler state
STATE=$(gcloud scheduler jobs describe $SCHEDULER_JOB_NAME \
  --location=$REGION \
  --project=$PROJECT_ID \
  --format="value(state)")

NEXT_RUN=$(gcloud scheduler jobs describe $SCHEDULER_JOB_NAME \
  --location=$REGION \
  --project=$PROJECT_ID \
  --format="value(nextRunTime)")

echo "Scheduler State: $STATE"
echo "Next Run Time: $NEXT_RUN"

if [ "$STATE" = "ENABLED" ]; then
    echo "✅ Scheduler is ENABLED and running"
else
    echo "⚠️  Scheduler is $STATE"
fi

echo ""
echo "========================================"
echo "Manual Actions You Can Take:"
echo "========================================"
echo ""
echo "1. Force run the scheduler job now:"
echo "   gcloud scheduler jobs run $SCHEDULER_JOB_NAME --location=$REGION --project=$PROJECT_ID"
echo ""
echo "2. Pause the scheduler:"
echo "   gcloud scheduler jobs pause $SCHEDULER_JOB_NAME --location=$REGION --project=$PROJECT_ID"
echo ""
echo "3. Resume the scheduler:"
echo "   gcloud scheduler jobs resume $SCHEDULER_JOB_NAME --location=$REGION --project=$PROJECT_ID"
echo ""
echo "4. View scheduler in Cloud Console:"
echo "   https://console.cloud.google.com/cloudscheduler?project=$PROJECT_ID"
echo ""