package ui

import (
	"fmt"
	"log"
	"migraptor/internal/config"
	"os"
	"time"

	"github.com/fatih/color"
)

var (
	// Color definitions matching bash script
	red         = color.New(color.FgRed)
	green       = color.New(color.FgGreen)
	yellow      = color.New(color.FgYellow)
	blue        = color.New(color.FgBlue)
	magenta     = color.New(color.FgMagenta)
	cyan        = color.New(color.FgCyan)
	gray        = color.New(color.FgHiBlack)
	white       = color.New(color.FgWhite)
	lightGreen  = color.New(color.FgHiGreen)
	lightYellow = color.New(color.FgHiYellow)
	lightBlue   = color.New(color.FgHiBlue)

	logFile *os.File
	logger  *log.Logger
	verbose bool
)

type UI struct {
	verbose bool
	logFile *os.File
	logger  *log.Logger
}

// Init initializes the UI system with logging
func Init(verboseMode bool) (*UI, error) {

	verbose = verboseMode

	// Open or create log file with fallback strategies
	var err error
	logPaths := []string{
		"migrate.log",                     // Try current directory first
		os.ExpandEnv("$HOME/migrate.log"), // Try home directory
		"/tmp/migrate.log",                // Try temp directory
	}

	for _, logPath := range logPaths {
		logFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			logger = log.New(logFile, "", log.LstdFlags)
			break
		}
	}

	// If all paths fail, use stderr as fallback
	if logFile == nil {
		yellow.Printf("⚠️  Warning: Could not open log file, using stderr instead\n")
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	ui := &UI{
		verbose: verboseMode,
		logFile: logFile,
		logger:  logger,
	}

	// Print welcome message
	lightGreen.Printf("")
	lightGreen.Printf("========================================\n")
	lightGreen.Printf("👋 Welcome MigRaptor 🦖\n")
	lightGreen.Printf("========================================\n")

	return ui, nil
}

// Close closes the log file
func Close() error {
	if logFile != nil {
		return logFile.Close()
	}
	return nil
}

// Question prints a question message
func (ui *UI) Question(format string, args ...interface{}) {
	blue.Printf("❓ "+format, args...)
	ui.logger.Printf("[QUESTION] "+format, args...)
}

// Debug prints debug messages if verbose mode is enabled
func (ui *UI) Debug(format string, args ...interface{}) {
	if ui.verbose {
		lightYellow.Printf(format+"\n", args...)
		ui.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Info prints informational messages
func (ui *UI) Info(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
	logger.Printf("[INFO] "+format, args...)
}

// Success prints success messages
func (ui *UI) Success(format string, args ...interface{}) {
	green.Printf("✅ "+format+"\n", args...)
	logger.Printf("[SUCCESS] "+format, args...)
}

// Warning prints warning messages
func (ui *UI) Warning(format string, args ...interface{}) {
	yellow.Printf("⚠️ "+format+"\n", args...)
	logger.Printf("[WARNING] "+format, args...)
}

// Error prints error messages
func (ui *UI) Error(format string, args ...interface{}) {
	red.Printf("❌ "+format+"\n", args...)
	logger.Printf("[ERROR] "+format, args...)
}

// PrintHeader prints a formatted header
func (ui *UI) PrintHeader(text string) {
	cyan.Printf("==========================\n")
	cyan.Printf(" %s\n", text)
	cyan.Printf("==========================\n")
	gray.Printf("")
}

// PrintSection prints a section separator
func (ui *UI) PrintSection(text string) {
	cyan.Printf("==========================\n")
	cyan.Printf(" %s\n", text)
	cyan.Printf("==========================\n")
	gray.Printf("")
}

// PrintProjectHeader prints a project-specific header
func (ui *UI) PrintProjectHeader(projectName, action string) {
	cyan.Printf("==========================\n")
	lightBlue.Printf(" %s %s project\n", action, projectName)
	cyan.Printf("==========================\n")
	gray.Printf("")
}

// PrintMigrationStart prints the migration start message
func (ui *UI) PrintMigrationStart(config *config.Config) {

	// Print migration summary
	cyan.Printf("----------------------------------------\n")
	cyan.Printf(" 🦊 GitLab Migration Tool Summary\n")
	cyan.Printf("----------------------------------------\n")
	cyan.Printf(" 🛫 From group:   ")
	lightBlue.Printf("%s\n", config.OldGroupName)
	cyan.Printf(" 🛬 To group:     ")
	lightBlue.Printf("%s\n", config.NewGroupName)
	cyan.Printf(" 🦊 GitLab URL:   ")
	lightBlue.Printf("%s\n", config.GitLabInstance)
	cyan.Printf(" 🐳 Registry URL: ")
	lightBlue.Printf("%s\n", config.GitLabRegistry)
	if len(config.ProjectsList) > 0 {
		cyan.Printf(" 📋 Project filtered list: ")
		lightBlue.Printf("%s\n", config.ProjectsList)
	}
	if len(config.TagsList) > 0 {
		cyan.Printf(" 🔖 Image tag filters:")
		lightBlue.Printf("%s\n", config.TagsList)
	}
	if config.Verbose {
		lightYellow.Printf(" 🔬 DEBUG on\n")
	}
	if config.DryRun {
		lightYellow.Printf(" 🌵 DRY RUN\n")
	}
	cyan.Printf("========================================\n")

	// Add confirmation message be starting
	fmt.Printf("❓Everything is ok ? (y/n) ")
	var response string
	fmt.Scanln(&response)
	if response != "y" && response != "Y" {
		red.Printf("Migration cancelled by user.\n")
		os.Exit(1)
	}

	cyan.Printf("🛫 Starting migration...\n")
}

// PrintMigrationComplete prints the migration completion message
func (ui *UI) PrintMigrationComplete(projectName string) {
	cyan.Printf("=============================\n")
	cyan.Printf(" 🛬 Migration of ")
	lightBlue.Printf("%s", projectName)
	green.Printf(" is ok 🎉\n")
	cyan.Printf("=============================\n")
	gray.Printf("")
}

// PrintDryRunSuccess prints dry run success message
func (ui *UI) PrintDryRunSuccess() {
	green.Printf("==========================\n")
	green.Printf("= 🌵Dry run succeeded 🎉 =\n")
	green.Printf("==========================\n")
}

// PrintGroupFound prints group found message
func (ui *UI) PrintGroupFound(groupName string, groupID int) {
	green.Printf("🙈 Group ")
	lightBlue.Printf("%s", groupName)
	green.Printf(" already exist with id ")
	lightBlue.Printf("%d\n", groupID)
}

// PrintGroupCreated prints group created message
func (ui *UI) PrintGroupCreated(groupName string, groupID interface{}) {
	green.Printf("✅ Group ")
	lightBlue.Printf("%s", groupName)
	green.Printf(" create with id ")
	lightBlue.Printf("%v\n", groupID)
}

// PrintGroupNotFound prints group not found error
func (ui *UI) PrintGroupNotFound(groupName string) {
	red.Printf("😢 Group ")
	lightBlue.Printf("%s", groupName)
	red.Printf(" not found\n")
}

// PrintNoProjectsFound prints no projects found message
func (ui *UI) PrintNoProjectsFound() {
	magenta.Printf("😢 No project found in group\n")
}

// PrintProjectCreated prints project created message
func (ui *UI) PrintProjectCreated(projectName string, projectID interface{}, groupID interface{}) {
	green.Printf("✅ Project ")
	lightBlue.Printf("%s", projectName)
	green.Printf(" create with id ")
	lightBlue.Printf("%v", projectID)
	green.Printf(" in group ")
	lightBlue.Printf("%v\n", groupID)
}

// PrintImageList prints image list
func (ui *UI) PrintImageList(projectID, registryID string, images string) {
	green.Printf("🎞 Project ")
	lightBlue.Printf("%s", projectID)
	green.Printf(" images in registry ")
	lightBlue.Printf("%s", registryID)
	green.Printf(" are :\n")
	white.Printf("%s\n", images)
}

// PrintNoImagesAfterFilter prints no images after filtering
func (ui *UI) PrintNoImagesAfterFilter(projectID string) {
	yellow.Printf("🎞 Project ")
	lightBlue.Printf("%s", projectID)
	yellow.Printf(" has no images in registry after filtering\n")
}

// PrintPullingImages prints pulling images message
func (ui *UI) PrintPullingImages() {
	cyan.Printf("📩 Pulling existing images...\n")
}

// PrintTaggingAndPushing prints tagging and pushing message
func (ui *UI) PrintTaggingAndPushing() {
	cyan.Printf("✍️ Tagging & pushing existing images to new registry...\n")
}

// PrintTagAndPush prints tag and push message for a specific image
func (ui *UI) PrintTagAndPush(newImage string) {
	fmt.Printf("✍️ ")
	cyan.Printf("Tag & push ")
	white.Printf("%s\n", newImage)
}

// PrintDockerNotStarted prints Docker not started error
func (ui *UI) PrintDockerNotStarted() {
	red.Printf("⛔️ Docker not started\n")
	red.Printf("🐳 You must first start docker daemon.\n")
	lightBlue.Printf("Use : sudo service docker start\n")
}

// PrintDockerLoginSuccess prints Docker login success
func (ui *UI) PrintDockerLoginSuccess() {
	lightBlue.Printf("🐳 Consider Docker login as done !\n")
}

// PrintDockerLoginFailed prints Docker login failure
func (ui *UI) PrintDockerLoginFailed() {
	red.Printf("😭 Sorry I don't succeed...\n")
	cyan.Printf("Please log in to registry and re-run script\n")
}

// PrintNotLoggedToRegistry prints not logged to registry error
func (ui *UI) PrintNotLoggedToRegistry(registry string) {
	red.Printf("🚫 Not logged to gitlab registry\n")
	red.Printf("You must first log in to target gitlab registry: %s.\n", registry)
	lightBlue.Printf("🐳 Use : docker login %s -u USERNAME -p DOCKER_TOKEN\n", registry)
}

// PrintTransferringGroup prints transferring group message
func (ui *UI) PrintTransferringGroup(oldGroup, newGroup string) {
	cyan.Printf("⏩ Transfering ")
	lightBlue.Printf("%s", oldGroup)
	cyan.Printf(" to ")
	lightBlue.Printf("%s", newGroup)
	fmt.Println()
}

// PrintTransferringProject prints transferring project message
func (ui *UI) PrintTransferringProject(projectName string, groupID interface{}) {
	cyan.Printf("⏩ Transfering ")
	lightBlue.Printf("%s", projectName)
	cyan.Printf(" to ")
	lightBlue.Printf("%v", groupID)
	fmt.Println()
}

// PrintMoveResult prints move result
func (ui *UI) PrintMoveResult(result string) {
	if result == "201" {
		fmt.Printf("⏩ Project transfer done\n")
	} else {
		fmt.Printf("😱 Project transfer failed with error %s\n", result)
	}

}

// PrintCannotMoveGroup prints cannot move group error
func (ui *UI) PrintCannotMoveGroup(errorCode string) {
	red.Printf("Cannot move group, probably not empty... (error : %s)\n", errorCode)
}

// PrintArchivedMessage prints archived project message
func (ui *UI) PrintArchivedMessage(projectName string) {
	yellow.Printf("%s was archived, re-archive it\n", projectName)
}

// PrintUnarchivedMessage prints unarchived project message
func (ui *UI) PrintUnarchivedMessage(projectName string) {
	yellow.Printf("%s is archived, unarchive it temporarly\n", projectName)
}

// PrintRemovingRegistry prints removing registry message
func (ui *UI) PrintRemovingRegistry() {
	cyan.Printf("🚮 Removing registry...\n")
}

// PrintNoRegistryFound prints no registry found message
func (ui *UI) PrintNoRegistryFound() {
	yellow.Printf("🤷 No registry found... continue...\n")
}

// PrintInvalidOption prints invalid option error
func (ui *UI) PrintInvalidOption(option string) {
	red.Printf("⛔️ Invalid option %s\n", option)
}

// PrintOptionNeedsArgument prints option needs argument error
func (ui *UI) PrintOptionNeedsArgument(option string) {
	red.Printf("⛔️ Option %s needs a valid argument\n", option)
}

// LogToFile logs a message to the log file
func (ui *UI) LogToFile(format string, args ...interface{}) {
	if logger != nil {
		logger.Printf(format, args...)
	}
}

// SleepWithLog sleeps for the specified duration and logs it in debug mode
func (ui *UI) SleepWithLog(duration time.Duration) {
	if verbose {
		ui.Debug("Sleeping for %v", duration)
	}
	time.Sleep(duration)
}

func PrintUsage() string {
	fmt.Println("Usage : ./migrate -g <GITLAB_TOKEN> -o <OLD_GROUP_NAME> -n <NEW_GROUP_NAME>")
	fmt.Println("=============================================================================")
	fmt.Println("Mandatory options")
	fmt.Println("-----------------")
	fmt.Println("-g : your gitlab API token")
	fmt.Println("-n : the full path of group that will contain the migrated projects")
	fmt.Println("-o : the group containing the projects you want to migrate")
	fmt.Println("-s : the simple path of group containing the projects you want to migrate, in same parent group then original one")
	fmt.Println("-----------------")
	fmt.Println("Other options")
	fmt.Println("-------------")
	fmt.Println("-d : parent group id (if there are multiple with same name on the instance)")
	fmt.Println("-f : fake run")
	fmt.Println("-h : display usage")
	fmt.Println("-i : change gitlab instance. By default, it's gitlab.com")
	fmt.Println("-k : keep the group containing the project, it will be moved into group specified with -n")
	fmt.Println("-l : list projects to move if you want to keep some in origin group")
	fmt.Println("-p : password for registry")
	fmt.Println("-r : change gitlab registry name if not registry.<gitlab_instance>. By default, it's registry.gitlab.com")
	fmt.Println("-t : filter tags to keep when moving images & registries")
	fmt.Println("-v : verbose mode to debug your migration")
	return ""
}
