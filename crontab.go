package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

const marker = "# crongui:"

// Job represents a cron job managed by crongui.
type Job struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Enabled  bool   `json:"enabled"`
}

// parseCrontab reads the current user's crontab and extracts crongui-managed jobs.
func parseCrontab() ([]Job, []string, error) {
	out, err := exec.Command("crontab", "-l").CombinedOutput()
	if err != nil {
		// No crontab = empty
		if strings.Contains(string(out), "no crontab") {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("crontab -l: %s", string(out))
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var jobs []Job
	var other []string // non-crongui lines

	idRe := regexp.MustCompile(`^# crongui:(\S+)\s+(.*)$`)

	i := 0
	for i < len(lines) {
		m := idRe.FindStringSubmatch(lines[i])
		if m != nil && i+1 < len(lines) {
			id := m[1]
			name := m[2]
			cronLine := lines[i+1]

			enabled := true
			if strings.HasPrefix(cronLine, "#DISABLED# ") {
				cronLine = strings.TrimPrefix(cronLine, "#DISABLED# ")
				enabled = false
			}

			// Parse: schedule (5 fields) + command
			schedule, command := splitCronLine(cronLine)

			// Strip wrapper if present
			command = stripWrapper(command)

			jobs = append(jobs, Job{
				ID:       id,
				Name:     name,
				Schedule: schedule,
				Command:  command,
				Enabled:  enabled,
			})
			i += 2
		} else {
			other = append(other, lines[i])
			i++
		}
	}

	return jobs, other, nil
}

// writeCrontab writes jobs back to crontab, preserving non-crongui lines.
func writeCrontab(jobs []Job, other []string) error {
	var lines []string

	// Keep non-crongui lines first
	for _, l := range other {
		if l != "" {
			lines = append(lines, l)
		}
	}

	// Add crongui jobs
	for _, j := range jobs {
		lines = append(lines, fmt.Sprintf("%s%s %s", marker, j.ID, j.Name))

		// Build cron line with wrapper
		selfPath, _ := exec.LookPath("crongui")
		if selfPath == "" {
			selfPath = "crongui"
		}
		cronLine := fmt.Sprintf("%s %s wrap %s %s", j.Schedule, selfPath, j.ID, j.Command)

		if !j.Enabled {
			cronLine = "#DISABLED# " + cronLine
		}

		lines = append(lines, cronLine)
	}

	content := strings.Join(lines, "\n") + "\n"

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab write: %s", string(out))
	}
	return nil
}

// splitCronLine splits "*/5 * * * * echo hello" into schedule and command.
func splitCronLine(line string) (string, string) {
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return line, ""
	}
	schedule := strings.Join(fields[:5], " ")
	command := strings.Join(fields[5:], " ")
	return schedule, command
}

// stripWrapper removes "crongui wrap <id>" prefix from command.
func stripWrapper(cmd string) string {
	re := regexp.MustCompile(`^(?:\S+/)?crongui\s+wrap\s+\S+\s+`)
	return re.ReplaceAllString(cmd, "")
}

// findJob returns the job with the given ID.
func findJob(jobs []Job, id string) *Job {
	for i := range jobs {
		if jobs[i].ID == id {
			return &jobs[i]
		}
	}
	return nil
}

// generateID creates a short unique ID.
func generateID() string {
	b := make([]byte, 6)
	f, _ := exec.Command("head", "-c", "6", "/dev/urandom").Output()
	if len(f) >= 6 {
		b = f
	}
	return fmt.Sprintf("%x", b)
}
