package backlight

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Backlight struct {
	DevName        string
	BrightnessPath string
	MaxPath        string
	Max            int
}

func (b *Backlight) GetPercentage() (float64, error) {
	cur, err := b.GetCurrent()
	if err != nil {
		return 0, err
	}
	valPerc := float64(cur) / float64(b.Max)

	valPerc = math.Trunc(valPerc*100) / 100

	return valPerc, nil
}

func (b *Backlight) GetCurrent() (int, error) {
	cur, err := readIntFromFile(b.BrightnessPath)
	if err != nil {
		return 0, fmt.Errorf("can't read brightness: %w", err)
	}

	return cur, nil
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

	cur, err := b.GetCurrent()
	if err != nil {
		return err
	}

	if isRelative {
		cur += delta
	} else {
		cur = delta
	}

	if cur < 0 {
		cur = 0
	}

	if cur > b.Max {
		cur = b.Max
	}

	err = writeIntToFile(b.BrightnessPath, cur)
	if err != nil {
		return fmt.Errorf("failed to save brightness: %w", err)
	}

	return nil
}

func (b *Backlight) Restore(stateDir string) error {
	lastBrightnessPath := filepath.Join(stateDir, "brightctl", "last_brightness")

	lastBrightness, err := readIntFromFile(lastBrightnessPath)
	if err != nil {
		return fmt.Errorf("error: failed to restore last brightness: %w", err)
	}

	err = writeIntToFile(b.BrightnessPath, lastBrightness)
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

	cur, err := b.GetCurrent()
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(stateFile, "%d", cur)
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

	maxBri, err := readIntFromFile(maxBrightnessPath)
	if err != nil {
		return nil, fmt.Errorf("can't read max brightness: %w", err)
	}

	return &Backlight{
		DevName:        devName,
		BrightnessPath: brightnessPath,
		MaxPath:        maxBrightnessPath,
		Max:            maxBri,
	}, nil
}
