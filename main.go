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

var appName = ""

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

func (b *backlight) write(baseBacklightDir, statePath string) error {
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

	if err := b.saveState(statePath); err != nil {
		fmt.Fprintln(os.Stderr, "warning: can't save current brightness: %w", err)
	}

	return nil
}

func (b *backlight) saveState(stateDir string) error {
	if stateDir == "" {
		return fmt.Errorf("state path not set")
	}

	brightctlPath := filepath.Join(stateDir, "brightctl")

	err := os.MkdirAll(brightctlPath, 0o0755)
	if err != nil {
		return fmt.Errorf("couldn't create state directory")
	}

	stateFile, err := os.Create(filepath.Join(brightctlPath, "last_brightness"))
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

func (b *backlight) restore(baseBacklightPath, statePath string) error {
	lastBrightnessPath := filepath.Join(statePath, "brightctl", "last_brightness")

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

	err = b.write(baseBacklightPath, statePath)
	if err != nil {
		return fmt.Errorf("error: failed to restore last brightness: %w", err)
	}

	return nil
}

func main() {
	const baseBacklightPath = "/sys/class/backlight"

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: can't get binary name\n")
		os.Exit(1)
	}

	appName := filepath.Base(exePath)

	args := os.Args[1:]
	if len(args) < 1 {
		printUsage(appName)
		os.Exit(1)
	}

	statePath := os.Getenv("XDG_STATE_HOME")
	if statePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stdout, "Warning: can't save current brightness to state dir: %s", err)
		} else {
			statePath = filepath.Join(homeDir, ".local", "state")
		}
	}

	bl := &backlight{}

	err = bl.read(baseBacklightPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch args[0] {
	case "set":
		if len(args) != 2 {
			printUsage(appName)
			os.Exit(1)
		}

		err = bl.set(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		err = bl.write(baseBacklightPath, statePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		fmt.Fprintln(os.Stdout, "Brightness changed")
	case "get":
		if len(args) != 1 {
			printUsage(appName)
			os.Exit(1)
		}

		fmt.Fprintln(os.Stdout, bl.get())
	case "restore":
		if len(args) != 1 {
			printUsage(appName)
			os.Exit(1)
		}

		err := bl.restore(baseBacklightPath, statePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
	default:
		printUsage(appName)
		os.Exit(1)
	}
}

func printUsage(appName string) {
	fmt.Fprintf(os.Stdout, "Usage:")
	fmt.Fprintf(os.Stdout, "	%s set 50%%\n", appName)
	fmt.Fprintf(os.Stdout, "	%s set +5%%\n", appName)
	fmt.Fprintf(os.Stdout, "	%s set -5%%\n", appName)
	fmt.Fprintf(os.Stdout, "	%s get\n", appName)
	fmt.Fprintf(os.Stdout, "	%s restore (to use within a startup script)\n", appName)
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
