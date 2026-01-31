package porcelain

import (
	"fmt"

	"github.com/brickster241/GitEngine/utils"
)

// Invoked from main.go. LSTree handles the 'gegit ls-tree' command to list the contents of a tree object. It only calls this function if first argument is "ls-tree".
func LSTree(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("ls-tree",
		"Lists the contents of a given tree-ish object (commit or tree), like what \"/bin/ls -a\" does in the current working directory. ",
		"gegit ls-tree [-d] [-r] [-t] <tree-ish>")
	d := fls.Bool("d", false, "Show only the named tree entry itself, not its children.")
	r := fls.Bool("r", false, "Recurse into sub-trees.")
	t := fls.Bool("t", false, "Show tree entries even when going to recurse them. Has no effect if -r was not passed. -d implies -t.")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// Implement logic
	fmt.Println(d, r, t, pos)

}
