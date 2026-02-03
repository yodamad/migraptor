package command

import (
	"fmt"
	"maps"
	"migraptor/internal/check"
	"migraptor/internal/migration"
	"migraptor/internal/ui"
	"os"

	tea "github.com/charmbracelet/bubbletea"
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
	consoleUI.Info("📦 Found %d projects", len(allProjects))

	// Collect all images from all projects
	consoleUI.Info("🔍 Collecting images from all registries...")
	imageMigrator := migration.NewImageMigrator(gitlabClient, nil, cfg.DryRun, consoleUI)
	allImagesPtr, err := imageMigrator.GetAllImagesFromProjects(allProjects, cfg.TagsList)
	if err != nil {
		consoleUI.Error("Failed to collect images: %v", err)
		os.Exit(1)
	}

	if len(allImagesPtr) == 0 {
		consoleUI.Info("No images found in any registry")
		return
	}

	// Convert pointers to values
	allImages := make([]ui.ImageItem, len(allImagesPtr))
	for i, img := range allImagesPtr {
		allImages[i] = *img
	}

	consoleUI.Info("📸 Found %d images across all registries", len(allImages))
	consoleUI.Info("Opening image selector...")

	// Create and run bubbletea program
	model := ui.NewImageSelectorModel(allImages, gitlabClient, cfg.DryRun)
	program := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := program.Run()
	if err != nil {
		consoleUI.Error("Failed to run image selector: %v", err)
		os.Exit(1)
	}

	// Get final model state and print selected images
	if selectorModel, ok := finalModel.(*ui.ImageSelectorModel); ok {
		selectedImages := selectorModel.GetSelectedImages()
		if len(selectedImages) > 0 {
			consoleUI.Info("")
			consoleUI.Info("========================================")
			consoleUI.Info("Selected Images Summary")
			consoleUI.Info("========================================")
			for _, img := range selectedImages {
				consoleUI.Info("Project: %s", img.ProjectName)
				consoleUI.Info("Registry: %s", img.RegistryPath)
				consoleUI.Info("Image: %s (%s)", img.ImageInfo.Name, img.ImageInfo.Location)
				consoleUI.Info("")
			}
			consoleUI.Info("Total: %d selected image(s)", len(selectedImages))
			consoleUI.Info("========================================")
		} else {
			consoleUI.Info("No images were selected.")
		}
	}
}
