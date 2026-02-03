package gitlab

import (
	"fmt"
	"net/http"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// Client wraps the GitLab API client
type Client struct {
	client     *gitlab.Client
	baseURL    string
	maxRetries int
}

// NewClient creates a new GitLab client
func NewClient(token, instance string) (*Client, error) {
	baseURL := fmt.Sprintf("https://%s/api/v4", instance)

	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Note: Timeout is handled by the HTTP client internally
	_ = time.Second // Keep time import

	return &Client{
		client:     client,
		baseURL:    baseURL,
		maxRetries: 3,
	}, nil
}

// GetClient returns the underlying GitLab client
func (c *Client) GetClient() *gitlab.Client {
	return c.client
}

// SearchGroup searches for a group by name/path
func (c *Client) SearchGroup(name string) (*gitlab.Group, error) {
	opt := &gitlab.GetGroupOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 200,
		},
	}

	group, _, err := c.client.Groups.GetGroup(name, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to search groups: %w", err)
	}
	return group, nil
}

// GetGroup retrieves a group by ID
func (c *Client) GetGroup(groupID int) (*gitlab.Group, *gitlab.Response, error) {
	return c.client.Groups.GetGroup(int64(groupID), nil)
}

// CreateGroup creates a new group
func (c *Client) CreateGroup(name string, parentID *int) (*gitlab.Group, *gitlab.Response, error) {
	opt := &gitlab.CreateGroupOptions{
		Name: &name,
		Path: &name,
	}

	if parentID != nil {
		parentID64 := int64(*parentID)
		opt.ParentID = &parentID64
	}

	return c.client.Groups.CreateGroup(opt)
}

// TransferGroup transfers a group to another group
func (c *Client) TransferGroup(groupID, targetGroupID int) (*gitlab.Response, error) {
	// Use the HTTP client directly since TransferGroup might not be in the SDK
	// or might have a different signature
	req, err := c.client.NewRequest("POST", fmt.Sprintf("/groups/%d/transfer", groupID), map[string]interface{}{
		"group_id": targetGroupID,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transfer request: %w", err)
	}

	resp, err := c.client.Do(req, nil)
	if err != nil {
		return resp, fmt.Errorf("failed to transfer group: %w", err)
	}

	return resp, nil
}

func (c *Client) GetSubGroups(groupID int64) ([]*gitlab.Group, error) {
	subgroups, _, err := c.client.Groups.ListSubGroups(groupID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list subgroups: %w", err)
	}
	return subgroups, nil
}

// ListProjects lists projects in a group
func (c *Client) ListProjects(groupID int) ([]*gitlab.Project, *gitlab.Response, error) {
	opt := &gitlab.ListGroupProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	return c.client.Groups.ListGroupProjects(int64(groupID), opt)
}

// TransferProject transfers a project to another namespace
func (c *Client) TransferProject(projectID, namespaceID int) (*gitlab.Response, error) {
	namespaceID64 := int64(namespaceID)
	opt := &gitlab.TransferProjectOptions{
		Namespace: &namespaceID64,
	}

	_, resp, err := c.client.Projects.TransferProject(int64(projectID), opt)
	if err != nil {
		return resp, fmt.Errorf("failed to transfer project: %w", err)
	}

	return resp, nil
}

// ArchiveProject archives a project
func (c *Client) ArchiveProject(projectID int) (*gitlab.Response, error) {
	_, resp, err := c.client.Projects.ArchiveProject(int64(projectID))
	if err != nil {
		return resp, fmt.Errorf("failed to archive project: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp, nil
}

// UnarchiveProject unarchives a project
func (c *Client) UnarchiveProject(projectID int) (*gitlab.Response, error) {
	_, resp, err := c.client.Projects.UnarchiveProject(int64(projectID))
	if err != nil {
		return resp, fmt.Errorf("failed to unarchive project: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp, nil
}

// ListRegistryRepositories lists container registry repositories for a project
func (c *Client) ListRegistryRepositories(projectID int) ([]*gitlab.RegistryRepository, *gitlab.Response, error) {
	return c.client.ContainerRegistry.ListProjectRegistryRepositories(int64(projectID), nil)
}

// ListRegistryRepositoryTags lists tags for a registry repository
func (c *Client) ListRegistryRepositoryTags(projectID, repositoryID int) ([]*gitlab.RegistryRepositoryTag, *gitlab.Response, error) {
	return c.client.ContainerRegistry.ListRegistryRepositoryTags(int64(projectID), int64(repositoryID), nil)
}

// DeleteRegistryRepository deletes a registry repository
func (c *Client) DeleteRegistryRepository(projectID, repositoryID int) (*gitlab.Response, error) {
	resp, err := c.client.ContainerRegistry.DeleteRegistryRepository(int64(projectID), int64(repositoryID))
	if err != nil {
		return resp, fmt.Errorf("failed to delete registry repository: %w", err)
	}

	return resp, nil
}

// DeleteRegistryRepositoryTag deletes a specific tag from a registry repository
func (c *Client) DeleteRegistryRepositoryTag(projectID, repositoryID int, tagName string) (*gitlab.Response, error) {
	// Use the HTTP client directly since DeleteRegistryRepositoryTag might not be in the SDK
	req, err := c.client.NewRequest("DELETE", fmt.Sprintf("/projects/%d/registry/repositories/%d/tags/%s", projectID, repositoryID, tagName), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create delete tag request: %w", err)
	}

	resp, err := c.client.Do(req, nil)
	if err != nil {
		return resp, fmt.Errorf("failed to delete registry repository tag: %w", err)
	}

	return resp, nil
}

// GetCurrentUser gets the current authenticated user
func (c *Client) GetCurrentUser() (*gitlab.User, *gitlab.Response, error) {
	return c.client.Users.CurrentUser()
}

// CheckConnection verifies that the client can connect to GitLab
func (c *Client) CheckConnection() error {
	_, _, err := c.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to connect to GitLab: %w", err)
	}
	return nil
}
