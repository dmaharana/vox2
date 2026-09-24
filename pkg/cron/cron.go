package cron

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-harness/pkg/db"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// RunnerFunc represents the execution handler called when a cron job ticks.
type RunnerFunc func(ctx context.Context, job db.CronJob) error

// Manager orchestrates parsing, scheduling, persistence, and execution of cron jobs.
type Manager struct {
	mu         sync.RWMutex
	db         *db.DB
	cron       *cron.Cron
	parser     cron.Parser
	runner     RunnerFunc
	jobEntries map[string]cron.EntryID
	running    sync.Map
}

// NewManager creates an instance of the cron manager.
func NewManager(database *db.DB, runner RunnerFunc) *Manager {
	parser := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)

	c := cron.New(cron.WithParser(parser))

	return &Manager{
		db:         database,
		cron:       c,
		parser:     parser,
		runner:     runner,
		jobEntries: make(map[string]cron.EntryID),
	}
}

// Start begins scheduling ticks in the background.
func (m *Manager) Start() {
	m.cron.Start()
	log.Info().Msg("Cron scheduler manager started")
}

// Stop gracefully halts the scheduler.
func (m *Manager) Stop() {
	ctx := m.cron.Stop()
	<-ctx.Done()
	log.Info().Msg("Cron scheduler manager stopped")
}

// ValidateSchedule verifies if the schedule expression is valid.
func (m *Manager) ValidateSchedule(schedule string) error {
	schedule = strings.TrimSpace(schedule)
	if schedule == "" {
		return fmt.Errorf("schedule expression cannot be empty")
	}
	_, err := m.parser.Parse(schedule)
	if err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", schedule, err)
	}
	return nil
}

// CalculateNextRun computes the next trigger time from the current moment.
func (m *Manager) CalculateNextRun(schedule string) (*time.Time, error) {
	sched, err := m.parser.Parse(strings.TrimSpace(schedule))
	if err != nil {
		return nil, err
	}
	next := sched.Next(time.Now().UTC())
	return &next, nil
}

// LoadJobs reloads and schedules all active jobs from the database.
func (m *Manager) LoadJobs(ctx context.Context) error {
	if m.db == nil {
		return nil
	}

	jobs, err := m.db.ListCronJobs()
	if err != nil {
		return fmt.Errorf("failed to load cron jobs from database: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing entries
	for _, entryID := range m.jobEntries {
		m.cron.Remove(entryID)
	}
	m.jobEntries = make(map[string]cron.EntryID)

	for _, job := range jobs {
		if job.LastStatus == "running" {
			_ = m.db.UpdateCronJobRun(job.ID, job.LastRun, job.NextRun, "pending", "Interrupted by system restart")
			job.LastStatus = "pending"
		}
		if !job.Enabled {
			continue
		}
		m.scheduleJobLocked(job)
	}

	log.Info().Int("scheduled_count", len(m.jobEntries)).Msg("Restored cron jobs from SQLite")
	return nil
}

func (m *Manager) scheduleJobLocked(job db.CronJob) {
	jobID := job.ID
	entryID, err := m.cron.AddFunc(job.Schedule, func() {
		_ = m.ExecuteJob(context.Background(), jobID)
	})
	if err != nil {
		log.Error().Err(err).Str("job_id", jobID).Str("schedule", job.Schedule).Msg("Failed to schedule job in cron")
		return
	}
	m.jobEntries[jobID] = entryID
}

// AddJob registers a new cron job, persists it to the database, and schedules it.
func (m *Manager) AddJob(ctx context.Context, name, schedule, intent, convID string) (*db.CronJob, error) {
	schedule = strings.TrimSpace(schedule)
	if err := m.ValidateSchedule(schedule); err != nil {
		return nil, err
	}

	intent = strings.TrimSpace(intent)
	if intent == "" {
		return nil, fmt.Errorf("intent cannot be empty")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = intent
	}

	nextRun, err := m.CalculateNextRun(schedule)
	if err != nil {
		return nil, err
	}

	jobID := "cron-" + uuid.New().String()[:8]
	if convID == "" {
		convTitle := fmt.Sprintf("[Cron] %s", name)
		if m.db != nil {
			if conv, err := m.db.CreateConversation(convTitle); err == nil && conv != nil {
				convID = conv.ID
			}
		}
		if convID == "" {
			convID = fmt.Sprintf("cron-%s", jobID)
		}
	}

	job := db.CronJob{
		ID:             jobID,
		Name:           name,
		Schedule:       schedule,
		Intent:         intent,
		ConversationID: convID,
		Enabled:        true,
		NextRun:        nextRun,
		LastStatus:     "pending",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if m.db != nil {
		if err := m.db.CreateCronJob(job); err != nil {
			return nil, err
		}
	}

	m.mu.Lock()
	m.scheduleJobLocked(job)
	m.mu.Unlock()

	return &job, nil
}

// GetJob returns a cron job by ID.
func (m *Manager) GetJob(id string) (*db.CronJob, error) {
	if m.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return m.db.GetCronJob(id)
}

// FindJobByNameOrIndex resolves a job by ID, alias name, or 1-based index (#1, #2...).
func (m *Manager) FindJobByNameOrIndex(query string) (*db.CronJob, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("job query is empty")
	}

	// 1. Try direct ID match
	if job, err := m.GetJob(query); err == nil && job != nil {
		return job, nil
	}

	// 2. Try index match (#1, 1, #2, etc.)
	cleanIndex := strings.TrimPrefix(query, "#")
	if idx, err := strconv.Atoi(cleanIndex); err == nil && idx > 0 {
		jobs, err := m.ListJobs()
		if err == nil && idx <= len(jobs) {
			return &jobs[idx-1], nil
		}
	}

	// 3. Try name match
	if m.db != nil {
		if job, err := m.db.FindCronJobByName(query); err == nil && job != nil {
			return job, nil
		}
	}

	return nil, fmt.Errorf("job %q not found", query)
}

// ListJobs retrieves all scheduled jobs from persistence.
func (m *Manager) ListJobs() ([]db.CronJob, error) {
	if m.db == nil {
		return nil, nil
	}
	return m.db.ListCronJobs()
}

// SetJobEnabled toggles a job between active and paused.
func (m *Manager) SetJobEnabled(ctx context.Context, id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if err := m.db.SetCronJobEnabled(id, enabled); err != nil {
		return err
	}

	// Update in-memory cron
	if entryID, exists := m.jobEntries[id]; exists {
		m.cron.Remove(entryID)
		delete(m.jobEntries, id)
	}

	job, err := m.db.GetCronJob(id)
	if err != nil {
		return err
	}

	if enabled {
		m.scheduleJobLocked(*job)
	}

	return nil
}

// DeleteJob deletes a job from DB and scheduler.
func (m *Manager) DeleteJob(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if entryID, exists := m.jobEntries[id]; exists {
		m.cron.Remove(entryID)
		delete(m.jobEntries, id)
	}

	return m.db.DeleteCronJob(id)
}

// TriggerNow executes a job immediately in a background goroutine.
func (m *Manager) TriggerNow(ctx context.Context, id string) error {
	job, err := m.GetJob(id)
	if err != nil {
		return err
	}

	go func() {
		_ = m.ExecuteJob(context.Background(), job.ID)
	}()

	return nil
}

// ExecuteJob runs a single execution turn of the job, protected by the Single-Flight Guard.
func (m *Manager) ExecuteJob(ctx context.Context, id string) error {
	job, err := m.GetJob(id)
	if err != nil {
		return err
	}

	// Single-Flight Guard: skip if already executing
	if _, loaded := m.running.LoadOrStore(id, true); loaded {
		log.Warn().Str("job_id", id).Str("name", job.Name).Msg("Skipping cron tick: previous execution still running (single-flight)")
		if m.db != nil {
			next, _ := m.CalculateNextRun(job.Schedule)
			_ = m.db.UpdateCronJobRun(id, job.LastRun, next, "skipped", "Skipped tick: previous run still in progress")
		}
		return nil
	}
	defer m.running.Delete(id)

	now := time.Now().UTC()
	next, _ := m.CalculateNextRun(job.Schedule)

	if m.db != nil {
		_ = m.db.UpdateCronJobRun(id, &now, next, "running", "")
	}

	var runErr error
	if m.runner != nil {
		runErr = m.runner(ctx, *job)
	}

	finishTime := time.Now().UTC()
	status := "success"
	errMsg := ""

	if runErr != nil {
		status = "error"
		errMsg = runErr.Error()
		log.Error().Err(runErr).Str("job_id", id).Str("name", job.Name).Msg("Cron job execution failed")
	} else {
		log.Info().Str("job_id", id).Str("name", job.Name).Msg("Cron job executed successfully")
	}

	if m.db != nil {
		_ = m.db.UpdateCronJobRun(id, &finishTime, next, status, errMsg)
	}

	return runErr
}
