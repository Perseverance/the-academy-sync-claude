package processing

import (
	"context"
	"time"

	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/google"
	"github.com/Perseverance/the-academy-sync-claude/internal/pkg/strava"
)

// StravaClient defines the interface for Strava API operations
type StravaClient interface {
	GetActivities(ctx context.Context, after time.Time) ([]strava.Activity, error)
}

// SheetsClient defines the interface for Google Sheets API operations
type SheetsClient interface {
	ReadRange(ctx context.Context, spreadsheetID, rangeSpec string) ([][]interface{}, error)
	ValidateAccess(ctx context.Context, spreadsheetID string) error
	GetSpreadsheetInfo(ctx context.Context, spreadsheetID string) (*google.SpreadsheetInfo, error)
	WriteActivities(ctx context.Context, spreadsheetID string, activities []strava.Activity) error
	BatchUpdateTrainingPlan(ctx context.Context, spreadsheetID string, updates []*google.SpreadsheetUpdate) error
}