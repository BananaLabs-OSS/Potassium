package config

import (
	"log"
	"os"
	"strconv"
)

// RequireEnv returns the value of an environment variable or logs fatal if unset/empty.
func RequireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("%s is required", key)
	}
	return val
}

// EnvOrDefault returns the value of an environment variable or the fallback if unset.
func EnvOrDefault(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

// EnvOrDefaultInt returns the integer value of an environment variable or the fallback.
func EnvOrDefaultInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

// Resolve returns the first non-zero value in priority order: CLI flag, env var, fallback.
func Resolve(cli, env, fallback string) string {
	if cli != "" {
		return cli
	}
	if env != "" {
		return env
	}
	return fallback
}

// ResolveInt returns the first non-zero value in priority order: CLI flag, env var, fallback.
func ResolveInt(cli, env, fallback int) int {
	if cli != 0 {
		return cli
	}
	if env != 0 {
		return env
	}
	return fallback
}
