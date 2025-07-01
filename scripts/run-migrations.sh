#!/bin/bash
set -e

# Enable verbose output
set -x

echo "========================================="
echo "Starting migration script"
echo "========================================="
echo "Environment variables:"
echo "POSTGRES_HOST: ${POSTGRES_HOST}"
echo "POSTGRES_USER: ${POSTGRES_USER}"
echo "POSTGRES_DB: ${POSTGRES_DB}"
echo "POSTGRES_PASSWORD: [REDACTED - length: ${#POSTGRES_PASSWORD}]"
echo "========================================="

echo "Waiting for PostgreSQL to be ready..."
echo "Testing connection to: $POSTGRES_HOST:5432"

# Counter for connection attempts
attempt=0
max_attempts=30

# Wait for PostgreSQL to be ready with timeout
until PGPASSWORD=$POSTGRES_PASSWORD psql -h "$POSTGRES_HOST" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c '\q'; do
  attempt=$((attempt + 1))
  echo "PostgreSQL connection attempt $attempt/$max_attempts failed - sleeping 2 seconds"
  
  if [ $attempt -ge $max_attempts ]; then
    echo "ERROR: Could not connect to PostgreSQL after $max_attempts attempts"
    echo "Connection string: postgres://$POSTGRES_USER:***@$POSTGRES_HOST:5432/$POSTGRES_DB"
    exit 1
  fi
  
  sleep 2
done

echo "========================================="
echo "PostgreSQL is ready - preparing to run migrations"
echo "========================================="

# Check if migrations directory exists
echo "Checking migrations directory..."
if [ -d "/migrations" ]; then
  echo "Migrations directory exists"
  echo "Contents of /migrations:"
  ls -la /migrations/
else
  echo "ERROR: Migrations directory /migrations does not exist"
  exit 1
fi

# Check if migrate command exists
echo "Checking migrate command..."
if command -v migrate &> /dev/null; then
  echo "migrate command found at: $(which migrate)"
  migrate -version
else
  echo "ERROR: migrate command not found"
  exit 1
fi

echo "========================================="
echo "Running migrations..."
echo "========================================="

# Build the database URL
DATABASE_URL="postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@$POSTGRES_HOST:5432/$POSTGRES_DB?sslmode=disable"
echo "Database URL (password hidden): postgres://$POSTGRES_USER:***@$POSTGRES_HOST:5432/$POSTGRES_DB?sslmode=disable"

# Run migrations with verbose output
migrate -path /migrations -database "$DATABASE_URL" -verbose up

migration_exit_code=$?
echo "Migration command exit code: $migration_exit_code"

if [ $migration_exit_code -eq 0 ]; then
  echo "========================================="
  echo "Migrations completed successfully"
  echo "========================================="
else
  echo "========================================="
  echo "ERROR: Migrations failed with exit code $migration_exit_code"
  echo "========================================="
  exit $migration_exit_code
fi