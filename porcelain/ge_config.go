package porcelain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brickster241/GitEngine/utils"
	"github.com/brickster241/GitEngine/utils/types"
	"gopkg.in/ini.v1"
)

// Invoked from main.go. GetOrSetConfig handles 'gegit config' command which is stored at .git/config.
func GetOrSetConfig(args []string) {

	// Define flagset
	fls := utils.CreateCommandFlagSet("config",
		"Get and set repo config options. This command allows you to read and write configuration values used by GitEngine. Configuration keys are stored as simple key-value pairs (in .git/config).",
		"gegit config (get <key> | set <key> <value>)")

	// Parse flags from args
	fls.Parse(args[1:])

	// Positional arguments (non-flag)
	pos := fls.Args()

	// If no args are provided
	if len(pos) == 0 {
		fmt.Println("usage: gegit config (get <key> | set <key> <value>)")
		os.Exit(1)
	}

	switch pos[0] {
	case "set": // Set config value for specific key
		if len(pos) != 3 {
			fmt.Println("usage: gegit config set <key> <value>")
			os.Exit(1)
		}

		if err := setConfig(pos[1], pos[2]); err != nil {
			fmt.Println("Error setting Config:", err)
		}
	case "get": // Get config value for specific key
		if len(pos) != 2 {
			fmt.Println("usage: gegit config get <key>")
			os.Exit(1)
		}
		val, err := getConfig(pos[1])
		if err != nil {
			fmt.Println("Error getting Config:", err)
			os.Exit(1)
		}

		fmt.Println(val)

	default:
		fmt.Println("unknown config command:", pos[0])
		fmt.Println("usage: gegit config (get <key> | set <key> <value>)")
	}

}

// Get Value for a specific Config key.
func getConfig(key string) (string, error) {

	// .git/config file Path
	cfgPath := filepath.Join(".git", "config")

	// Load the config file
	cfg, err := ini.Load(cfgPath)
	if err != nil {
		return "", err
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid config key: %s", key)
	}

	// Check val for specific key
	section, name := parts[0], parts[1]
	val := cfg.Section(section).Key(name).String()
	if val == "" {
		return "", fmt.Errorf("config key not found: %s", key)
	}

	return val, nil
}

// Set Value for a specific Config key.
func setConfig(key, value string) error {

	// .git/config file Path
	cfgPath := filepath.Join(".git", "config")

	// Load the config file
	cfg, err := ini.Load(cfgPath)
	if err != nil {
		return err
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid config key: %s", key)
	}

	// Check val for specific key
	section, name := parts[0], parts[1]
	cfg.Section(section).Key(name).SetValue(value)

	return cfg.SaveTo(cfgPath)
}

// getAuthorInfo fetches the Author information present in .git/config
func getAuthorInfo() (types.Author, error) {

	// Get user.name
	name, err := getConfig("user.name")
	if err != nil {
		return types.Author{}, err
	}

	// Get user.email
	email, err := getConfig("user.email")
	if err != nil {
		return types.Author{}, err
	}

	return types.Author{
		Name:  name,
		Email: email,
	}, nil
}
