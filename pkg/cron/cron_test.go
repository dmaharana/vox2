package cron_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-harness/pkg/cron"
	"go-harness/pkg/db"
)

func TestCronManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cron-engine-test-*")
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

	var runCount int32
	runner := func(ctx context.Context, job db.CronJob) error {
		atomic.AddInt32(&runCount, 1)
		return nil
	}

	mgr := cron.NewManager(database, runner)
	mgr.Start()
	defer mgr.Stop()

	// 1. Test schedule validation
	if err := mgr.ValidateSchedule("*/10 * * * *"); err != nil {
		t.Fatalf("expected valid 5-field cron, got %v", err)
	}
	if err := mgr.ValidateSchedule("@every 10m"); err != nil {
		t.Fatalf("expected valid descriptor, got %v", err)
	}
	if err := mgr.ValidateSchedule("invalid schedule string"); err == nil {
		t.Fatalf("expected error for invalid schedule, got nil")
	}

	// 2. Add job
	ctx := context.Background()
	job, err := mgr.AddJob(ctx, "Stock Checker", "*/10 * * * *", "/check-stock-price AMD", "conv-123")
	if err != nil {
		t.Fatalf("failed to add job: %v", err)
	}
	if job.ID == "" || job.Name != "Stock Checker" {
		t.Fatalf("unexpected job result: %+v", job)
	}
	if job.NextRun == nil {
		t.Fatalf("expected calculated next_run, got nil")
	}

	// 3. TriggerNow
	err = mgr.TriggerNow(ctx, job.ID)
	if err != nil {
		t.Fatalf("failed to trigger now: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&runCount) < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if atomic.LoadInt32(&runCount) != 1 {
		t.Fatalf("expected runCount = 1, got %d", atomic.LoadInt32(&runCount))
	}

	// 4. Toggle disabled
	err = mgr.SetJobEnabled(ctx, job.ID, false)
	if err != nil {
		t.Fatalf("failed to disable job: %v", err)
	}
	fetched, err := mgr.GetJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if fetched.Enabled {
		t.Fatalf("expected job to be disabled")
	}

	// 5. Delete job
	err = mgr.DeleteJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("failed to delete job: %v", err)
	}
	jobs, err := mgr.ListJobs()
	if err != nil {
		t.Fatalf("failed to list jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs after deletion, got %d", len(jobs))
	}
}

func TestSingleFlightGuard(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cron-singleflight-test-*")
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

	started := make(chan struct{})
	blocker := make(chan struct{})
	var executions int32

	runner := func(ctx context.Context, job db.CronJob) error {
		atomic.AddInt32(&executions, 1)
		close(started)
		<-blocker
		return nil
	}

	mgr := cron.NewManager(database, runner)
	mgr.Start()
	defer mgr.Stop()

	ctx := context.Background()
	job, err := mgr.AddJob(ctx, "Slow Task", "*/10 * * * *", "/slow", "conv-slow")
	if err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = mgr.ExecuteJob(ctx, job.ID)
	}()

	<-started

	// While first execution is blocked in runner, a second execution should be skipped
	err = mgr.ExecuteJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("expected skipped execution not to error, got: %v", err)
	}

	// Release first execution
	close(blocker)
	wg.Wait()

	if atomic.LoadInt32(&executions) != 1 {
		t.Fatalf("expected exactly 1 execution due to single-flight guard, got %d", atomic.LoadInt32(&executions))
	}
}

func TestLoadJobsResetRunningStatus(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cron-restart-test-*")
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

	mgr := cron.NewManager(database, nil)
	ctx := context.Background()

	job, err := mgr.AddJob(ctx, "Interrupted Job", "*/10 * * * *", "/check", "")
	if err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	// Simulate system crash while job was running
	now := time.Now().UTC()
	err = database.UpdateCronJobRun(job.ID, &now, nil, "running", "")
	if err != nil {
		t.Fatalf("failed to simulate running job: %v", err)
	}

	// Re-load jobs as if harness restarted
	newMgr := cron.NewManager(database, nil)
	if err := newMgr.LoadJobs(ctx); err != nil {
		t.Fatalf("failed to load jobs: %v", err)
	}

	recovered, err := newMgr.GetJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get recovered job: %v", err)
	}
	if recovered.LastStatus != "pending" {
		t.Fatalf("expected recovered job status to be 'pending', got %q", recovered.LastStatus)
	}
	if recovered.LastError == "" {
		t.Fatalf("expected explanatory error message about restart interruption")
	}
}

func TestCronManagerErrorStatus(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cron-err-test-*")
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

	runner := func(ctx context.Context, job db.CronJob) error {
		return fmt.Errorf("llm stream rate limit exceeded")
	}

	mgr := cron.NewManager(database, runner)
	mgr.Start()
	defer mgr.Stop()

	ctx := context.Background()
	job, err := mgr.AddJob(ctx, "Failing Job", "*/10 * * * *", "/fail", "")
	if err != nil {
		t.Fatalf("failed to add job: %v", err)
	}

	err = mgr.ExecuteJob(ctx, job.ID)
	if err == nil {
		t.Fatalf("expected execution error to be returned from ExecuteJob")
	}

	updated, err := mgr.GetJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get updated job: %v", err)
	}
	if updated.LastStatus != "error" {
		t.Fatalf("expected status 'error', got %q", updated.LastStatus)
	}
	if updated.LastError != "llm stream rate limit exceeded" {
		t.Fatalf("expected last_error to be recorded, got %q", updated.LastError)
	}
}

