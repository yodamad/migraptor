package config

import (
	"fmt"
	"os"
	"path/filepath"
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

// copyAliasedValues copies values from aliased keys (snake_case from config file) to actual keys (kebab-case)
// This is needed because:
// 1. viper.Unmarshal() doesn't use aliases
// 2. RegisterAlias doesn't work properly with ReadConfig
// So we check if the snake_case keys exist in the config file and copy them to kebab-case keys
func copyAliasedValues() {
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

	// Configure config file paths
	viper.SetConfigName("gitlab-migraptor")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	homeDir, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(homeDir)
	}

	// Try to read config file (ignore errors if file doesn't exist)
	err = viper.ReadInConfig()
	// Note: We ignore errors here because the config file might not exist

	// Copy values from aliased keys (snake_case) to actual keys (kebab-case) for Unmarshal
	// This is needed because:
	// 1. viper.Unmarshal() doesn't use aliases
	// 2. RegisterAlias doesn't work properly with ReadConfig
	// So we manually copy values from config file keys to the keys Unmarshal expects
	// Only do this if a config file was actually read
	if err == nil {
		copyAliasedValues()
	}

	// Enable automatic environment variable binding (before binding flags)
	viper.AutomaticEnv()

	// Set environment variable prefix and key replacer
	// This allows env vars like MIGRAPTOR_TOKEN, MIGRAPTOR_OLD_GROUP, etc.
	viper.SetEnvPrefix("MIGRAPTOR")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Also support legacy env var names without prefix
	// Map legacy env var names to viper keys (must be done before binding flags)
	viper.BindEnv("token", "GITLAB_TOKEN")
	viper.BindEnv("instance", "GITLAB_INSTANCE")
	viper.BindEnv("registry", "GITLAB_REGISTRY")
	viper.BindEnv("docker-password", "DOCKER_TOKEN")
	viper.BindEnv("old-group", "OLD_GROUP_NAME")
	viper.BindEnv("new-group", "NEW_GROUP_NAME")
	viper.BindEnv("parent-group-id", "PARENT_GROUP_ID")
	viper.BindEnv("projects", "PROJECTS_LIST")
	viper.BindEnv("tags", "TAGS_LIST")
	viper.BindEnv("keep-parent", "KEEP_PARENT")
	viper.BindEnv("dry-run", "DRY_RUN")
	viper.BindEnv("verbose", "VERBOSE")

	// Bind individual Cobra flags to Viper (highest priority)
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

	migraptorEnvOverrides := map[string]string{
		"token":           "MIGRAPTOR_TOKEN",
		"instance":        "MIGRAPTOR_INSTANCE",
		"registry":        "MIGRAPTOR_REGISTRY",
		"docker-password": "MIGRAPTOR_DOCKER_PASSWORD",
		"old-group":       "MIGRAPTOR_OLD_GROUP",
		"new-group":       "MIGRAPTOR_NEW_GROUP",
		"parent-group-id": "MIGRAPTOR_PARENT_GROUP_ID",
		"projects":        "MIGRAPTOR_PROJECTS",
		"tags":            "MIGRAPTOR_TAGS",
		"keep-parent":     "MIGRAPTOR_KEEP_PARENT",
		"dry-run":         "MIGRAPTOR_DRY_RUN",
		"verbose":         "MIGRAPTOR_VERBOSE",
	}

	// Override config file values with env vars, but only if flags haven't been set
	for viperKey, envVarName := range envVarOverrides {
		// Check if flag has a value - if so, skip env var override (flags have highest priority)
		flag := cmd.Flags().Lookup(getFlagNameForViperKey(viperKey))
		if flag != nil {
			// Check if flag has a non-empty value (works for both Changed and Set())
			flagValue := flag.Value.String()
			if flagValue != "" && flagValue != flag.DefValue {
				continue // Flag has a value, skip env var override
			}
		}
		// Check if env var is set and override config file value
		if envValue := os.Getenv(envVarName); envValue != "" {
			viper.Set(viperKey, envValue)
		}
	}

	// Also check MIGRAPTOR prefixed env vars
	for viperKey, envVarName := range migraptorEnvOverrides {
		// Check if flag has a value - if so, skip env var override
		flag := cmd.Flags().Lookup(getFlagNameForViperKey(viperKey))
		if flag != nil {
			flagValue := flag.Value.String()
			if flagValue != "" && flagValue != flag.DefValue {
				continue // Flag has a value, skip env var override
			}
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

// GetConfigFilePath returns the path to the config file if it exists
func GetConfigFilePath() string {
	configPaths := []string{
		"./gitlab-migraptor.yaml",
		"./gitlab-migraptor.yml",
		"./gitlab-migraptor.json",
		"./gitlab-migraptor.toml",
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPaths = append(configPaths,
			filepath.Join(homeDir, ".gitlab-migraptor.yaml"),
			filepath.Join(homeDir, ".gitlab-migraptor.yml"),
			filepath.Join(homeDir, ".gitlab-migraptor.json"),
			filepath.Join(homeDir, ".gitlab-migraptor.toml"),
		)
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
