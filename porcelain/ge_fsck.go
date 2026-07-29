package porcelain

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/types"
)

// Invoked from main.go. FsckObjects handles 'gegit fsck': walk every loose
// object in .git/objects, re-hash its content to prove the SHA on disk is
// the SHA of the bytes, and verify that every tree/commit reference resolves
// — the integrity check a production object store owes its users.
func FsckObjects(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("fsck",
		"Verify the connectivity and validity of every object in the object database: re-hash contents against their SHA and resolve all tree/commit references.",
		"gegit fsck")
	fls.Parse(args[1:])

	start := time.Now()
	counts := map[types.ObjectType]int{}
	corrupt := 0
	dangling := 0

	objectsDir := filepath.Join(".git", "objects")
	dirs, err := os.ReadDir(objectsDir)
	if err != nil {
		fmt.Println("Error reading objects directory:", err)
		os.Exit(1)
	}

	for _, d := range dirs {
		// Loose object fan-out dirs are exactly two hex chars.
		if !d.IsDir() || len(d.Name()) != 2 {
			continue
		}
		files, err := os.ReadDir(filepath.Join(objectsDir, d.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			shaHex := d.Name() + f.Name()

			// Read + inflate, then re-hash: the object's name must be the
			// SHA-1 of its canonical "<type> <size>\0<content>" encoding.
			objType, content, err := plumbing.ReadObject(shaHex)
			if err != nil {
				fmt.Printf("error: %s: unreadable object (%v)\n", shaHex, err)
				corrupt++
				continue
			}
			recomputed, _ := plumbing.HashObject(objType, content)
			if hex.EncodeToString(recomputed[:]) != shaHex {
				fmt.Printf("error: %s: hash mismatch — object store corruption\n", shaHex)
				corrupt++
				continue
			}
			counts[objType]++

			// Referential integrity: trees must point at existing objects,
			// commits at existing trees/parents.
			switch objType {
			case types.TreeObject:
				entries, err := plumbing.ReadTreeCurrentLevel(shaHex)
				if err != nil {
					fmt.Printf("error: %s: unparseable tree (%v)\n", shaHex, err)
					corrupt++
					continue
				}
				for _, e := range entries {
					if _, _, err := plumbing.ReadObject(hex.EncodeToString(e.SHA[:])); err != nil {
						fmt.Printf("dangling reference: tree %s -> %s (%s)\n",
							shaHex[:7], hex.EncodeToString(e.SHA[:])[:7], e.Name)
						dangling++
					}
				}
			case types.CommitObject:
				var c [20]byte
				copy(c[:], recomputed[:])
				commit, err := plumbing.ReadCommit(c)
				if err != nil {
					fmt.Printf("error: %s: unparseable commit (%v)\n", shaHex, err)
					corrupt++
					continue
				}
				if _, _, err := plumbing.ReadObject(hex.EncodeToString(commit.TreeSHA[:])); err != nil {
					fmt.Printf("dangling reference: commit %s -> tree %s\n",
						shaHex[:7], hex.EncodeToString(commit.TreeSHA[:])[:7])
					dangling++
				}
				for _, p := range commit.ParentsSHA {
					if _, _, err := plumbing.ReadObject(hex.EncodeToString(p[:])); err != nil {
						fmt.Printf("dangling reference: commit %s -> parent %s\n",
							shaHex[:7], hex.EncodeToString(p[:])[:7])
						dangling++
					}
				}
			}
		}
	}

	elapsed := time.Since(start)
	total := counts[types.BlobObject] + counts[types.TreeObject] + counts[types.CommitObject]
	rate := float64(total) / elapsed.Seconds()

	fmt.Printf("Verified %d objects (%d blobs, %d trees, %d commits) in %.1fms — %.0f objects/sec\n",
		total, counts[types.BlobObject], counts[types.TreeObject], counts[types.CommitObject],
		float64(elapsed.Microseconds())/1000, rate)
	if corrupt == 0 && dangling == 0 {
		fmt.Println("Object store integrity: OK (0 corrupt, 0 dangling)")
	} else {
		fmt.Printf("Object store integrity: FAILED (%d corrupt, %d dangling)\n", corrupt, dangling)
		os.Exit(1)
	}
}
