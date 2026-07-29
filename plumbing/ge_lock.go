package plumbing

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/brickster241/GitEngine/utils/types"
)

// AcquireIndexLock takes .git/index.lock exclusively — the same protocol real
// Git uses so concurrent writers cannot corrupt the index. O_EXCL makes the
// create atomic at the filesystem level. Waiters retry with backoff up to
// maxWait before giving up, so contending processes serialize instead of
// failing or corrupting.
func AcquireIndexLock(maxWait time.Duration) (string, error) {
	lockPath := filepath.Join(".git", "index.lock")
	deadline := time.Now().Add(maxWait)
	backoff := 2 * time.Millisecond

	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			f.Close()
			return lockPath, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("unable to create '.git/index.lock': File exists — another gegit process seems to be running")
		}
		time.Sleep(backoff)
		if backoff < 100*time.Millisecond {
			backoff *= 2
		}
	}
}

// ReleaseIndexLock removes the lockfile without publishing an index (the
// abort path).
func ReleaseIndexLock(lockPath string) {
	os.Remove(lockPath)
}

// WriteIndexLocked serializes entries into the lockfile and atomically
// renames it over .git/index — concurrent readers observe either the old or
// the new index in full, never a torn write. This is the write path every
// index-mutating command goes through.
func WriteIndexLocked(entries []types.IndexEntry) error {
	lockPath, err := AcquireIndexLock(2 * time.Second)
	if err != nil {
		return err
	}

	// Serialize into the lockfile, then atomically publish.
	if err := os.WriteFile(lockPath, serializeIndex(entries), 0644); err != nil {
		ReleaseIndexLock(lockPath)
		return err
	}
	if err := os.Rename(lockPath, filepath.Join(".git", "index")); err != nil {
		ReleaseIndexLock(lockPath)
		return err
	}
	return nil
}
