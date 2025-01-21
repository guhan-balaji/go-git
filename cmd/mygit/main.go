package main

import (
        "bytes"
        "compress/zlib"
        "crypto/sha1"
	"fmt"
        "io"
	"os"
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
                handleCatFile()
        
        case "hash-object":
                handleHashObject()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}

func handleCatFile() {
        if os.Args[2] != "-p" {
                fmt.Fprintf(os.Stderr, "Invalid flag: %s\n", os.Args[2])
		os.Exit(1)
        }

        hash := os.Args[3]
        if len(hash) != 40 {
                fmt.Fprintf(os.Stderr, "Invalid hash: %s\n", hash)
		os.Exit(1)
        }

        dir, file := hash[:2], hash[2:]
        b, err := os.ReadFile("./.git/objects/" + dir + "/" + file)
        if err != nil {
                fmt.Fprintf(os.Stderr, "Error reading blob: %s\n", err)
		os.Exit(1)
        }

        buff := bytes.NewBuffer(b)
        r, err := zlib.NewReader(buff)
        if err != nil {
                fmt.Fprintf(os.Stderr, "Error reading blob: %s\n", err)
		os.Exit(1)
        }
        defer r.Close()

        blob, err := io.ReadAll(r)
        if err != nil {
                fmt.Fprintf(os.Stderr, "Error reading blob: %s\n", err)
		os.Exit(1)
        }

        parts := bytes.Split(blob, []byte("\x00"))
        content := string(parts[1])
        fmt.Print(content)
}

func handleHashObject() {
        fn := os.Args[3]
        b, err := os.ReadFile(fn)
        if err != nil {
                fmt.Fprintf(os.Stderr, "Error reading file %s: %s\n", fn, err)
		os.Exit(1)
        }
        s := string(b)
        content := fmt.Sprintf("blob %d\x00%s", len(s), s)

        h := sha1.New()
        h.Write([]byte(content))
        hash := fmt.Sprintf("%x", h.Sum(nil))
        fmt.Println(hash)

        if os.Args[2] == "-w" {
                fn = fmt.Sprintf("./.git/objects/%s/%s", hash[:2], hash[2:])
                err = os.MkdirAll("./.git/objects/" + hash[:2], 0755)
                if err != nil {
                        fmt.Fprintf(os.Stderr, "Error writing object file %s: %s\n", fn, err)
                        os.Exit(1)
                }

                var b bytes.Buffer
                w := zlib.NewWriter(&b)
                w.Write([]byte(content))
                w.Close()

                err = os.WriteFile(fn, b.Bytes(), 0755)
                if err != nil {
                        fmt.Fprintf(os.Stderr, "Error writing object file %s: %s\n", fn, err)
                        os.Exit(1)
                }
        }
}
