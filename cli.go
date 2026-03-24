package main

import (
	"fmt"
	"os"
	"text/tabwriter"
)

func listJobsCLI() {
	jobs, _, err := parseCrontab()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(jobs) == 0 {
		fmt.Println("No crongui jobs. Use 'crongui serve' to manage jobs via web UI.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSCHEDULE\tENABLED\tCOMMAND")
	for _, j := range jobs {
		status := "yes"
		if !j.Enabled {
			status = "no"
		}
		cmd := j.Command
		if len(cmd) > 50 {
			cmd = cmd[:50] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", j.ID, j.Name, j.Schedule, status, cmd)
	}
	w.Flush()
}

func showLogsCLI(jobID string) {
	runs, err := getRuns(jobID, 20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(runs) == 0 {
		fmt.Println("No runs recorded yet.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tJOB\tSTATUS\tEXIT\tDURATION\tTIME")
	for _, r := range runs {
		dur := fmt.Sprintf("%dms", r.DurationMs)
		if r.DurationMs > 1000 {
			dur = fmt.Sprintf("%.1fs", float64(r.DurationMs)/1000)
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\t%s\n",
			r.ID, r.JobID, r.Status, r.ExitCode, dur, r.StartedAt)
	}
	w.Flush()
}
