package config

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// Config holds the application configuration.
type Config struct {
	Address      string        `json:"address"`
	Concurrency  int           `json:"concurrency"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	// GCPercent   int           `json:"gc_percent"` // Removed as it's set directly in main
	// MaxHeapSize int64         `json:"max_heap_size"` // Removed as SetMemoryLimit is commented out
}

// Load reads configuration from config.json or uses defaults.
func Load() *Config {
	// Default values
	cfg := &Config{
		Address:      ":8080",
		Concurrency:  256 * 1024, // Default from fasthttp
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		// GCPercent:   100,
		// MaxHeapSize: 512 * 1024 * 1024, // 512MB
	}

	// Load from file if it exists
	if _, err := os.Stat("config.json"); err == nil {
		file, err := os.Open("config.json")
		if err != nil {
			log.Printf("Warning: Could not open config.json: %v. Using default config.", err)
		} else {
			defer file.Close()
			decoder := json.NewDecoder(file)
			err = decoder.Decode(cfg)
			if err != nil {
				log.Printf("Warning: Could not decode config.json: %v. Using default config.", err)
			}
		}
	}

	return cfg
}
