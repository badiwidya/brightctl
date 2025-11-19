package main

import (
	"fmt"
	"os"

	"github.com/badiwidya/brightctl/backlight"
)

const appName = "brightctl"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	const baseBacklightDir = "/sys/class/backlight"

	if len(args) < 1 {
		printUsage()
		return fmt.Errorf("error: no command specified")
	}

	stateDir := getStateDir()

	bl, err := backlight.New(baseBacklightDir)
	if err != nil {
		return err
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "set":
		// USAGE: brightctl set ARG
		if len(cmdArgs) != 1 {
			printUsage()
			return fmt.Errorf("error: 'set' requires exactly one argument")
		}

		err = bl.Set(cmdArgs[0])
		if err != nil {
			return err
		}

		err = bl.Write(baseBacklightDir)
		if err != nil {
			return err
		}

		err = bl.SaveState(stateDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save state: %s\n", err)
		}

		fmt.Fprintln(os.Stdout, "Brightness changed")
	case "get":
		// USAGE: brightctl get
		if len(cmdArgs) != 0 {
			printUsage()
			return fmt.Errorf("error: 'get' takes no argument")
		}

		fmt.Fprintln(os.Stdout, bl.GetPercentage())
	case "restore":
		// USAGE: brightctl restore
		if len(cmdArgs) != 0 {
			printUsage()
			return fmt.Errorf("error: 'restore' takes no argument")
		}

		err := bl.Restore(baseBacklightDir, stateDir)
		if err != nil {
			return err
		}
	default:
		printUsage()
	}

	return nil
}
