package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the migration tool
type Config struct {
	GitLabToken    string
	GitLabInstance string
	GitLabRegistry string
	DockerToken    string
	OldGroupName   string
	NewGroupName   string
	ParentGroupID  int
	ProjectsList   []string
	TagsList       []string
	KeepParent     bool
	DryRun         bool
	Verbose        bool
}

// loadConfigFile loads configuration from YAML/TOML/JSON config file
func LoadConfigFile(cfg *Config) {
	viper.SetConfigName("gitlab-migrate")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME")

	// Try to read config file (ignore errors if file doesn't exist)
	_ = viper.ReadInConfig()

	if viper.IsSet("gitlab_token") {
		cfg.GitLabToken = viper.GetString("gitlab_token")
	}
	if viper.IsSet("gitlab_instance") {
		cfg.GitLabInstance = viper.GetString("gitlab_instance")
	}
	if viper.IsSet("gitlab_registry") {
		cfg.GitLabRegistry = viper.GetString("gitlab_registry")
	}
	if viper.IsSet("docker_token") {
		cfg.DockerToken = viper.GetString("docker_token")
	}
	if viper.IsSet("old_group_name") {
		cfg.OldGroupName = viper.GetString("old_group_name")
	}
	if viper.IsSet("new_group_name") {
		cfg.NewGroupName = viper.GetString("new_group_name")
	}
	if viper.IsSet("parent_group_id") {
		cfg.ParentGroupID = viper.GetInt("parent_group_id")
	}
	if viper.IsSet("projects_list") {
		cfg.ProjectsList = viper.GetStringSlice("projects_list")
	}
	if viper.IsSet("tags_list") {
		cfg.TagsList = viper.GetStringSlice("tags_list")
	}
	if viper.IsSet("keep_parent") {
		cfg.KeepParent = viper.GetBool("keep_parent")
	}
	if viper.IsSet("dry_run") {
		cfg.DryRun = viper.GetBool("dry_run")
	}
	if viper.IsSet("verbose") {
		cfg.Verbose = viper.GetBool("verbose")
	}
}

// loadFromEnv loads configuration from environment variables
func LoadFromEnv(cfg *Config) {
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		cfg.GitLabToken = token
	}
	if instance := os.Getenv("GITLAB_INSTANCE"); instance != "" {
		cfg.GitLabInstance = instance
	}
	if registry := os.Getenv("GITLAB_REGISTRY"); registry != "" {
		cfg.GitLabRegistry = registry
	}
	if dockerToken := os.Getenv("DOCKER_TOKEN"); dockerToken != "" {
		cfg.DockerToken = dockerToken
	}
	if oldGroup := os.Getenv("OLD_GROUP_NAME"); oldGroup != "" {
		cfg.OldGroupName = oldGroup
	}
	if newGroup := os.Getenv("NEW_GROUP_NAME"); newGroup != "" {
		cfg.NewGroupName = newGroup
	}
	if parentID := os.Getenv("PARENT_GROUP_ID"); parentID != "" {
		var id int
		if _, err := fmt.Sscanf(parentID, "%d", &id); err == nil {
			cfg.ParentGroupID = id
		}
	}
	if projects := os.Getenv("PROJECTS_LIST"); projects != "" {
		cfg.ProjectsList = strings.Split(projects, ",")
	}
	if tags := os.Getenv("TAGS_LIST"); tags != "" {
		cfg.TagsList = strings.Split(tags, ",")
	}
	if keepParent := os.Getenv("KEEP_PARENT"); keepParent != "" {
		cfg.KeepParent = keepParent == "true" || keepParent == "1" || keepParent == "yes"
	}
	if dryRun := os.Getenv("DRY_RUN"); dryRun != "" {
		cfg.DryRun = dryRun == "true" || dryRun == "1" || dryRun == "yes"
	}
	if verbose := os.Getenv("VERBOSE"); verbose != "" {
		cfg.Verbose = verbose == "true" || verbose == "1" || verbose == "yes"
	}
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
		"./gitlab-migrate.yaml",
		"./gitlab-migrate.yml",
		"./gitlab-migrate.json",
		"./gitlab-migrate.toml",
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPaths = append(configPaths,
			filepath.Join(homeDir, ".gitlab-migrate.yaml"),
			filepath.Join(homeDir, ".gitlab-migrate.yml"),
			filepath.Join(homeDir, ".gitlab-migrate.json"),
			filepath.Join(homeDir, ".gitlab-migrate.toml"),
		)
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
