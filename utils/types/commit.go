package types

// CommitNode represents a commit object
type CommitNode struct {
	TreeSHA    [20]byte   // root tree SHA
	ParentsSHA [][20]byte // parents commit SHA, can be multiple for merges
	Author     Author     // author info
	Committer  string     // committer info
	Message    string     // commit message
}

// Author Info is stored in this struct
type Author struct {
	Name  string
	Email string
}
