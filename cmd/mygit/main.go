package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

type ObjectType string

const (
	Blob ObjectType = "blob"
	Tree            = "tree"
)

type FileType string

const (
	Dir            FileType = "040000"
	Symlink                 = "120000"
	RegularFile             = "100644"
	ExecutableFile          = "100755"
)

// TODO Implement Constructors and Stringer.
// TODO Make related functions methods
type BlobObject struct {
	size    uint
	content []byte
}

// TODO Implement Constructors and Stringer.
// TODO Make related functions methods
type TreeObject struct {
	size    uint
	entries []TreeEntry
}

type TreeEntry struct {
	mode FileType
	name string
	sha  []byte
	kind ObjectType
}

func (t *TreeEntry) String() string {
	return fmt.Sprintf("%s %s %x    %s\n", t.mode, t.kind, t.sha, t.name)
}

// TODO Support blob and tree objects
func CatFile(hash string) (string, error) {
	dir, file := hash[:2], hash[2:]
	b, err := os.ReadFile("./.git/objects/" + dir + "/" + file)
	if err != nil {
		return "", err
	}

	buf := bytes.NewBuffer(b)
	r, err := zlib.NewReader(buf)
	if err != nil {
		return "", err
	}
	defer r.Close()

	obj, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	parts := bytes.Split(obj, []byte("\x00"))
	return string(parts[1]), nil
}

// TODO Refactor using hashobject constructor
func HashBlobObject(b []byte, write bool) (string, error) {
	s := string(b)
	content := fmt.Sprintf("blob %d\x00%s", len(s), s)
	h := sha1.New()
	h.Write([]byte(content))
	hash := fmt.Sprintf("%x", h.Sum(nil))

	if !write {
		return hash, nil
	}

	fn := fmt.Sprintf("./.git/objects/%s/%s", hash[:2], hash[2:])
	err := os.MkdirAll("./.git/objects/"+hash[:2], 0755)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write([]byte(content))
	w.Close()

	err = os.WriteFile(fn, buf.Bytes(), 0755)
	if err != nil {
		return "", err
	}

	return hash, nil
}

// TODO Refactor using tree object constructor
func LsTree(hash string, nameOnly bool) (string, error) {
	dir, file := hash[:2], hash[2:]
	b, err := os.ReadFile("./.git/objects/" + dir + "/" + file)
	if err != nil {
		return "", err
	}

	buff := bytes.NewBuffer(b)
	r, err := zlib.NewReader(buff)
	if err != nil {
		return "", err
	}
	defer r.Close()

	tree, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	parts := bytes.Split(tree, []byte("\x00"))
	//	if bytes.HasPrefix(parts[0], []byte("tree")) {
	//		return "", errors.New("Not a tree object.")
	//	}

	entries := make([]string, len(parts)-2)
	for i, part := range parts[1 : len(parts)-1] {
		var mode, name, sha []byte
		var info [][]byte

		if i == 1 {
			info = bytes.Split(part, []byte(" "))
		} else {
			info = bytes.Split(part[20:], []byte(" "))
		}

		mode = info[0]
		name = info[1]
		sha = parts[i+1][:20]

		if nameOnly {
			entries[i-1] = fmt.Sprintf("%s\n", name)
		} else {
			entries[i-1] = fmt.Sprintf("%s %s %x\n", mode, name, sha)
		}
	}
	//TODO return output
	return "", nil
}

// TODO Implement writeTree
func WriteTree(path string) (string, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	entries := make([]TreeEntry, len(files)+1)
	for i, file := range files {
		info, err := file.Info()
		if err != nil {
			return "", nil
		}
		switch {
		case info.IsDir():
			fmt.Println("dir: ", file.Name())
			entries[i+1].mode = Dir
			entries[i+1].name = file.Name()
			fmt.Println(entries[i+1])

		case info.Mode()&fs.ModeSymlink != 0:
			fmt.Println("sym: ", file.Name())
			entries[i+1].mode = Symlink
			entries[i+1].name = file.Name()
			fmt.Println(entries[i+1])

		case file.Type().IsRegular() && (info.Mode()&0111) != 0:
			fmt.Println("exe: ", file.Name())
			entries[i+1].mode = ExecutableFile
			entries[i+1].name = file.Name()
			fmt.Println(entries[i+1])

		case file.Type().IsRegular():
			fmt.Println("file: ", file.Name())
			entries[i+1].mode = RegularFile
			entries[i+1].name = file.Name()
			fmt.Println(entries[i+1])

		default:
			return "", errors.New("Could not handle unknown file type.")
		}
	}

	//TODO return correct val
	return "", nil
}

// Usage: your_program.sh <command> <arg1> <arg2> ...
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			}
		}

		headFileContents := []byte("ref: refs/heads/main\n")
		if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}

		fmt.Println("Initialized git directory")

	case "cat-file":
		if os.Args[2] != "-p" {
			fmt.Fprintf(os.Stderr, "Invalid flag: %s\n", os.Args[2])
			os.Exit(1)
		}

		hash := os.Args[3]
		if len(hash) != 40 {
			fmt.Fprintf(os.Stderr, "Invalid hash: %s\n", hash)
			os.Exit(1)
		}

		content, err := CatFile(hash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while running cat-file: %s\n", err)
		}

		fmt.Print(content)

	case "hash-object":
		var fn string
		if len(os.Args) > 3 && os.Args[2] != "-w" {
			fmt.Fprintf(os.Stderr, "Invalid flag: %s\n", os.Args[2])
			os.Exit(1)
		}

		if len(os.Args) > 3 {
			fn = os.Args[3]
		} else {
			fn = os.Args[2]
		}

		b, err := os.ReadFile(fn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %s\n", fn, err)
			os.Exit(1)
		}

		write := os.Args[2] == "-w"
		hash, err := HashBlobObject(b, write)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while running hash-object: %s\n", err)
		}

		fmt.Println(hash)

	case "ls-tree":
		var hash string
		var nameOnly bool
		if len(os.Args) > 3 && os.Args[2] == "--name-only" {
			hash = os.Args[3]
			nameOnly = true
		} else {
			hash = os.Args[2]
		}

		if len(hash) != 40 {
			fmt.Fprintf(os.Stderr, "Invalid hash: %s\n", hash)
			os.Exit(1)
		}

		entries, err := LsTree(hash, nameOnly)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running ls-tree: %s\n", err)
			os.Exit(1)
		}

		for _, entry := range entries {
			fmt.Println(entry)
		}

		//TODO Implement write-tree
	case "write-tree":
		path, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting cwd: %s\n", err)
		}

		WriteTree(path)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
