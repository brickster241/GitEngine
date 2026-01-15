package porcelain

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DefaultFilePerm = 0o644
	DefaultDirPerm  = 0o755
)

// Invoked from main.go. InitRepo handles the 'gegit init' command to initialize a new GitEngine repository. It only calls this function if first argument is init.
func InitRepo(args []string) {
	var repoPath string

	// Determine repository path
	switch len(args) {
	case 1:
		repoPath = "."
	case 2:
		repoPath = args[1]

		if err := os.MkdirAll(repoPath, DefaultDirPerm); err != nil {
			if !os.IsExist(err) {
				fmt.Println("Error Creating directory:", err)
				os.Exit(1)
			}
		}

	default:
		// Invalid usage
		fmt.Println("usage: gegit init [<directory>]")
		os.Exit(1)
	}

	// Resolve absolute path, clean path BEFORE chdir
	absRepopath, err := filepath.Abs(repoPath)
	if err != nil {
		fmt.Println("Error Resolving path:", err)
		os.Exit(1)
	}

	// Clean the path
	absRepopath = filepath.Clean(absRepopath)

	// Change working directory to the repository path
	if err := os.Chdir(absRepopath); err != nil {
		fmt.Println("Error Changing directory:", err)
		os.Exit(1)
	}

	// Check whether .gegit already exists
	reinitialize := false
	if _, err := os.Stat(".gegit"); err == nil {
		reinitialize = true
	}

	// Create .gegit directory structure
	if err := createGitDirs(); err != nil {
		fmt.Println("Error Initializing repository:", err)
		os.Exit(1)
	}

	// Success message
	gitDirPath := filepath.Join(absRepopath, ".gegit")
	if reinitialize {
		fmt.Printf("Reinitialized existing Git repository in %s\n", gitDirPath)
	} else {
		fmt.Printf("Initialized empty Git repository in %s\n", gitDirPath)
	}
}

// Invoked from initRepo function. createGitDirs initializes a new .gegit directory structure. This assumes the main repository directory already exists and is the current working directory.
func createGitDirs() error {

	// Define the necessary directory structure
	paths := []string{
		".gegit",
		".gegit/objects",
		".gegit/refs",
		".gegit/refs/heads",
		".gegit/refs/tags",
		// ".gegit/hooks",
		// ".gegit/info",
	}

	// Create the necessary directories
	for _, path := range paths {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(path, DefaultDirPerm); err != nil {
			return err
		}
	}

	// HEAD file (unborn branch)
	head := "ref: refs/heads/master\n"
	if err := os.WriteFile(".gegit/HEAD", []byte(head), DefaultFilePerm); err != nil {
		return err
	}

	// Config file (default settings)
	config := `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
	ignorecase = true
	precomposeunicode = true
	`

	// Write config file
	if err := os.WriteFile(".gegit/config", []byte(config), DefaultFilePerm); err != nil {
		return err
	}
	return nil
}
