package backlight_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/badiwidya/brightctl/backlight"
)

const (
	testDeviceName = "bl_device"
	defaultMax     = 100
)

type testEnv struct {
	sysfsDir          string
	blDeviceDir       string
	stateDir          string
	brightctlStateDir string
	brightnessPath    string
	maxPath           string
	lastStatePath     string
}

func TestGet(t *testing.T) {
	env := setupTestEnv(t)
	setupFiles(t, env, 25, defaultMax)

	bl := &backlight.Backlight{
		BrightnessPath: env.brightnessPath,
		Max:            defaultMax,
	}

	want := 0.25
	got, err := bl.GetPercentage()
	assertNoError(t, err)

	if got != want {
		t.Errorf("want %.2f; got %.2f", want, got)
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		Description string
		Argument    string
		StartVal    int
		Want        int
		WantErr     bool
	}{
		{Description: "plus percentage", Argument: "+5%", StartVal: 20, Want: 25},
		{Description: "minus percentage", Argument: "-5%", StartVal: 20, Want: 15},
		{Description: "plus decimal", Argument: "+0.05", StartVal: 20, Want: 25},
		{Description: "minus decimal", Argument: "-0.05", StartVal: 20, Want: 15},
		{Description: "exact percentage", Argument: "50%", StartVal: 20, Want: 50},
		{Description: "exact decimal", Argument: "0.5", StartVal: 20, Want: 50},
		{Description: "very big absolute decimal returns error", Argument: "50000", StartVal: 20, WantErr: true},
		{Description: "very big absolute percent returns error", Argument: "5000%", StartVal: 20, WantErr: true},
		{Description: "very big relative percent returns error", Argument: "+5000%", StartVal: 20, WantErr: true},
		{Description: "very big relative decimal returns error", Argument: "+50000", StartVal: 20, WantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.Description, func(t *testing.T) {
			env := setupTestEnv(t)
			setupFiles(t, env, tt.StartVal, defaultMax)

			bl := &backlight.Backlight{
				BrightnessPath: env.brightnessPath,
				Max:            defaultMax,
			}

			err := bl.Set(tt.Argument)

			if tt.WantErr {
				assertError(t, err)
				return
			}

			assertNoError(t, err)

			curr, err := bl.GetCurrent()
			assertNoError(t, err)

			if curr != tt.Want {
				t.Errorf("want %d; got %d", tt.Want, curr)
			}
		})
	}
}

func TestSaveState(t *testing.T) {
	env := setupTestEnv(t)
	curVal := 3480
	setupFiles(t, env, curVal, 5000)

	bl := &backlight.Backlight{BrightnessPath: env.brightnessPath}

	err := bl.SaveState(env.stateDir)
	assertNoError(t, err)

	assertFileContent(t, env.lastStatePath, curVal)
}

func TestNew(t *testing.T) {
	t.Run("paths and max brightness information retrieved successfully", func(t *testing.T) {
		env := setupTestEnv(t)
		setupFiles(t, env, 0, defaultMax)

		bl, err := backlight.New(env.sysfsDir)
		assertNoError(t, err)

		if bl.Max != defaultMax {
			t.Errorf("got max %d; want %d", bl.Max, defaultMax)
		}
		if bl.BrightnessPath != env.brightnessPath {
			t.Errorf("got path %s; want %s", bl.BrightnessPath, env.brightnessPath)
		}
		if bl.MaxPath != env.maxPath {
			t.Errorf("got path %s; want %s", bl.MaxPath, env.maxPath)
		}
	})

	t.Run("device not found", func(t *testing.T) {
		env := setupTestEnv(t)

		_, err := backlight.New(env.sysfsDir)
		assertErrorContains(t, err, "no such file")
	})

	t.Run("sysfs directory not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		fakeSysfs := filepath.Join(tmpDir, "nowhere")

		_, err := backlight.New(fakeSysfs)
		assertErrorContains(t, err, "failed to list")
	})
}

func TestRestore(t *testing.T) {
	env := setupTestEnv(t)
	setupFiles(t, env, 20, defaultMax)

	createIntFile(t, env.lastStatePath, 100)

	bl := &backlight.Backlight{
		DevName:        testDeviceName,
		BrightnessPath: env.brightnessPath,
		Max:            defaultMax,
	}

	err := bl.Restore(env.stateDir)
	assertNoError(t, err)

	assertFileContent(t, env.brightnessPath, 100)
}

func setupTestEnv(t testing.TB) *testEnv {
	t.Helper()
	sysfsDir := t.TempDir()
	stateDir := t.TempDir()

	blDeviceDir := filepath.Join(sysfsDir, testDeviceName)
	err := os.MkdirAll(blDeviceDir, 0o0755)
	assertNoError(t, err)

	brightctlStateDir := filepath.Join(stateDir, "brightctl")
	err = os.MkdirAll(brightctlStateDir, 0o0755)
	assertNoError(t, err)

	return &testEnv{
		sysfsDir:          sysfsDir,
		stateDir:          stateDir,
		blDeviceDir:       blDeviceDir,
		brightctlStateDir: brightctlStateDir,
		brightnessPath:    filepath.Join(blDeviceDir, "brightness"),
		maxPath:           filepath.Join(blDeviceDir, "max_brightness"),
		lastStatePath:     filepath.Join(brightctlStateDir, "last_brightness"),
	}
}

func setupFiles(t testing.TB, env *testEnv, cur, max int) {
	t.Helper()
	createIntFile(t, env.brightnessPath, cur)
	createIntFile(t, env.maxPath, max)
}

func createIntFile(t testing.TB, path string, content int) {
	t.Helper()
	err := os.WriteFile(path, []byte(strconv.Itoa(content)), 0o644)
	assertNoError(t, err)
}

func assertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertError(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, but got nil")
	}
}

func assertErrorContains(t testing.TB, err error, sub string) {
	t.Helper()
	assertError(t, err)
	if !strings.Contains(err.Error(), sub) {
		t.Errorf("expected error to contain %q, got %q", sub, err.Error())
	}
}

func assertFileContent(t testing.TB, path string, expected int) {
	t.Helper()
	data, err := os.ReadFile(path)
	assertNoError(t, err)

	got, err := strconv.Atoi(strings.TrimSpace(string(data)))
	assertNoError(t, err)

	if got != expected {
		t.Errorf("file content %s: want %d, got %d", filepath.Base(path), expected, got)
	}
}
