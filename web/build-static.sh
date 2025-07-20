#!/bin/bash
# Build script for static export that temporarily removes API routes

# Save the exit code
BUILD_SUCCESS=0

# Temporarily move API routes completely out of the app directory
# to prevent Next.js from seeing them during build
if [ -d "app/api" ]; then
  mkdir -p ../temp-api-backup
  mv app/api ../temp-api-backup/
fi

# Run the build and capture exit code
npm run build
BUILD_SUCCESS=$?

# Always restore API routes, even if build fails
if [ -d "../temp-api-backup/api" ]; then
  mv ../temp-api-backup/api app/
  rmdir ../temp-api-backup
fi

# Exit with the build status
exit $BUILD_SUCCESS