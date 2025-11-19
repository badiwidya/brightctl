package backlight

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

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
