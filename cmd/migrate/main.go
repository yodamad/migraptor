package main

import (
	"fmt"
	"maps"
	"migraptor/internal/check"
	"os"
	"strings"
	"time"

	"migraptor/internal/config"
	"migraptor/internal/migration"
	"migraptor/internal/ui"

	"github.com/spf13/cobra"
)

var (
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
	// Note: Flags are not bound to struct fields - they will be loaded via Viper
	rootCmd.Flags().StringP(config.GITLAB_TOKEN, "g", "", "your gitlab API token")
	rootCmd.Flags().StringP(config.OLD_GROUP_NAME, "o", "", "the group containing the projects you want to migrate")
	rootCmd.Flags().StringP(config.NEW_GROUP_NAME, "n", "", "the full path of group that will contain the migrated projects")
	rootCmd.Flags().BoolP(config.DRY_RUN, "f", false, "fake run")
	rootCmd.Flags().StringP(config.GITLAB_INSTANCE, "i", "", "change gitlab instance. By default, it's gitlab.com")
	rootCmd.Flags().BoolP(config.KEEP_PARENT, "k", false, "don't keep the parent group, transfer projects individually instead")
	rootCmd.Flags().StringSliceP(config.PROJECTS_LIST, "l", []string{}, "list projects to move if you want to keep some in origin group (comma-separated)")
	rootCmd.Flags().StringP(config.DOCKER_PASSWORD, "p", "", "password for registry")
	rootCmd.Flags().StringP(config.GITLAB_REGISTRY, "r", "", "change gitlab registry name if not registry.<gitlab_instance>. By default, it's registry.gitlab.com")
	rootCmd.Flags().StringSliceP(config.TAGS_LIST, "t", []string{}, "filter tags to keep when moving images & registries (comma-separated)")
	rootCmd.Flags().BoolP(config.VERBOSE, "v", false, "verbose mode to debug your migration")

	//rootCmd.SetHelpTemplate(ui.PrintUsage())

	rootCmd.AddCommand()
}

func runMigration(cmd *cobra.Command, args []string) {
	currentUI, err := ui.Init(false)
	consoleUI = currentUI
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize UI: %v\n", err)
		os.Exit(1)
	}
	defer ui.Close()

	gitlabClient, dockerClient, cfg, err := check.CheckBeforeStarting(currentUI, cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check before starting: %v\n", err)
		os.Exit(1)
	}

	// Print start message
	consoleUI.PrintMigrationStart(cfg)

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

	allProjects := make(map[int]*migration.ProjectInfo)
	for _, proj := range projects {
		allProjects[proj.ID] = &proj
	}

	subGroups, subProjects, err := groupMigrator.GetSubGroupsAndProjects(groupFound.ID, cfg.ProjectsList)

	maps.Copy(allProjects, subProjects)

	if len(subGroups) > 0 {
		consoleUI.Info("📂 Found %d sub-groups to consider", len(subGroups))
	}
	consoleUI.Info("📦 Found %d projects to migrate", len(allProjects))

	// Store image lists per project
	projectImages := make(map[int][]string)

	// Backup phase: For each project
	for _, project := range allProjects {
		if !migration.ShouldMigrateProject(*project, cfg.ProjectsList, cfg.KeepParent) {
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
			images, err := imageMigrator.BackupImages(project, cfg.TagsList)
			consoleUI.Info("👀 Found %d registries in project %s", len(project.RegistryRepositoriesIDs), project.Path)
			if err != nil {
				consoleUI.Error("Failed to backup images: %v", err)
				os.Exit(99)
			}
			projectImages[project.ID] = images
		}
	}
	if len(projectImages) > 0 {
		if err := imageMigrator.CheckIfRemainingImages(allProjects, cfg.TagsList); err != nil {
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
	for _, project := range allProjects {
		if !migration.ShouldMigrateProject(*project, cfg.ProjectsList, cfg.KeepParent) {
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
