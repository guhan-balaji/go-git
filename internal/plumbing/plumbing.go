// Package plumbing provides the ability to do some git plumbing like
// operations
package plumbing

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type ObjectType string

const (
	Blob ObjectType = "blob"
	Tree            = "tree"
)

var ErrInvalidBlobObject = errors.New("Invalid blob object: raw binary data does not represent a blob.")
var ErrInvalidTreeObject = errors.New("Invalid tree object: raw binary data does not represent a tree.")
var ErrInvalidGitObject = errors.New("Invalid git object.")

type FileType string

const (
	Dir        FileType = "40000"
	Symlink             = "120000"
	Regular             = "100644"
	Executable          = "100755"
)

func fileInfoToFileType(info os.FileInfo) FileType {
	switch mode := info.Mode(); {
	case info.IsDir():
		return Dir
	case mode&os.ModeSymlink == os.ModeSymlink:
		return Symlink
	case mode&0111 != 0:
		return Executable
	default:
		return Regular
	}
}

// FileWriter interface wraps a WriteToFile method targeting filesystems.
//
// Types that implelment FileWriter attempt to save the object state to the
// filesystem. For example, we can write git objects to the filesystem.
//
// The return value is nil on success.
type FileWriter interface {
	WriteToFile() error
}

type GitObject interface {
	fmt.Stringer
	FileWriter
}

type GitObjectMetadata struct {
	Header ObjectType
	Size   int
}

type BlobObject struct {
	Header  ObjectType
	Mode    FileType
	Size    int
	Content []byte
	Sha     []byte
}

func (b BlobObject) String() string {
	return fmt.Sprintf("%s", b.Content)
}

func (b *BlobObject) serializeDecompressed() []byte {
	var blob bytes.Buffer
	blob.WriteString(fmt.Sprintf("%s %d\x00", b.Header, b.Size))
	blob.Write(b.Content)
	return blob.Bytes()
}

func (b *BlobObject) serializeCompressed() []byte {
	blob := b.serializeDecompressed()

	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write(blob)
	w.Close()

	return compressed.Bytes()
}

func (b *BlobObject) WriteToFile() error {
	blob := b.serializeCompressed()
	dir, file := b.Sha[0], b.Sha[1:]
	err := os.MkdirAll(fmt.Sprintf("./.git/objects/%x", dir), 0755)
	if err != nil {
		return err
	}
	fn := fmt.Sprintf("./.git/objects/%x/%x", dir, file)
	return os.WriteFile(fn, blob, 0444)
}

// NewBlobObjectFromFilePath takes in the path to a file and returns a blob object
// representation of the file and nil if conversion was succesful. Otherwise,
// nil and an error value is returned.
func NewBlobObjectFromFilePath(filePath string) (*BlobObject, error) {
	info, err := os.Lstat(filePath)
	if err != nil {
		return nil, err
	} else if info.IsDir() {
		return nil, errors.New("Blob objects can't be made from directories.")
	}

	var content []byte
	if info.Mode()&os.ModeSymlink == os.ModeSymlink {
		fp, err := os.Readlink(filePath)
		if err != nil {
			return nil, err
		}
		content = []byte(fp)
	} else {
		content, err = os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
	}

	var blob bytes.Buffer
	blob.WriteString(fmt.Sprintf("blob %d\x00", len(content)))
	blob.Write(content)

	return deserializeDecompressedBlobObject(blob.Bytes())
}

// NewBlobObjectFromHash takes the sha1 hex string of the blob object and
// returns a (*BlobObject, nil) on success. Otherwise, (nil, error).
func NewBlobObjectFromHash(hash string) (*BlobObject, error) {
	fn := fmt.Sprintf("./.git/objects/%s/%s", hash[:2], hash[2:])
	b, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(b)

	return deserializeCompressedBlobObject(r)
}
func deserializeCompressedBlobObject(r io.Reader) (*BlobObject, error) {
	b, err := zlib.NewReader(r)
	defer b.Close()

	if err != nil {
		return nil, err
	}

	blob, err := io.ReadAll(b)
	if err != nil {
		return nil, err
	}

	return deserializeDecompressedBlobObject(blob)
}

func deserializeDecompressedBlobObject(b []byte) (*BlobObject, error) {
	first, rest, ok := bytes.Cut(b, []byte(" "))
	if !ok {
		return nil, ErrInvalidBlobObject
	}

	sz, content, ok := bytes.Cut(rest, []byte("\x00"))
	if !ok {
		return nil, ErrInvalidBlobObject
	}

	header := ObjectType(first)

	size, err := strconv.Atoi(string(sz))
	if err != nil || header != Blob || size != len(content) {
		return nil, err
	}

	h := sha1.New()
	h.Write(b)
	hash := h.Sum(nil)

	return &BlobObject{
		Header:  header,
		Size:    size,
		Content: content,
		Sha:     hash,
	}, nil

}

type TreeEntry struct {
	Header ObjectType
	Mode   FileType
	Name   string
	Sha    []byte
	w      FileWriter
}

func (t *TreeEntry) WriteToFile() error {
	return t.w.WriteToFile()
}

func (e TreeEntry) String() string {
	return fmt.Sprintf("%06s %s %x    %s", e.Mode, e.Header, e.Sha, e.Name)
}

type TreeObject struct {
	Header  ObjectType
	Entries []TreeEntry
	Size    int
	Sha     []byte
}

func (t *TreeObject) StringNameOnly() string {
	var s strings.Builder
	for _, entry := range t.Entries {
		fmt.Fprintln(&s, entry.Name)
	}
	return s.String()
}

func (t TreeObject) String() string {
	var s strings.Builder
	for _, entry := range t.Entries {
		fmt.Fprintln(&s, entry)
	}
	return s.String()
}

func (t *TreeObject) serializeDecompressed() []byte {
	var tree, content bytes.Buffer

	for _, entry := range t.Entries {
		content.WriteString(fmt.Sprintf("%s %s\x00", entry.Mode, entry.Name))
		content.Write(entry.Sha)
	}

	tree.WriteString(fmt.Sprintf("%s %d\x00", t.Header, content.Len()))
	tree.Write(content.Bytes())

	return tree.Bytes()
}
func (t *TreeObject) serializeCompressed() []byte {
	tree := t.serializeDecompressed()

	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	w.Write(tree)
	w.Close()

	return compressed.Bytes()
}

func (t *TreeObject) WriteToFile() error {
	for _, entry := range t.Entries {
		err := entry.w.WriteToFile()
		if err != nil {
			return err
		}
	}

	tree := t.serializeCompressed()
	dir, file := t.Sha[0], t.Sha[1:]
	err := os.MkdirAll(fmt.Sprintf("./.git/objects/%x", dir), 0755)
	if err != nil {
		return err
	}
	fn := fmt.Sprintf("./.git/objects/%x/%x", dir, file)
	return os.WriteFile(fn, tree, 4644)
}

// NewTreeObjectFromFilePath takes the base dir path as argument and creates all
// a TreeObject, along with all its sub TreeObject(s) and BlobObject(s). The
// function returns a (*TreeObject, nil) on success. Oterwise, it returns (nil,
// error).
func NewTreeObjectFromFilePath(dirPath string) (*TreeObject, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var t TreeObject
	var tree, content bytes.Buffer
	t.Header = Tree

	for _, file := range files {
		if file.Name() == ".git" {
			continue
		}
		var entry TreeEntry
		entry.Name = file.Name()
		if file.IsDir() {
			entry.Header = Tree
			entry.Mode = Dir

			subtree, err := NewTreeObjectFromFilePath(dirPath + "/" + file.Name())
			if err != nil {
				return nil, err
			}

			entry.w = subtree
			entry.Sha = subtree.Sha
		} else {
			entry.Header = Blob
			blob, err := NewBlobObjectFromFilePath(dirPath + "/" + file.Name())
			if err != nil {
				return nil, err
			}

			entry.w = blob
			entry.Sha = blob.Sha

			info, err := file.Info()
			if err != nil {
				return nil, err
			}

			entry.Mode = fileInfoToFileType(info)
		}

		content.WriteString(fmt.Sprintf("%s %s\x00", entry.Mode, entry.Name))
		content.Write(entry.Sha)
		t.Entries = append(t.Entries, entry)
	}

	tree.WriteString(fmt.Sprintf("%s %d\x00", t.Header, content.Len()))
	tree.Write(content.Bytes())

	h := sha1.New()
	h.Write(tree.Bytes())
	hash := h.Sum(nil)
	t.Sha = hash
	return &t, nil
}

// NewTreeObjectFromHash takes the sha1 hex string of the blob object and
// returns a (*TreeObject, nil) on success. Otherwise, (nil, error).
//
// Also, this function does not create any subsequent objects for any of the
// entries in the tree object.
func NewTreeObjectFromHash(hash string) (*TreeObject, error) {
	fn := fmt.Sprintf("./.git/objects/%s/%s", hash[:2], hash[2:])
	b, err := os.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(b)

	return deserializeCompressedTreeObject(r)
}

// LsTree lists the entries of a tree object. It takes in a sha1 hex string and
// an nameOnly bool. It returns the output string and nil on success. Otherwise,
// it returns ("", nil).
func LsTree(hash string, nameOnly bool) (string, error) {
	tree, err := NewTreeObjectFromHash(hash)

	if err != nil {
		return "", err
	}

	if nameOnly {
		return tree.StringNameOnly(), nil
	} else {
		return tree.String(), nil
	}
}

func deserializeCompressedTreeObject(r io.Reader) (*TreeObject, error) {
	t, err := zlib.NewReader(r)
	defer t.Close()

	if err != nil {
		return nil, err
	}

	tree, err := io.ReadAll(t)
	if err != nil {
		return nil, err
	}

	return deserializeDecompressedTreeObject(tree)
}

func deserializeDecompressedTreeObject(b []byte) (*TreeObject, error) {
	first, content, ok := bytes.Cut(b, []byte("\x00"))
	meta := bytes.Split(first, []byte(" "))
	if !ok || len(meta) != 2 {
		return nil, ErrInvalidTreeObject
	}

	header := ObjectType(meta[0])
	size, err := strconv.Atoi(string(meta[1]))
	if err != nil || header != Tree || size != len(content) {
		return nil, ErrInvalidTreeObject
	}

	var t TreeObject
	t.Header = Tree
	t.Size = size

	for ok {
		var entry TreeEntry
		var name []byte
		first, content, ok = bytes.Cut(content, []byte(" "))
		if !ok {
			return nil, ErrInvalidTreeObject
		}

		entry.Mode = FileType(first)
		if entry.Mode == Dir {
			entry.Header = Tree
		} else {
			entry.Header = Blob
		}

		name, content, ok = bytes.Cut(content, []byte("\x00"))
		if !ok {
			return nil, ErrInvalidTreeObject
		}

		entry.Name = string(name)

		if len(content) == 20 {
			entry.Sha = content
			ok = false
		} else {
			entry.Sha = content[:20]
			content = content[20:]
		}
		t.Entries = append(t.Entries, entry)
	}

	h := sha1.New()
	h.Write(b)
	hash := h.Sum(nil)
	t.Sha = hash

	return &t, nil
}

// GetGitObjectMetadata takes a sha1 hex string and returns the metadata of the
// git object represented by the hash. It return (GitObjectMetada, error) on
// success. Otherwise, it returns (nil, error).
func GetGitObjectMetadata(hash string) (GitObjectMetadata, error) {
	var meta GitObjectMetadata
	fn := fmt.Sprintf("./.git/objects/%s/%s", hash[:2], hash[2:])
	b, err := os.ReadFile(fn)
	if err != nil {
		return meta, err
	}

	buf := bytes.NewBuffer(b)
	r, err := zlib.NewReader(buf)
	if err != nil {
		return meta, err
	}
	defer r.Close()

	decompressed, err := io.ReadAll(r)
	if err != nil {
		return meta, err
	}

	h, _, ok := bytes.Cut(decompressed, []byte("\x00"))
	if !ok {
		return meta, ErrInvalidGitObject
	}

	parts := bytes.Split(h, []byte(" "))
	if len(parts) != 2 {
		return meta, ErrInvalidGitObject
	}

	header := ObjectType(parts[0])
	if header != Tree && header != Blob {
		return meta, ErrInvalidGitObject
	}

	size, err := strconv.Atoi(string(parts[1]))
	if err != nil || size < 0 {
		return meta, ErrInvalidGitObject
	}

	meta.Header = header
	meta.Size = size

	return meta, nil
}

// NewGitObjectFromHash takes a sha1 hex string and returns (GitObject, error)
// on success. Othrwise, it returns (nil, error).
//
// Also, this function does not create any subsequent objects for any of the
// entries in nested GitObjects like TreeObject(s), etc.
func NewGitObjectFromHash(hash string) (GitObject, error) {
	meta, err := GetGitObjectMetadata(hash)
	if err != nil {
		return nil, err
	}

	switch meta.Header {
	case Blob:
		return NewBlobObjectFromHash(hash)

	case Tree:
		return NewTreeObjectFromHash(hash)

	default:
		return nil, ErrInvalidGitObject
	}
}
