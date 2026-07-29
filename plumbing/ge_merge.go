package plumbing

import (
	"encoding/hex"

	"github.com/brickster241/GitEngine/utils/types"
)

// MergeConflict describes one path the 3-way tree merge could not resolve.
// Content carries the conflict-marked file body to place in the worktree.
type MergeConflict struct {
	Path    string
	Kind    string // "content" or "modify/delete"
	Content []byte
}

// MergeTreesThreeWay merges ourTree and theirTree against their common
// ancestor baseTree. It returns the merged path→entry map (the future index /
// tree) and the list of conflicts. Blob-level double edits go through Merge3;
// a conflicted path stays in the result carrying its marker-annotated blob so
// the caller can materialize it in the worktree.
func MergeTreesThreeWay(baseTree, ourTree, theirTree [20]byte, oursLabel, theirsLabel string) (map[string]types.TreeEntry, []MergeConflict, error) {

	// Flatten all three trees to path → entry.
	base, err := FlattenTree(baseTree)
	if err != nil {
		return nil, nil, err
	}
	ours, err := FlattenTree(ourTree)
	if err != nil {
		return nil, nil, err
	}
	theirs, err := FlattenTree(theirTree)
	if err != nil {
		return nil, nil, err
	}

	// Union of every blob path across the three trees (tree entries are
	// implied by their blobs).
	paths := map[string]bool{}
	for _, m := range []map[string]types.TreeEntry{base, ours, theirs} {
		for p, e := range m {
			if e.Type == types.BlobObject {
				paths[p] = true
			}
		}
	}

	merged := map[string]types.TreeEntry{}
	var conflicts []MergeConflict

	for path := range paths {
		b, hasB := base[path]
		o, hasO := ours[path]
		t, hasT := theirs[path]

		switch {
		// Identical on both sides (same SHA or both absent): take ours.
		case hasO == hasT && (!hasO || o.SHA == t.SHA):
			if hasO {
				merged[path] = o
			}

		// Ours untouched since base: theirs wins (including their delete).
		case hasB == hasO && (!hasB || b.SHA == o.SHA):
			if hasT {
				merged[path] = t
			}

		// Theirs untouched since base: ours wins (including our delete).
		case hasB == hasT && (!hasB || b.SHA == t.SHA):
			if hasO {
				merged[path] = o
			}

		// One side deleted, the other modified: modify/delete conflict —
		// keep the modified version in the worktree, flag the path.
		case !hasO || !hasT:
			kept := o
			if !hasO {
				kept = t
			}
			_, content, err := ReadObject(hex.EncodeToString(kept.SHA[:]))
			if err != nil {
				return nil, nil, err
			}
			merged[path] = kept
			conflicts = append(conflicts, MergeConflict{Path: path, Kind: "modify/delete", Content: content})

		// Both sides modified: content-level 3-way merge.
		default:
			var baseContent []byte
			if hasB {
				_, baseContent, err = ReadObject(hex.EncodeToString(b.SHA[:]))
				if err != nil {
					return nil, nil, err
				}
			}
			_, ourContent, err := ReadObject(hex.EncodeToString(o.SHA[:]))
			if err != nil {
				return nil, nil, err
			}
			_, theirContent, err := ReadObject(hex.EncodeToString(t.SHA[:]))
			if err != nil {
				return nil, nil, err
			}

			mergedContent, hasConflict := Merge3(baseContent, ourContent, theirContent, oursLabel, theirsLabel)
			blobSHA, err := WriteObject(types.BlobObject, mergedContent)
			if err != nil {
				return nil, nil, err
			}
			entry := types.TreeEntry{Name: path, Mode: o.Mode, SHA: blobSHA, Type: types.BlobObject}
			merged[path] = entry
			if hasConflict {
				conflicts = append(conflicts, MergeConflict{Path: path, Kind: "content", Content: mergedContent})
			}
		}
	}

	return merged, conflicts, nil
}

// EntriesToIndex converts a merged path→entry map into index entries ready
// for BuildTreeFromIndex / WriteIndexLocked.
func EntriesToIndex(merged map[string]types.TreeEntry) []types.IndexEntry {
	entries := make([]types.IndexEntry, 0, len(merged))
	for path, e := range merged {
		if e.Type != types.BlobObject {
			continue
		}
		entries = append(entries, types.IndexEntry{Filename: path, SHA1: e.SHA, Mode: e.Mode})
	}
	return entries
}
