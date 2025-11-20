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
		return 0, err
	}

	valStr := strings.TrimSpace(string(buffer))
	valInt, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, fmt.Errorf("expected number from %s, but got %s", path, valStr)
	}

	return valInt, nil
}

func writeIntToFile(path string, content int) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%d", content)
	if err != nil {
		return err
	}

	return nil
}
