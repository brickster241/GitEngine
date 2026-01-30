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

// Invoked from main.go. Computes the object ID value for an object with specified type with the contents of the named file (which can be outside of the work tree), and optionally writes the resulting object into the object database.
func HashAndWriteObject(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("hash-object",
		"Computes the object ID value for an object with specified type with the contents of the named file (which can be outside of the work tree), and optionally writes the resulting object into the object database.",
		"gegit hash-object [-w] [-t <type>] <file>")
	objType := fls.String("t", "blob", "Specify the type of object to be created (default: \"blob\"). Possible values are commit, tree, blob")
	write := fls.Bool("w", false, "Actually write the object into the object database.")

	// Parse flags from args
	fls.Parse(args[1:])

	// Remaining args after flags : check for <file-path>
	rest := fls.Args()
	if len(rest) != 1 {
		fmt.Println("usage: gegit hash-object [-w] [-t <type>] <file>")
		os.Exit(1)
	}

	// Get filename, validate objType
	cleanPath := filepath.ToSlash(filepath.Clean(rest[0]))
	if *objType != string(types.BlobObject) && *objType != string(types.CommitObject) && *objType != string(types.TreeObject) {
		fmt.Printf("Error unsupported object type: %s\n", *objType)
		os.Exit(1)
	}

	// Read File, hash it
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(1)
	}

	var sha [20]byte

	if *write {
		// Compute hash and also write in the object database
		sha, err = plumbing.WriteObject(types.ObjectType(*objType), data)
		if err != nil {
			fmt.Println("Error hashing file:", err)
			os.Exit(1)
		}

	} else {
		// Compute hash only
		sha, err = plumbing.HashObject(types.ObjectType(*objType), data)
		if err != nil {
			fmt.Println("Error hashing file:", err)
			os.Exit(1)
		}
	}

	// Output the Encoded sha value.
	fmt.Println(hex.EncodeToString(sha[:]))
}
