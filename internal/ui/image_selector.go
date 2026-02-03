package ui

import (
	"fmt"
	"migraptor/internal/gitlab"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ImageItem holds image information with project/registry context for selection UI
type ImageItem struct {
	ImageInfo    ImageInfo
	ProjectID    int
	ProjectName  string
	RegistryID   int
	RegistryPath string
	Selected     bool
}

// ImageInfo holds information about a Docker image
type ImageInfo struct {
	Name     string
	Path     string
	Location string
}

var (
	// Styles
	titleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	selectedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	unselectedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	projectStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	registryStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	imageStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cursorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	statusBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Background(lipgloss.Color("236"))
	helpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
	confirmStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	checkboxStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	checkboxEmptyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// TreeNode represents a node in the hierarchical tree
type TreeNode struct {
	Type         string // "project", "registry", "image"
	ProjectID    int
	ProjectName  string
	RegistryID   int
	RegistryPath string
	Image        *ImageItem
	Expanded     bool
	Selected     bool
	Children     []*TreeNode
	Parent       *TreeNode
}

// Model represents the bubbletea model for image selection
type ImageSelectorModel struct {
	images          []ImageItem
	tree            []*TreeNode
	cursor          int
	gitlabClient    *gitlab.Client
	dryRun          bool
	showConfirm     bool
	confirmMsg      string
	showQuitConfirm bool
	quitConfirmMsg  string
	deleting        bool
	deletedCount    int
	failedCount     int
	width           int
	height          int
	finalSelected   []ImageItem // Store selected images when quitting
}

// NewImageSelectorModel creates a new image selector model
func NewImageSelectorModel(images []ImageItem, gitlabClient *gitlab.Client, dryRun bool) *ImageSelectorModel {
	model := &ImageSelectorModel{
		images:          images,
		gitlabClient:    gitlabClient,
		dryRun:          dryRun,
		showConfirm:     false,
		showQuitConfirm: false,
		deleting:        false,
		deletedCount:    0,
		failedCount:     0,
	}

	model.buildTree()
	return model
}

// buildTree builds the hierarchical tree structure from flat image list
func (m *ImageSelectorModel) buildTree() {
	projectMap := make(map[int]*TreeNode)
	registryMap := make(map[string]*TreeNode)

	for _, img := range m.images {
		// Get or create project node
		projectNode, exists := projectMap[img.ProjectID]
		if !exists {
			projectNode = &TreeNode{
				Type:        "project",
				ProjectID:   img.ProjectID,
				ProjectName: img.ProjectName,
				Expanded:    true,
				Children:    []*TreeNode{},
			}
			projectMap[img.ProjectID] = projectNode
			m.tree = append(m.tree, projectNode)
		}

		// Get or create registry node
		registryKey := fmt.Sprintf("%d-%d", img.ProjectID, img.RegistryID)
		registryNode, exists := registryMap[registryKey]
		if !exists {
			registryNode = &TreeNode{
				Type:         "registry",
				ProjectID:    img.ProjectID,
				RegistryID:   img.RegistryID,
				RegistryPath: img.RegistryPath,
				Expanded:     true,
				Parent:       projectNode,
				Children:     []*TreeNode{},
			}
			registryMap[registryKey] = registryNode
			projectNode.Children = append(projectNode.Children, registryNode)
		}

		// Create image node
		imgCopy := img
		imageNode := &TreeNode{
			Type:       "image",
			ProjectID:  img.ProjectID,
			RegistryID: img.RegistryID,
			Image:      &imgCopy,
			Parent:     registryNode,
		}
		registryNode.Children = append(registryNode.Children, imageNode)
	}
}

// Init initializes the model
func (m *ImageSelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m *ImageSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle quit message
	if _, ok := msg.(quitMsg); ok {
		return m, tea.Quit
	}

	// Handle deletion complete message
	if msg, ok := msg.(deletionCompleteMsg); ok {
		m.handleDeletionComplete(msg)
		return m, nil
	}

	// Check quit confirmation first (takes precedence)
	if m.showQuitConfirm {
		return m.updateQuitConfirm(msg)
	}

	if m.showConfirm {
		return m.updateConfirm(msg)
	}

	if m.deleting {
		return m, nil // Wait for deletion to complete
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			selectedCount := m.getSelectedCount()
			m.showQuitConfirm = true
			if selectedCount > 0 {
				m.quitConfirmMsg = fmt.Sprintf("Quit? %d selected image(s) will be deleted. (y/n)", selectedCount)
			} else {
				m.quitConfirmMsg = "Quit? (y/n)"
			}
			return m, nil

		case "up", "k":
			m.moveCursor(-1)
			return m, nil

		case "down", "j":
			m.moveCursor(1)
			return m, nil

		case " ":
			m.toggleSelection()
			return m, nil

		case "enter":
			m.toggleExpand()
			return m, nil

		case "tab":
			m.toggleExpandAll()
			return m, nil

		case "d":
			if m.getSelectedCount() > 0 {
				m.showConfirm = true
				if m.dryRun {
					m.confirmMsg = fmt.Sprintf("DRY RUN: Delete %d selected image(s)? (y/n)", m.getSelectedCount())
				} else {
					m.confirmMsg = fmt.Sprintf("Delete %d selected image(s)? This cannot be undone! (y/n)", m.getSelectedCount())
				}
				return m, nil
			}
			return m, nil
		}
	}

	return m, nil
}

// updateConfirm handles confirmation dialog
func (m *ImageSelectorModel) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.showConfirm = false
			m.deleting = true
			return m, m.deleteSelected()
		case "n", "N", "esc":
			m.showConfirm = false
			return m, nil
		}
	}
	return m, nil
}

// updateQuitConfirm handles quit confirmation dialog
func (m *ImageSelectorModel) updateQuitConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.showQuitConfirm = false
			m.finalSelected = m.getSelectedImages()
			return m, func() tea.Msg {
				return quitMsg{}
			}
		case "n", "N", "esc":
			m.showQuitConfirm = false
			return m, nil
		}
	}
	return m, nil
}

// quitMsg is sent when user confirms quit
type quitMsg struct{}

// GetSelectedImages returns all selected images (public method for external access)
func (m *ImageSelectorModel) GetSelectedImages() []ImageItem {
	// Return finalSelected if set (after quit), otherwise get current selection
	if len(m.finalSelected) > 0 {
		return m.finalSelected
	}
	return m.getSelectedImages()
}

// RestoreSelections restores previous selections from a list of ImageItem
func (m *ImageSelectorModel) RestoreSelections(selectedImages []ImageItem) {
	// Create a map for quick lookup
	selectedMap := make(map[string]bool)
	for _, img := range selectedImages {
		key := fmt.Sprintf("%d-%d-%s", img.ProjectID, img.RegistryID, img.ImageInfo.Name)
		selectedMap[key] = true
	}

	// Traverse tree and restore selections
	var traverse func(nodes []*TreeNode)
	traverse = func(nodes []*TreeNode) {
		for _, node := range nodes {
			if node.Type == "image" && node.Image != nil {
				key := fmt.Sprintf("%d-%d-%s", node.ProjectID, node.RegistryID, node.Image.ImageInfo.Name)
				if selectedMap[key] {
					node.Selected = true
					node.Image.Selected = true
				} else {
					node.Selected = false
					node.Image.Selected = false
				}
			}
			if len(node.Children) > 0 {
				traverse(node.Children)
			}
		}
	}
	traverse(m.tree)

	// Clear finalSelected so GetSelectedImages uses current tree state
	m.finalSelected = nil
}

// View renders the UI
func (m *ImageSelectorModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("🧼 GitLab Image Cleaner"))
	b.WriteString("\n\n")

	// Tree view
	flatNodes := m.getFlatNodes()
	start := 0
	end := len(flatNodes)
	maxHeight := m.height - 10 // Reserve space for status bar and help

	if len(flatNodes) > maxHeight {
		if m.cursor >= maxHeight {
			start = m.cursor - maxHeight + 1
			end = m.cursor + 1
		} else {
			end = maxHeight
		}
	}

	for i := start; i < end && i < len(flatNodes); i++ {
		node := flatNodes[i]
		line := m.renderNode(node, i == m.cursor)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Status bar
	status := m.renderStatusBar()
	b.WriteString(statusBarStyle.Width(m.width).Render(status))
	b.WriteString("\n")

	// Help text
	help := m.renderHelp()
	b.WriteString(helpStyle.Render(help))

	// Quit confirmation dialog (takes precedence)
	if m.showQuitConfirm {
		b.WriteString("\n\n")
		b.WriteString(confirmStyle.Render(m.quitConfirmMsg))
	} else if m.showConfirm {
		// Deletion confirmation dialog
		b.WriteString("\n\n")
		b.WriteString(confirmStyle.Render(m.confirmMsg))
	}

	return b.String()
}

// getFlatNodes returns a flat list of visible nodes
func (m *ImageSelectorModel) getFlatNodes() []*TreeNode {
	var result []*TreeNode
	var traverse func(nodes []*TreeNode, depth int)
	traverse = func(nodes []*TreeNode, depth int) {
		for _, node := range nodes {
			result = append(result, node)
			if node.Expanded && len(node.Children) > 0 {
				traverse(node.Children, depth+1)
			}
		}
	}
	traverse(m.tree, 0)
	return result
}

// renderNode renders a single node
func (m *ImageSelectorModel) renderNode(node *TreeNode, isCursor bool) string {
	var content string
	var style lipgloss.Style

	depth := m.getDepth(node)
	indent := strings.Repeat("  ", depth)

	// Cursor indicator
	cursor := " "
	if isCursor {
		cursor = cursorStyle.Render("▶")
	}

	switch node.Type {
	case "project":
		expand := "▼"
		if !node.Expanded {
			expand = "▶"
		}
		content = fmt.Sprintf("%s %s", expand, node.ProjectName)
		style = projectStyle
		if isCursor {
			style = style.Bold(true).Underline(true)
		}

	case "registry":
		expand := "▼"
		if !node.Expanded {
			expand = "▶"
		}
		content = fmt.Sprintf("%s Registry: %s", expand, node.RegistryPath)
		style = registryStyle
		if isCursor {
			style = style.Bold(true).Underline(true)
		}

	case "image":
		checkbox := checkboxEmptyStyle.Render("☐")
		if node.Selected {
			checkbox = checkboxStyle.Render("☑")
		}
		textContent := node.Image.ImageInfo.Name
		style = imageStyle
		if node.Selected {
			style = selectedStyle
		}
		if isCursor {
			style = style.Bold(true).Underline(true)
		}
		styledText := style.Render(textContent)
		return fmt.Sprintf("%s%s %s %s", cursor, indent, checkbox, styledText)
	}

	return fmt.Sprintf("%s%s %s", cursor, indent, style.Render(content))
}

// getDepth calculates the depth of a node
func (m *ImageSelectorModel) getDepth(node *TreeNode) int {
	depth := 0
	current := node.Parent
	for current != nil {
		depth++
		current = current.Parent
	}
	return depth
}

// moveCursor moves the cursor up or down
func (m *ImageSelectorModel) moveCursor(delta int) {
	flatNodes := m.getFlatNodes()
	if len(flatNodes) == 0 {
		return
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	} else if m.cursor >= len(flatNodes) {
		m.cursor = len(flatNodes) - 1
	}
}

// toggleSelection toggles selection of the current item
func (m *ImageSelectorModel) toggleSelection() {
	flatNodes := m.getFlatNodes()
	if m.cursor >= len(flatNodes) {
		return
	}

	node := flatNodes[m.cursor]
	if node.Type == "image" {
		node.Selected = !node.Selected
		if node.Image != nil {
			node.Image.Selected = node.Selected
		}
	} else if node.Type == "project" || node.Type == "registry" {
		// Toggle all children
		m.toggleNodeChildren(node, !node.Selected)
	}
}

// toggleNodeChildren toggles all children of a node
func (m *ImageSelectorModel) toggleNodeChildren(node *TreeNode, selected bool) {
	var traverse func(n *TreeNode)
	traverse = func(n *TreeNode) {
		if n.Type == "image" {
			n.Selected = selected
			if n.Image != nil {
				n.Image.Selected = selected
			}
		}
		for _, child := range n.Children {
			traverse(child)
		}
	}
	traverse(node)
}

// toggleExpand toggles expansion of the current node
func (m *ImageSelectorModel) toggleExpand() {
	flatNodes := m.getFlatNodes()
	if m.cursor >= len(flatNodes) {
		return
	}

	node := flatNodes[m.cursor]
	if node.Type == "project" || node.Type == "registry" {
		node.Expanded = !node.Expanded
	}
}

// toggleExpandAll toggles expansion of all nodes
func (m *ImageSelectorModel) toggleExpandAll() {
	expand := true
	for _, node := range m.tree {
		if node.Expanded {
			expand = false
			break
		}
	}

	var traverse func(nodes []*TreeNode)
	traverse = func(nodes []*TreeNode) {
		for _, node := range nodes {
			if node.Type == "project" || node.Type == "registry" {
				node.Expanded = expand
			}
			if len(node.Children) > 0 {
				traverse(node.Children)
			}
		}
	}
	traverse(m.tree)
}

// getSelectedCount returns the number of selected images
func (m *ImageSelectorModel) getSelectedCount() int {
	count := 0
	var traverse func(nodes []*TreeNode)
	traverse = func(nodes []*TreeNode) {
		for _, node := range nodes {
			if node.Type == "image" && node.Selected {
				count++
			}
			if len(node.Children) > 0 {
				traverse(node.Children)
			}
		}
	}
	traverse(m.tree)
	return count
}

// getSelectedImages returns all selected images (private method)
func (m *ImageSelectorModel) getSelectedImages() []ImageItem {
	var selectedImages []ImageItem
	var traverse func(nodes []*TreeNode)
	traverse = func(nodes []*TreeNode) {
		for _, node := range nodes {
			if node.Type == "image" && node.Selected && node.Image != nil {
				selectedImages = append(selectedImages, *node.Image)
			}
			if len(node.Children) > 0 {
				traverse(node.Children)
			}
		}
	}
	traverse(m.tree)
	return selectedImages
}

// getTotalCount returns the total number of images
func (m *ImageSelectorModel) getTotalCount() int {
	return len(m.images)
}

// renderStatusBar renders the status bar
func (m *ImageSelectorModel) renderStatusBar() string {
	selected := m.getSelectedCount()
	total := m.getTotalCount()
	dryRunText := ""
	if m.dryRun {
		dryRunText = " | 🌵 DRY RUN"
	}
	deletingText := ""
	if m.deleting {
		deletingText = fmt.Sprintf(" | 🗑️  Deleting... (%d deleted, %d failed)", m.deletedCount, m.failedCount)
	}
	return fmt.Sprintf("Total: %d | Selected: %d%s%s", total, selected, dryRunText, deletingText)
}

// renderHelp renders the help text
func (m *ImageSelectorModel) renderHelp() string {
	if m.showQuitConfirm {
		return "Press 'y' to confirm quit, 'n' to cancel"
	}
	if m.showConfirm {
		return "Press 'y' to confirm, 'n' to cancel"
	}
	return "↑/↓: Navigate | Space: Toggle | Enter: Expand/Collapse | Tab: Expand/Collapse All | d: Delete Selected | q: Quit"
}

// deleteSelected deletes all selected images
func (m *ImageSelectorModel) deleteSelected() tea.Cmd {
	return func() tea.Msg {
		var selectedImages []ImageItem
		var traverse func(nodes []*TreeNode)
		traverse = func(nodes []*TreeNode) {
			for _, node := range nodes {
				if node.Type == "image" && node.Selected && node.Image != nil {
					selectedImages = append(selectedImages, *node.Image)
				}
				if len(node.Children) > 0 {
					traverse(node.Children)
				}
			}
		}
		traverse(m.tree)

		deletedCount := 0
		failedCount := 0

		for _, img := range selectedImages {
			if m.dryRun {
				deletedCount++
			} else {
				_, err := m.gitlabClient.DeleteRegistryRepositoryTag(img.ProjectID, img.RegistryID, img.ImageInfo.Name)
				if err != nil {
					failedCount++
				} else {
					deletedCount++
				}
			}
		}

		// Remove deleted images from tree
		m.removeDeletedImages(selectedImages)

		return deletionCompleteMsg{
			deletedCount: deletedCount,
			failedCount:  failedCount,
		}
	}
}

// deletionCompleteMsg is sent when deletion is complete
type deletionCompleteMsg struct {
	deletedCount int
	failedCount  int
}

// removeDeletedImages removes deleted images from the tree
func (m *ImageSelectorModel) removeDeletedImages(deleted []ImageItem) {
	deletedMap := make(map[string]bool)
	for _, img := range deleted {
		key := fmt.Sprintf("%d-%d-%s", img.ProjectID, img.RegistryID, img.ImageInfo.Name)
		deletedMap[key] = true
	}

	var removeFromNode func(node *TreeNode) bool
	removeFromNode = func(node *TreeNode) bool {
		if node.Type == "image" && node.Image != nil {
			key := fmt.Sprintf("%d-%d-%s", node.ProjectID, node.RegistryID, node.Image.ImageInfo.Name)
			if deletedMap[key] {
				return true // Mark for removal
			}
		}

		// Filter children
		var newChildren []*TreeNode
		for _, child := range node.Children {
			if !removeFromNode(child) {
				newChildren = append(newChildren, child)
			}
		}
		node.Children = newChildren

		return false
	}

	for i := len(m.tree) - 1; i >= 0; i-- {
		if removeFromNode(m.tree[i]) {
			m.tree = append(m.tree[:i], m.tree[i+1:]...)
		}
	}

	// Rebuild images list
	m.images = nil
	var collectImages func(nodes []*TreeNode)
	collectImages = func(nodes []*TreeNode) {
		for _, node := range nodes {
			if node.Type == "image" && node.Image != nil {
				m.images = append(m.images, *node.Image)
			}
			if len(node.Children) > 0 {
				collectImages(node.Children)
			}
		}
	}
	collectImages(m.tree)
}

// Handle deletion complete message
func (m *ImageSelectorModel) handleDeletionComplete(msg deletionCompleteMsg) {
	m.deletedCount = msg.deletedCount
	m.failedCount = msg.failedCount
	m.deleting = false
}
