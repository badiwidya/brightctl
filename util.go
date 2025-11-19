package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage:")
	fmt.Fprintf(os.Stderr, "	%s set 50%%\n", appName)
	fmt.Fprintf(os.Stderr, "	%s set +5%%\n", appName)
	fmt.Fprintf(os.Stderr, "	%s set -5%%\n", appName)
	fmt.Fprintf(os.Stderr, "	%s get\n", appName)
	fmt.Fprintf(os.Stderr, "	%s restore (to use within a startup script)\n", appName)
}

func getStateDir() (stateDir string) {
	stateDir = os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			stateDir = filepath.Join(homeDir, ".local", "state")
		}
	}

	// Let empty if state directory cannot get retrieved
	return stateDir
}
