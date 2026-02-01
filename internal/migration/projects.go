package migration

import (
	"fmt"

	"migraptor/internal/gitlab"
	"migraptor/internal/ui"
)

// ProjectInfo holds information about a project
type ProjectInfo struct {
	ID                       int
	Name                     string
	Path                     string
	ContainerRegistryEnabled bool
	Archived                 bool
	RegistryRepositoriesIDs  []int
}

// ProjectMigrator handles project-related migration operations
type ProjectMigrator struct {
	client    *gitlab.Client
	dryRun    bool
	consoleUI *ui.UI
}

// NewProjectMigrator creates a new ProjectMigrator
func NewProjectMigrator(client *gitlab.Client, dryRun bool, cUI *ui.UI) *ProjectMigrator {
	return &ProjectMigrator{
		client:    client,
		dryRun:    dryRun,
		consoleUI: cUI,
	}
}

// ListProjects lists projects in a group, optionally filtered
func (pm *ProjectMigrator) ListProjects(groupID int64, filterList []string) ([]ProjectInfo, error) {
	projects, _, err := pm.client.ListProjects(int(groupID))
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	var result []ProjectInfo
	for _, project := range projects {
		// Apply filter if provided
		if len(filterList) > 0 {
			found := false
			for _, filter := range filterList {
				if project.Path == filter {
					found = true
					break
				}
			}
			if !found {
				// If project has no registry and keep_parent is true, migrate anyway
				// This logic is handled in the main migration flow
				continue
			}
		}

		info := ProjectInfo{
			ID:                       int(project.ID),
			Name:                     project.Name,
			Path:                     project.Path,
			ContainerRegistryEnabled: project.ContainerRegistryEnabled,
			Archived:                 project.Archived,
		}

		result = append(result, info)
	}

	return result, nil
}

// UnarchiveProject unarchives a project
func (pm *ProjectMigrator) UnarchiveProject(projectName string, projectID int) error {
	pm.consoleUI.PrintUnarchivedMessage(projectName)

	if pm.dryRun {
		pm.consoleUI.Info("🌵 DRY RUN: Would unarchive project %d (%s)", projectID, projectName)
		return nil
	}

	resp, err := pm.client.UnarchiveProject(projectID)
	if err != nil {
		return fmt.Errorf("failed to unarchive project: %w", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		pm.consoleUI.Error("Unable to unarchive project, need to do it by hand")
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// ArchiveProject archives a project
func (pm *ProjectMigrator) ArchiveProject(projectName string, projectID int) error {
	pm.consoleUI.PrintArchivedMessage(projectName)

	if pm.dryRun {
		pm.consoleUI.Info("🌵 DRY RUN: Would archive project %d (%s)", projectID, projectName)
		return nil
	}

	resp, err := pm.client.ArchiveProject(projectID)
	if err != nil {
		return fmt.Errorf("failed to archive project: %w", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		pm.consoleUI.Error("Unable to archive project, need to do it by hand")
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// TransferProject transfers a project to another namespace
func (pm *ProjectMigrator) TransferProject(projectName string, projectID, targetGroupID int) error {
	pm.consoleUI.PrintTransferringProject(projectName, targetGroupID)

	if pm.dryRun {
		pm.consoleUI.Info("🌵 DRY RUN: Would transfer project %d (%s) to group %d", projectID, projectName, targetGroupID)
		return nil
	}

	resp, err := pm.client.TransferProject(projectID, targetGroupID)
	if err != nil {
		return fmt.Errorf("failed to transfer project: %w", err)
	}

	pm.consoleUI.PrintMoveResult(fmt.Sprintf("%d", resp.StatusCode))
	return nil
}

// ShouldMigrateProject checks if a project should be migrated based on filters
func ShouldMigrateProject(project ProjectInfo, filterList []string, keepParent bool) bool {
	if len(filterList) == 0 {
		return true
	}

	for _, filter := range filterList {
		if project.Path == filter {
			return true
		}
	}

	// If project has no registry and keep_parent is true, migrate anyway
	if !project.ContainerRegistryEnabled && keepParent {
		return true
	}

	return false
}
