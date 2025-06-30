# Retry Package

The retry package provides a robust, configurable retry mechanism for handling transient failures in distributed systems. It implements exponential backoff with jitter and includes specialized support for HTTP operations.

## Features

- **Exponential Backoff**: Automatically increases delay between retry attempts
- **Jitter**: Adds randomization to prevent thundering herd problems
- **Custom Retry Predicates**: Fine-grained control over which errors trigger retries
- **HTTP-Aware Retries**: Built-in support for HTTP status codes and Retry-After headers
- **Generic Support**: Works with functions that return values using Go generics
- **Context Awareness**: Respects context cancellation for graceful shutdowns
- **Comprehensive Logging**: Structured logging for debugging and monitoring

## Installation

```go
import "github.com/Perseverance/the-academy-sync-claude/internal/pkg/retry"
```

## Quick Start

### Basic Retry

```go
// Simple operation retry
cfg := retry.DefaultConfig()
err := retry.WithExponentialBackoff(ctx, cfg, logger, "database_connect", func() error {
    return db.Connect()
})
```

### Retry with Jitter

```go
// Prevent thundering herd with jitter
cfg := retry.DefaultConfigWithJitter()
err := retry.WithExponentialBackoffJitter(ctx, cfg, logger, "api_call", func() error {
    return callExternalAPI()
})
```

### HTTP Client with Retry

```go
// Create an HTTP client that automatically retries
client := retry.NewHTTPClientWithRetry(retry.DefaultHTTPConfig(), logger)
resp, err := client.Get("https://api.example.com/data")
```

### Custom Retry Logic

```go
// Only retry specific errors
predicate := retry.RetryPredicate(func(err error) bool {
    var tempErr *TemporaryError
    return errors.As(err, &tempErr)
})

err := retry.WithExponentialBackoffJitterPredicate(
    ctx, cfg, logger, "custom_operation", predicate, operation,
)
```

### Retry with Return Values

```go
// Retry operations that return data
user, err := retry.DoWithResult(ctx, cfg, logger, "fetch_user", func() (*User, error) {
    return fetchUserFromAPI(userID)
})
```

## Configuration

### Basic Configuration

```go
cfg := retry.Config{
    MaxAttempts: 3,                    // Total attempts (including first try)
    BaseDelay:   1 * time.Second,      // Initial delay
    MaxDelay:    10 * time.Second,     // Maximum delay between attempts
}
```

### Configuration with Jitter

```go
cfg := retry.ConfigWithJitter{
    Config: retry.Config{
        MaxAttempts: 3,
        BaseDelay:   1 * time.Second,
        MaxDelay:    10 * time.Second,
    },
    JitterFactor: 0.2,  // ±20% randomization
}
```

### HTTP Configuration

```go
cfg := retry.HTTPConfig{
    ConfigWithJitter: retry.DefaultConfigWithJitter(),
    RetryableStatusCodes: []int{429, 502, 503, 504},
    RespectRetryAfter:    true,
    MaxRetryAfter:        30 * time.Second,
}
```

## Retry Strategies

### Exponential Backoff

The delay between attempts grows exponentially:
- Attempt 1: No delay (immediate)
- Attempt 2: BaseDelay * 2^0 = BaseDelay
- Attempt 3: BaseDelay * 2^1 = BaseDelay * 2
- Attempt 4: BaseDelay * 2^2 = BaseDelay * 4
- And so on, capped at MaxDelay

### Jitter

Jitter adds randomization to delays to prevent synchronized retries:
- With JitterFactor = 0.2, a 1-second delay becomes 0.8-1.2 seconds
- This helps distribute load when multiple clients retry simultaneously

### Fixed Delay

For operations that don't benefit from exponential backoff:

```go
err := retry.WithSimpleRetry(ctx, 5, 2*time.Second, logger, "poll_status", func() error {
    return checkStatus()
})
```

## HTTP Retry Features

### Automatic Status Code Handling

By default, retries on:
- 429 (Too Many Requests)
- 502 (Bad Gateway)
- 503 (Service Unavailable)
- 504 (Gateway Timeout)

### Retry-After Header Support

Automatically respects Retry-After headers:
- Parses both second-based and HTTP-date formats
- Caps wait time at MaxRetryAfter
- Falls back to exponential backoff if header is missing

### Request Body Preservation

Automatically preserves request bodies across retries:

```go
body := bytes.NewReader([]byte(`{"data": "value"}`))
req, _ := http.NewRequest("POST", url, body)
resp, err := retry.WithHTTPRetry(ctx, client, req, config, logger)
```

## Advanced Usage

### Combining Predicates

Create complex retry conditions:

```go
// Retry on network errors AND specific status codes
predicate := retry.CombinePredicates(
    retry.IsNetworkError(),
    retry.IsRetryableHTTPStatus(500, 502),
)

// Retry on network errors OR specific status codes
predicate := retry.AnyPredicate(
    retry.IsNetworkError(),
    retry.IsRetryableHTTPStatus(500, 502),
)
```

### Custom HTTP Transport

Add retry to existing HTTP clients:

```go
transport := &retry.HTTPRetryTransport{
    Base:   http.DefaultTransport,
    Config: retry.DefaultHTTPConfig(),
    Logger: logger,
}
client := &http.Client{
    Transport: transport,
    Timeout:   30 * time.Second,
}
```

### Integration with Existing Code

Update the Strava client example:

```go
// Before: Manual retry logic
resp, err := c.httpClient.Do(req)
if err != nil {
    // Handle error
}

// After: Automatic retry with configuration
c.httpClient = retry.NewHTTPClientWithRetry(retryConfig, logger)
resp, err := c.httpClient.Do(req)  // Retries happen automatically
```

## Best Practices

1. **Always Use Context**: Pass a context to respect cancellation
2. **Set Reasonable Limits**: Don't retry indefinitely
3. **Use Jitter**: Prevent thundering herd in production
4. **Log Appropriately**: Use structured logging for debugging
5. **Fail Fast on Permanent Errors**: Don't retry 4xx client errors
6. **Monitor Retry Metrics**: Track retry rates and success rates
7. **Test Retry Logic**: Include retry scenarios in your tests

## Examples

### Database Connection with Retry

```go
func connectToDatabase(ctx context.Context) (*sql.DB, error) {
    cfg := retry.CriticalConfig()  // More aggressive retries for critical operations
    
    var db *sql.DB
    err := retry.WithExponentialBackoff(ctx, cfg, logger, "db_connect", func() error {
        var connErr error
        db, connErr = sql.Open("postgres", dsn)
        if connErr != nil {
            return connErr
        }
        return db.Ping()
    })
    
    return db, err
}
```

### API Client with Retry

```go
type APIClient struct {
    client *http.Client
    logger *logger.Logger
}

func NewAPIClient(logger *logger.Logger) *APIClient {
    config := retry.HTTPConfig{
        ConfigWithJitter:     retry.DefaultConfigWithJitter(),
        RetryableStatusCodes: []int{429, 500, 502, 503, 504},
        RespectRetryAfter:    true,
        MaxRetryAfter:        60 * time.Second,
    }
    
    return &APIClient{
        client: retry.NewHTTPClientWithRetry(config, logger),
        logger: logger,
    }
}
```

### Conditional Retry

```go
// Only retry if the error indicates a temporary condition
predicate := retry.RetryPredicate(func(err error) bool {
    // Don't retry on context cancellation
    if errors.Is(err, context.Canceled) {
        return false
    }
    
    // Check for temporary error interface
    var tempErr interface{ Temporary() bool }
    if errors.As(err, &tempErr) {
        return tempErr.Temporary()
    }
    
    // Default to retry
    return true
})

err := retry.WithExponentialBackoffJitterPredicate(
    ctx, cfg, logger, "conditional_op", predicate, operation,
)
```

## Testing

The package includes comprehensive tests demonstrating various retry scenarios:

```bash
go test ./internal/pkg/retry -v
```

Key test scenarios:
- Success on various attempt numbers
- Context cancellation handling
- Delay calculation verification
- Jitter randomization
- HTTP status code handling
- Retry-After header parsing
- Concurrent retry operations

## Performance Considerations

- **Goroutine Safety**: All retry functions are safe for concurrent use
- **Memory Usage**: Request bodies are buffered for HTTP retries
- **CPU Usage**: Jitter calculation uses minimal CPU
- **Network Load**: Exponential backoff reduces load on failing services

## Troubleshooting

### Retries Not Happening

1. Check if the error matches your retry predicate
2. Verify context isn't cancelled
3. Ensure MaxAttempts > 1
4. Check logs for retry decisions

### Too Many Retries

1. Reduce MaxAttempts
2. Implement custom predicates for specific errors
3. Use shorter MaxDelay for faster failure

### Thundering Herd

1. Increase JitterFactor (up to 0.5 for 50% randomization)
2. Use longer BaseDelay
3. Stagger initial requests

## Migration Guide

### From Manual Retry Logic

Before:
```go
for i := 0; i < 3; i++ {
    err := doOperation()
    if err == nil {
        return nil
    }
    time.Sleep(time.Duration(i+1) * time.Second)
}
```

After:
```go
return retry.WithExponentialBackoff(ctx, retry.DefaultConfig(), logger, "operation", doOperation)
```

### From Other Retry Libraries

Most retry libraries have similar concepts. Map your existing configuration:
- Max attempts → MaxAttempts
- Initial delay → BaseDelay
- Max delay → MaxDelay
- Backoff multiplier → Fixed at 2 (exponential)
- Jitter → JitterFactor