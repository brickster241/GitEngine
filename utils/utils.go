package utils

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/brickster241/GitEngine/utils/constants"
	"github.com/brickster241/GitEngine/utils/types"
)

// Utility function to create a new flag set, Will be used once per command.
func CreateCommandFlagSet(name, desc, usage string) *flag.FlagSet {
	// Define flagset
	fls := flag.NewFlagSet(name, flag.ExitOnError)
	fls.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n%sDescription:%s\n\n\t %s\n\n", constants.BoldColor, constants.ResetColor, desc)
		fmt.Fprintf(os.Stderr, "%sUsage: %s%s%s\n\n", constants.BoldColor, constants.GreenColor, usage, constants.ResetColor)
		fls.PrintDefaults()
	}
	return fls
}

// Sort based on keys
func SortedKeys(m map[string]types.StatusType) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Parse Mode string and check if it is a valid value.
func ParseMode(modeStr string) (uint32, error) {
	switch modeStr {
	case constants.ModeFileStr:
		return constants.ModeFile, nil
	case constants.ModeExecStr:
		return constants.ModeExec, nil
	case constants.ModeSymlinkStr:
		return constants.ModeSymlink, nil
	case constants.ModeTreeStr:
		return constants.ModeTree, nil
	default:
		return 0, fmt.Errorf("invalid mode: %s", modeStr)
	}
}
