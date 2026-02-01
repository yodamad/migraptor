package migration

import (
	"fmt"
	"gitlab-transfer-script/internal/gitlab"
	"gitlab-transfer-script/internal/ui"

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
