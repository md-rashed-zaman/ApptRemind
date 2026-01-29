package config

import (
	"fmt"
	"os"
	"strconv"
)

func String(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func RequiredString(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return v, nil
}

func Port(key, fallback string) (string, error) {
	v := String(key, fallback)
	p, err := strconv.Atoi(v)
	if err != nil || p < 1 || p > 65535 {
		return "", fmt.Errorf("%s must be a valid TCP port (got %q)", key, v)
	}
	return v, nil
}

