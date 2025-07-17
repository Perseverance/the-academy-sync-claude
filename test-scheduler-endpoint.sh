#!/bin/bash

# Test script for the scheduler endpoint
# This tests the /tasks/invoke-scheduler endpoint

echo "Testing scheduler endpoint..."
echo "==============================="

# Test with curl
echo "Making POST request to http://localhost:8080/tasks/invoke-scheduler"
curl -X POST http://localhost:8080/tasks/invoke-scheduler \
  -H "Content-Type: application/json" \
  -i

echo -e "\n\nTest completed."