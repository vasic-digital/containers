package envconfig

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// loadDotEnv reads a .env file and sets environment variables.
// Existing environment variables take precedence.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := parseDotEnvLine(line)
		if !ok {
			continue
		}

		// Only set if not already defined.
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

// parseDotEnvLine parses a KEY=VALUE line, handling quotes and
// inline comments.
func parseDotEnvLine(line string) (key, value string, ok bool) {
	// Remove export prefix.
	line = strings.TrimPrefix(line, "export ")

	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}

	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])

	// Handle quoted values.
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
			return key, value, true
		}
	}

	// Strip inline comments (only for unquoted values).
	if idx := strings.IndexByte(value, '#'); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}

	return key, value, true
}
