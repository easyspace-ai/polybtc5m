package config

import (
	"os"
	"strings"
)

// LoadDotEnv reads ./.env from the current working directory (if present) and
// sets environment variables that are not already set. This matches what
// `set -a && . ./.env` does so `go run ./cmd/...` from the repo root picks up
// environment values automatically.
func LoadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			switch {
			case val[0] == '"' && val[len(val)-1] == '"':
				val = val[1 : len(val)-1]
			case val[0] == '\'' && val[len(val)-1] == '\'':
				val = val[1 : len(val)-1]
			}
		}
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, val)
	}
}

// GetEnv returns the environment variable value, or fallback if empty/unset.
func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
