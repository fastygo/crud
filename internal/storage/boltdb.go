package storage

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cms/internal/models"

	"github.com/valyala/bytebufferpool"
	"go.etcd.io/bbolt"
)

const (
	contentBucket  = "content"
	settingsBucket = "settings"
	usersBucket    = "users"
)

// EphemeralBoltDB manages a temporary BoltDB instance.
type EphemeralBoltDB struct {
	db         *bbolt.DB
	tempDir    string
	tempFile   string
	bufferPool *bytebufferpool.Pool
	mu         sync.RWMutex
}

// NewEphemeralBoltDB creates a temporary BoltDB instance, copying initial data from embedded FS.
func NewEphemeralBoltDB(fs embed.FS, initialDBPath string) (*EphemeralBoltDB, error) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "cms-db-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Path for the temporary DB file
	tempFile := filepath.Join(tempDir, "data.db")

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

	// Open the BoltDB file
	db, err := bbolt.Open(tempFile, 0600, &bbolt.Options{
		Timeout: 1 * time.Second, // Set a reasonable timeout
	})
	if err != nil {
		os.RemoveAll(tempDir) // Clean up on error
		return nil, fmt.Errorf("failed to open boltdb file %s: %w", tempFile, err)
	}

	// Ensure necessary buckets exist
	err = db.Update(func(tx *bbolt.Tx) error {
		buckets := []string{contentBucket, settingsBucket, usersBucket}
		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return fmt.Errorf("failed to create bucket '%s': %w", bucket, err)
			}
		}
		return nil
	})

	if err != nil {
		db.Close()
		os.RemoveAll(tempDir) // Clean up on error
		return nil, fmt.Errorf("failed to initialize db buckets: %w", err)
	}

	return &EphemeralBoltDB{
		db:         db,
		tempDir:    tempDir,
		tempFile:   tempFile,
		bufferPool: &bytebufferpool.Pool{},
	}, nil
}

// Close closes the database and removes temporary files.
func (e *EphemeralBoltDB) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db == nil {
		return nil // Already closed or not initialized
	}

	dbErr := e.db.Close()
	e.db = nil // Prevent double close

	removeErr := os.RemoveAll(e.tempDir)

	if dbErr != nil {
		return fmt.Errorf("error closing db: %w", dbErr)
	}
	if removeErr != nil {
		return fmt.Errorf("error removing temp dir %s: %w", e.tempDir, removeErr)
	}

	return nil
}

// GetContent retrieves a content item by ID.
// Returns the raw JSON data as []byte.
func (e *EphemeralBoltDB) GetContent(id string) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var contentData []byte

	err := e.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(contentBucket))
		if b == nil {
			return fmt.Errorf("bucket '%s' not found", contentBucket)
		}
		val := b.Get([]byte(id))
		if val != nil {
			// Important: Copy the data, BoltDB values are only valid during the transaction.
			contentData = make([]byte, len(val))
			copy(contentData, val)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	if contentData == nil {
		return nil, nil // Not found, but not an error
	}

	return contentData, nil
}

// ListContent retrieves all content items.
// Returns a slice of models.Content.
func (e *EphemeralBoltDB) ListContent() ([]models.Content, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var contents []models.Content

	err := e.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(contentBucket))
		if b == nil {
			return fmt.Errorf("bucket '%s' not found", contentBucket)
		}

		return b.ForEach(func(k, v []byte) error {
			var content models.Content
			// Use a pooled buffer for potentially large JSON
			buf := e.bufferPool.Get()
			buf.Write(v) // Write data to buffer

			if err := json.Unmarshal(buf.Bytes(), &content); err != nil {
				e.bufferPool.Put(buf) // Ensure buffer is returned on error
				// Log or handle the error for the specific item, maybe continue?
				fmt.Fprintf(os.Stderr, "Error unmarshaling content %s: %v\n", string(k), err)
				return nil // Continue processing other items
			}
			e.bufferPool.Put(buf) // Return buffer to pool

			contents = append(contents, content)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return contents, nil
}

// CreateContent adds a new content item.
// Takes ID and raw JSON data as []byte.
func (e *EphemeralBoltDB) CreateContent(id string, data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(contentBucket))
		if b == nil {
			return fmt.Errorf("bucket '%s' not found", contentBucket)
		}
		// Check if ID already exists? Depends on requirements.
		// if b.Get([]byte(id)) != nil {
		// 	 return fmt.Errorf("content with id '%s' already exists", id)
		// }
		return b.Put([]byte(id), data)
	})
}

// UpdateContent updates an existing content item.
// Takes ID and raw JSON data as []byte.
func (e *EphemeralBoltDB) UpdateContent(id string, data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(contentBucket))
		if b == nil {
			return fmt.Errorf("bucket '%s' not found", contentBucket)
		}
		// Check if item exists before updating
		if b.Get([]byte(id)) == nil {
			return fmt.Errorf("content with id '%s' not found", id)
		}
		return b.Put([]byte(id), data)
	})
}

// DeleteContent removes a content item by ID.
func (e *EphemeralBoltDB) DeleteContent(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(contentBucket))
		if b == nil {
			return fmt.Errorf("bucket '%s' not found", contentBucket)
		}
		// Check if item exists before deleting
		if b.Get([]byte(id)) == nil {
			// Return nil or error depending on desired behavior for non-existent deletes
			return nil // Idempotent delete
			// return fmt.Errorf("content with id '%s' not found", id)
		}
		return b.Delete([]byte(id))
	})
}

// ExportDatabase exports the entire database content as JSON.
func (e *EphemeralBoltDB) ExportDatabase() (map[string]map[string]json.RawMessage, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	exportData := make(map[string]map[string]json.RawMessage)

	err := e.db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(bucketName []byte, b *bbolt.Bucket) error {
			bucketData := make(map[string]json.RawMessage)
			err := b.ForEach(func(k, v []byte) error {
				// Copy value as it's only valid during the transaction
				valueCopy := make(json.RawMessage, len(v))
				copy(valueCopy, v)
				bucketData[string(k)] = valueCopy
				return nil
			})
			if err != nil {
				return fmt.Errorf("error iterating bucket %s: %w", bucketName, err)
			}
			exportData[string(bucketName)] = bucketData
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return exportData, nil
}

// ImportDatabase imports data from a JSON structure into the database.
// WARNING: This typically replaces existing data in the specified buckets.
func (e *EphemeralBoltDB) ImportDatabase(importData map[string]map[string]json.RawMessage) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.db.Update(func(tx *bbolt.Tx) error {
		for bucketName, bucketData := range importData {
			// Check if bucket exists, optionally create it or return error
			b := tx.Bucket([]byte(bucketName))
			if b == nil {
				// Option 1: Create bucket if not exists
				var err error
				b, err = tx.CreateBucketIfNotExists([]byte(bucketName))
				if err != nil {
					return fmt.Errorf("failed to create bucket '%s' during import: %w", bucketName, err)
				}
				// Option 2: Return error if bucket must pre-exist
				// return fmt.Errorf("bucket '%s' not found during import", bucketName)
			}

			// Clear existing bucket content? Or merge? Currently replaces.
			// If clearing is desired:
			// if err := tx.DeleteBucket([]byte(bucketName)); err != nil {
			// 	 return fmt.Errorf("failed to clear bucket '%s': %w", bucketName, err)
			// }
			// b, err = tx.CreateBucket([]byte(bucketName))
			// if err != nil {
			// 	 return fmt.Errorf("failed to recreate bucket '%s': %w", bucketName, err)
			// }

			for key, value := range bucketData {
				if err := b.Put([]byte(key), value); err != nil {
					return fmt.Errorf("failed to put key '%s' in bucket '%s': %w", key, bucketName, err)
				}
			}
		}
		return nil
	})
}
