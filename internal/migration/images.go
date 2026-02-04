package migration

import (
	"fmt"
	"strings"
	"time"

	"migraptor/internal/docker"
	"migraptor/internal/gitlab"
	"migraptor/internal/ui"

	gitlabCore "gitlab.com/gitlab-org/api/client-go"
)

// ImageInfo holds information about a Docker image
type ImageInfo struct {
	Name     string
	Path     string
	Location string
}

// ImageMigrator handles Docker image migration operations
type ImageMigrator struct {
	gitlabClient *gitlab.Client
	dockerClient *docker.Client
	dryRun       bool
	consoleUI    *ui.UI
}

// NewImageMigrator creates a new ImageMigrator
func NewImageMigrator(gitlabClient *gitlab.Client, dockerClient *docker.Client, dryRun bool, cUI *ui.UI) *ImageMigrator {
	return &ImageMigrator{
		gitlabClient: gitlabClient,
		dockerClient: dockerClient,
		dryRun:       dryRun,
		consoleUI:    cUI,
	}
}

// GetImages gets all images for a project's registry repository
func (im *ImageMigrator) GetImages(projectID, repositoryID int, tagFilter []string) ([]ImageInfo, error) {
	tags, _, err := im.gitlabClient.ListRegistryRepositoryTags(projectID, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to list repository tags: %w", err)
	}

	var images []ImageInfo
	for _, tag := range tags {
		// Apply tag filter if provided
		if len(tagFilter) > 0 {
			found := false
			for _, filter := range tagFilter {
				if tag.Name == filter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		images = append(images, ImageInfo{
			Name:     tag.Name,
			Path:     tag.Path,
			Location: tag.Location,
		})
	}

	return images, nil
}

// BackupImages backs up all images from a project's registry
func (im *ImageMigrator) BackupImages(project *ProjectInfo, tagFilter []string) ([]string, []*gitlabCore.RegistryRepository, error) {
	repositories, _, err := im.gitlabClient.ListRegistryRepositories(project.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list registry repositories: %w", err)
	}

	if len(repositories) == 0 {
		im.consoleUI.PrintNoRegistryFound()
		return nil, nil, nil
	}

	var allImages []string
	var allRepositoryIDs []int

	for _, repo := range repositories {
		im.consoleUI.Debug("Found registry with ID %d", repo.ID)
		im.consoleUI.Debug("Working on repository %d from project %d", repo.ID, project.ID)

		allRepositoryIDs = append(allRepositoryIDs, int(repo.ID))

		images, err := im.GetImages(project.ID, int(repo.ID), tagFilter)
		if err != nil {
			im.consoleUI.Error("Error occurred during image search on project %d - repository %d: %v", project.ID, repo.ID, err)
			continue
		}

		if len(images) == 0 {
			im.consoleUI.PrintNoImagesAfterFilter(fmt.Sprintf("%d", project.ID))
			continue
		}

		// Display images
		var imageList strings.Builder
		for _, img := range images {
			imageList.WriteString(fmt.Sprintf("%s\t%s\t%s\n", img.Name, img.Path, img.Location))
		}
		im.consoleUI.PrintImageList(fmt.Sprintf("%d", project.ID), fmt.Sprintf("%d", repo.ID), imageList.String())

		// Pull images
		im.consoleUI.PrintPullingImages()
		for _, img := range images {
			imageRef := img.Location
			if im.dryRun {
				im.consoleUI.Info("🌵DRY RUN: Would pull image %s", imageRef)
			} else {
				im.consoleUI.Info("🔌 Pulling image %s...", imageRef)
				if err := im.dockerClient.PullImage(imageRef); err != nil {
					im.consoleUI.Error("Failed to pull image %s: %v", imageRef, err)
					return nil, nil, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
				}
			}
			allImages = append(allImages, imageRef)
		}
	}
	return allImages, repositories, nil
}

func (im *ImageMigrator) DeleteRegistries(project *ProjectInfo, repositories []*gitlabCore.RegistryRepository) error {
	for _, repo := range repositories {
		if im.dryRun {
			im.consoleUI.Info("🌵 DRY RUN: Would delete registry repository %d", repo.ID)
		} else {
			_, err := im.gitlabClient.DeleteRegistryRepository(project.ID, int(repo.ID))
			if err != nil {
				im.consoleUI.Error("Failed to delete registry repository %d: %v", repo.ID, err)
			} else {
				im.consoleUI.Debug("Removed registry %d on project %d", repo.ID, project.ID)
			}

			// Wait a bit after deletion
			im.consoleUI.SleepWithLog(10 * time.Second)
		}
	}
	return nil
}

func (im *ImageMigrator) CheckIfRemainingImages(projects map[int]*ProjectInfo, tagFilter []string) error {
	// Wait for images to be deleted from registry if there is any temporization
	im.consoleUI.Info("🔄 Waiting for images to be deleted from registry...")
	// Wait until images are deleted from registry before proceeding
	maxRetries := 30
	retryDelay := 20 * time.Second
	for _, project := range projects {
		if project.ContainerRegistryEnabled {
			im.consoleUI.Info("⏳ Waiting for images to be deleted from project %s", project.Path)
			retries := 0
			if im.dryRun {
				im.consoleUI.Info("🌵DRY RUN: Would wait for images to be deleted from project %s", project.Path)
				break
			}

			// Wait until all images for this project's repositories are deleted or timeout
			for {
				repositories, _, _ := im.gitlabClient.ListRegistryRepositories(project.ID)
				if len(repositories) == 0 {
					im.consoleUI.Info("🚮 All registries deleted for project %s", project.Path)
					break
				}
				if retries >= maxRetries {
					im.consoleUI.Warning("⌛️Images for project %s were not deleted after waiting. Continuing...", project.Path)
					return fmt.Errorf("images for project %s were not deleted after waiting", project.Path)
				}
				im.consoleUI.Info("⏳Still images remaining for project %s (probably some delay configured on GitLab instance). Waiting...", project.Path)
				im.consoleUI.SleepWithLog(retryDelay)
				retries++
			}
		}
	}

	return nil
}

// RestoreImages restores images to the new registry location
func (im *ImageMigrator) RestoreImages(imageList []string, oldFullPath, newGroupPath string, keepParent bool) error {
	if len(imageList) == 0 {
		return nil
	}

	im.consoleUI.PrintTaggingAndPushing()

	for _, img := range imageList {
		img = strings.Trim(img, `"`)
		im.consoleUI.Debug("image is %s", img)

		// Build new image path
		var newImage string
		if keepParent {
			// Extract the group path from old full path
			oldPath := strings.Trim(oldFullPath, `"`)
			newImage = strings.Replace(img, oldPath, newGroupPath, 1)
		} else {
			// Simple replacement
			oldPath := strings.Trim(oldFullPath, `"`)
			newImage = strings.Replace(img, oldPath, newGroupPath, 1)
		}

		im.consoleUI.Debug("new_image is %s based on %s and %s", newImage, oldFullPath, newGroupPath)
		im.consoleUI.PrintTagAndPush(newImage)

		if im.dryRun {
			im.consoleUI.Info("🌵DRY RUN: Would tag %s as %s", img, newImage)
			im.consoleUI.Info("🌵DRY RUN: Would push %s", newImage)
		} else {
			// Tag the image
			if err := im.dockerClient.TagImage(img, newImage); err != nil {
				im.consoleUI.Error("Failed to tag image %s as %s: %v", img, newImage, err)
				continue
			}

			// Push the image
			im.consoleUI.Info("🔌 Pushing image %s...", newImage)
			if err := im.dockerClient.PushImage(newImage); err != nil {
				im.consoleUI.Error("Failed to push image %s: %v", newImage, err)
				continue
			}
		}
	}

	return nil
}

// GetAllImagesFromProjects collects all images from all projects and registries
func (im *ImageMigrator) GetAllImagesFromProjects(projects map[int]*ProjectInfo, tagFilter []string) ([]*ui.ImageItem, error) {
	var allImages []*ui.ImageItem

	for _, project := range projects {
		if !project.ContainerRegistryEnabled {
			continue
		}

		repositories, _, err := im.gitlabClient.ListRegistryRepositories(project.ID)
		if err != nil {
			im.consoleUI.Debug("Failed to list registry repositories for project %d: %v", project.ID, err)
			continue
		}

		if len(repositories) == 0 {
			continue
		}

		for _, repo := range repositories {
			images, err := im.GetImages(project.ID, int(repo.ID), tagFilter)
			if err != nil {
				im.consoleUI.Debug("Error occurred during image search on project %d - repository %d: %v", project.ID, repo.ID, err)
				continue
			}

			for _, img := range images {
				allImages = append(allImages, &ui.ImageItem{
					ImageInfo: ui.ImageInfo{
						Name:     img.Name,
						Path:     img.Path,
						Location: img.Location,
					},
					ProjectID:    project.ID,
					ProjectName:  project.Name,
					RegistryID:   int(repo.ID),
					RegistryPath: repo.Path,
					Selected:     false,
				})
			}
		}
	}

	return allImages, nil
}
