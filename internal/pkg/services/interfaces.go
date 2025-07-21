package services

import (
	"context"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/database"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/queue"
)

// UserRepository interface for user data access
type UserRepository interface {
	GetUserByID(ctx context.Context, userID int) (*database.User, error)
	GetUsersInProcessingWindow(ctx context.Context) ([]int, error)
	SetGoogleReauthRequired(ctx context.Context, userID int, required bool) error
	SetStravaReauthRequired(ctx context.Context, userID int, required bool) error
}

// QueueClient interface for queue operations
type QueueClient interface {
	EnqueueJob(ctx context.Context, jobType queue.JobType, userID int, data map[string]interface{}) (*queue.Job, error)
	AcquireUserProcessingLock(ctx context.Context, userID int, duration time.Duration) (bool, error)
	ReleaseUserProcessingLock(ctx context.Context, userID int) error
}