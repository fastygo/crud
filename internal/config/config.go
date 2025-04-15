package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"
)

// Config holds the application configuration.
type Config struct {
	Address           string        `json:"address"`
	Concurrency       int           `json:"concurrency"`
	ReadTimeout       time.Duration `json:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout"`
	AuthUser          string        `json:"-"` // Loaded from ENV
	AuthPass          string        `json:"-"` // Loaded from ENV
	LoginLimitAttempt int           `json:"-"` // Loaded from ENV
	LoginLockDuration time.Duration `json:"-"` // Loaded from ENV
	// GCPercent   int           `json:"gc_percent"` // Removed
	// MaxHeapSize int64         `json:"max_heap_size"` // Removed
}

// Load reads configuration from config.json or uses defaults and ENV variables.
func Load() *Config {
	// Default values
	cfg := &Config{
		Address:           ":8080",
		Concurrency:       1024 * 16,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		LoginLimitAttempt: 5,             // Default login attempts
		LoginLockDuration: 1 * time.Hour, // Default lockout duration
	}

	// Load Auth credentials from environment variables
	cfg.AuthUser = os.Getenv("AUTH_USER")
	cfg.AuthPass = os.Getenv("AUTH_PASS")

	if cfg.AuthUser == "" || cfg.AuthPass == "" {
		log.Println("Warning: AUTH_USER or AUTH_PASS environment variables not set. Authentication will not work.")
	}

	// Load Login attempt limits from environment variables
	if limitStr := os.Getenv("LOGIN_LIMIT_ATTEMPT"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			cfg.LoginLimitAttempt = limit
		} else {
			log.Printf("Warning: Invalid LOGIN_LIMIT_ATTEMPT value '%s'. Using default: %d", limitStr, cfg.LoginLimitAttempt)
		}
	}
	if lockDurStr := os.Getenv("LOGIN_LOCK_DURATION"); lockDurStr != "" {
		if lockDur, err := time.ParseDuration(lockDurStr); err == nil && lockDur > 0 {
			cfg.LoginLockDuration = lockDur
		} else {
			log.Printf("Warning: Invalid LOGIN_LOCK_DURATION value '%s'. Using default: %v", lockDurStr, cfg.LoginLockDuration)
		}
	}

	// Check for PORT environment variable (common in cloud environments)
	if port := os.Getenv("PORT"); port != "" {
		cfg.Address = ":" + port
	}

	// Load from file if it exists (overrides defaults but not ENV for auth/port/limits)
	if _, err := os.Stat("config.json"); err == nil {
		file, err := os.Open("config.json")
		if err != nil {
			log.Printf("Warning: Could not open config.json: %v. Using default/ENV config.", err)
		} else {
			defer file.Close()
			decoder := json.NewDecoder(file)
			// Decode into a temporary struct to avoid overwriting ENV vars
			fileCfg := &Config{}
			err = decoder.Decode(fileCfg)
			if err != nil {
				log.Printf("Warning: Could not decode config.json: %v. Using default/ENV config.", err)
			} else {
				// Apply file config only for specific fields, excluding those set by ENV
				if fileCfg.Address != "" {
					cfg.Address = fileCfg.Address
				}
				if fileCfg.Concurrency != 0 {
					cfg.Concurrency = fileCfg.Concurrency
				}
				if fileCfg.ReadTimeout != 0 {
					cfg.ReadTimeout = fileCfg.ReadTimeout
				}
				if fileCfg.WriteTimeout != 0 {
					cfg.WriteTimeout = fileCfg.WriteTimeout
				}
			}
		}
	}

	// Re-check PORT env var in case config.json overwrote it
	if port := os.Getenv("PORT"); port != "" {
		cfg.Address = ":" + port
	}

	log.Printf("Config loaded: Address=%s, AuthUser=%s, Attempts=%d, Lockout=%v",
		cfg.Address, cfg.AuthUser, cfg.LoginLimitAttempt, cfg.LoginLockDuration)
	return cfg
}
