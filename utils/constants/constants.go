package constants

const (
	ModeFileStr           = "100644"
	ModeExecStr           = "100755"
	ModeSymlinkStr        = "120000"
	ModeTreeStr           = "040000"
	ModeFile       uint32 = 0100644
	ModeExec       uint32 = 0100755
	ModeSymlink    uint32 = 0120000
	ModeTree       uint32 = 0040000

	DefaultFilePerm = 0o644 // rw-r--r--
	DefaultDirPerm  = 0o755 // rwxr-xr-x
	ResetColor      = "\033[0m"
	BoldColor       = "\033[1m"
	GreenColor      = "\033[32m"
	RedColor        = "\033[31m"
	Head            = "ref: refs/heads/master\n" // Default .git/HEAD content
	DirModeStr      = "040000"
	FileModeStr     = "100644"
	Config          = `[core]
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
