package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go-harness/pkg/config"
	"go-harness/pkg/cron"
	"go-harness/pkg/db"
	"go-harness/pkg/server"
)

func TestCronRESTEndpoints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cron-server-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	database, _ := db.Open(filepath.Join(tempDir, "test.db"))
	defer database.Close()

	runner := func(ctx context.Context, job db.CronJob) error {
		return nil
	}
	cronMgr := cron.NewManager(database, runner)
	cronMgr.Start()
	defer cronMgr.Stop()

	cfg := &config.Config{
		Host: "localhost",
		Port: 8080,
	}

	srv := server.New(cfg, nil, server.ServerOptions{
		DB:      database,
		CronMgr: cronMgr,
	})
	router := srv.Router()

	// 1. Initial GET /api/cron/jobs -> empty array
	req := httptest.NewRequest("GET", "/api/cron/jobs", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}
	var list []db.CronJob
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(list))
	}

	// 2. POST /api/cron/jobs -> Create Job
	body := map[string]string{
		"name":     "AMD Stock Check",
		"schedule": "*/10 * * * *",
		"intent":   "/check-stock-price AMD",
	}
	b, _ := json.Marshal(body)
	req = httptest.NewRequest("POST", "/api/cron/jobs", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rr.Code, rr.Body.String())
	}
	var created db.CronJob
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	if created.ID == "" || created.Name != "AMD Stock Check" {
		t.Fatalf("unexpected created job: %+v", created)
	}

	// 3. POST /api/cron/jobs/{id}/toggle -> Pause Job
	toggleBody := map[string]bool{"enabled": false}
	tb, _ := json.Marshal(toggleBody)
	req = httptest.NewRequest("POST", "/api/cron/jobs/"+created.ID+"/toggle", bytes.NewReader(tb))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}
	var toggled db.CronJob
	_ = json.Unmarshal(rr.Body.Bytes(), &toggled)
	if toggled.Enabled {
		t.Fatalf("expected job to be disabled")
	}

	// 4. POST /api/cron/jobs/{id}/run -> Trigger Job
	req = httptest.NewRequest("POST", "/api/cron/jobs/"+created.ID+"/run", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	// 5. DELETE /api/cron/jobs/{id}
	req = httptest.NewRequest("DELETE", "/api/cron/jobs/"+created.ID, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	// Verify empty list after delete
	req = httptest.NewRequest("GET", "/api/cron/jobs", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	_ = json.Unmarshal(rr.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("expected 0 jobs after delete, got %d", len(list))
	}
}
