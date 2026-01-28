package plumbing

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/brickster241/GitEngine/utils/constants"
	"github.com/brickster241/GitEngine/utils/types"
)

// HashObject computes the SHA-1 hash of a Git object WITHOUT writing it to disk. It constructs the canonical Git object format "<type> <size>\0<content>".
func HashObject(objType types.ObjectType, content []byte) ([20]byte, error) {
	header := fmt.Sprintf("%s %d\x00", objType, len(content))
	store := append([]byte(header), content...)

	return sha1.Sum(store), nil
}

// WriteObject writes a Git object (blob, tree, or commit) to .git/objects. If the object already exists, it is NOT rewritten.
func WriteObject(objType types.ObjectType, content []byte) ([20]byte, error) {

	// Get SHA-1 Hash for file content
	sha, err := HashObject(objType, content)
	if err != nil {
		return [20]byte{}, err
	}

	// Get SHA Hex, then calculate dir/path (aa/bbbbb....)
	hexSha := hex.EncodeToString(sha[:])
	dir := filepath.Join(".git", "objects", hexSha[:2])
	filePath := filepath.Join(dir, hexSha[2:])

	// If object already exists, do nothing
	if _, err := os.Stat(filePath); err == nil {
		return sha, nil
	} else if !os.IsNotExist(err) {
		return [20]byte{}, err
	}

	// Create directory
	if err := os.MkdirAll(dir, constants.DefaultDirPerm); err != nil {
		return [20]byte{}, err
	}

	// "<type> <size>\0<content>"
	header := fmt.Sprintf("%s %d\x00", objType, len(content))
	store := append([]byte(header), content...)

	// Z-lib compress and write the object
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(store); err != nil {
		return [20]byte{}, err
	}

	// Close the writer
	if err := w.Close(); err != nil {
		return [20]byte{}, err
	}

	// Write to filePath
	if err := os.WriteFile(filePath, buf.Bytes(), constants.DefaultFilePerm); err != nil {
		return [20]byte{}, err
	}

	return sha, nil
}

// ReadObject reads and inflates a Git object from .git/objects. It returns: object type (blob/tree/commit), raw content (WITHOUT header), error if any
func ReadObject(shaHex string) (types.ObjectType, []byte, error) {

	// Check SHA length
	if len(shaHex) != 40 {
		return "", nil, fmt.Errorf("invalid SHA length")
	}

	// Read File at path
	filePath := filepath.Join(".git", "objects", shaHex[:2], shaHex[2:])
	f, err := os.Open(filePath)
	if err != nil {
		return "", nil, err
	}

	// Defer file closing
	defer f.Close()

	// Z-lib decompress and read the object
	zr, err := zlib.NewReader(f)
	if err != nil {
		return "", nil, err
	}

	// Defer z-lib reader closure
	defer zr.Close()

	// Read all data
	data, err := io.ReadAll(zr)
	if err != nil {
		return "", nil, err
	}

	// Split Header, Content -> then Header to parts
	nullIdx := bytes.IndexByte(data, 0)
	if nullIdx == -1 {
		return "", nil, fmt.Errorf("corrupt object")
	}

	header := string(data[:nullIdx])
	content := data[nullIdx+1:]

	parts := strings.Split(header, " ")
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid object header")
	}

	// Return ObjType and Content
	objType := types.ObjectType(parts[0])
	return objType, content, nil

}
