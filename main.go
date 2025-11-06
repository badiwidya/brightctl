package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type backlight struct {
	devName string
	current int
	max     int
}

func (b *backlight) set(args string) error {
	valStr := strings.TrimSuffix(args, "%")

	val, err := strconv.ParseFloat(strings.TrimSpace(valStr), 64)
	if err != nil {
		return fmt.Errorf("Error: invalid value format")
	}

	var fact float64
	if strings.HasSuffix(args, "%") {
		fact = val / 100
	} else {
		fact = val
	}

	if fact < -1 || fact > 1 {
		return fmt.Errorf("Error: value should be between 0 to 1 or 0%% to 100%%")
	}

	val = fact * float64(b.max)

	if strings.HasPrefix(args, "+") || strings.HasPrefix(args, "-") {
		b.current = b.current + int(val)
	} else {
		b.current = int(val)
	}

	if b.current < 0 {
		b.current = 0
	}

	return nil
}

func (b *backlight) get() float64 {
	var valPerc float64
	valPerc = float64(b.current) / float64(b.max)

	valPerc = math.Trunc(valPerc*100) / 100

	return valPerc
}

func (b *backlight) write(baseBacklightPath, statePath string) error {
	brightnessPath := fmt.Sprintf("%s/%s/brightness", baseBacklightPath, b.devName)

	brightnessFile, err := os.OpenFile(brightnessPath, os.O_WRONLY|os.O_TRUNC, 0o0644)
	if err != nil {
		return fmt.Errorf("Error: failed to open brightness file: %w", err)
	}
	defer brightnessFile.Close()

	_, err = fmt.Fprintf(brightnessFile, "%d", b.current)
	if err != nil {
		return fmt.Errorf("Error: failed to write brightness: %w", err)
	}

	if statePath == "" {
		return nil
	}

	brightctlPath := filepath.Join(statePath, "brightctl")

	err = os.MkdirAll(brightctlPath, 0o0755)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Warning: can't save current brightness to state dir: %s", err)
		return nil
	}

	stateFile, err := os.Create(filepath.Join(brightctlPath, "last_brightness"))
	if err != nil {
		fmt.Fprintf(os.Stdout, "Warning: can't save current brightness to state dir: %s", err)
		return nil
	}
	defer stateFile.Close()

	_, err = fmt.Fprintf(stateFile, "%d", b.current)
	if err != nil {
		fmt.Fprintf(os.Stdout, "Warning: can't save current brightness to state dir: %s", err)
		return nil
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
	brightctlPath := filepath.Join(statePath, "brightctl")

	err := os.MkdirAll(brightctlPath, 0o0755)
	if err != nil {
		return fmt.Errorf("Error: can't make brightctl state dir: %w", err)
	}

	stateFile, err := os.OpenFile(filepath.Join(brightctlPath, "last_brightness"), os.O_RDONLY|os.O_CREATE, 0o0644)
	if err != nil {
		return fmt.Errorf("Error: can't write to brightctl state dir: %w", err)
	}
	defer stateFile.Close()

	buffer := make([]byte, 32)
	n, err := stateFile.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("Error: can't read last brightness: %w", err)
	}

	curr, err := strconv.Atoi(strings.TrimSpace(string(buffer[:n])))
	if err != nil {
		return fmt.Errorf("Error: bad value: %w", err)
	}

	b.current = curr

	err = b.set("+0")
	if err != nil {
		return fmt.Errorf("Error: failed to restore last brightness: %w", err)
	}

	err = b.write(baseBacklightPath, statePath)
	if err != nil {
		return fmt.Errorf("Error: failed to restore last brightness: %w", err)
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
