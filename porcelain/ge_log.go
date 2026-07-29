package porcelain

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/brickster241/GitEngine/plumbing"
	"github.com/brickster241/GitEngine/utils"
)

// Invoked from main.go. LogCommits handles 'gegit log': walk history from
// HEAD following first parents (the branch's own line of development),
// newest first.
func LogCommits(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("log",
		"Show the commit history reachable from HEAD, following first parents, newest first.",
		"gegit log [--oneline] [-n <count>]")
	oneline := fls.Bool("oneline", false, "One line per commit: abbreviated SHA + message subject.")
	limit := fls.Int("n", 0, "Limit the number of commits shown (0 = no limit).")
	fls.Parse(args[1:])

	// Start at HEAD.
	headInfo, err := plumbing.ReadHEADInfo()
	if err != nil {
		fmt.Println("Error reading .git/HEAD:", err)
		os.Exit(1)
	}
	if headInfo.SHA == [20]byte{} {
		fmt.Println("fatal: your current branch does not have any commits yet")
		os.Exit(1)
	}

	curr := headInfo.SHA
	shown := 0
	for {
		commit, err := plumbing.ReadCommit(curr)
		if err != nil {
			fmt.Println("Error reading commit:", err)
			os.Exit(1)
		}
		hexSHA := hex.EncodeToString(curr[:])

		if *oneline {
			fmt.Printf("%s %s\n", hexSHA[:7], firstLine(commit.Message))
		} else {
			fmt.Printf("commit %s\n", hexSHA)
			if len(commit.ParentsSHA) > 1 {
				parents := make([]string, len(commit.ParentsSHA))
				for i, p := range commit.ParentsSHA {
					parents[i] = hex.EncodeToString(p[:])[:7]
				}
				fmt.Printf("Merge: %s\n", strings.Join(parents, " "))
			}
			fmt.Printf("Author: %s <%s>\n", commit.Author.Name, commit.Author.Email)
			fmt.Printf("Date:   %s\n\n", commit.AuthorDate)
			for _, line := range strings.Split(strings.TrimRight(commit.Message, "\n"), "\n") {
				fmt.Printf("    %s\n", line)
			}
			fmt.Println()
		}

		shown++
		if *limit > 0 && shown >= *limit {
			return
		}
		if len(commit.ParentsSHA) == 0 {
			return
		}
		curr = commit.ParentsSHA[0]
	}
}
