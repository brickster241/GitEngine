package types

// HeadInfo represents the state of .git/HEAD
type HeadInfo struct {
	Branch   string   // refs/heads/<branch> (empty if detached)
	SHA      [20]byte // valid if detached
	Detached bool
}
