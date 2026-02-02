package porcelain

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/types"
)

// Invoked from main.go. RegisterFileAndUpdateIndex handles the 'gegit update-index' command to register file contents in the working tree to the index.
func RegisterFileAndUpdateIndex(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("update-index",
		"Register file contents in the working tree to the index using mode, object sha, and file path.",
		"gegit update-index --cacheinfo <mode> <object> <file>")
	cacheInfo := fls.Bool("cacheinfo", false, "Directly insert the specified <mode>, <object> and <file> into the index.")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// Check if there are no Args / cacheinfo is not present
	if !*cacheInfo || len(pos) != 3 {
		fmt.Println("usage: gegit update-index --cacheinfo <mode> <object> <file>")
		os.Exit(1)
	}

	// Mode, shaHex, and filePath
	mode, shaHex, fp := pos[0], pos[1], pos[2]

	if len(shaHex) == 40 {

		// Clean Path
		cleanPath := filepath.ToSlash(filepath.Clean(fp))

		// Load Index
		entries, err := plumbing.LoadIndex()
		if err != nil {
			fmt.Println("Error loading index:", err)
			os.Exit(1)
		}

		// Check whether it exists, and add it accordingly.
		idx := -1
		for i := range entries {
			if entries[i].Filename == cleanPath {
				idx = i
				break
			}
		}

		// Check whether shaHex is valid.
		sha, err := hex.DecodeString(shaHex)
		if err != nil {
			fmt.Println("Error decoding <object> hex")
			os.Exit(1)
		}

		// Check whether Mode is valid.
		uint32Mode, err := utils.ParseModeStr(mode)
		if err != nil {
			fmt.Println("Error parsing Mode string:", err)
			os.Exit(1)
		}

		if idx != -1 {
			// Already file exists in Index, just overwrite it without doing any other changes.
			entries[idx].SHA1 = [20]byte(sha)
			entries[idx].Mode = uint32Mode
			entries[idx].Filename = cleanPath
		} else {
			entries = append(entries, types.IndexEntry{
				SHA1:     [20]byte(sha),
				Mode:     uint32Mode,
				Filename: cleanPath,
			})

		}

		// Write to Index (Will sort entries based on Filename)
		if err := plumbing.WriteIndex(entries); err != nil {
			fmt.Println("Error updating Index:", err)
			os.Exit(1)
		}

	} else {
		// Print out generic message
		fmt.Println("error: invalid object id")
		os.Exit(1)
	}

}
