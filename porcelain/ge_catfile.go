package porcelain

import (
	"fmt"
	"os"
	"slices"
)

// Invoked from main.go. CatFileObject handles the 'gegit cat-file' command to display type, size or content for a specific repo object.
func CatFileRepoObject(args []string) {

	// Check args length and whether acceptable flag is present.
	flags := []string{"-p", "-s", "-t"}
	if len(args) != 3 || !slices.Contains(flags, args[1]) || len(args[2]) != 40 {
		fmt.Println("usage: gegit cat-file -p|-s|-t <sha>")
		os.Exit(1)
	}

	// Check whether SHA is valid, and exists in .git/objects

	// Decompress zlib

	// Content is : "<type> <size>\0<body>"

	// Print based on flags

}
