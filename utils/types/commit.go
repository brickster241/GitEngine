package types

// CommitNode represents a commit object
type CommitNode struct {
	treeSHA    [20]byte   // root tree SHA
	parentsSHA [][20]byte // parents commit SHA, can be multiple for merges
	author     Author     // author info
	committer  string     // committer info
	message    string     // commit message
}

type Author struct {
	Name  string
	Email string
}
