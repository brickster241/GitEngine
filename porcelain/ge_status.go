package porcelain

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/brickster241/GitEngine/plumbing"
)

// Invoked from main.go. ShowStatus handles the 'gegit status' command to show the working tree status.
func ShowStatus(args []string) {

	if len(args) != 1 {
		// Invalid usage
		fmt.Println("usage: gegit status")
		os.Exit(1)
	}

	// Load the index
	entries, err := plumbing.LoadIndex()
	if err != nil {
		fmt.Println("Error loading index:", err)
		return
	}

	// Create a map for quick lookup of existing entries
	indexMap := plumbing.IndexToMap(entries)

	// Keep track of files in the working directory.
	workingSet := map[string]bool{}
	tracked, modified, newFiles, deleted := []string{}, []string{}, []string{}, []string{}

	// Walk the working directory to find all files
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

		// Clean and normalize the path
		cleanPath := filepath.ToSlash(filepath.Clean(path))
		workingSet[cleanPath] = true

		// Get file info
		info, err := os.Stat(cleanPath)
		if err != nil {
			fmt.Println("Error stating file:", err)
			os.Exit(1)
		}
		stat := info.Sys().(*syscall.Stat_t)

		// Check if already tracked i.e. present in index
		existing, isTracked := indexMap[cleanPath]

		// Keep track of modified, new and deleted files
		if isTracked {
			unchanged :=
				existing.Dev == uint32(stat.Dev) &&
					existing.Ino == uint32(stat.Ino) &&
					existing.FileSize == uint32(info.Size()) &&
					existing.Mtime == uint32(stat.Mtimespec.Sec) &&
					existing.MtimeNs == uint32(stat.Mtimespec.Nsec) &&
					existing.Ctime == uint32(stat.Ctimespec.Sec) &&
					existing.CtimeNs == uint32(stat.Ctimespec.Nsec)

			if unchanged {
				// File is unchanged and staged
				tracked = append(tracked, cleanPath)
			} else {
				// File is modified but not staged
				modified = append(modified, cleanPath)
			}
		} else {
			// New file, untracked not present in index
			newFiles = append(newFiles, cleanPath)
		}
		return nil
	})

	// Check for deleted files
	for filename := range indexMap {
		// If file in index is not in working set, it is deleted
		if _, exists := workingSet[filename]; !exists {
			deleted = append(deleted, filename)
		}
	}

	// Sort the lists for consistent output
	sort.Strings(tracked)
	sort.Strings(modified)
	sort.Strings(newFiles)
	sort.Strings(deleted)

	// Print the status
	fmt.Println("On branch master")

	// Print tracked files
	if len(tracked) > 0 {
		fmt.Println("\nChanges to be committed:")
		for _, f := range tracked {
			fmt.Printf("\tnewFile/modified/deleted:   %s\n", f)
		}
	}
	if len(modified) > 0 || len(deleted) > 0 {
		fmt.Println("\nChanges not staged for commit:")
		for _, f := range modified {
			fmt.Printf("\tmodified:   %s\n", f)
		}
		for _, f := range deleted {
			fmt.Printf("\tdeleted:    %s\n", f)
		}
	}
	// Print untracked files
	if len(newFiles) > 0 {
		fmt.Println("\nUntracked files:")
		for _, f := range newFiles {
			fmt.Printf("\t%s\n", f)
		}
	}
}
