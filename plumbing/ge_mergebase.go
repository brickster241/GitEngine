package plumbing

import (
	"fmt"
)

// MergeBase returns the best common ancestor of two commits — the commit used
// as the base of a 3-way merge. Ancestors of a are collected breadth-first,
// then b's history is walked breadth-first; the first commit already seen from
// a's side is the merge base (for criss-cross histories this returns one valid
// base, like git merge-base without --all).
func MergeBase(a, b [20]byte) ([20]byte, error) {

	// Collect every ancestor of a (including a itself).
	ancestors := map[[20]byte]bool{}
	queue := [][20]byte{a}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if ancestors[curr] {
			continue
		}
		ancestors[curr] = true
		commit, err := ReadCommit(curr)
		if err != nil {
			return [20]byte{}, err
		}
		queue = append(queue, commit.ParentsSHA...)
	}

	// Walk b's history breadth-first; first hit in a's ancestor set wins.
	seen := map[[20]byte]bool{}
	queue = [][20]byte{b}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if seen[curr] {
			continue
		}
		seen[curr] = true
		if ancestors[curr] {
			return curr, nil
		}
		commit, err := ReadCommit(curr)
		if err != nil {
			return [20]byte{}, err
		}
		queue = append(queue, commit.ParentsSHA...)
	}
	return [20]byte{}, fmt.Errorf("no common ancestor found")
}

// IsAncestor reports whether ancestor is reachable from tip.
func IsAncestor(ancestor, tip [20]byte) (bool, error) {
	if ancestor == tip {
		return true, nil
	}
	seen := map[[20]byte]bool{}
	queue := [][20]byte{tip}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if seen[curr] {
			continue
		}
		seen[curr] = true
		if curr == ancestor {
			return true, nil
		}
		commit, err := ReadCommit(curr)
		if err != nil {
			return false, err
		}
		queue = append(queue, commit.ParentsSHA...)
	}
	return false, nil
}

// RevListFirstParent returns the commits from tip back to (excluding) stop,
// following only first parents, ordered OLDEST first — exactly the commits a
// rebase must replay.
func RevListFirstParent(tip, stop [20]byte) ([][20]byte, error) {
	var chain [][20]byte
	curr := tip
	for curr != stop {
		chain = append(chain, curr)
		commit, err := ReadCommit(curr)
		if err != nil {
			return nil, err
		}
		if len(commit.ParentsSHA) == 0 {
			break
		}
		curr = commit.ParentsSHA[0]
	}
	// Reverse in place: oldest first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}
