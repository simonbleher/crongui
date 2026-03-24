package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("crongui - visual crontab manager")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  crongui serve [--port 7272]    Start web dashboard")
		fmt.Println("  crongui wrap <id> <cmd...>     Execute and log a job (used by cron)")
		fmt.Println("  crongui list                   List managed jobs")
		fmt.Println("  crongui logs [id]              Show recent run history")
		os.Exit(0)
	}

	switch os.Args[1] {
	case "serve":
		port := "7272"
		for i, arg := range os.Args {
			if (arg == "--port" || arg == "-p") && i+1 < len(os.Args) {
				port = os.Args[i+1]
			}
		}
		startServer(port)

	case "wrap":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: crongui wrap <job-id> <command...>")
			os.Exit(1)
		}
		jobID := os.Args[2]
		cmdArgs := os.Args[3:]
		os.Exit(wrapCommand(jobID, cmdArgs))

	case "list":
		listJobsCLI()

	case "logs":
		jobID := ""
		if len(os.Args) > 2 {
			jobID = os.Args[2]
		}
		showLogsCLI(jobID)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
