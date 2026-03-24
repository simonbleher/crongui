package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Run represents a single job execution.
type Run struct {
	ID         int64  `json:"id"`
	JobID      string `json:"job_id"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	DurationMs int64  `json:"duration_ms"`
	Trigger    string `json:"trigger"`
}

// Stats for the dashboard.
type Stats struct {
	TotalJobs  int `json:"total_jobs"`
	EnabledJobs int `json:"enabled_jobs"`
	TotalRuns  int `json:"total_runs"`
	Success24h int `json:"success_24h"`
	Failed24h  int `json:"failed_24h"`
}

func dbPath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".config", "crongui")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "runs.db")
}

func openDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath())
	if err != nil {
		return nil, err
	}
	db.Exec("PRAGMA journal_mode=WAL")

	db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL,
			status TEXT NOT NULL,
			exit_code INTEGER DEFAULT 0,
			stdout TEXT DEFAULT '',
			stderr TEXT DEFAULT '',
			started_at TEXT NOT NULL,
			finished_at TEXT DEFAULT '',
			duration_ms INTEGER DEFAULT 0,
			trigger_type TEXT DEFAULT 'schedule'
		);
		CREATE INDEX IF NOT EXISTS idx_runs_job ON runs(job_id);
		CREATE INDEX IF NOT EXISTS idx_runs_time ON runs(started_at);
	`)
	return db, nil
}

func insertRun(r *Run) error {
	db, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	res, err := db.Exec(
		`INSERT INTO runs (job_id, status, exit_code, stdout, stderr, started_at, finished_at, duration_ms, trigger_type)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.JobID, r.Status, r.ExitCode, r.Stdout, r.Stderr,
		r.StartedAt, r.FinishedAt, r.DurationMs, r.Trigger,
	)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

func getRuns(jobID string, limit int) ([]Run, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var rows *sql.Rows
	if jobID != "" {
		rows, err = db.Query(
			"SELECT id, job_id, status, exit_code, stdout, stderr, started_at, finished_at, duration_ms, trigger_type FROM runs WHERE job_id = ? ORDER BY id DESC LIMIT ?",
			jobID, limit,
		)
	} else {
		rows, err = db.Query(
			"SELECT id, job_id, status, exit_code, stdout, stderr, started_at, finished_at, duration_ms, trigger_type FROM runs ORDER BY id DESC LIMIT ?",
			limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		var r Run
		rows.Scan(&r.ID, &r.JobID, &r.Status, &r.ExitCode, &r.Stdout, &r.Stderr,
			&r.StartedAt, &r.FinishedAt, &r.DurationMs, &r.Trigger)
		runs = append(runs, r)
	}
	return runs, nil
}

func getStats(totalJobs, enabledJobs int) Stats {
	s := Stats{TotalJobs: totalJobs, EnabledJobs: enabledJobs}

	db, err := openDB()
	if err != nil {
		return s
	}
	defer db.Close()

	db.QueryRow("SELECT COUNT(*) FROM runs").Scan(&s.TotalRuns)

	since := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	db.QueryRow("SELECT COUNT(*) FROM runs WHERE status = 'success' AND started_at > ?", since).Scan(&s.Success24h)
	db.QueryRow("SELECT COUNT(*) FROM runs WHERE status != 'success' AND status != 'running' AND started_at > ?", since).Scan(&s.Failed24h)

	return s
}
