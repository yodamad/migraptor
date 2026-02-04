package command

import (
	"fmt"
	"maps"
	"migraptor/internal/check"
	"migraptor/internal/config"
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

func init() {
	Clean.Flags().BoolP(config.BACKUP_IMAGES, "b", true, "Backup images before deleting them")
}

func cleanImages(cmd *cobra.Command) {
	consoleUI, err := ui.Init(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize UI: %v\n", err)
		os.Exit(1)
	}
	defer ui.Close()

	gitlabClient, dockerClient, cfg, err := check.CheckBeforeStarting(consoleUI, cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to check before starting: %v\n", err)
		os.Exit(1)
	}

	// Print start message
	consoleUI.PrintCleanStart(cfg)

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

	// Create initial image selector model
	selectorModel := ui.NewImageSelectorModel(allImages, gitlabClient, cfg.DryRun)

	// Loop between selector and summary until user confirms
	selectedImages := []ui.ImageItem{}
	for {
		// Run image selector
		program := tea.NewProgram(selectorModel, tea.WithAltScreen())
		finalModel, err := program.Run()
		if err != nil {
			consoleUI.Error("Failed to run image selector: %v", err)
			os.Exit(1)
		}

		// Get final model state
		var ok bool
		selectorModel, ok = finalModel.(*ui.ImageSelectorModel)
		if !ok {
			break
		}

		selectedImages = selectorModel.GetSelectedImages()
		if len(selectedImages) == 0 {
			consoleUI.Info("🤔 No images were selected.")
			break
		}

		// Show summary
		summaryModel := ui.NewImageSummaryModel(selectedImages)
		summaryProgram := tea.NewProgram(summaryModel, tea.WithAltScreen())
		summaryFinalModel, err := summaryProgram.Run()
		if err != nil {
			consoleUI.Error("Failed to run summary display: %v", err)
			break
		}

		// Check if user wants to go back
		if finalSummaryModel, ok := summaryFinalModel.(*ui.ImageSummaryModel); ok {
			if finalSummaryModel.WentBack() {
				// Restore selections and continue loop
				selectorModel.RestoreSelections(selectedImages)
				continue
			}
		}

		// User quit summary normally, exit loop
		break
	}

	if len(selectedImages) == 0 {
		consoleUI.Warning("No image was selected.")
		os.Exit(0)
	}

	// Add confirmation message be starting
	consoleUI.Confirmation("🙈 Delete %d images ? (y/n)", len(selectedImages))
	var response string
	fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		consoleUI.Error("Cleaning cancelled by user.")
		os.Exit(1)
	}

	if cfg.BackupImages {
		consoleUI.Info("🛟 Backup images locally before deleting")
	} else {
		consoleUI.Confirmation("🛟 Backup images before (docker pull) ? (y/n)")
	}

	fmt.Scanln(&response)
	if response == "y" || response == "Y" {
		// Group selected images by project ID upfront for O(1) lookup
		// This avoids iterating through all selected images for each project
		// Complexity: O(S + P) instead of O(P * S)
		imagesByProject := make(map[int][]string)
		for _, img := range selectedImages {
			imagesByProject[img.ProjectID] = append(imagesByProject[img.ProjectID], img.ImageInfo.Name)
		}

		// Iterate through projects and only process those with selected images
		for _, proj := range allProjects {
			projectSelectedImages, hasImages := imagesByProject[proj.ID]
			if !hasImages || len(projectSelectedImages) == 0 {
				continue
			}

			_, _, err := imageMigrator.BackupImages(proj, projectSelectedImages)
			if err != nil {
				consoleUI.Error("Failed to backup images: %v", err)
				os.Exit(1)
			}
		}
	} else {
		consoleUI.Warning("Backup skipped.")
	}

	// Delete selected images
	consoleUI.Info("🗑️  Starting deletion of %d images...", len(selectedImages))

	deletedCount := 0
	failedCount := 0
	totalImages := len(selectedImages)

	for i, img := range selectedImages {
		imageNum := i + 1
		if cfg.DryRun {
			consoleUI.Info("🌵 DRY RUN: Would delete image %d of %d: %s (Project: %s, Registry: %s)",
				imageNum, totalImages, img.ImageInfo.Name, img.ProjectName, img.RegistryPath)
			deletedCount++
		} else {
			consoleUI.Info("🗑️  Deleting image %d of %d: %s (Project: %s, Registry: %s)",
				imageNum, totalImages, img.ImageInfo.Name, img.ProjectName, img.RegistryPath)

			_, err := gitlabClient.DeleteRegistryRepositoryTag(img.ProjectID, img.RegistryID, img.ImageInfo.Name)
			if err != nil {
				consoleUI.Error("Failed to delete image %s: %v", img.ImageInfo.Name, err)
				failedCount++
			} else {
				deletedCount++
			}
		}
	}

	// Display final summary
	if cfg.DryRun {
		consoleUI.Info("🌵 DRY RUN: Would have deleted %d images", deletedCount)
	} else {
		consoleUI.Info("✅ Successfully deleted %d images", deletedCount)
		if failedCount > 0 {
			consoleUI.Error("❌ Failed to delete %d images", failedCount)
		}
	}

	// Exit with appropriate code
	if failedCount > 0 {
		os.Exit(1)
	}
}
