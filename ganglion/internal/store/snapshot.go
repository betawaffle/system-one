package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// Snapshot writes consistent copies of index.db and blobs.db into outDir with
// VACUUM INTO, which reads inside one transaction, so it is safe while the
// server is writing. Each copy is written to a temporary name and renamed into
// place, so outDir never holds a half-written file. The backup runs this right
// before copying the data directory, because a plain file copy of a live
// SQLite database (and its -wal file) can be inconsistent.
func (s *Store) Snapshot(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for _, d := range []struct {
		name string
		db   *sql.DB
	}{{"index.db", s.db}, {"blobs.db", s.blobs}} {
		final := filepath.Join(outDir, d.name)
		tmp := final + ".tmp"
		os.Remove(tmp)
		if _, err := d.db.Exec("VACUUM INTO ?", tmp); err != nil {
			os.Remove(tmp)
			return fmt.Errorf("snapshot %s: %w", d.name, err)
		}
		if err := os.Rename(tmp, final); err != nil {
			return fmt.Errorf("snapshot %s: %w", d.name, err)
		}
	}
	return nil
}
