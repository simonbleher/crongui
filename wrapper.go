package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// wrapCommand executes a command, captures output, and logs the run.
// This is called by cron via: crongui wrap <job-id> <command...>
func wrapCommand(jobID string, args []string) int {
	started := time.Now().UTC()

	// Build command
	cmdStr := strings.Join(args, " ")
	cmd := exec.Command("sh", "-c", cmdStr)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin

	// Run
	err := cmd.Run()
	finished := time.Now().UTC()
	duration := finished.Sub(started).Milliseconds()

	exitCode := 0
	status := "success"
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
		status = "failed"
	}

	// Truncate output to 50KB
	stdoutStr := truncate(stdout.String(), 50000)
	stderrStr := truncate(stderr.String(), 50000)

	// Also write to actual stdout/stderr for cron's MAILTO
	os.Stdout.WriteString(stdoutStr)
	os.Stderr.WriteString(stderrStr)

	// Log to database
	run := &Run{
		JobID:      jobID,
		Status:     status,
		ExitCode:   exitCode,
		Stdout:     stdoutStr,
		Stderr:     stderrStr,
		StartedAt:  started.Format(time.RFC3339),
		FinishedAt: finished.Format(time.RFC3339),
		DurationMs: duration,
		Trigger:    "schedule",
	}

	if err := insertRun(run); err != nil {
		fmt.Fprintf(os.Stderr, "crongui: failed to log run: %v\n", err)
	}

	return exitCode
}

func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "\n... (truncated)"
	}
	return s
}
