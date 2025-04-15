package storage

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cms/internal/models"

	"github.com/valyala/bytebufferpool"
	"go.etcd.io/bbolt"
)

const (
	contentBucket = "content"
	// settingsBucket = "settings" // Keep if settings are still needed from initial db
	// usersBucket    = "users"    // Keep if user info is stored here
)

// InitialDataReader provides read-only access to the initial database state.
type InitialDataReader struct {
	db         *bbolt.DB
	tempDir    string // Keep track for cleanup
	bufferPool *bytebufferpool.Pool
	mu         sync.RWMutex // Keep RWMutex for potential concurrent reads
}

// NewInitialDataReader creates a reader for the initial BoltDB data, copied from embedded FS.
func NewInitialDataReader(fs embed.FS, initialDBPath string) (*InitialDataReader, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "cms-initial-db-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Path for the temporary DB file
	tempFile := filepath.Join(tempDir, "initial_data.db")

	// Extract the initial database from the embedded FS
	src, err := fs.Open(initialDBPath)
	if err != nil {
		os.RemoveAll(tempDir) // Clean up on error
		return nil, fmt.Errorf("failed to open embedded db %s: %w", initialDBPath, err)
	}
	defer src.Close()

	// Create the temporary destination DB file
	dst, err := os.Create(tempFile)
	if err != nil {
		os.RemoveAll(tempDir) // Clean up on error
		return nil, fmt.Errorf("failed to create temp db file %s: %w", tempFile, err)
	}

	// Copy the initial database content
	if _, err = io.Copy(dst, src); err != nil {
		dst.Close()
		os.RemoveAll(tempDir) // Clean up on error
		return nil, fmt.Errorf("failed to copy initial db content: %w", err)
	}
	dst.Close() // Close dst after successful copy

	// Open the BoltDB file in read-only mode
	db, err := bbolt.Open(tempFile, 0400, &bbolt.Options{
		ReadOnly: true, // Open in read-only mode
		Timeout:  1 * time.Second,
	})
	if err != nil {
		os.RemoveAll(tempDir) // Clean up on error
		return nil, fmt.Errorf("failed to open boltdb file %s: %w", tempFile, err)
	}

	// No need to create buckets in read-only mode

	return &InitialDataReader{
		db:         db,
		tempDir:    tempDir,
		bufferPool: &bytebufferpool.Pool{},
	}, nil
}

// Close closes the database reader and removes temporary files.
func (r *InitialDataReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.db == nil {
		return nil // Already closed or not initialized
	}

	dbErr := r.db.Close()
	r.db = nil // Prevent double close

	// Attempt to remove the temporary directory
	removeErr := os.RemoveAll(r.tempDir)

	if dbErr != nil {
		return fmt.Errorf("error closing initial db reader: %w", dbErr)
	}
	if removeErr != nil {
		// Log the error but don't necessarily fail the Close operation
		log.Printf("Warning: error removing temp dir %s: %v", r.tempDir, removeErr)
	}

	return nil
}

// LoadInitialContent reads all content items from the initial database.
// Returns a map[string]models.Content for easy use in session storage.
func (r *InitialDataReader) LoadInitialContent() (map[string]models.Content, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.db == nil {
		return nil, fmt.Errorf("InitialDataReader database is not open")
	}

	contentMap := make(map[string]models.Content)

	err := r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(contentBucket))
		if b == nil {
			// If the initial DB doesn't have the bucket, return an empty map
			log.Printf("Warning: Initial DB missing '%s' bucket", contentBucket)
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var content models.Content
			// Create a copy of v because it's only valid during the transaction
			dataCopy := make([]byte, len(v))
			copy(dataCopy, v)

			// Unmarshal the copied data
			if err := json.Unmarshal(dataCopy, &content); err != nil {
				// Log or handle the error for the specific item, maybe continue?
				log.Printf("Error unmarshaling initial content %s: %v", string(k), err)
				return nil // Continue processing other items
			}
			contentMap[string(k)] = content
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("error reading initial content: %w", err)
	}

	log.Printf("Loaded %d items from initial database.", len(contentMap))
	return contentMap, nil
}

/* --- Deprecated Write Operations ---
   These functions are no longer needed as user data is stored in the session.

// GetContent retrieves a content item by ID.
// Returns the raw JSON data as []byte.
func (e *EphemeralBoltDB) GetContent(id string) ([]byte, error) { ... }

// ListContent retrieves all content items.
// Returns a slice of models.Content.
func (e *EphemeralBoltDB) ListContent() ([]models.Content, error) { ... }

// CreateContent adds a new content item.
// Takes ID and raw JSON data as []byte.
func (e *EphemeralBoltDB) CreateContent(id string, data []byte) error { ... }

// UpdateContent updates an existing content item.
// Takes ID and raw JSON data as []byte.
func (e *EphemeralBoltDB) UpdateContent(id string, data []byte) error { ... }

// DeleteContent removes a content item by ID.
func (e *EphemeralBoltDB) DeleteContent(id string) error { ... }

// ExportDatabase exports the entire database content.
func (e *EphemeralBoltDB) ExportDatabase() (map[string]map[string]json.RawMessage, error) { ... }

// ImportDatabase imports data into the database, replacing existing data.
func (e *EphemeralBoltDB) ImportDatabase(importData map[string]map[string]json.RawMessage) error { ... }

*/
