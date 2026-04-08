package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ExporterBaseURL           string
	DatabaseURL               string
	ListenAddr                string
	CollectInterval           time.Duration
	DefaultRegion             string
	SourceRevision            string
	PricePerByte              float64
	InitialIncludedQuotaBytes int64
	InitialBalance            float64
}

func Load() (Config, error) {
	cfg := Config{
		ExporterBaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("EXPORTER_BASE_URL")), "/"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("DATABASE_URL")),
		ListenAddr:      strings.TrimSpace(os.Getenv("LISTEN_ADDR")),
		DefaultRegion:   strings.TrimSpace(os.Getenv("DEFAULT_REGION")),
		SourceRevision:  strings.TrimSpace(os.Getenv("SOURCE_REVISION")),
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8081"
	}
	if cfg.SourceRevision == "" {
		cfg.SourceRevision = "billing-service-v1"
	}

	if cfg.ExporterBaseURL == "" {
		return Config{}, fmt.Errorf("EXPORTER_BASE_URL is required")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	interval := strings.TrimSpace(os.Getenv("COLLECT_INTERVAL"))
	if interval == "" {
		cfg.CollectInterval = time.Minute
	} else {
		parsed, err := time.ParseDuration(interval)
		if err != nil {
			return Config{}, fmt.Errorf("parse COLLECT_INTERVAL: %w", err)
		}
		cfg.CollectInterval = parsed
	}

	cfg.PricePerByte = parseFloatEnv("PRICE_PER_BYTE", 0)
	cfg.InitialBalance = parseFloatEnv("INITIAL_BALANCE", 0)
	cfg.InitialIncludedQuotaBytes = parseIntEnv("INITIAL_INCLUDED_QUOTA_BYTES", 0)
	return cfg, nil
}

func parseFloatEnv(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseIntEnv(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
