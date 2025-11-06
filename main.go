package main

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	bl := &backlight{}

	err := bl.read(baseBacklightDir)
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

		err = bl.set(cmdArgs[0])
		if err != nil {
			return err
		}

		err = bl.write(baseBacklightDir, stateDir)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, "Brightness changed")
	case "get":
		// USAGE: brightctl get
		if len(cmdArgs) != 0 {
			printUsage()
			return fmt.Errorf("error: 'get' takes no argument")
		}

		fmt.Fprintln(os.Stdout, bl.get())
	case "restore":
		// USAGE: brightctl restore
		if len(cmdArgs) != 0 {
			printUsage()
			return fmt.Errorf("error: 'restore' takes no argument")
		}

		err := bl.restore(baseBacklightDir, stateDir)
		if err != nil {
			return err
		}
	default:
		printUsage()
	}

	return nil
}

type backlight struct {
	devName string
	current int
	max     int
}

func (b *backlight) set(args string) error {
	args = strings.TrimSpace(args)

	var isRelative, isPercent bool

	if strings.HasSuffix(args, "%") {
		isPercent = true
	}

	if strings.HasPrefix(args, "+") || strings.HasPrefix(args, "-") {
		isRelative = true
	}

	valStr := args
	if isPercent {
		valStr = strings.TrimSuffix(valStr, "%")
	}

	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return fmt.Errorf("error: invalid argument, expected a number")
	}

	if isPercent {
		val = val / 100.0
	}

	if !isRelative && val < 0 {
		return fmt.Errorf("error: absolute value cannot be negative")
	}

	if val > 1 || val < -1 {
		return fmt.Errorf("error: value must be between +/- 100%% or +/- 1.0")
	}

	delta := int(val * float64(b.max))

	if isRelative {
		b.current += delta
	} else {
		b.current = delta
	}

	if b.current < 0 {
		b.current = 0
	}

	if b.current > b.max {
		b.current = b.max
	}

	return nil
}

func (b *backlight) get() float64 {
	var valPerc float64
	valPerc = float64(b.current) / float64(b.max)

	valPerc = math.Trunc(valPerc*100) / 100

	return valPerc
}

func (b *backlight) write(baseBacklightDir, stateDir string) error {
	brightnessPath := filepath.Join(baseBacklightDir, b.devName, "brightness")

	brightnessFile, err := os.OpenFile(brightnessPath, os.O_WRONLY|os.O_TRUNC, 0o0644)
	if err != nil {
		return fmt.Errorf("error: failed to open brightness file: %w", err)
	}
	defer brightnessFile.Close()

	_, err = fmt.Fprintf(brightnessFile, "%d", b.current)
	if err != nil {
		return fmt.Errorf("error: failed to write brightness: %w", err)
	}

	if err := b.saveState(stateDir); err != nil {
		fmt.Fprintln(os.Stderr, "warning: can't save current brightness: %w", err)
	}

	return nil
}

func (b *backlight) saveState(stateDir string) error {
	if stateDir == "" {
		return fmt.Errorf("state path not set")
	}

	brightctlStateDir := filepath.Join(stateDir, "brightctl")

	err := os.MkdirAll(brightctlStateDir, 0o0755)
	if err != nil {
		return fmt.Errorf("couldn't create state directory")
	}

	stateFile, err := os.Create(filepath.Join(brightctlStateDir, "last_brightness"))
	if err != nil {
		return fmt.Errorf("couldn't create state file")
	}
	defer stateFile.Close()

	_, err = fmt.Fprintf(stateFile, "%d", b.current)
	if err != nil {
		return fmt.Errorf("couldn't write to state file")
	}

	return nil
}

func (b *backlight) read(baseBacklightDir string) error {
	backlightDirs, err := os.ReadDir(baseBacklightDir)
	if err != nil {
		return fmt.Errorf("error: failed to list %s: %w", baseBacklightDir, err)
	}

	var devName string
	for _, entry := range backlightDirs {
		path := filepath.Join(baseBacklightDir, entry.Name())

		stat, err := os.Stat(path)
		if err != nil {
			continue
		}

		if stat.IsDir() {
			devName = entry.Name()
			break
		}
	}

	if devName == "" {
		return fmt.Errorf("error: no backlight device found in %s", baseBacklightDir)
	}

	brightnessPath := filepath.Join(baseBacklightDir, devName, "brightness")
	maxBrightnessPath := filepath.Join(baseBacklightDir, devName, "max_brightness")

	cur, err := readIntFromFile(brightnessPath)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	max, err := readIntFromFile(maxBrightnessPath)
	if err != nil {
		return fmt.Errorf("error: %w", err)
	}

	b.devName = devName
	b.current = cur
	b.max = max

	return nil
}

func (b *backlight) restore(baseBacklightDir, stateDir string) error {
	lastBrightnessPath := filepath.Join(stateDir, "brightctl", "last_brightness")

	buffer, err := os.ReadFile(lastBrightnessPath)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("error: no saved brightness found")
		}

		return fmt.Errorf("error: can't read saved brightness: %w", err)
	}

	currStr := strings.TrimSpace(string(buffer))

	currInt, err := strconv.Atoi(currStr)
	if err != nil {
		return fmt.Errorf("error: expected number from %s, but got %s", lastBrightnessPath, currStr)
	}

	b.current = currInt

	err = b.write(baseBacklightDir, stateDir)
	if err != nil {
		return fmt.Errorf("error: failed to restore last brightness: %w", err)
	}

	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage:")
	fmt.Fprintf(os.Stderr, "	%s set 50%%\n", appName)
	fmt.Fprintf(os.Stderr, "	%s set +5%%\n", appName)
	fmt.Fprintf(os.Stderr, "	%s set -5%%\n", appName)
	fmt.Fprintf(os.Stderr, "	%s get\n", appName)
	fmt.Fprintf(os.Stderr, "	%s restore (to use within a startup script)\n", appName)
}

func readIntFromFile(path string) (int, error) {
	buffer, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	valStr := strings.TrimSpace(string(buffer))
	valInt, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("expected number from %s, but got %s", path, valStr)
	}

	return valInt, nil
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
