package plumbing

import (
	"bytes"
	"strings"
)

// This file implements content-level 3-way merge: an LCS diff aligns each
// side against the common base, then a diff3 sweep combines the two edit
// scripts, emitting conflict markers where both sides changed the same base
// region differently.

// editRegion says: base lines [BaseStart,BaseEnd) were replaced by side lines
// [SideStart,SideEnd). BaseStart==BaseEnd is a pure insertion before base
// line BaseStart.
type editRegion struct {
	BaseStart, BaseEnd int
	SideStart, SideEnd int
}

// diffRegions computes replace-regions turning base into side via a
// longest-common-subsequence DP. O(n·m) — merges operate on source files,
// not gigabyte blobs, and correctness beats asymptotics here.
func diffRegions(base, side []string) []editRegion {
	n, m := len(base), len(side)
	dp := make([][]int32, n+1)
	for i := range dp {
		dp[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if base[i] == side[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var regions []editRegion
	i, j := 0, 0
	for i < n || j < m {
		if i < n && j < m && base[i] == side[j] {
			i++
			j++
			continue
		}
		startB, startS := i, j
		for i < n || j < m {
			if i < n && j < m && base[i] == side[j] {
				break
			}
			if i < n && (j >= m || dp[i+1][j] >= dp[i][j+1]) {
				i++
			} else {
				j++
			}
		}
		regions = append(regions, editRegion{BaseStart: startB, BaseEnd: i, SideStart: startS, SideEnd: j})
	}
	return regions
}

// regionTouches reports whether region r participates in base block [lo,hi):
// real overlaps count, and insertions count when their anchor sits in
// [lo,hi) — an insertion exactly at hi belongs to the NEXT block.
func regionTouches(r editRegion, lo, hi int) bool {
	if r.BaseStart == r.BaseEnd {
		return r.BaseStart >= lo && r.BaseStart < hi
	}
	return r.BaseStart < hi && r.BaseEnd > lo
}

// anyChangeAt reports whether a side has a region participating at base
// position i (treating [i,i+1) as the probe window).
func anyChangeAt(regions []editRegion, i int) bool {
	for _, r := range regions {
		if regionTouches(r, i, i+1) {
			return true
		}
	}
	return false
}

// blockEnd extends [lo,hi) until it covers every region of both sides that
// touches it (fixed point). Returns the final hi.
func blockEnd(lo, hi int, ours, theirs []editRegion) int {
	for {
		grew := false
		for _, rs := range [2][]editRegion{ours, theirs} {
			for _, r := range rs {
				if regionTouches(r, lo, hi) && r.BaseEnd > hi {
					hi = r.BaseEnd
					grew = true
				}
			}
		}
		if !grew {
			return hi
		}
	}
}

// mapToSide translates a base position to the side's coordinate space.
// Non-insert regions ending at or before p shift by their length delta;
// insertions strictly before p shift by their inserted length. An insertion
// exactly AT p is deliberately not counted — the caller decides which block
// owns it via the [lo,hi) convention.
func mapToSide(p int, regions []editRegion) int {
	off := 0
	for _, r := range regions {
		if r.BaseStart == r.BaseEnd {
			if r.BaseStart < p {
				off += r.SideEnd - r.SideStart
			}
			continue
		}
		if r.BaseEnd <= p {
			off += (r.SideEnd - r.SideStart) - (r.BaseEnd - r.BaseStart)
		}
	}
	return p + off
}

// sideBlock returns the side's replacement text for base block [lo,hi).
// mapToSide excludes an insertion anchored exactly at a boundary, which is
// precisely the [lo,hi) ownership convention: an insert at lo lands INSIDE
// this block (sLo points before it), an insert at hi belongs to the next.
func sideBlock(side []string, regions []editRegion, lo, hi int) []string {
	sLo := mapToSide(lo, regions)
	sHi := mapToSide(hi, regions)
	// Pure-insertion block ([lo,lo)): mapToSide excluded the insertion at lo
	// from BOTH ends, so widen the end to cover its inserted lines.
	if lo == hi {
		for _, r := range regions {
			if r.BaseStart == r.BaseEnd && r.BaseStart == lo {
				sHi += r.SideEnd - r.SideStart
			}
		}
	}
	if sLo < 0 {
		sLo = 0
	}
	if sHi > len(side) {
		sHi = len(side)
	}
	if sLo > sHi {
		sLo = sHi
	}
	return side[sLo:sHi]
}

// Merge3 performs a diff3-style line merge of ours and theirs against base.
// It returns the merged content and whether any conflict markers were
// embedded. Conflicted regions use the standard format:
//
//	<<<<<<< <oursLabel>
//	our lines
//	=======
//	their lines
//	>>>>>>> <theirsLabel>
func Merge3(base, ours, theirs []byte, oursLabel, theirsLabel string) ([]byte, bool) {
	baseLines := splitLines(base)
	ourLines := splitLines(ours)
	theirLines := splitLines(theirs)

	ourRegions := diffRegions(baseLines, ourLines)
	theirRegions := diffRegions(baseLines, theirLines)

	var out []string
	conflicted := false

	i := 0
	for i <= len(baseLines) {
		oChanged := anyChangeAt(ourRegions, i)
		tChanged := anyChangeAt(theirRegions, i)

		if !oChanged && !tChanged {
			if i < len(baseLines) {
				out = append(out, baseLines[i])
			}
			i++
			continue
		}

		// A change block starts here: find its full extent across both sides.
		lo := i
		hi := blockEnd(lo, lo+1, ourRegions, theirRegions)
		if hi > len(baseLines) {
			hi = len(baseLines)
		}

		ourSlice := sideBlock(ourLines, ourRegions, lo, hi)
		theirSlice := sideBlock(theirLines, theirRegions, lo, hi)

		// A pure-insertion block has an empty base window [lo,lo); probe
		// participation with [lo,lo+1) so the insertion anchored at lo counts.
		probeHi := hi
		if probeHi == lo {
			probeHi = lo + 1
		}

		switch {
		case linesEqual(ourSlice, theirSlice):
			out = append(out, ourSlice...) // both sides agree
		case !anyRealChange(ourRegions, lo, probeHi):
			out = append(out, theirSlice...) // only theirs changed
		case !anyRealChange(theirRegions, lo, probeHi):
			out = append(out, ourSlice...) // only ours changed
		default:
			conflicted = true
			out = append(out, "<<<<<<< "+oursLabel)
			out = append(out, ourSlice...)
			out = append(out, "=======")
			out = append(out, theirSlice...)
			out = append(out, ">>>>>>> "+theirsLabel)
		}

		if hi > lo {
			i = hi
		} else {
			// Pure-insertion block: no base lines were consumed, and by
			// construction neither side modified line lo itself (that would
			// have grown the block) — so keep the base line and move on.
			if lo < len(baseLines) {
				out = append(out, baseLines[lo])
			}
			i = lo + 1
		}
	}

	merged := strings.Join(out, "\n")
	if len(out) > 0 {
		merged += "\n"
	}
	return []byte(merged), conflicted
}

// anyRealChange reports whether the side actually modifies block [lo,hi).
func anyRealChange(regions []editRegion, lo, hi int) bool {
	for _, r := range regions {
		if regionTouches(r, lo, hi) {
			return true
		}
	}
	return false
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	s := string(bytes.TrimSuffix(b, []byte("\n")))
	return strings.Split(s, "\n")
}
