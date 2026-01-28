package types

// HeadInfo represents the state of .git/HEAD
type HeadInfo struct {
	Ref      string   // refs/heads/branchName (empty if detached)
	SHA      [20]byte // valid if detached
	Detached bool
}
