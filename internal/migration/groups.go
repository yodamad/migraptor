package migration

import (
	"fmt"
	"maps"
	"migraptor/internal/gitlab"
	"migraptor/internal/ui"

	gitlabCore "gitlab.com/gitlab-org/api/client-go"
)

// GroupMigrator handles group-related migration operations
type GroupMigrator struct {
	client    *gitlab.Client
	consoleUI *ui.UI
	dryRun    bool
}

// NewGroupMigrator creates a new GroupMigrator
func NewGroupMigrator(client *gitlab.Client, dryRun bool, cUI *ui.UI) *GroupMigrator {
	return &GroupMigrator{
		client:    client,
		dryRun:    dryRun,
		consoleUI: cUI,
	}
}

// SearchGroup searches for a group by name/path
func (gm *GroupMigrator) SearchGroup(name string) (*gitlabCore.Group, error) {
	gm.consoleUI.Debug("Searching for group: %s", name)

	result, err := gm.client.SearchGroup(name)
	if err != nil {
		return nil, fmt.Errorf("failed to search group %s: %w", name, err)
	}

	return result, nil
}

func (gm *GroupMigrator) GetSubGroupsAndProjects(groupID int64, filterList []string) (map[int64]*gitlabCore.Group, map[int]*ProjectInfo, error) {

	allProjects := make(map[int]*ProjectInfo)
	allSubGroups := make(map[int64]*gitlabCore.Group)

	subgroups, err := gm.client.GetSubGroups(groupID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get subgroups for group %d: %w", groupID, err)
	}

	for _, subgroup := range subgroups {
		subGrpID := subgroup.ID
		allSubGroups[subGrpID] = &*subgroup
		subprojects, _, _ := gm.client.ListProjects(int(subGrpID))
		for _, subproject := range FilterProjects(subprojects, filterList) {
			allProjects[subproject.ID] = &subproject
		}

		innerGroups, innerProjects, err := gm.GetSubGroupsAndProjects(subGrpID, filterList)
		maps.Copy(allProjects, innerProjects)
		maps.Copy(allSubGroups, innerGroups)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get inner subgroups for group %d: %w", subGrpID, err)
		}
	}

	return allSubGroups, allProjects, nil
}

// TransferGroup transfers a group to another group
func (gm *GroupMigrator) TransferGroup(groupID int64, targetGroupID int) error {
	gm.consoleUI.PrintTransferringGroup(fmt.Sprintf("group-%d", groupID), fmt.Sprintf("group-%d", targetGroupID))

	if gm.dryRun {
		gm.consoleUI.Info("🌵 DRY RUN: Would transfer group %d to group %d", groupID, targetGroupID)
		return nil
	}

	resp, err := gm.client.TransferGroup(int(groupID), targetGroupID)
	if err != nil {
		return fmt.Errorf("failed to transfer group: %w", err)
	}

	if resp.StatusCode != 201 {
		gm.consoleUI.PrintCannotMoveGroup(fmt.Sprintf("%d", resp.StatusCode))
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	gm.consoleUI.PrintMoveResult(fmt.Sprintf("%d", resp.StatusCode))
	return nil
}
