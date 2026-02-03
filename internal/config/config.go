package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the migration tool
type Config struct {
	GitLabToken    string   `mapstructure:"token"`
	GitLabInstance string   `mapstructure:"instance"`
	GitLabRegistry string   `mapstructure:"registry"`
	DockerToken    string   `mapstructure:"docker-password"`
	OldGroupName   string   `mapstructure:"old-group"`
	NewGroupName   string   `mapstructure:"new-group"`
	ParentGroupID  int      `mapstructure:"parent-group-id"`
	ProjectsList   []string `mapstructure:"projects"`
	TagsList       []string `mapstructure:"tags"`
	KeepParent     bool     `mapstructure:"keep-parent"`
	DryRun         bool     `mapstructure:"dry-run"`
	Verbose        bool     `mapstructure:"verbose"`
}

const GITLAB_TOKEN = "token"
const GITLAB_INSTANCE = "instance"
const GITLAB_REGISTRY = "registry"
const DOCKER_PASSWORD = "docker-password"
const OLD_GROUP_NAME = "old-group"
const NEW_GROUP_NAME = "new-group"
const PROJECTS_LIST = "projects"
const TAGS_LIST = "tags"
const KEEP_PARENT = "keep-parent"
const DRY_RUN = "dry-run"
const VERBOSE = "verbose"

// getFlagNameForViperKey returns the flag name (constant) for a given viper key
func getFlagNameForViperKey(viperKey string) string {
	flagMap := map[string]string{
		"token":           GITLAB_TOKEN,
		"instance":        GITLAB_INSTANCE,
		"registry":        GITLAB_REGISTRY,
		"docker-password": DOCKER_PASSWORD,
		"old-group":       OLD_GROUP_NAME,
		"new-group":       NEW_GROUP_NAME,
		"parent-group-id": "parent-group-id", // No constant for this, use key directly
		"projects":        PROJECTS_LIST,
		"tags":            TAGS_LIST,
		"keep-parent":     KEEP_PARENT,
		"dry-run":         DRY_RUN,
		"verbose":         VERBOSE,
	}
	if flagName, ok := flagMap[viperKey]; ok {
		return flagName
	}
	return viperKey // Fallback to viper key itself
}

// isFlagSet checks if a flag was explicitly set by the user using flag.Changed
func isFlagSet(cmd *cobra.Command, viperKey string) bool {
	flagName := getFlagNameForViperKey(viperKey)
	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		return false
	}
	return flag.Changed
}

// copyAliasedValues copies values from aliased keys (snake_case from config file) to actual keys (kebab-case)
// This is needed because:
// 1. viper.Unmarshal() doesn't use aliases
// 2. RegisterAlias doesn't work properly with ReadConfig
// So we check if the snake_case keys exist in the config file and copy them to kebab-case keys
// It skips copying if a flag was already set for that key (flags have highest priority)
func copyAliasedValues(cmd *cobra.Command) {
	aliasMap := map[string]string{
		"gitlab_token":    "token",
		"gitlab_instance": "instance",
		"gitlab_registry": "registry",
		"docker_token":    "docker-password",
		"old_group_name":  "old-group",
		"new_group_name":  "new-group",
		"parent_group_id": "parent-group-id",
		"projects_list":   "projects",
		"tags_list":       "tags",
		"keep_parent":     "keep-parent",
		"dry_run":         "dry-run",
	}

	// Try to read the config file directly to get raw keys
	// This is more reliable than AllSettings() which might process aliases
	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		if data, err := os.ReadFile(configFile); err == nil {
			var rawConfig map[string]interface{}
			if err := yaml.Unmarshal(data, &rawConfig); err == nil {
				// Check for alias keys in raw config file
				for aliasKey, actualKey := range aliasMap {
					// Skip if flag was already set (flags have highest priority)
					if cmd != nil && isFlagSet(cmd, actualKey) {
						continue
					}
					if value, exists := rawConfig[aliasKey]; exists && value != nil {
						viper.Set(actualKey, value)
					}
				}
				return // Successfully processed raw config file
			}
		}
	}

	// Fallback: Use AllSettings() and viper.Get() if direct file read fails
	allSettings := viper.AllSettings()
	for aliasKey, actualKey := range aliasMap {
		// Skip if flag was already set (flags have highest priority)
		if cmd != nil && isFlagSet(cmd, actualKey) {
			continue
		}
		// Check AllSettings first (might have raw keys)
		if allSettings != nil {
			if value, exists := allSettings[aliasKey]; exists && value != nil {
				viper.Set(actualKey, value)
				continue
			}
		}
		// Fallback: Try viper.Get() with alias key
		if !viper.IsSet(actualKey) {
			if value := viper.Get(aliasKey); value != nil {
				viper.Set(actualKey, value)
			}
		}
	}
}

// LoadConfig loads configuration from multiple sources with proper precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Config file
// 4. Defaults
func LoadConfig(cmd *cobra.Command) (*Config, error) {
	// Set defaults
	viper.SetDefault("instance", "gitlab.com")
	viper.SetDefault("keep-parent", true)

	// Set up aliases for config file keys (snake_case) to flag keys (kebab-case)
	// This allows the config file to use keys like "gitlab_token", "old_group_name", etc.
	// Must be done before reading config file
	viper.RegisterAlias("gitlab_token", "token")
	viper.RegisterAlias("gitlab_instance", "instance")
	viper.RegisterAlias("gitlab_registry", "registry")
	viper.RegisterAlias("docker_token", "docker-password")
	viper.RegisterAlias("old_group_name", "old-group")
	viper.RegisterAlias("new_group_name", "new-group")
	viper.RegisterAlias("parent_group_id", "parent-group-id")
	viper.RegisterAlias("projects_list", "projects")
	viper.RegisterAlias("tags_list", "tags")
	viper.RegisterAlias("keep_parent", "keep-parent")
	viper.RegisterAlias("dry_run", "dry-run")

	// Enable automatic environment variable binding
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Also support legacy env var names without prefix
	// Map legacy env var names to viper keys
	err := viper.BindEnv("token", "GITLAB_TOKEN")
	err = viper.BindEnv("instance", "GITLAB_INSTANCE")
	err = viper.BindEnv("registry", "GITLAB_REGISTRY")
	err = viper.BindEnv("docker-password", "DOCKER_TOKEN")
	err = viper.BindEnv("old-group", "OLD_GROUP_NAME")
	err = viper.BindEnv("new-group", "NEW_GROUP_NAME")
	err = viper.BindEnv("parent-group-id", "PARENT_GROUP_ID")
	err = viper.BindEnv("projects", "PROJECTS_LIST")
	err = viper.BindEnv("tags", "TAGS_LIST")
	err = viper.BindEnv("keep-parent", "KEEP_PARENT")
	err = viper.BindEnv("dry-run", "DRY_RUN")
	err = viper.BindEnv("verbose", "VERBOSE")
	if err != nil {
		return nil, err
	}

	// STEP 1: Bind individual Cobra flags to Viper FIRST (highest priority)
	// Use individual BindPFlag calls instead of BindPFlags() for reliability
	bindFlag := func(key, flagName string) error {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			return fmt.Errorf("flag %s not found", flagName)
		}
		return viper.BindPFlag(key, flag)
	}

	if err := bindFlag("token", GITLAB_TOKEN); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", GITLAB_TOKEN, err)
	}
	if err := bindFlag("old-group", OLD_GROUP_NAME); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", OLD_GROUP_NAME, err)
	}
	if err := bindFlag("new-group", NEW_GROUP_NAME); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", NEW_GROUP_NAME, err)
	}
	if err := bindFlag("dry-run", DRY_RUN); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", DRY_RUN, err)
	}
	if err := bindFlag("instance", GITLAB_INSTANCE); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", GITLAB_INSTANCE, err)
	}
	if err := bindFlag("keep-parent", KEEP_PARENT); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", KEEP_PARENT, err)
	}
	if err := bindFlag("projects", PROJECTS_LIST); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", PROJECTS_LIST, err)
	}
	if err := bindFlag("docker-password", DOCKER_PASSWORD); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", DOCKER_PASSWORD, err)
	}
	if err := bindFlag("registry", GITLAB_REGISTRY); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", GITLAB_REGISTRY, err)
	}
	if err := bindFlag("tags", TAGS_LIST); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", TAGS_LIST, err)
	}
	if err := bindFlag("verbose", VERBOSE); err != nil {
		return nil, fmt.Errorf("failed to bind flag %s: %w", VERBOSE, err)
	}

	// Explicitly set flag values in Viper if flags were changed
	// This ensures flags override config file values
	// Note: We use viper.BindPFlag which should handle this automatically,
	// but we explicitly set values here to ensure flags override config file
	setFlagValue := func(viperKey string) {
		if !isFlagSet(cmd, viperKey) {
			return
		}
		flagName := getFlagNameForViperKey(viperKey)
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			return
		}

		// Get the actual typed value from the flag based on viper key type
		switch viperKey {
		case "dry-run", "keep-parent", "verbose":
			// Boolean flags
			if boolVal, err := cmd.Flags().GetBool(flagName); err == nil {
				viper.Set(viperKey, boolVal)
			}
		case "projects", "tags":
			// String slice flags
			if sliceVal, err := cmd.Flags().GetStringSlice(flagName); err == nil {
				viper.Set(viperKey, sliceVal)
			}
		default:
			// String flags (and other types)
			viper.Set(viperKey, flag.Value.String())
		}
	}

	flagKeys := []string{"token", "old-group", "new-group", "dry-run", "instance", "keep-parent", "projects", "docker-password", "registry", "tags", "verbose"}
	for _, viperKey := range flagKeys {
		setFlagValue(viperKey)
	}

	// STEP 2: Configure config file paths
	viper.SetConfigName("gitlab-migraptor")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	homeDir, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(homeDir)
	}

	// STEP 3: Try to read config file (ignore errors if file doesn't exist)
	err = viper.ReadInConfig()
	// Note: We ignore errors here because the config file might not exist

	// Copy values from aliased keys (snake_case) to actual keys (kebab-case) for Unmarshal
	// This is needed because:
	// 1. viper.Unmarshal() doesn't use aliases
	// 2. RegisterAlias doesn't work properly with ReadConfig
	// So we manually copy values from config file keys to the keys Unmarshal expects
	// Only do this if a config file was actually read
	// copyAliasedValues will skip copying if flags were already set
	if err == nil {
		copyAliasedValues(cmd)
	}

	// STEP 4: Ensure flags still override config file values (in case copyAliasedValues set something)
	// This is a safety check to ensure flags always win
	for _, viperKey := range flagKeys {
		setFlagValue(viperKey)
	}

	// Manually ensure env vars override config file values (but flags still have highest priority)
	// This is needed because viper might cache config file values and not re-check env vars
	// We check flags first - if a flag has a non-empty value, we skip env var override for that key
	envVarOverrides := map[string]string{
		"token":           "GITLAB_TOKEN",
		"instance":        "GITLAB_INSTANCE",
		"registry":        "GITLAB_REGISTRY",
		"docker-password": "DOCKER_TOKEN",
		"old-group":       "OLD_GROUP_NAME",
		"new-group":       "NEW_GROUP_NAME",
		"parent-group-id": "PARENT_GROUP_ID",
		"projects":        "PROJECTS_LIST",
		"tags":            "TAGS_LIST",
		"keep-parent":     "KEEP_PARENT",
		"dry-run":         "DRY_RUN",
		"verbose":         "VERBOSE",
	}

	// STEP 5: Override config file values with env vars, but only if flags haven't been set
	// Flags have highest priority, so skip env var override if flag was set
	for viperKey, envVarName := range envVarOverrides {
		// Check if flag was set - if so, skip env var override (flags have highest priority)
		if isFlagSet(cmd, viperKey) {
			continue // Flag was set, skip env var override
		}
		// Check if env var is set and override config file value
		if envValue := os.Getenv(envVarName); envValue != "" {
			viper.Set(viperKey, envValue)
		}
	}

	// Unmarshal into Config struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Handle legacy comma-separated env vars for lists
	if projectsEnv := os.Getenv("PROJECTS_LIST"); projectsEnv != "" && len(cfg.ProjectsList) == 0 {
		cfg.ProjectsList = strings.Split(projectsEnv, ",")
		for i := range cfg.ProjectsList {
			cfg.ProjectsList[i] = strings.TrimSpace(cfg.ProjectsList[i])
		}
	}
	if tagsEnv := os.Getenv("TAGS_LIST"); tagsEnv != "" && len(cfg.TagsList) == 0 {
		cfg.TagsList = strings.Split(tagsEnv, ",")
		for i := range cfg.TagsList {
			cfg.TagsList[i] = strings.TrimSpace(cfg.TagsList[i])
		}
	}

	// Apply post-processing defaults
	if cfg.GitLabRegistry == "" {
		cfg.GitLabRegistry = "registry." + cfg.GitLabInstance
	}
	if cfg.DockerToken == "" {
		cfg.DockerToken = cfg.GitLabToken
	}

	return &cfg, nil
}

// Validate checks that all required configuration values are set
func (c *Config) Validate() error {
	if c.GitLabToken == "" {
		return fmt.Errorf("GitLab token is required")
	}
	if c.OldGroupName == "" {
		return fmt.Errorf("old group name is required")
	}
	if c.NewGroupName == "" {
		return fmt.Errorf("new group name is required")
	}
	return nil
}
