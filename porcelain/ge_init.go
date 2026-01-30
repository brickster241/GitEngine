package porcelain

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/constants"
)

// Invoked from main.go. InitRepo handles the 'gegit init' command to initialize a new GitEngine repository. It only calls this function if first argument is init.
func InitRepo(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("init",
		"This command creates an empty Git repository or reinitializes it - basically a .git directory with subdirectories for objects, refs/heads, refs/tags, index, HEAD and config files. An initial branch without any commits will be created.",
		"gegit init [<directory>]")
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	var repoPath string

	// Determine repository path
	switch len(pos) {
	case 0:
		repoPath = "."
	case 1:
		repoPath = args[1]

		if err := os.MkdirAll(repoPath, constants.DefaultDirPerm); err != nil {
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
	if _, err := os.Stat(".git"); err == nil {
		reinitialize = true
	}

	// Create .gegit directory structure
	if err := createGitDirs(); err != nil {
		fmt.Println("Error Initializing repository:", err)
		os.Exit(1)
	}

	// Success message
	gitDirPath := filepath.Join(absRepopath, ".git")
	if reinitialize {
		fmt.Printf("Reinitialized existing Git repository in %s\n", gitDirPath)
	} else {
		fmt.Printf("Initialized empty Git repository in %s\n", gitDirPath)
	}
}

// Invoked from initRepo function. createGitDirs initializes a new .gegit directory structure. This assumes the main repository directory already exists and is the current working directory.
func createGitDirs() error {

	// Create the necessary directories
	for _, path := range constants.Dir_paths {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(path, constants.DefaultDirPerm); err != nil {
			return err
		}
	}

	// Create HEAD file which will point to master branch
	if err := os.WriteFile(".git/HEAD", []byte(constants.Head), constants.DefaultFilePerm); err != nil {
		return err
	}

	// Write config file
	if err := os.WriteFile(".git/config", []byte(constants.Config), constants.DefaultFilePerm); err != nil {
		return err
	}
	return nil
}
