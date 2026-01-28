package porcelain

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils/types"
)

// addOrUpdatePath adds or updates the index entry for the given path.
func addOrUpdatePath(path string, indexMap map[string]types.IndexEntry, workingSet map[string]bool, trackWorkingSet bool) {

	// Clean and normalize the path
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	if trackWorkingSet {
		workingSet[cleanPath] = true
	}
	// Get file info
	info, err := os.Stat(cleanPath)
	if err != nil {
		return
	}
	stat := info.Sys().(*syscall.Stat_t)

	// Check if already tracked
	existing, tracked := indexMap[cleanPath]

	if tracked {
		unchanged :=
			existing.Dev == uint32(stat.Dev) &&
				existing.Ino == uint32(stat.Ino) &&
				existing.FileSize == uint32(info.Size()) &&
				existing.Mtime == uint32(stat.Mtimespec.Sec) &&
				existing.MtimeNs == uint32(stat.Mtimespec.Nsec) &&
				existing.Ctime == uint32(stat.Ctimespec.Sec) &&
				existing.CtimeNs == uint32(stat.Ctimespec.Nsec)

		if unchanged {
			return
		}
	}

	// Read the file content
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	// Compute new hash and create new index entry
	hash, err := plumbing.WriteObject(types.BlobObject, data)
	if err != nil {
		fmt.Println("Error hashing file object:", err)
		return
	}

	// Create new index entry
	entry, err := plumbing.GetIndexEntryFromStat(cleanPath, hash)
	if err != nil {
		fmt.Println("Error creating index entry:", err)
		return
	}

	// Update the index map
	indexMap[cleanPath] = entry
}

// Invoked from main.go. AddFiles handles the 'gegit add' command to add files to the staging area. It only calls this function if first argument is add.
func AddFiles(args []string) {

	if len(args) < 2 {
		// No files specified
		fmt.Println("usage: gegit add . | <file> [<file> ...]")
		os.Exit(1)
	}

	entries, err := plumbing.LoadIndex()
	if err != nil {
		fmt.Println("Error loading index:", err)
		os.Exit(1)
	}

	// Create a map for quick lookup of existing entries
	indexMap := plumbing.IndexToMap(entries)

	// Keep track of files in the working directory if '.' is specified
	workingSet := map[string]bool{}
	isAddAll := slices.Contains(args[1:], ".")

	// Handle the case where '.' is provided as an argument
	if isAddAll {
		_ = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				fmt.Println("Error accessing path:", err)
				return nil
			}
			// Skip the root directory itself
			if path == "." {
				return nil
			}

			// Skip the .gegit directory
			if d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}

			// Skip directories, only add files
			if d.IsDir() {
				return nil
			}

			// Add or update the file in the index
			addOrUpdatePath(path, indexMap, workingSet, true)
			return nil
		})
	} else {
		// Handle specific files
		for _, path := range args[1:] {
			addOrUpdatePath(path, indexMap, workingSet, false)
		}
	}

	if isAddAll {
		// Handle deletions: remove entries not in working set, only use in add .
		for path := range indexMap {
			if !workingSet[path] {
				delete(indexMap, path)
			}
		}
	}

	// Write to Index file
	if err = plumbing.WriteIndex(plumbing.MapToSortedIndex(indexMap)); err != nil {
		fmt.Println("Error writing to .git/index file:", err)
		return
	}
}
