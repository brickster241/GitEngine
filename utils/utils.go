package utils

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/brickster241/GitEngine/utils/constants"
	"github.com/brickster241/GitEngine/utils/types"
)

// Utility function to create a new flag set, Will be used once per command.
func CreateCommandFlagSet(name, desc, usage string) *flag.FlagSet {
	// Define flagset
	fls := flag.NewFlagSet(name, flag.ExitOnError)
	fls.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n%sDescription:%s\n\n\t %s\n\n", constants.BoldColor, constants.ResetColor, desc)
		fmt.Fprintf(os.Stderr, "%sUsage:  %s%s%s\n\n", constants.BoldColor, constants.GreenColor, usage, constants.ResetColor)
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
func ParseModeStr(modeStr string) (uint32, error) {
	// Parse as octal: native Git writes tree modes WITHOUT leading zeros
	// ("40000" for directories, "100644"/"100755" for blobs), while older
	// GitEngine trees carried zero-padded "040000". Accepting both keeps us
	// readable against every real repository.
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode: %s", modeStr)
	}
	switch uint32(mode) {
	case constants.ModeFile, constants.ModeExec, constants.ModeSymlink, constants.ModeTree:
		return uint32(mode), nil
	default:
		return 0, fmt.Errorf("unsupported mode: %s", modeStr)
	}
}
