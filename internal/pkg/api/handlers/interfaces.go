package handlers

import "context"

// SchedulingService interface for scheduling operations
type SchedulingService interface {
	ProcessScheduledRun(ctx context.Context) (int, error)
}