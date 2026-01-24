package porcelain

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

const (
	RegularFileMode    = 0o100644
	ExecutableFileMode = 0o100755
	SymlinkFileMode    = 0o120000
)

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

// upsertIndexEntry updates an existing entry or adds a new one.
func upsertIndexEntry(entries []IndexEntry, newEntry IndexEntry) []IndexEntry {
	for i, e := range entries {
		if e.Filename == newEntry.Filename {
			entries[i] = newEntry
			return entries
		}
	}
	return append(entries, newEntry)
}

// Invoked from main.go. AddFiles handles the 'gegit add' command to add files to the staging area. It only calls this function if first argument is add.
func AddFiles(args []string) {

	indexPath := filepath.Join(".gegit", "index")
	entries, err := loadIndex(indexPath)
	if err != nil {
		fmt.Println("Error loading index:", err)
		return
	}

	// Process each file argument
	for _, path := range args {
		hash, err := hashFileObject(path)
		if err != nil {
			fmt.Println("Error hashing file:", err)
			continue
		}
		entry, err := newIndexEntry(path, hash)
		if err != nil {
			fmt.Println("Error creating index entry:", err)
			continue
		}
		entries = upsertIndexEntry(entries, entry)
	}

	// Sort entries by filename
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Filename < entries[j].Filename
	})

	// Build new index buffer
	indexBuffer, err := buildIndexBuffer(entries)
	if err != nil {
		fmt.Println("Error building index buffer:", err)
		return
	}

	// Write updated index file
	if err := os.WriteFile(indexPath, indexBuffer, DefaultFilePerm); err != nil {
		fmt.Println("Error writing index file:", err)
		return
	}
}

// hashFileObject creates a blob object for the given file and stores it in the object database.
func hashFileObject(path string) ([20]byte, error) {

	// Read the file content
	data, err := os.ReadFile(path)
	if err != nil {
		return [20]byte{}, err
	}

	// Create the content to store
	header := fmt.Sprintf("blob %d\x00", len(data))
	store := append([]byte(header), data...)

	// Compute SHA-1 hash
	sum := sha1.Sum(store)
	hash := hex.EncodeToString(sum[:])

	// Prepare the object file path
	dir := filepath.Join(".gegit", "objects", hash[:2])
	file := filepath.Join(dir, hash[2:])

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, DefaultDirPerm); err != nil {
		return [20]byte{}, err
	}

	// Z-lib compress and write the object
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(store); err != nil {
		return [20]byte{}, err
	}

	w.Close()

	// Write to file
	if err := os.WriteFile(file, buf.Bytes(), DefaultFilePerm); err != nil {
		return [20]byte{}, err
	}

	return sum, nil
}

// buildIndexBuffer returns serialized index bytes.
func buildIndexBuffer(entries []IndexEntry) ([]byte, error) {
	var buffer []byte

	// 12-byte header: "DIRC" + version(2) + entry count
	buffer = append(buffer, []byte("DIRC")...)
	buffer = binary.BigEndian.AppendUint32(buffer, 2)                    // version 2
	buffer = binary.BigEndian.AppendUint32(buffer, uint32(len(entries))) // entry count

	// Add each index entry
	for _, entry := range entries {

		entryStart := len(buffer)

		// 40 bytes of metadata
		buffer = binary.BigEndian.AppendUint32(buffer, entry.Ctime)
		buffer = binary.BigEndian.AppendUint32(buffer, entry.CtimeNs)
		buffer = binary.BigEndian.AppendUint32(buffer, entry.Mtime)
		buffer = binary.BigEndian.AppendUint32(buffer, entry.MtimeNs)
		buffer = binary.BigEndian.AppendUint32(buffer, entry.Dev)
		buffer = binary.BigEndian.AppendUint32(buffer, entry.Ino)
		buffer = binary.BigEndian.AppendUint32(buffer, entry.Mode)
		buffer = binary.BigEndian.AppendUint32(buffer, entry.Uid)
		buffer = binary.BigEndian.AppendUint32(buffer, entry.Gid)
		buffer = binary.BigEndian.AppendUint32(buffer, entry.FileSize)

		// 20 bytes SHA-1
		buffer = append(buffer, entry.SHA1[:]...)

		// Get actual filename length
		nameLen := len(entry.Filename)

		// Flags field only has 12 bits for length (max 4095)
		// If filename is longer, store max value in flags
		if nameLen > 0xFFF { // 0xFFF = 4095 = 12 bits all set
			nameLen = 0xFFF
		}

		// Write the (possibly capped) length to flags field
		buffer = binary.BigEndian.AppendUint16(buffer, uint16(nameLen))

		// Write the FULL filename (not truncated!)
		buffer = append(buffer, []byte(entry.Filename)...)

		// Padding: entries must be padded to multiple of 8 bytes
		// Minimum 1 null byte, upto 8 null bytes
		entryLen := len(buffer) - entryStart
		padLen := (8 - (entryLen % 8)) % 8
		buffer = append(buffer, make([]byte, padLen)...)
	}

	// 20-byte SHA-1 checksum of all previous contents
	hash := sha1.Sum(buffer)
	buffer = append(buffer, hash[:]...)

	return buffer, nil
}

// loadIndex reads the index file and returns the list of IndexEntry.
func loadIndex(indexPath string) ([]IndexEntry, error) {

	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		return []IndexEntry{}, nil // No index file yet
	}

	// Read the entire index file
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	// Check index file size
	if len(data) < 12+20 {
		return nil, fmt.Errorf("index file is too short")
	}

	// Validate header, version and get entry count
	if string(data[:4]) != "DIRC" {
		return nil, fmt.Errorf("invalid index file header")
	}

	version := binary.BigEndian.Uint32(data[4:8])
	if version != 2 {
		return nil, fmt.Errorf("unsupported index version: %d", version)
	}

	entryCount := binary.BigEndian.Uint32(data[8:12])

	// Parse entries
	content := data[:len(data)-20]
	entries := make([]IndexEntry, 0, entryCount)
	offset := 12

	// Loop through entries
	for i := uint32(0); i < entryCount; i++ {
		if offset+62 > len(content) {
			return nil, fmt.Errorf("corrupt index entry")
		}

		// Read fixed-size fields
		var ie IndexEntry
		ie.Ctime = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		ie.CtimeNs = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		ie.Mtime = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		ie.MtimeNs = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		ie.Dev = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		ie.Ino = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		ie.Mode = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		ie.Uid = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		ie.Gid = binary.BigEndian.Uint32(data[offset:])
		offset += 4
		ie.FileSize = binary.BigEndian.Uint32(data[offset:])
		offset += 4

		copy(ie.SHA1[:], data[offset:offset+20])
		offset += 20

		// Read flags, including filename length
		flags := binary.BigEndian.Uint16(data[offset:])
		offset += 2

		nameLen := int(flags & 0x0FFF) // lower 12 bits

		// Read filename
		if offset+nameLen > len(content) {
			return nil, fmt.Errorf("corrupt index entry filename")
		}

		ie.Filename = string(data[offset : offset+nameLen])
		offset += nameLen

		for offset%8 != 0 {
			offset++
		}

		// Append entry to list
		entries = append(entries, ie)
	}

	return entries, nil
}

// newIndexEntry creates a new IndexEntry for the given file path and SHA-1 hash.
func newIndexEntry(path string, sha1sum [20]byte) (IndexEntry, error) {

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return IndexEntry{}, err
	}

	// clean the path to use as filename
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		return IndexEntry{}, fmt.Errorf("absolute paths are not supported in index entries")
	}
	cleanPath = filepath.ToSlash(cleanPath) // Use forward slashes

	// Get system-specific file info
	stat := info.Sys().(*syscall.Stat_t)
	return IndexEntry{
		Ctime:    uint32(stat.Ctimespec.Sec),
		CtimeNs:  uint32(stat.Ctimespec.Nsec),
		Mtime:    uint32(stat.Mtimespec.Sec),
		MtimeNs:  uint32(stat.Mtimespec.Nsec),
		Dev:      uint32(stat.Dev),
		Ino:      uint32(stat.Ino),
		Mode:     RegularFileMode,
		Uid:      uint32(stat.Uid),
		Gid:      uint32(stat.Gid),
		FileSize: uint32(info.Size()),
		SHA1:     sha1sum,
		Filename: cleanPath,
	}, nil
}
