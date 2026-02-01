package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	dockerclient "github.com/docker/docker/client"
)

// Client wraps the Docker API client
type Client struct {
	cli      *dockerclient.Client
	ctx      context.Context
	registry string
	username string
	password string
	loggedIn bool
	authInfo string
}

// NewClient creates a new Docker client
func NewClient() (*Client, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{
		cli:      cli,
		ctx:      context.Background(),
		loggedIn: false,
	}, nil
}

// Close closes the Docker client
func (c *Client) Close() error {
	return c.cli.Close()
}

// SetAuthInfo sets the authentication information
func (c *Client) SetAuthInfo(authInfo string) {
	c.authInfo = authInfo
}

// CheckDockerRunning checks if Docker daemon is running
func (c *Client) CheckDockerRunning() error {
	_, err := c.cli.Ping(c.ctx)
	if err != nil {
		return fmt.Errorf("docker daemon is not running: %w", err)
	}
	return nil
}

// CheckRegistryLogin checks if already logged in to the registry
func (c *Client) CheckRegistryLogin(registry string) bool {
	configPath := filepath.Join(os.Getenv("HOME"), ".docker", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		return false
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}

	return strings.Contains(string(configData), registry)
}

// Login logs in to the GitLab registry
func (c *Client) Login(registryUrl, username, password string) (string, error) {
	c.registry = registryUrl
	c.username = username
	c.password = password

	// 1. Define your credentials
	authConfig := registry.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: registryUrl,
	}

	// 2. Perform the Login check
	client, err := NewClient()
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	_, err = client.cli.RegistryLogin(client.ctx, authConfig)
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}

	// Attempt a login by trying to obtain a registry token
	_, err = client.cli.RegistryLogin(client.ctx, authConfig)
	if err != nil {
		return "", fmt.Errorf("failed to login to registry %s: %w", registryUrl, err)
	}
	// Set as logged in if successful
	// 3. Prepare the encoded string for Pull/Push operations
	// Most SDK methods expect a base64-encoded JSON string of the AuthConfig
	encodedAuth, err := encodeAuthToBase64(authConfig)
	if err != nil {
		return "", fmt.Errorf("failed to encode auth to base64: %w", err)
	}
	return encodedAuth, nil
}

func encodeAuthToBase64(auth registry.AuthConfig) (string, error) {
	buf, err := json.Marshal(auth)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// PullImage pulls an image from the registry
func (c *Client) PullImage(imageRef string) error {

	// Options include the Base64 encoded auth string
	options := image.PullOptions{
		RegistryAuth: c.authInfo,
	}

	reader, err := c.cli.ImagePull(c.ctx, imageRef, options)
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}
	defer reader.Close()

	// Consume the output to show progress (can be made verbose)
	_, _ = io.Copy(io.Discard, reader)

	return nil
}

// TagImage tags an image with a new name
func (c *Client) TagImage(sourceImage, targetImage string) error {
	err := c.cli.ImageTag(c.ctx, sourceImage, targetImage)
	if err != nil {
		return fmt.Errorf("failed to tag image %s as %s: %w", sourceImage, targetImage, err)
	}
	return nil
}

// PushImage pushes an image to the registry
func (c *Client) PushImage(imageRef string) error {
	options := image.PushOptions{
		RegistryAuth: c.authInfo,
	}
	reader, err := c.cli.ImagePush(c.ctx, imageRef, options)
	if err != nil {
		return fmt.Errorf("failed to push image %s: %w", imageRef, err)
	}
	defer reader.Close()

	// Consume the output to show progress (can be made verbose)
	_, _ = io.Copy(io.Discard, reader)

	return nil
}

// ImageExists checks if an image exists locally
func (c *Client) ImageExists(imageRef string) (bool, error) {
	_, err := c.cli.ImageInspect(c.ctx, imageRef)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListImages lists all local images
func (c *Client) ListImages() ([]image.Summary, error) {
	images, err := c.cli.ImageList(c.ctx, image.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}
	return images, nil
}

// RemoveImage removes a local image
func (c *Client) RemoveImage(imageRef string) error {
	_, err := c.cli.ImageRemove(c.ctx, imageRef, image.RemoveOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove image %s: %w", imageRef, err)
	}
	return nil
}
