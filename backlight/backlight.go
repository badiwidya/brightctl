package backlight

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

type Backlight struct {
	DevName string
	Current int
	Max     int
}

func (b *Backlight) GetPercentage() float64 {
	var valPerc float64
	valPerc = float64(b.Current) / float64(b.Max)

	valPerc = math.Trunc(valPerc*100) / 100

	return valPerc
}

func (b *Backlight) Set(arg string) error {
	arg = strings.TrimSpace(arg)

	var isRelative, isPercent bool

	if strings.HasSuffix(arg, "%") {
		isPercent = true
	}

	if strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-") {
		isRelative = true
	}

	valStr := arg
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

	delta := int(val * float64(b.Max))

	if isRelative {
		b.Current += delta
	} else {
		b.Current = delta
	}

	if b.Current < 0 {
		b.Current = 0
	}

	if b.Current > b.Max {
		b.Current = b.Max
	}

	return nil
}

func (b *Backlight) Write(baseBacklightDir string) error {
	brightnessPath := filepath.Join(baseBacklightDir, b.DevName, "brightness")

	brightnessFile, err := os.OpenFile(brightnessPath, os.O_WRONLY|os.O_TRUNC, 0o0644)
	if err != nil {
		return fmt.Errorf("error: failed to open brightness file: %w", err)
	}
	defer brightnessFile.Close()

	_, err = fmt.Fprintf(brightnessFile, "%d", b.Current)
	if err != nil {
		return fmt.Errorf("error: failed to write brightness: %w", err)
	}

	return nil
}

func (b *Backlight) Restore(baseBacklightDir, stateDir string) error {
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

	b.Current = currInt

	err = b.Write(baseBacklightDir)
	if err != nil {
		return fmt.Errorf("error: failed to restore last brightness: %w", err)
	}

	return nil
}

func (b *Backlight) SaveState(stateDir string) error {
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

	_, err = fmt.Fprintf(stateFile, "%d", b.Current)
	if err != nil {
		return fmt.Errorf("couldn't write to state file")
	}

	return nil
}

func New(baseBacklightDir string) (*Backlight, error) {
	backlightDirs, err := os.ReadDir(baseBacklightDir)
	if err != nil {
		return nil, fmt.Errorf("error: failed to list %s: %w", baseBacklightDir, err)
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
		return nil, fmt.Errorf("error: no backlight device found in %s", baseBacklightDir)
	}

	brightnessPath := filepath.Join(baseBacklightDir, devName, "brightness")
	maxBrightnessPath := filepath.Join(baseBacklightDir, devName, "max_brightness")

	cur, err := readIntFromFile(brightnessPath)
	if err != nil {
		return nil, fmt.Errorf("error: %w", err)
	}

	max, err := readIntFromFile(maxBrightnessPath)
	if err != nil {
		return nil, fmt.Errorf("error: %w", err)
	}

	return &Backlight{
		DevName: devName,
		Current: cur,
		Max:     max,
	}, nil
}
