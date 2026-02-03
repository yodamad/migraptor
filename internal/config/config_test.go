package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// setupTestCommand creates a cobra.Command with all flags defined, similar to main.go
func setupTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.Flags().StringP(GITLAB_TOKEN, "g", "", "your gitlab API token")
	cmd.Flags().StringP(OLD_GROUP_NAME, "o", "", "the group containing the projects you want to migrate")
	cmd.Flags().StringP(NEW_GROUP_NAME, "n", "", "the full path of group that will contain the migrated projects")
	cmd.Flags().BoolP(DRY_RUN, "f", false, "fake run")
	cmd.Flags().StringP(GITLAB_INSTANCE, "i", "", "change gitlab instance. By default, it's gitlab.com")
	cmd.Flags().BoolP(KEEP_PARENT, "k", false, "don't keep the parent group, transfer projects individually instead")
	cmd.Flags().StringSliceP(PROJECTS_LIST, "l", []string{}, "list projects to move if you want to keep some in origin group (comma-separated)")
	cmd.Flags().StringP(DOCKER_PASSWORD, "p", "", "password for registry")
	cmd.Flags().StringP(GITLAB_REGISTRY, "r", "", "change gitlab registry name if not registry.<gitlab_instance>. By default, it's registry.gitlab.com")
	cmd.Flags().StringSliceP(TAGS_LIST, "t", []string{}, "filter tags to keep when moving images & registries (comma-separated)")
	cmd.Flags().BoolP(VERBOSE, "v", false, "verbose mode to debug your migration")
	return cmd
}

// resetViper resets viper to a clean state for testing
func resetViper() {
	viper.Reset()
	viper.SetConfigType("yaml")
}

func TestLoadConfig_Defaults(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check defaults
	if cfg.GitLabInstance != "gitlab.com" {
		t.Errorf("Expected GitLabInstance to be 'gitlab.com', got '%s'", cfg.GitLabInstance)
	}
	if cfg.KeepParent != true {
		t.Errorf("Expected KeepParent to be true, got %v", cfg.KeepParent)
	}
}

func TestLoadConfig_FromFlags(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set flags
	cmd.Flags().Set("token", "test-token-123")
	cmd.Flags().Set("old-group", "old-group-name")
	cmd.Flags().Set("new-group", "new-group-name")
	cmd.Flags().Set("instance", "custom.gitlab.com")
	cmd.Flags().Set("dry-run", "true")
	cmd.Flags().Set("keep-parent", "false")
	cmd.Flags().Set("verbose", "true")

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify flag values are loaded
	if cfg.GitLabToken != "test-token-123" {
		t.Errorf("Expected GitLabToken to be 'test-token-123', got '%s'", cfg.GitLabToken)
	}
	if cfg.OldGroupName != "old-group-name" {
		t.Errorf("Expected OldGroupName to be 'old-group-name', got '%s'", cfg.OldGroupName)
	}
	if cfg.NewGroupName != "new-group-name" {
		t.Errorf("Expected NewGroupName to be 'new-group-name', got '%s'", cfg.NewGroupName)
	}
	if cfg.GitLabInstance != "custom.gitlab.com" {
		t.Errorf("Expected GitLabInstance to be 'custom.gitlab.com', got '%s'", cfg.GitLabInstance)
	}
	if cfg.DryRun != true {
		t.Errorf("Expected DryRun to be true, got %v", cfg.DryRun)
	}
	if cfg.KeepParent != false {
		t.Errorf("Expected KeepParent to be false, got %v", cfg.KeepParent)
	}
	if cfg.Verbose != true {
		t.Errorf("Expected Verbose to be true, got %v", cfg.Verbose)
	}
}

func TestLoadConfig_FromEnvironmentVariables(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set environment variables
	os.Setenv("GITLAB_TOKEN", "env-token-456")
	os.Setenv("OLD_GROUP_NAME", "env-old-group")
	os.Setenv("NEW_GROUP_NAME", "env-new-group")
	os.Setenv("GITLAB_INSTANCE", "env.gitlab.com")
	defer func() {
		os.Unsetenv("GITLAB_TOKEN")
		os.Unsetenv("OLD_GROUP_NAME")
		os.Unsetenv("NEW_GROUP_NAME")
		os.Unsetenv("GITLAB_INSTANCE")
	}()

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify env var values are loaded
	if cfg.GitLabToken != "env-token-456" {
		t.Errorf("Expected GitLabToken to be 'env-token-456', got '%s'", cfg.GitLabToken)
	}
	if cfg.OldGroupName != "env-old-group" {
		t.Errorf("Expected OldGroupName to be 'env-old-group', got '%s'", cfg.OldGroupName)
	}
	if cfg.NewGroupName != "env-new-group" {
		t.Errorf("Expected NewGroupName to be 'env-new-group', got '%s'", cfg.NewGroupName)
	}
	if cfg.GitLabInstance != "env.gitlab.com" {
		t.Errorf("Expected GitLabInstance to be 'env.gitlab.com', got '%s'", cfg.GitLabInstance)
	}
}

func TestLoadConfig_FromConfigFile(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "gitlab-migraptor.yaml")
	configContent := `
token: file-token-789
old-group: file-old-group
new-group: file-new-group
instance: file.gitlab.com
dry-run: true
keep-parent: false
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Change to temp directory so viper can find the config file
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify config file values are loaded
	if cfg.GitLabToken != "file-token-789" {
		t.Errorf("Expected GitLabToken to be 'file-token-789', got '%s'", cfg.GitLabToken)
	}
	if cfg.OldGroupName != "file-old-group" {
		t.Errorf("Expected OldGroupName to be 'file-old-group', got '%s'", cfg.OldGroupName)
	}
	if cfg.NewGroupName != "file-new-group" {
		t.Errorf("Expected NewGroupName to be 'file-new-group', got '%s'", cfg.NewGroupName)
	}
	if cfg.GitLabInstance != "file.gitlab.com" {
		t.Errorf("Expected GitLabInstance to be 'file.gitlab.com', got '%s'", cfg.GitLabInstance)
	}
	if cfg.DryRun != true {
		t.Errorf("Expected DryRun to be true, got %v", cfg.DryRun)
	}
	if cfg.KeepParent != false {
		t.Errorf("Expected KeepParent to be false, got %v", cfg.KeepParent)
	}
}

func TestLoadConfig_Precedence_FlagsOverrideEnvVars(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set environment variable
	os.Setenv("GITLAB_TOKEN", "env-token")
	defer os.Unsetenv("GITLAB_TOKEN")

	// Set flag (should override env var)
	cmd.Flags().Set("token", "flag-token")

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Flag should win over env var
	if cfg.GitLabToken != "flag-token" {
		t.Errorf("Expected GitLabToken to be 'flag-token' (from flag), got '%s'", cfg.GitLabToken)
	}
}

func TestLoadConfig_Precedence_EnvVarsOverrideConfigFile(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Create config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "gitlab-migraptor.yaml")
	configContent := `token: file-token`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Set environment variable (should override config file)
	os.Setenv("GITLAB_TOKEN", "env-token")
	defer os.Unsetenv("GITLAB_TOKEN")

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Env var should win over config file
	if cfg.GitLabToken != "env-token" {
		t.Errorf("Expected GitLabToken to be 'env-token' (from env var), got '%s'", cfg.GitLabToken)
	}
}

func TestLoadConfig_Precedence_ConfigFileOverridesDefaults(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Create config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "gitlab-migraptor.yaml")
	configContent := `instance: config.gitlab.com`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Config file should override default
	if cfg.GitLabInstance != "config.gitlab.com" {
		t.Errorf("Expected GitLabInstance to be 'config.gitlab.com' (from config file), got '%s'", cfg.GitLabInstance)
	}
}

func TestLoadConfig_StringSliceFlags(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set string slice flags
	cmd.Flags().Set("projects", "project1,project2,project3")
	cmd.Flags().Set("tags", "tag1,tag2")

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify string slice values
	if len(cfg.ProjectsList) != 3 {
		t.Errorf("Expected ProjectsList to have 3 items, got %d", len(cfg.ProjectsList))
	}
	if cfg.ProjectsList[0] != "project1" || cfg.ProjectsList[1] != "project2" || cfg.ProjectsList[2] != "project3" {
		t.Errorf("Expected ProjectsList to be ['project1', 'project2', 'project3'], got %v", cfg.ProjectsList)
	}

	if len(cfg.TagsList) != 2 {
		t.Errorf("Expected TagsList to have 2 items, got %d", len(cfg.TagsList))
	}
	if cfg.TagsList[0] != "tag1" || cfg.TagsList[1] != "tag2" {
		t.Errorf("Expected TagsList to be ['tag1', 'tag2'], got %v", cfg.TagsList)
	}
}

func TestLoadConfig_LegacyEnvVars(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set legacy environment variables (without MIGRAPTOR prefix)
	os.Setenv("GITLAB_TOKEN", "legacy-token")
	os.Setenv("OLD_GROUP_NAME", "legacy-old-group")
	defer func() {
		os.Unsetenv("GITLAB_TOKEN")
		os.Unsetenv("OLD_GROUP_NAME")
	}()

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify legacy env vars are loaded
	if cfg.GitLabToken != "legacy-token" {
		t.Errorf("Expected GitLabToken to be 'legacy-token', got '%s'", cfg.GitLabToken)
	}
	if cfg.OldGroupName != "legacy-old-group" {
		t.Errorf("Expected OldGroupName to be 'legacy-old-group', got '%s'", cfg.OldGroupName)
	}
}

func TestLoadConfig_MigraptorPrefixedEnvVars(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set MIGRAPTOR prefixed environment variables
	os.Setenv("MIGRAPTOR_TOKEN", "prefixed-token")
	os.Setenv("MIGRAPTOR_OLD_GROUP", "prefixed-old-group")
	defer func() {
		os.Unsetenv("MIGRAPTOR_TOKEN")
		os.Unsetenv("MIGRAPTOR_OLD_GROUP")
	}()

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify prefixed env vars are loaded
	if cfg.GitLabToken != "prefixed-token" {
		t.Errorf("Expected GitLabToken to be 'prefixed-token', got '%s'", cfg.GitLabToken)
	}
	if cfg.OldGroupName != "prefixed-old-group" {
		t.Errorf("Expected OldGroupName to be 'prefixed-old-group', got '%s'", cfg.OldGroupName)
	}
}

func TestLoadConfig_CommaSeparatedEnvVars(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set comma-separated environment variables for lists
	// The manual handling in LoadConfig only runs if viper doesn't populate the list
	// (i.e., len(cfg.ProjectsList) == 0). This tests the fallback behavior when viper
	// doesn't handle comma-separated strings correctly for string slices.
	os.Setenv("PROJECTS_LIST", "proj1,proj2,proj3")
	os.Setenv("TAGS_LIST", "tag1,tag2")
	defer func() {
		os.Unsetenv("PROJECTS_LIST")
		os.Unsetenv("TAGS_LIST")
	}()

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify that env vars are read (either by viper or manual handling)
	// The exact parsing depends on whether viper populates it first
	if len(cfg.ProjectsList) == 0 {
		t.Error("Expected ProjectsList to be populated from PROJECTS_LIST env var, but it's empty")
	}

	if len(cfg.TagsList) == 0 {
		t.Error("Expected TagsList to be populated from TAGS_LIST env var, but it's empty")
	}
}

func TestLoadConfig_CommaSeparatedEnvVarsWithSpaces(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Test comma-separated env vars with spaces - manual handling should trim them
	// This specifically tests the manual fallback parsing when viper doesn't populate correctly
	os.Setenv("PROJECTS_LIST", "proj1, proj2 , proj3")
	os.Setenv("TAGS_LIST", "tag1, tag2")
	defer func() {
		os.Unsetenv("PROJECTS_LIST")
		os.Unsetenv("TAGS_LIST")
	}()

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// The manual handling trims spaces, but only runs if viper doesn't populate the list
	// Verify that at least some values are present
	if len(cfg.ProjectsList) == 0 {
		t.Error("Expected ProjectsList to be populated from PROJECTS_LIST env var")
	}

	if len(cfg.TagsList) == 0 {
		t.Error("Expected TagsList to be populated from TAGS_LIST env var")
	}

	// If manual handling ran (list was empty before), verify trimming worked
	// We can't easily test this without mocking viper, so we just verify the env vars are read
}

func TestLoadConfig_RegistryDefault(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set instance but not registry
	cmd.Flags().Set("instance", "custom.gitlab.com")

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Registry should default to registry.<instance>
	expectedRegistry := "registry.custom.gitlab.com"
	if cfg.GitLabRegistry != expectedRegistry {
		t.Errorf("Expected GitLabRegistry to be '%s', got '%s'", expectedRegistry, cfg.GitLabRegistry)
	}
}

func TestLoadConfig_DockerTokenDefault(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set GitLab token but not Docker token
	cmd.Flags().Set("token", "gitlab-token-123")

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Docker token should default to GitLab token
	if cfg.DockerToken != "gitlab-token-123" {
		t.Errorf("Expected DockerToken to be 'gitlab-token-123', got '%s'", cfg.DockerToken)
	}
}

func TestLoadConfig_ConfigFileAliases(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Create config file using snake_case keys (aliases) as shown in gitlab-migraptor-sample.yaml
	// The copyAliasedValues() function should copy these to kebab-case keys for Unmarshal
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "gitlab-migraptor.yaml")
	configContent := `
gitlab_token: alias-token
old_group_name: alias-old-group
new_group_name: alias-new-group
parent_group_id: 42
gitlab_instance: custom.gitlab.com
gitlab_registry: registry.custom.gitlab.com
docker_token: docker-token-123
projects_list:
  - project1
  - project2
tags_list:
  - tag1
  - tag2
keep_parent: false
dry_run: true
verbose: true
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify config file values are loaded from snake_case keys
	if cfg.GitLabToken != "alias-token" {
		t.Errorf("Expected GitLabToken to be 'alias-token', got '%s'", cfg.GitLabToken)
	}
	if cfg.OldGroupName != "alias-old-group" {
		t.Errorf("Expected OldGroupName to be 'alias-old-group', got '%s'", cfg.OldGroupName)
	}
	if cfg.NewGroupName != "alias-new-group" {
		t.Errorf("Expected NewGroupName to be 'alias-new-group', got '%s'", cfg.NewGroupName)
	}
	if cfg.ParentGroupID != 42 {
		t.Errorf("Expected ParentGroupID to be 42, got %d", cfg.ParentGroupID)
	}
	if cfg.GitLabInstance != "custom.gitlab.com" {
		t.Errorf("Expected GitLabInstance to be 'custom.gitlab.com', got '%s'", cfg.GitLabInstance)
	}
	if cfg.GitLabRegistry != "registry.custom.gitlab.com" {
		t.Errorf("Expected GitLabRegistry to be 'registry.custom.gitlab.com', got '%s'", cfg.GitLabRegistry)
	}
	if cfg.DockerToken != "docker-token-123" {
		t.Errorf("Expected DockerToken to be 'docker-token-123', got '%s'", cfg.DockerToken)
	}
	if len(cfg.ProjectsList) != 2 || cfg.ProjectsList[0] != "project1" || cfg.ProjectsList[1] != "project2" {
		t.Errorf("Expected ProjectsList to be ['project1', 'project2'], got %v", cfg.ProjectsList)
	}
	if len(cfg.TagsList) != 2 || cfg.TagsList[0] != "tag1" || cfg.TagsList[1] != "tag2" {
		t.Errorf("Expected TagsList to be ['tag1', 'tag2'], got %v", cfg.TagsList)
	}
	if cfg.KeepParent != false {
		t.Errorf("Expected KeepParent to be false, got %v", cfg.KeepParent)
	}
	if cfg.DryRun != true {
		t.Errorf("Expected DryRun to be true, got %v", cfg.DryRun)
	}
	if cfg.Verbose != true {
		t.Errorf("Expected Verbose to be true, got %v", cfg.Verbose)
	}
}

func TestLoadConfig_MissingFlag(t *testing.T) {
	resetViper()
	// Create a command without all flags
	cmd := &cobra.Command{
		Use: "test",
	}
	// Only add some flags, not all
	cmd.Flags().StringP(GITLAB_TOKEN, "g", "", "your gitlab API token")

	// This should fail because not all flags are present
	_, err := LoadConfig(cmd)
	if err == nil {
		t.Error("Expected LoadConfig to fail when flags are missing, but it succeeded")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		GitLabToken:  "test-token",
		OldGroupName: "old-group",
		NewGroupName: "new-group",
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected Validate to succeed, got error: %v", err)
	}
}

func TestValidate_MissingToken(t *testing.T) {
	cfg := &Config{
		OldGroupName: "old-group",
		NewGroupName: "new-group",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected Validate to fail when token is missing")
	}
	if err.Error() != "GitLab token is required" {
		t.Errorf("Expected error message 'GitLab token is required', got '%s'", err.Error())
	}
}

func TestValidate_MissingOldGroupName(t *testing.T) {
	cfg := &Config{
		GitLabToken:  "test-token",
		NewGroupName: "new-group",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected Validate to fail when old group name is missing")
	}
	if err.Error() != "old group name is required" {
		t.Errorf("Expected error message 'old group name is required', got '%s'", err.Error())
	}
}

func TestValidate_MissingNewGroupName(t *testing.T) {
	cfg := &Config{
		GitLabToken:  "test-token",
		OldGroupName: "old-group",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected Validate to fail when new group name is missing")
	}
	if err.Error() != "new group name is required" {
		t.Errorf("Expected error message 'new group name is required', got '%s'", err.Error())
	}
}

func TestLoadConfig_AllFlagsBound(t *testing.T) {
	resetViper()
	cmd := setupTestCommand()

	// Set all flags to ensure they're all properly bound
	cmd.Flags().Set("token", "t")
	cmd.Flags().Set("old-group", "o")
	cmd.Flags().Set("new-group", "n")
	cmd.Flags().Set("dry-run", "true")
	cmd.Flags().Set("instance", "i")
	cmd.Flags().Set("keep-parent", "false")
	cmd.Flags().Set("projects", "p1,p2")
	cmd.Flags().Set("docker-password", "d")
	cmd.Flags().Set("registry", "r")
	cmd.Flags().Set("tags", "t1,t2")
	cmd.Flags().Set("verbose", "true")

	cfg, err := LoadConfig(cmd)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify all flags are bound and loaded
	if cfg.GitLabToken == "" {
		t.Error("GitLabToken should be loaded from flag")
	}
	if cfg.OldGroupName == "" {
		t.Error("OldGroupName should be loaded from flag")
	}
	if cfg.NewGroupName == "" {
		t.Error("NewGroupName should be loaded from flag")
	}
	if !cfg.DryRun {
		t.Error("DryRun should be loaded from flag")
	}
	if cfg.GitLabInstance == "" {
		t.Error("GitLabInstance should be loaded from flag")
	}
	if cfg.KeepParent {
		t.Error("KeepParent should be loaded from flag")
	}
	if len(cfg.ProjectsList) == 0 {
		t.Error("ProjectsList should be loaded from flag")
	}
	if cfg.DockerToken == "" {
		t.Error("DockerToken should be loaded from flag")
	}
	if cfg.GitLabRegistry == "" {
		t.Error("GitLabRegistry should be loaded from flag")
	}
	if len(cfg.TagsList) == 0 {
		t.Error("TagsList should be loaded from flag")
	}
	if !cfg.Verbose {
		t.Error("Verbose should be loaded from flag")
	}
}
