#!/bin/bash
# Build script for static export that temporarily removes API routes

# Move API routes out of the way
if [ -d "app/api" ]; then
  mv app/api app/api.bak
fi

# Run the build
npm run build

# Restore API routes
if [ -d "app/api.bak" ]; then
  mv app/api.bak app/api
fi