package plumbing

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/brickster241/GitEngine/utils/constants"
	"github.com/brickster241/GitEngine/utils/types"
)

// LoadIndex reads the index file and returns the list of IndexEntry.
func LoadIndex() ([]types.IndexEntry, error) {

	indexPath := filepath.Join(".git", "index")
	if _, err := os.Stat(indexPath); errors.Is(err, os.ErrNotExist) {
		return []types.IndexEntry{}, nil // No index file yet
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
	entries := make([]types.IndexEntry, 0, entryCount)
	offset := 12

	// Loop through entries
	for i := uint32(0); i < entryCount; i++ {
		entryStart := offset // Track where this entry starts
		if offset+62 > len(content) {
			return nil, fmt.Errorf("corrupt index entry")
		}

		// Read fixed-size fields
		var ie types.IndexEntry
		ie.Ctime = binary.BigEndian.Uint32(content[offset:])
		offset += 4
		ie.CtimeNs = binary.BigEndian.Uint32(content[offset:])
		offset += 4
		ie.Mtime = binary.BigEndian.Uint32(content[offset:])
		offset += 4
		ie.MtimeNs = binary.BigEndian.Uint32(content[offset:])
		offset += 4
		ie.Dev = binary.BigEndian.Uint32(content[offset:])
		offset += 4
		ie.Ino = binary.BigEndian.Uint32(content[offset:])
		offset += 4
		ie.Mode = binary.BigEndian.Uint32(content[offset:])
		offset += 4
		ie.Uid = binary.BigEndian.Uint32(content[offset:])
		offset += 4
		ie.Gid = binary.BigEndian.Uint32(content[offset:])
		offset += 4
		ie.FileSize = binary.BigEndian.Uint32(content[offset:])
		offset += 4

		copy(ie.SHA1[:], content[offset:offset+20])
		offset += 20

		// Read flags, including filename length
		ie.Flags = binary.BigEndian.Uint16(content[offset:])
		offset += 2

		start := offset
		for offset < len(content) && content[offset] != 0 {
			offset++
		}
		if offset >= len(content) {
			return nil, fmt.Errorf("unterminated filename in index")
		}

		ie.Filename = string(content[start:offset])
		offset++ // Skip null terminator

		// Align to next multiple of 8 bytes FROM THE ENTRY START
		entryLen := offset - entryStart
		for (entryLen % 8) != 0 {
			offset++
			entryLen++
		}

		// Append entry to list
		entries = append(entries, ie)
	}

	return entries, nil
}

// WriteIndex writes entries back to .git/index (handles adding each entry + checksum)
func WriteIndex(entries []types.IndexEntry) error {

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

		// Add null terminator
		buffer = append(buffer, 0x00)

		// Padding: entries must be padded to multiple of 8 bytes from entryStart
		entryLen := len(buffer) - entryStart
		padLen := (8 - (entryLen % 8)) % 8
		buffer = append(buffer, make([]byte, padLen)...)
	}

	// 20-byte SHA-1 checksum of all previous contents
	hash := sha1.Sum(buffer)
	buffer = append(buffer, hash[:]...)

	// Write updated index file
	if err := os.WriteFile(filepath.Join(".git", "index"), buffer, constants.DefaultFilePerm); err != nil {
		return err
	}
	return nil
}

// IndexToMap converts entries to map for fast lookup
func IndexToMap(entries []types.IndexEntry) map[string]types.IndexEntry {
	indexMap := map[string]types.IndexEntry{}
	for _, e := range entries {
		indexMap[e.Filename] = e
	}
	return indexMap
}

// MapToSortedIndex converts an index map back into a sorted slice. Entries are sorted lexicographically by filename, as required by Git.
func MapToSortedIndex(indexMap map[string]types.IndexEntry) []types.IndexEntry {

	entries := make([]types.IndexEntry, 0, len(indexMap))
	for _, entry := range indexMap {
		entries = append(entries, entry)
	}

	// Sort based on filename lexicographically
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Filename < entries[j].Filename
	})

	return entries
}

// GetIndexEntryFromStat creates a fully populated index entry from the current filesystem state of the given path.
func GetIndexEntryFromStat(path string, sha1sum [20]byte) (types.IndexEntry, error) {

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return types.IndexEntry{}, err
	}

	// clean the path to use as filename
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		return types.IndexEntry{}, fmt.Errorf("absolute paths are not supported in index entries")
	}
	cleanPath = filepath.ToSlash(cleanPath) // Use forward slashes

	// Get system-specific file info
	stat := info.Sys().(*syscall.Stat_t)
	return types.IndexEntry{
		Ctime:    uint32(stat.Ctimespec.Sec),
		CtimeNs:  uint32(stat.Ctimespec.Nsec),
		Mtime:    uint32(stat.Mtimespec.Sec),
		MtimeNs:  uint32(stat.Mtimespec.Nsec),
		Dev:      uint32(stat.Dev),
		Ino:      uint32(stat.Ino),
		Mode:     constants.ModeFile,
		Uid:      uint32(stat.Uid),
		Gid:      uint32(stat.Gid),
		FileSize: uint32(info.Size()),
		SHA1:     sha1sum,
		Filename: cleanPath,
	}, nil
}
