package main

import (
        "bytes"
        "compress/zlib"
	"fmt"
        "io"
	"os"
        "strings"
)

// Usage: your_program.sh <command> <arg1> <arg2> ...
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		// Uncomment this block to pass the first stage!
		
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
                }

                hash := os.Args[3]
                if len(hash) != 40 {
			fmt.Fprintf(os.Stderr, "Invalid hash: %s\n", hash)
                }
                
                dir, file := hash[:2], hash[2:]
                b, err := os.ReadFile("./.git/objects/" + dir + "/" + file)
                if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading blob: %s\n", err)
                }

                buff := bytes.NewBuffer(b)
                r, err := zlib.NewReader(buff)
                if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading blob: %s\n", err)
                }
                defer r.Close()

                var sb strings.Builder
                io.Copy(&sb, r)
                blob := sb.String()
                for i, c := range blob {
                        if c == 0 {
                                content := blob[i+1:]
                                fmt.Print(content)
                                break
                        }
                }

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
