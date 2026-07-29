package plumbing

import (
	"strings"
	"testing"
)

func mustMerge(t *testing.T, base, ours, theirs string, wantConflict bool) string {
	t.Helper()
	merged, conflicted := Merge3([]byte(base), []byte(ours), []byte(theirs), "ours", "theirs")
	if conflicted != wantConflict {
		t.Fatalf("conflicted = %v, want %v\nmerged:\n%s", conflicted, wantConflict, merged)
	}
	return string(merged)
}

func TestMerge3DistinctRegionsMergeClean(t *testing.T) {
	base := "a\nb\nc\nd\ne\n"
	ours := "A\nb\nc\nd\ne\n"   // edits line 1
	theirs := "a\nb\nc\nd\nE\n" // edits line 5
	got := mustMerge(t, base, ours, theirs, false)
	if got != "A\nb\nc\nd\nE\n" {
		t.Fatalf("got:\n%q", got)
	}
}

func TestMerge3SameChangeBothSides(t *testing.T) {
	base := "a\nb\nc\n"
	both := "a\nX\nc\n"
	got := mustMerge(t, base, both, both, false)
	if got != "a\nX\nc\n" {
		t.Fatalf("got:\n%q", got)
	}
}

func TestMerge3ConflictOnSameLine(t *testing.T) {
	base := "a\nb\nc\n"
	ours := "a\nOURS\nc\n"
	theirs := "a\nTHEIRS\nc\n"
	got := mustMerge(t, base, ours, theirs, true)
	for _, marker := range []string{"<<<<<<< ours", "OURS", "=======", "THEIRS", ">>>>>>> theirs"} {
		if !strings.Contains(got, marker) {
			t.Fatalf("missing %q in:\n%s", marker, got)
		}
	}
}

func TestMerge3TheirsOnlyChange(t *testing.T) {
	base := "a\nb\nc\n"
	theirs := "a\nb2\nc\nextra\n"
	got := mustMerge(t, base, base, theirs, false)
	if got != theirs {
		t.Fatalf("got %q, want %q", got, theirs)
	}
}

func TestMerge3OursDeletesTheirsUntouched(t *testing.T) {
	base := "a\nb\nc\n"
	ours := "a\nc\n"
	got := mustMerge(t, base, ours, base, false)
	if got != "a\nc\n" {
		t.Fatalf("got %q", got)
	}
}

func TestMerge3InsertionsAtDistinctPoints(t *testing.T) {
	base := "a\nb\n"
	ours := "start\na\nb\n"
	theirs := "a\nb\nend\n"
	got := mustMerge(t, base, ours, theirs, false)
	if got != "start\na\nb\nend\n" {
		t.Fatalf("got %q", got)
	}
}

func TestMerge3EmptyBaseBothAddDifferent(t *testing.T) {
	got := mustMerge(t, "", "ours\n", "theirs\n", true)
	if !strings.Contains(got, "<<<<<<<") {
		t.Fatalf("expected conflict markers, got %q", got)
	}
}
