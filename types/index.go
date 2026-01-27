package types

// IndexEntry represents a single entry in the Git index (staging area).
type IndexEntry struct {
	Ctime    uint32   // seconds since epoch
	CtimeNs  uint32   // nanoseconds
	Mtime    uint32   // seconds since epoch
	MtimeNs  uint32   // nanoseconds
	Dev      uint32   // device
	Ino      uint32   // inode
	Mode     uint32   // file mode - 0100644 for regular file
	Uid      uint32   // user id
	Gid      uint32   // group id
	FileSize uint32   // size in bytes
	SHA1     [20]byte // SHA-1 hash of the file content
	Flags    uint16   // flags
	Filename string   // file name
}
