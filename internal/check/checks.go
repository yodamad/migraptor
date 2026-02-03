package check

import (
	"bufio"
	"fmt"
	"migraptor/internal/config"
	"migraptor/internal/docker"
	"migraptor/internal/gitlab"
	"migraptor/internal/ui"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func CheckBeforeStarting(currentUI *ui.UI, cmd *cobra.Command) (*gitlab.Client, *docker.Client, *config.Config, error) {
	// Initialize UI
	consoleUI := currentUI

	consoleUI.Info("🛂 Doing some prechecks...")
	consoleUI.Info("----------------------------------------")

	// Load config from all sources (flags, env, config file)
	cfg, err := LoadConfig(cmd, consoleUI)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		consoleUI.Error("Configuration error: %v", err)
		ui.PrintUsage()
		return nil, nil, nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Initialize GitLab client
	consoleUI.Info("🦊 Creating GitLab client...")
	gitlabClient, err := gitlab.NewClient(cfg.GitLabToken, cfg.GitLabInstance)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Check GitLab connection
	if err := gitlabClient.CheckConnection(); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to GitLab: %w", err)
	}
	consoleUI.Success("GitLab client created successfully")

	// Initialize Docker client
	consoleUI.Info("🐳 Creating Docker client...")
	dockerClient, err := docker.NewClient()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()
	consoleUI.Success("Docker client created successfully")

	// Check Docker is running
	if err := dockerClient.CheckDockerRunning(); err != nil {
		consoleUI.PrintDockerNotStarted()
		return nil, nil, nil, fmt.Errorf("Docker is not running: %w", err)
	}
	consoleUI.Success("Docker is running")

	// Check Docker registry login
	consoleUI.Info("🔑 Checking registry login...")

	// Try to login automatically
	user, _, err := gitlabClient.GetCurrentUser()
	if err != nil {
		consoleUI.PrintDockerLoginFailed()
		return nil, nil, nil, fmt.Errorf("failed to get current user: %w", err)
	}

	authInfo, err := dockerClient.Login(cfg.GitLabRegistry, user.Username, cfg.DockerToken)
	if err != nil {
		consoleUI.PrintDockerLoginFailed()
		return nil, nil, nil, fmt.Errorf("failed to login to Docker registry: %w", err)
	}
	dockerClient.SetAuthInfo(authInfo)
	consoleUI.PrintDockerLoginSuccess()

	consoleUI.Success("Registry login checked successfully")

	return gitlabClient, dockerClient, cfg, nil
}

// LoadConfig loads configuration from multiple sources with priority:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Config file
// 4. Defaults
// 5. Interactive prompts (for missing mandatory values)
func LoadConfig(cmd *cobra.Command, consoleUI *ui.UI) (*config.Config, error) {
	// Load config using unified Viper-based loader
	cfg, err := config.LoadConfig(cmd)
	if err != nil {
		return nil, err
	}

	// Handle keep-parent flag logic
	// The -k flag means "don't keep parent" (inverted logic)
	// The flag default is false, but KeepParent should default to true
	// So we need to check if the flag was explicitly set
	if cmd.Flags().Changed(config.KEEP_PARENT) {
		keepParentFlag, _ := cmd.Flags().GetBool(config.KEEP_PARENT)
		// When -k is set (true), it means "don't keep parent", so KeepParent = false
		// When -k is not set but flag was changed to false, KeepParent should remain true
		// So we only set KeepParent to false if the flag is explicitly true
		if keepParentFlag {
			cfg.KeepParent = false // -k flag means don't keep parent
		}
	} else {
		// Flag was not set, ensure default is true (unless set by config file or env)
		// Viper default is already set to true in config.LoadConfig
	}

	// Interactive prompts for missing mandatory values
	if err := promptMissingValues(cfg, consoleUI); err != nil {
		return nil, err
	}

	return cfg, nil
}

// promptMissingValues prompts user for missing mandatory configuration values
func promptMissingValues(cfg *config.Config, consoleUI *ui.UI) error {
	if cfg.GitLabToken != "" && cfg.OldGroupName != "" && cfg.NewGroupName != "" {
		return nil
	}
	consoleUI.Warning("========================================\n")
	consoleUI.Warning("Missing some mandatory values...")
	reader := bufio.NewReader(os.Stdin)

	if cfg.GitLabToken == "" {
		consoleUI.Question("GitLab API Token: ")
		token, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read GitLab token: %w", err)
		}
		cfg.GitLabToken = strings.TrimSpace(token)
	}

	if cfg.OldGroupName == "" {
		consoleUI.Question("🏚️ Old Group Name (source): ")
		oldGroup, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read old group name: %w", err)
		}
		cfg.OldGroupName = strings.TrimSpace(oldGroup)
	}

	if cfg.NewGroupName == "" {
		consoleUI.Question("🏡 New Group Name (destination): ")
		newGroup, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read new group name: %w", err)
		}
		cfg.NewGroupName = strings.TrimSpace(newGroup)
	}

	// Set default registry if not set
	if cfg.GitLabRegistry == "" {
		cfg.GitLabRegistry = "registry." + cfg.GitLabInstance
	}

	// Use GitLab token as Docker token if not set
	if cfg.DockerToken == "" {
		cfg.DockerToken = cfg.GitLabToken
	}

	return nil
}
