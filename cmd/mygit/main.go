package main

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/git-starter-go/internal/plumbing"
)

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
		checkGitInitialized()

		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "Usage: cat-file -{t, s, p} sha1HexString")
			os.Exit(1)
		}

		flag := os.Args[2]
		hash := os.Args[3]
		if flag != "-p" && flag != "-t" && flag != "-s" {
			fmt.Fprintf(os.Stderr, "Invalid flag: %s\n", flag)
			os.Exit(1)
		}

		if len(hash) != 40 {
			fmt.Fprintf(os.Stderr, "Invalid hash: %s\n", hash)
			os.Exit(1)
		}

		meta, err := plumbing.GetGitObjectMetadata(hash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding object file: %s\n", err)
			os.Exit(1)
		}

		switch flag {
		case "-t":
			fmt.Println(meta.Header)

		case "-s":
			fmt.Println(meta.Size)

		case "-p":
			obj, err := plumbing.NewGitObjectFromHash(hash)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error decoding object file: %s\n", err)
				os.Exit(1)
			}

			fmt.Print(obj)

		default:
			fmt.Fprintf(os.Stderr, "Invalid flag: %s\n", flag)
			os.Exit(1)
		}

	case "hash-object":
		checkGitInitialized()

		if len(os.Args) > 3 && os.Args[2] != "-w" {
			fmt.Fprintf(os.Stderr, "Invalid flag: %s\n", os.Args[2])
			fmt.Fprintln(os.Stderr, "Usage: hash-object [-w] fileName")
			os.Exit(1)
		}

		var fn string
		if len(os.Args) > 3 {
			fn = os.Args[3]
		} else {
			fn = os.Args[2]
		}

		blob, err := plumbing.NewBlobObjectFromFilePath(fn)
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"Error creating blob object from file %s: %s\n",
				fn,
				err,
			)
			os.Exit(1)
		}

		if os.Args[2] == "-w" {
			err = blob.WriteToFile()
			if err != nil {
				fmt.Fprintf(
					os.Stderr,
					"Error writing blob object to file: %s\n",
					err,
				)
				os.Exit(1)
			}
		}

		fmt.Printf("%x\n", blob.Sha)

	case "ls-tree":
		checkGitInitialized()

		var hash string
		var nameOnly bool
		if len(os.Args) > 3 && os.Args[2] == "--name-only" {
			hash = os.Args[3]
			nameOnly = true
		} else if len(os.Args) == 3 {
			hash = os.Args[2]
		} else {
			fmt.Fprintln(os.Stderr, "Usage: ls-tree [--name-only] sha1HexString")
			os.Exit(1)
		}

		if len(hash) != 40 {
			fmt.Fprintf(os.Stderr, "Invalid hash: %s\n", hash)
			os.Exit(1)
		}

		meta, err := plumbing.GetGitObjectMetadata(hash)
		if err != nil || meta.Header != plumbing.Tree {
			fmt.Fprintf(os.Stderr, "Invalid tree object: %s\n", err)
			os.Exit(1)
		}

		ls, err := plumbing.LsTree(hash, nameOnly)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tree object: %s\n", err)
			os.Exit(1)
		}

		fmt.Print(ls)

	case "write-tree":
		checkGitInitialized()
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting cwd: %s\n", err)
			os.Exit(1)
		}

		tree, err := plumbing.NewTreeObjectFromFilePath(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating tree object: %s\n", err)
			os.Exit(1)
		}

		tree.WriteToFile()
		fmt.Printf("%x\n", tree.Sha)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}

func checkGitInitialized() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting cwd: %s\n", err)
		os.Exit(1)
	}

	_, err = os.Stat(dir + "/.git/")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Git not initialized.")
		os.Exit(1)
	}
}
