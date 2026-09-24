package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CronJob represents a scheduled task specification stored in SQLite.
type CronJob struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Schedule       string     `json:"schedule"`
	Intent         string     `json:"intent"`
	ConversationID string     `json:"conversation_id"`
	Enabled        bool       `json:"enabled"`
	LastRun        *time.Time `json:"last_run,omitempty"`
	NextRun        *time.Time `json:"next_run,omitempty"`
	LastStatus     string     `json:"last_status"` // "pending", "running", "success", "error", "skipped"
	LastError      string     `json:"last_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CreateCronJob inserts a new cron job into the database.
func (d *DB) CreateCronJob(job CronJob) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `
	INSERT INTO cron_jobs (
		id, name, schedule, intent, conversation_id, enabled,
		last_run, next_run, last_status, last_error, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`

	var lastRunStr *string
	if job.LastRun != nil {
		s := job.LastRun.Format(time.RFC3339)
		lastRunStr = &s
	}

	var nextRunStr *string
	if job.NextRun != nil {
		s := job.NextRun.Format(time.RFC3339)
		nextRunStr = &s
	}

	enabledInt := 0
	if job.Enabled {
		enabledInt = 1
	}

	if job.LastStatus == "" {
		job.LastStatus = "pending"
	}

	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = now
	}

	_, err := d.db.Exec(query,
		job.ID,
		job.Name,
		job.Schedule,
		job.Intent,
		job.ConversationID,
		enabledInt,
		lastRunStr,
		nextRunStr,
		job.LastStatus,
		job.LastError,
		job.CreatedAt.Format(time.RFC3339),
		job.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to insert cron job: %w", err)
	}

	return nil
}

// GetCronJob fetches a single cron job by ID.
func (d *DB) GetCronJob(id string) (*CronJob, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
	SELECT id, name, schedule, intent, conversation_id, enabled,
	       last_run, next_run, last_status, last_error, created_at, updated_at
	FROM cron_jobs
	WHERE id = ?;
	`

	row := d.db.QueryRow(query, id)
	return scanCronJob(row)
}

// FindCronJobByName fetches a cron job by exact or case-insensitive name.
func (d *DB) FindCronJobByName(name string) (*CronJob, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
	SELECT id, name, schedule, intent, conversation_id, enabled,
	       last_run, next_run, last_status, last_error, created_at, updated_at
	FROM cron_jobs
	WHERE LOWER(name) = LOWER(?)
	LIMIT 1;
	`

	row := d.db.QueryRow(query, name)
	return scanCronJob(row)
}

// ListCronJobs retrieves all cron jobs ordered by created_at.
func (d *DB) ListCronJobs() ([]CronJob, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
	SELECT id, name, schedule, intent, conversation_id, enabled,
	       last_run, next_run, last_status, last_error, created_at, updated_at
	FROM cron_jobs
	ORDER BY created_at ASC;
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query cron jobs: %w", err)
	}
	defer rows.Close()

	var jobs []CronJob
	for rows.Next() {
		job, err := scanCronJobFromRows(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}

	return jobs, nil
}

// SetCronJobEnabled toggles a cron job's active state.
func (d *DB) SetCronJobEnabled(id string, enabled bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	query := `
	UPDATE cron_jobs
	SET enabled = ?, updated_at = ?
	WHERE id = ?;
	`

	res, err := d.db.Exec(query, enabledInt, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("failed to toggle cron job: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("cron job %q not found", id)
	}

	return nil
}

// UpdateCronJobRun updates the execution metrics (last_run, next_run, status, error).
func (d *DB) UpdateCronJobRun(id string, lastRun, nextRun *time.Time, status, lastErr string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	var lastRunStr *string
	if lastRun != nil {
		s := lastRun.Format(time.RFC3339)
		lastRunStr = &s
	}

	var nextRunStr *string
	if nextRun != nil {
		s := nextRun.Format(time.RFC3339)
		nextRunStr = &s
	}

	query := `
	UPDATE cron_jobs
	SET last_run = ?, next_run = ?, last_status = ?, last_error = ?, updated_at = ?
	WHERE id = ?;
	`

	res, err := d.db.Exec(query, lastRunStr, nextRunStr, status, lastErr, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("failed to update cron job run: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("cron job %q not found", id)
	}

	return nil
}

// DeleteCronJob removes a cron job from the database.
func (d *DB) DeleteCronJob(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `DELETE FROM cron_jobs WHERE id = ?;`
	res, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete cron job: %w", err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("cron job %q not found", id)
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCronJob(row *sql.Row) (*CronJob, error) {
	return parseCronJobRow(row)
}

func scanCronJobFromRows(rows *sql.Rows) (*CronJob, error) {
	return parseCronJobRow(rows)
}

func parseCronJobRow(scanner rowScanner) (*CronJob, error) {
	var job CronJob
	var enabledInt int
	var lastRunStr, nextRunStr sql.NullString
	var createdAtStr, updatedAtStr string

	err := scanner.Scan(
		&job.ID,
		&job.Name,
		&job.Schedule,
		&job.Intent,
		&job.ConversationID,
		&enabledInt,
		&lastRunStr,
		&nextRunStr,
		&job.LastStatus,
		&job.LastError,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	job.Enabled = enabledInt == 1

	if lastRunStr.Valid && lastRunStr.String != "" {
		if t, err := time.Parse(time.RFC3339, lastRunStr.String); err == nil {
			job.LastRun = &t
		}
	}

	if nextRunStr.Valid && nextRunStr.String != "" {
		if t, err := time.Parse(time.RFC3339, nextRunStr.String); err == nil {
			job.NextRun = &t
		}
	}

	if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		job.CreatedAt = t
	}

	if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
		job.UpdatedAt = t
	}

	return &job, nil
}
