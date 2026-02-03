package command

import (
	"fmt"
	"maps"
	"migraptor/internal/check"
	"migraptor/internal/migration"
	"migraptor/internal/ui"
	"os"

	"github.com/spf13/cobra"
)

var Clean = &cobra.Command{
	Use:     "clean",
	Aliases: []string{"cl"},
	Short:   "Clean images from registries",
	Run: func(cmd *cobra.Command, args []string) {
		cleanImages(cmd)
	},
}

func cleanImages(cmd *cobra.Command) {
	consoleUI, err := ui.Init(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize UI: %v\n", err)
		os.Exit(1)
	}
	defer ui.Close()

	gitlabClient, _, cfg, err := check.CheckBeforeStarting(consoleUI, cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check before starting: %v\n", err)
		os.Exit(1)
	}

	// Print start message
	consoleUI.PrintCleanStart(cfg)

	// Initialize migrators
	groupMigrator := migration.NewGroupMigrator(gitlabClient, cfg.DryRun, consoleUI)
	projectMigrator := migration.NewProjectMigrator(gitlabClient, cfg.DryRun, consoleUI)
	// imageMigrator := migration.NewImageMigrator(gitlabClient, dockerClient, cfg.DryRun, consoleUI)

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
	// projectImages := make(map[int][]string)

	// Backup phase: For each project
	for _, project := range allProjects {
		consoleUI.Info("📦 Working on project %s (ID: %d)", project.Name, project.ID)
	}
}
