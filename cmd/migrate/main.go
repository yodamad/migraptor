package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitlab-transfer-script/internal/config"
	"gitlab-transfer-script/internal/docker"
	"gitlab-transfer-script/internal/gitlab"
	"gitlab-transfer-script/internal/migration"
	"gitlab-transfer-script/internal/ui"
)

var (
	cfg       *config.Config = &config.Config{}
	consoleUI *ui.UI
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "migrate",
	Short: "GitLab project migration tool",
	Long: `Migrate GitLab projects (with Docker container images) between groups.
This tool helps transfer GitLab projects that contain images in Container registry
from a group to another, as it's not possible through GitLab UI.`,
	Run: runMigration,
}

func init() {
	// Define flags matching bash script interface
	rootCmd.Flags().StringVarP(&cfg.GitLabToken, "token", "g", cfg.GitLabToken, "your gitlab API token")
	rootCmd.Flags().StringVarP(&cfg.OldGroupName, "old-group", "o", cfg.OldGroupName, "the group containing the projects you want to migrate")
	rootCmd.Flags().StringVarP(&cfg.NewGroupName, "new-group", "n", cfg.NewGroupName, "the full path of group that will contain the migrated projects")
	rootCmd.Flags().BoolVarP(&cfg.DryRun, "dry-run", "f", cfg.DryRun, "fake run")
	rootCmd.Flags().StringVarP(&cfg.GitLabInstance, "instance", "i", cfg.GitLabInstance, "change gitlab instance. By default, it's gitlab.com")
	rootCmd.Flags().BoolVarP(&keepParentFlag, "keep-parent", "k", false, "don't keep the parent group, transfer projects individually instead")
	rootCmd.Flags().StringVarP(&projectsListStr, "projects", "l", "", "list projects to move if you want to keep some in origin group (comma-separated)")
	rootCmd.Flags().StringVarP(&cfg.DockerToken, "docker-password", "p", cfg.DockerToken, "password for registry")
	rootCmd.Flags().StringVarP(&cfg.GitLabRegistry, "registry", "r", cfg.GitLabRegistry, "change gitlab registry name if not registry.<gitlab_instance>. By default, it's registry.gitlab.com")
	rootCmd.Flags().StringVarP(&tagsListStr, "tags", "t", "", "filter tags to keep when moving images & registries (comma-separated)")
	rootCmd.Flags().BoolVarP(&cfg.Verbose, "verbose", "v", cfg.Verbose, "verbose mode to debug your migration")

	//rootCmd.SetHelpTemplate(printUsage())
}

var (
	projectsListStr string
	tagsListStr     string
	keepParentFlag  bool
)

func runMigration(cmd *cobra.Command, args []string) {
	// Initialize UI
	currentUI, err := ui.Init(false)
	consoleUI = currentUI
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize UI: %v\n", err)
		os.Exit(1)
	}
	defer ui.Close()

	// Load base config
	cfg, err = LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	// Update keep-parent from flag
	// In bash script: -k sets keep_parent="n" (don't keep parent)
	// Default is keep_parent="y" (keep parent, transfer whole group)
	if keepParentFlag {
		cfg.KeepParent = false // -k flag means don't keep parent
	}
	// Otherwise keep the default value from config (true)

	// Parse comma-separated lists
	if projectsListStr != "" {
		cfg.ProjectsList = strings.Split(projectsListStr, ",")
		for i := range cfg.ProjectsList {
			cfg.ProjectsList[i] = strings.TrimSpace(cfg.ProjectsList[i])
		}
	}

	if tagsListStr != "" {
		cfg.TagsList = strings.Split(tagsListStr, ",")
		for i := range cfg.TagsList {
			cfg.TagsList[i] = strings.TrimSpace(cfg.TagsList[i])
		}
	}

	// Set default registry if not set
	if cfg.GitLabRegistry == "" {
		cfg.GitLabRegistry = "registry." + cfg.GitLabInstance
	}

	// Use GitLab token as Docker token if not set
	if cfg.DockerToken == "" {
		cfg.DockerToken = cfg.GitLabToken
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		consoleUI.Error("Configuration error: %v", err)
		printUsage()
		os.Exit(1)
	}

	// Print start message
	consoleUI.PrintMigrationStart(cfg)

	// Initialize GitLab client
	consoleUI.Info("🦊 Creating GitLab client...")
	gitlabClient, err := gitlab.NewClient(cfg.GitLabToken, cfg.GitLabInstance)
	if err != nil {
		consoleUI.Error("Failed to create GitLab client: %v", err)
		os.Exit(1)
	}

	// Check GitLab connection
	if err := gitlabClient.CheckConnection(); err != nil {
		consoleUI.Error("Failed to connect to GitLab: %v", err)
		os.Exit(1)
	}
	consoleUI.Success("GitLab client created successfully\n")

	// Initialize Docker client
	consoleUI.Info("🐳 Creating Docker client...")
	dockerClient, err := docker.NewClient()
	if err != nil {
		consoleUI.Error("Failed to create Docker client: %v", err)
		os.Exit(1)
	}
	defer dockerClient.Close()
	consoleUI.Success("Docker client created successfully\n")

	// Check Docker is running
	if err := dockerClient.CheckDockerRunning(); err != nil {
		consoleUI.PrintDockerNotStarted()
		os.Exit(99)
	}
	consoleUI.Success("Docker is running\n")

	// Check Docker registry login
	consoleUI.Info("🔑 Checking registry login...")

	// Try to login automatically
	user, _, err := gitlabClient.GetCurrentUser()
	if err != nil {
		consoleUI.PrintDockerLoginFailed()
		os.Exit(99)
	}

	authInfo, err := dockerClient.Login(cfg.GitLabRegistry, user.Username, cfg.DockerToken)
	if err != nil {
		consoleUI.PrintDockerLoginFailed()
		os.Exit(99)
	}
	dockerClient.SetAuthInfo(authInfo)
	consoleUI.PrintDockerLoginSuccess()

	consoleUI.Success("Registry login checked successfully\n")

	// Initialize migrators
	groupMigrator := migration.NewGroupMigrator(gitlabClient, cfg.DryRun, consoleUI)
	projectMigrator := migration.NewProjectMigrator(gitlabClient, cfg.DryRun, consoleUI)
	imageMigrator := migration.NewImageMigrator(gitlabClient, dockerClient, cfg.DryRun, consoleUI)

	// Search for source group
	consoleUI.Info("🔍 Searching for source group...")
	groupFound, err := groupMigrator.SearchGroup(cfg.OldGroupName)
	if err != nil {
		consoleUI.Error("Failed to search for group: %v", err)
		os.Exit(321)
	}

	if groupFound == nil {
		consoleUI.PrintGroupNotFound(cfg.OldGroupName)
		os.Exit(321)
	}

	consoleUI.Debug("Found group with ID %d", groupFound.ID)

	oldGroupFullPath := groupFound.FullPath
	oldGroupPath := groupFound.Path

	// Build new group path
	newGroupPath := strings.TrimPrefix(cfg.NewGroupName, "/")
	consoleUI.Info("🛤️ Migrating group to new path: %s", newGroupPath)

	// Create destination group structure
	newGroup, err := groupMigrator.SearchGroup(newGroupPath)
	if err != nil {
		consoleUI.Error("Failed to create groups: %v", err)
		os.Exit(99)
	}

	// List projects
	projects, err := projectMigrator.ListProjects(groupFound.ID, cfg.ProjectsList)
	if err != nil {
		consoleUI.Error("Failed to list projects: %v", err)
		os.Exit(1)
	}

	if len(projects) == 0 {
		consoleUI.PrintNoProjectsFound()
		os.Exit(1)
	}

	// Store image lists per project
	projectImages := make(map[int][]string)

	// Backup phase: For each project
	for _, project := range projects {
		if !migration.ShouldMigrateProject(project, cfg.ProjectsList, cfg.KeepParent) {
			consoleUI.Info("Not migrating %s, not in filter list", project.Path)
			continue
		}

		consoleUI.PrintProjectHeader(project.Path, "💾 Backup")

		// Unarchive if needed
		if project.Archived {
			if err := projectMigrator.UnarchiveProject(project.Path, project.ID); err != nil {
				consoleUI.Error("Failed to unarchive project: %v", err)
				continue
			}
		}

		// Backup images if registry is enabled
		if project.ContainerRegistryEnabled {
			images, err := imageMigrator.BackupImages(&project, cfg.TagsList)
			consoleUI.Info("👀 Found %d registries in project %s", len(project.RegistryRepositoriesIDs), project.Path)
			if err != nil {
				consoleUI.Error("Failed to backup images: %v", err)
				os.Exit(99)
			}
			projectImages[project.ID] = images
		}
	}
	if len(projectImages) > 0 {
		if err := imageMigrator.CheckIfRemainingImages(projects, cfg.TagsList); err != nil {
			consoleUI.Error("Failed to check if remaining images: %v", err)
			os.Exit(99)
		}
	}

	// Transfer group if keep-parent
	if cfg.KeepParent {
		if len(cfg.ProjectsList) == 0 {
			consoleUI.PrintTransferringGroup(cfg.OldGroupName, cfg.NewGroupName)
			if err := groupMigrator.TransferGroup(groupFound.ID, int(newGroup.ID)); err != nil {
				consoleUI.Error("Failed to transfer group: %v", err)
				os.Exit(99)
			}

			// Wait a bit after transfer
			if !cfg.DryRun {
				consoleUI.SleepWithLog(10 * time.Second)
			}
		} else {
			// Only migrate some projects, cannot use transfer group
			// Get the last part of old path to use as new group name
			oldPathParts := strings.Split(oldGroupPath, "/")
			simplePath := oldPathParts[len(oldPathParts)-1]

			newGroupFullPath := fmt.Sprintf("%s/%s", newGroupPath, simplePath)
			newGroupAlreadyExists, err := groupMigrator.SearchGroup(newGroupFullPath)
			if err != nil {
				consoleUI.Info("🪄 New group %s does not exist yet, creating it...", newGroupFullPath)
				newGroupID := int(newGroup.ID)
				newGroupCreated, _, err := gitlabClient.CreateGroup(simplePath, &newGroupID)
				if err != nil {
					consoleUI.Error("Failed to create new group: %v", err)
					os.Exit(99)
				}
				newGroup = newGroupCreated
			} else {
				consoleUI.Info("ℹ️ New group %s already exists, using it...", newGroupFullPath)
				newGroup = newGroupAlreadyExists
			}
		}
	}

	// Restore phase: For each project
	for _, project := range projects {
		if !migration.ShouldMigrateProject(project, cfg.ProjectsList, cfg.KeepParent) {
			continue
		}

		consoleUI.PrintProjectHeader(project.Path, "🪄 Restore")

		// Transfer project if not keep-parent or if keep-parent and project is in filter list
		if !cfg.KeepParent || len(cfg.ProjectsList) > 0 {
			if err := projectMigrator.TransferProject(project.Path, project.ID, int(newGroup.ID)); err != nil {
				consoleUI.Error("Failed to transfer project: %v", err)
				continue
			}

			// Wait a bit after transfer
			if !cfg.DryRun {
				consoleUI.SleepWithLog(10 * time.Second)
			}
		}

		// Restore images
		if images, ok := projectImages[project.ID]; ok && len(images) > 0 {
			var newPath string
			if cfg.KeepParent {
				newPath = fmt.Sprintf("%s/%s", newGroupPath, oldGroupPath)
			} else {
				newPath = newGroupPath
			}

			if err := imageMigrator.RestoreImages(images, oldGroupFullPath, newPath, cfg.KeepParent); err != nil {
				consoleUI.Error("Failed to restore images: %v", err)
				continue
			}
		}

		// Re-archive if needed
		if project.Archived {
			if err := projectMigrator.ArchiveProject(project.Path, project.ID); err != nil {
				consoleUI.Error("Failed to archive project: %v", err)
				continue
			}
		}

		consoleUI.PrintMigrationComplete(project.Path)
	}

	if cfg.DryRun {
		consoleUI.PrintDryRunSuccess()
	}
}

func printUsage() string {
	fmt.Println("Usage : ./migrate -g <GITLAB_TOKEN> -o <OLD_GROUP_NAME> -n <NEW_GROUP_NAME>")
	fmt.Println("=============================================================================")
	fmt.Println("Mandatory options")
	fmt.Println("-----------------")
	fmt.Println("-g : your gitlab API token")
	fmt.Println("-n : the full path of group that will contain the migrated projects")
	fmt.Println("-o : the group containing the projects you want to migrate")
	fmt.Println("-s : the simple path of group containing the projects you want to migrate, in same parent group then original one")
	fmt.Println("-----------------")
	fmt.Println("Other options")
	fmt.Println("-------------")
	fmt.Println("-d : parent group id (if there are multiple with same name on the instance)")
	fmt.Println("-f : fake run")
	fmt.Println("-h : display usage")
	fmt.Println("-i : change gitlab instance. By default, it's gitlab.com")
	fmt.Println("-k : keep the group containing the project, it will be moved into group specified with -n")
	fmt.Println("-l : list projects to move if you want to keep some in origin group")
	fmt.Println("-p : password for registry")
	fmt.Println("-r : change gitlab registry name if not registry.<gitlab_instance>. By default, it's registry.gitlab.com")
	fmt.Println("-t : filter tags to keep when moving images & registries")
	fmt.Println("-v : verbose mode to debug your migration")
	return ""
}

// LoadConfig loads configuration from multiple sources with priority:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Config file
// 4. Interactive prompts (for missing mandatory values)
func LoadConfig() (*config.Config, error) {
	cfg := &config.Config{
		GitLabInstance: "gitlab.com",
		KeepParent:     true,
	}

	// Load from config file first (lowest priority)
	config.LoadConfigFile(cfg)

	// Load from environment variables
	config.LoadFromEnv(cfg)

	// Interactive prompts for missing mandatory values
	if err := promptMissingValues(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// promptMissingValues prompts user for missing mandatory configuration values
func promptMissingValues(cfg *config.Config) error {
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
