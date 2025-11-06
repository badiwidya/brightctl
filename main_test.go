package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const testDeviceName = "bl_device"

func TestGet(t *testing.T) {
	bl := &backlight{
		max:     100,
		current: 25,
	}

	want := 0.25
	got := bl.get()

	if got != want {
		t.Errorf("want %f; got %f", want, got)
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		Description string
		Argument    string
		Want        int
		WantErr     bool
	}{
		{
			Description: "plus percentage",
			Argument:    "+5%",
			Want:        25,
		},
		{
			Description: "minus percentage",
			Argument:    "-5%",
			Want:        15,
		},
		{
			Description: "plus decimal",
			Argument:    "+0.05",
			Want:        25,
		},
		{
			Description: "minus decimal",
			Argument:    "-0.05",
			Want:        15,
		},
		{
			Description: "exact percentage",
			Argument:    "50%",
			Want:        50,
		},
		{
			Description: "exact decimal",
			Argument:    "0.5",
			Want:        50,
		},
		{
			Description: "very big absolute decimal will return error",
			Argument:    "50000",
			WantErr:     true,
		},
		{
			Description: "very big absolute percent will return error",
			Argument:    "5000%",
			WantErr:     true,
		},
		{
			Description: "very big relative percent value will return error",
			Argument:    "+5000%",
			WantErr:     true,
		},
		{
			Description: "very big relative decimal value will return error",
			Argument:    "+50000",
			WantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Description, func(t *testing.T) {
			bl := &backlight{
				max:     100,
				current: 20,
			}

			err := bl.set(tt.Argument)

			if tt.WantErr {
				requireError(t, err)
				return
			}

			requireNoError(t, err)

			if bl.current != tt.Want {
				t.Errorf("want %d; got %d", tt.Want, bl.current)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	t.Run("success writing to brightness and state file", func(t *testing.T) {
		sysfsDir, stateDir, blDeviceDir, brightctlStateDir := setupTempDirs(t)

		brightnessPath := filepath.Join(blDeviceDir, "brightness")
		lastBrightnessPath := filepath.Join(brightctlStateDir, "last_brightness")

		err := os.WriteFile(brightnessPath, nil, 0o0644)
		requireNoError(t, err)

		bl := &backlight{
			devName: testDeviceName,
			current: 3480,
		}

		err = bl.write(sysfsDir, stateDir)
		requireNoError(t, err)

		assertContainsInt(t, brightnessPath, bl.current)
		assertContainsInt(t, lastBrightnessPath, bl.current)
	})

	t.Run("failed writing to brightness file", func(t *testing.T) {
		sysfsDir, stateDir, _, _ := setupTempDirs(t)

		bl := &backlight{
			devName: testDeviceName,
			current: 2800,
		}

		err := bl.write(sysfsDir, stateDir)
		requireError(t, err)

		assertErrorToContains(t, err, "failed to open brightness")
	})
}

func TestRead(t *testing.T) {
	t.Run("brightness information retrieved successfully", func(t *testing.T) {
		sysfsDir, _, blDeviceDir, _ := setupTempDirs(t)

		brightnessPath := filepath.Join(blDeviceDir, "brightness")
		err := os.WriteFile(brightnessPath, []byte("20"), 0o0644)
		requireNoError(t, err)

		maxBrightnessPath := filepath.Join(blDeviceDir, "max_brightness")
		err = os.WriteFile(maxBrightnessPath, []byte("100"), 0o0644)
		requireNoError(t, err)

		bl := &backlight{}

		err = bl.read(sysfsDir)
		requireNoError(t, err)

		if bl.devName != testDeviceName {
			t.Errorf("want device name %s; got %s", testDeviceName, bl.devName)
		}

		if bl.current != 20 {
			t.Errorf("want %d; got %d", 20, bl.current)
		}

		if bl.max != 100 {
			t.Errorf("want %d; got %d", 100, bl.max)
		}
	})

	t.Run("device not found", func(t *testing.T) {
		sysfsDir := t.TempDir()

		bl := &backlight{}

		err := bl.read(sysfsDir)
		requireError(t, err)

		assertErrorToContains(t, err, "no backlight device found")
	})

	t.Run("sysfs directory not exist/not found", func(t *testing.T) {
		tmpDir := t.TempDir()

		sysfsDir := filepath.Join(tmpDir, "backlight")

		bl := &backlight{}

		err := bl.read(sysfsDir)
		requireError(t, err)

		assertErrorToContains(t, err, "failed to list")
	})
}

func TestRestore(t *testing.T) {
	sysfsDir, stateDir, blDeviceDir, brightctlStateDir := setupTempDirs(t)

	brightnessPath := filepath.Join(blDeviceDir, "brightness")
	err := os.WriteFile(brightnessPath, []byte("20"), 0o0644)
	requireNoError(t, err)

	lastBrightnessPath := filepath.Join(brightctlStateDir, "last_brightness")
	err = os.WriteFile(lastBrightnessPath, []byte("100"), 0o0644)
	requireNoError(t, err)

	bl := &backlight{
		devName: testDeviceName,
		max:     100,
		current: 20,
	}

	err = bl.restore(sysfsDir, stateDir)
	requireNoError(t, err)

	assertContainsInt(t, brightnessPath, 100)
}

func requireNoError(t testing.TB, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("should have not returns an error; but got %s", err)
	}
}

func requireError(t testing.TB, err error) {
	t.Helper()

	if err == nil {
		t.Fatalf("should have returns an error; but got none")
	}
}

func assertContainsInt(t testing.TB, path string, expected int) {
	t.Helper()

	buffer, err := os.ReadFile(path)
	requireNoError(t, err)

	got, err := strconv.Atoi(strings.TrimSpace(string(buffer)))
	requireNoError(t, err)

	if got != expected {
		t.Errorf("want %d; got %d", expected, got)
	}
}

func assertErrorToContains(t testing.TB, err error, content string) {
	t.Helper()

	if !strings.Contains(err.Error(), content) {
		t.Fatalf("expect error to contains '%s'; but got %s", content, err)
	}
}

func setupTempDirs(t testing.TB) (sysfsDir, stateDir, blDeviceDir, brightctlStateDir string) {
	t.Helper()

	sysfsDir = t.TempDir()
	stateDir = t.TempDir()

	blDeviceDir = filepath.Join(sysfsDir, testDeviceName)
	err := os.MkdirAll(blDeviceDir, 0o0755)
	requireNoError(t, err)

	brightctlStateDir = filepath.Join(stateDir, "brightctl")
	err = os.MkdirAll(brightctlStateDir, 0o0755)
	requireNoError(t, err)

	return sysfsDir, stateDir, blDeviceDir, brightctlStateDir
}
