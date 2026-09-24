package db_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go-harness/pkg/db"
)

func TestCronPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cron-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	now := time.Now().UTC().Truncate(time.Second)
	next := now.Add(10 * time.Minute)

	job := db.CronJob{
		ID:             "cron-test-1",
		Name:           "AMD Stock Check",
		Schedule:       "*/10 * * * *",
		Intent:         "/check-stock-price AMD",
		ConversationID: "conv-cron-1",
		Enabled:        true,
		NextRun:        &next,
		LastStatus:     "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 1. Create Job
	err = database.CreateCronJob(job)
	if err != nil {
		t.Fatalf("failed to create cron job: %v", err)
	}

	// 2. Get Job by ID
	fetched, err := database.GetCronJob("cron-test-1")
	if err != nil {
		t.Fatalf("failed to get cron job: %v", err)
	}
	if fetched.Name != job.Name || fetched.Schedule != job.Schedule || fetched.Intent != job.Intent {
		t.Fatalf("job data mismatch: %+v vs %+v", fetched, job)
	}

	// 3. List Jobs
	list, err := database.ListCronJobs()
	if err != nil {
		t.Fatalf("failed to list cron jobs: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 job, got %d", len(list))
	}

	// 4. Update status / last run
	runTime := time.Now().UTC().Truncate(time.Second)
	err = database.UpdateCronJobRun("cron-test-1", &runTime, &next, "success", "")
	if err != nil {
		t.Fatalf("failed to update cron job run: %v", err)
	}

	updated, err := database.GetCronJob("cron-test-1")
	if err != nil {
		t.Fatalf("failed to get updated job: %v", err)
	}
	if updated.LastStatus != "success" || updated.LastRun == nil {
		t.Fatalf("expected success status and non-nil last run, got %+v", updated)
	}

	// 5. Toggle enabled
	err = database.SetCronJobEnabled("cron-test-1", false)
	if err != nil {
		t.Fatalf("failed to toggle enabled: %v", err)
	}
	toggled, err := database.GetCronJob("cron-test-1")
	if err != nil {
		t.Fatalf("failed to get toggled job: %v", err)
	}
	if toggled.Enabled {
		t.Fatalf("expected enabled to be false, got true")
	}

	// 6. Delete
	err = database.DeleteCronJob("cron-test-1")
	if err != nil {
		t.Fatalf("failed to delete cron job: %v", err)
	}
	_, err = database.GetCronJob("cron-test-1")
	if err == nil {
		t.Fatalf("expected error getting deleted job, got nil")
	}
}
