package workflow

import (
	"context"
	"database/sql"
	"time"
)

type QueueMetrics struct {
	DueTimers    int64
	LeasedActors  int64
	NextLeaseExpiry time.Time
}

func CollectMetrics(ctx context.Context, db *sql.DB, now time.Time) (QueueMetrics, error) {
	if db == nil {
		return QueueMetrics{}, nil
	}
	var out QueueMetrics
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM _kora_workflow_timer
WHERE wake_at <= ?`, now.UTC()).Scan(&out.DueTimers); err != nil {
		return QueueMetrics{}, err
	}
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(1)
FROM _kora_workflow_actor
WHERE lease_owner <> '' AND lease_until IS NOT NULL AND lease_until >= ?`, now.UTC()).Scan(&out.LeasedActors); err != nil {
		return QueueMetrics{}, err
	}
	var nextExpiry sql.NullTime
	if err := db.QueryRowContext(ctx, `
SELECT lease_until
FROM _kora_workflow_actor
WHERE lease_owner <> '' AND lease_until IS NOT NULL
ORDER BY lease_until ASC
LIMIT 1`).Scan(&nextExpiry); err != nil {
		if err == sql.ErrNoRows {
			return out, nil
		}
		return QueueMetrics{}, err
	}
	if nextExpiry.Valid {
		out.NextLeaseExpiry = nextExpiry.Time
	}
	return out, nil
}
