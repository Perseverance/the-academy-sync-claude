#!/bin/bash
# Build script for static export that temporarily removes API routes

# Save the exit code
BUILD_SUCCESS=0

# Define cleanup function to restore API routes
cleanup() {
  echo "Cleaning up: Restoring API routes..."
  if [ -d "../temp-api-backup/api" ]; then
    mv ../temp-api-backup/api app/
    rmdir ../temp-api-backup
    echo "API routes restored."
  fi
}

# Set up trap to ensure cleanup happens on exit or interruption
trap cleanup EXIT SIGINT SIGTERM

# Temporarily move API routes completely out of the app directory
# to prevent Next.js from seeing them during build
if [ -d "app/api" ]; then
  mkdir -p ../temp-api-backup
  mv app/api ../temp-api-backup/
fi

# Run the build and capture exit code
npm run build
BUILD_SUCCESS=$?

# Exit with the build status (cleanup will run automatically via trap)
exit $BUILD_SUCCESS