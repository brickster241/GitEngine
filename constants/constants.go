package constants

const (
	RegularFileMode    = 0o100644
	ExecutableFileMode = 0o100755
	SymlinkFileMode    = 0o120000
	DefaultFilePerm    = 0o644                      // rw-r--r--
	DefaultDirPerm     = 0o755                      // rwxr-xr-x
	Head               = "ref: refs/heads/master\n" // Default .git/HEAD content
	DirModeStr         = "040000"
	FileModeStr        = "100644"
	Config             = `[core]
	repositoryformatversion = 0
	filemode = true
	bare = false
	logallrefupdates = true
	ignorecase = true
	precomposeunicode = true

[user]
	name = username
	email = user@email.com

	` // Default .git/config content
)

// Define the necessary directory structure
var Dir_paths = []string{
	".git",
	".git/objects",
	".git/refs",
	".git/refs/heads",
	".git/refs/tags",
	// ".git/hooks",
	// ".git/info",
}
