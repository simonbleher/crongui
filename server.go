package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed static/*
var staticFS embed.FS

func startServer(port string) {
	// API routes
	http.HandleFunc("/api/stats", handleStats)
	http.HandleFunc("/api/jobs", handleJobs)
	http.HandleFunc("/api/jobs/", handleJobByID)
	http.HandleFunc("/api/runs", handleAllRuns)

	// Static files
	sub, _ := fs.Sub(staticFS, "static")
	http.Handle("/", http.FileServer(http.FS(sub)))

	fmt.Printf("\n  crongui running at http://localhost:%s\n\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// GET /api/stats
func handleStats(w http.ResponseWriter, r *http.Request) {
	jobs, _, _ := parseCrontab()
	total := len(jobs)
	enabled := 0
	for _, j := range jobs {
		if j.Enabled {
			enabled++
		}
	}
	jsonResponse(w, getStats(total, enabled))
}

// GET/POST /api/jobs
func handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		jobs, _, err := parseCrontab()
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		if jobs == nil {
			jobs = []Job{}
		}

		// Attach last run
		type JobWithRun struct {
			Job
			LastRun *Run `json:"last_run"`
		}
		var result []JobWithRun
		for _, j := range jobs {
			jwr := JobWithRun{Job: j}
			runs, _ := getRuns(j.ID, 1)
			if len(runs) > 0 {
				jwr.LastRun = &runs[0]
			}
			result = append(result, jwr)
		}
		if result == nil {
			result = []JobWithRun{}
		}
		jsonResponse(w, result)

	case "POST":
		var req struct {
			Name            string `json:"name"`
			Command         string `json:"command"`
			Schedule        string `json:"schedule"`
			TimeoutSeconds  int    `json:"timeout_seconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", 400)
			return
		}
		if req.Name == "" || req.Command == "" || req.Schedule == "" {
			jsonError(w, "name, command, and schedule are required", 400)
			return
		}

		jobs, other, err := parseCrontab()
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}

		newJob := Job{
			ID:       generateID(),
			Name:     req.Name,
			Schedule: req.Schedule,
			Command:  req.Command,
			Enabled:  true,
		}
		jobs = append(jobs, newJob)

		if err := writeCrontab(jobs, other); err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		w.WriteHeader(201)
		jsonResponse(w, newJob)

	default:
		w.WriteHeader(405)
	}
}

// /api/jobs/{id}, /api/jobs/{id}/run, /api/jobs/{id}/runs
func handleJobByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	parts := strings.SplitN(path, "/", 2)
	jobID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case action == "run" && r.Method == "POST":
		handleRunJob(w, r, jobID)
	case action == "runs" && r.Method == "GET":
		handleJobRuns(w, r, jobID)
	case action == "" && r.Method == "PATCH":
		handleUpdateJob(w, r, jobID)
	case action == "" && r.Method == "DELETE":
		handleDeleteJob(w, r, jobID)
	case action == "" && r.Method == "GET":
		handleGetJob(w, r, jobID)
	default:
		w.WriteHeader(405)
	}
}

func handleGetJob(w http.ResponseWriter, r *http.Request, jobID string) {
	jobs, _, err := parseCrontab()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	j := findJob(jobs, jobID)
	if j == nil {
		jsonError(w, "not found", 404)
		return
	}
	jsonResponse(w, j)
}

func handleUpdateJob(w http.ResponseWriter, r *http.Request, jobID string) {
	var req struct {
		Name     *string `json:"name"`
		Command  *string `json:"command"`
		Schedule *string `json:"schedule"`
		Enabled  *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", 400)
		return
	}

	jobs, other, err := parseCrontab()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	j := findJob(jobs, jobID)
	if j == nil {
		jsonError(w, "not found", 404)
		return
	}

	if req.Name != nil {
		j.Name = *req.Name
	}
	if req.Command != nil {
		j.Command = *req.Command
	}
	if req.Schedule != nil {
		j.Schedule = *req.Schedule
	}
	if req.Enabled != nil {
		j.Enabled = *req.Enabled
	}

	if err := writeCrontab(jobs, other); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, j)
}

func handleDeleteJob(w http.ResponseWriter, r *http.Request, jobID string) {
	jobs, other, err := parseCrontab()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	var filtered []Job
	found := false
	for _, j := range jobs {
		if j.ID == jobID {
			found = true
			continue
		}
		filtered = append(filtered, j)
	}

	if !found {
		jsonError(w, "not found", 404)
		return
	}

	if err := writeCrontab(filtered, other); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResponse(w, map[string]bool{"ok": true})
}

func handleRunJob(w http.ResponseWriter, r *http.Request, jobID string) {
	jobs, _, err := parseCrontab()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	j := findJob(jobs, jobID)
	if j == nil {
		jsonError(w, "not found", 404)
		return
	}

	// Execute synchronously
	started := time.Now().UTC()
	exitCode := wrapCommand(jobID, []string{j.Command})

	// Get the run we just inserted
	runs, _ := getRuns(jobID, 1)
	if len(runs) > 0 {
		// Update trigger to manual
		runs[0].Trigger = "manual"
		_ = started // used in wrapCommand
		jsonResponse(w, runs[0])
	} else {
		jsonResponse(w, map[string]int{"exit_code": exitCode})
	}
}

func handleJobRuns(w http.ResponseWriter, r *http.Request, jobID string) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	runs, err := getRuns(jobID, limit)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	if runs == nil {
		runs = []Run{}
	}
	jsonResponse(w, runs)
}

func handleAllRuns(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 30
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	runs, err := getRuns("", limit)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	if runs == nil {
		runs = []Run{}
	}
	jsonResponse(w, runs)
}
