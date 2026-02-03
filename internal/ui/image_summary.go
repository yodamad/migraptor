package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Summary styles
	summaryTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	summaryProjectStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	summaryImageStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	summaryLocationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	summaryFooterStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Background(lipgloss.Color("236"))
	summaryHelpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
)

// ImageSummaryModel represents the bubbletea model for displaying selected images summary
type ImageSummaryModel struct {
	images       []ImageItem
	grouped      map[string][]ImageItem
	projectOrder []string
	cursor       int
	width        int
	height       int
	wentBack     bool // Track if user pressed 'b' to go back
}

// NewImageSummaryModel creates a new image summary model
func NewImageSummaryModel(images []ImageItem) *ImageSummaryModel {
	grouped, projectOrder := groupImagesByProject(images)
	return &ImageSummaryModel{
		images:       images,
		grouped:      grouped,
		projectOrder: projectOrder,
		cursor:       0,
	}
}

// groupImagesByProject groups images by project name and maintains order
func groupImagesByProject(images []ImageItem) (map[string][]ImageItem, []string) {
	grouped := make(map[string][]ImageItem)
	projectOrder := []string{}
	projectSet := make(map[string]bool)

	for _, img := range images {
		projectName := img.ProjectName
		if _, exists := grouped[projectName]; !exists {
			grouped[projectName] = []ImageItem{}
			if !projectSet[projectName] {
				projectOrder = append(projectOrder, projectName)
				projectSet[projectName] = true
			}
		}
		grouped[projectName] = append(grouped[projectName], img)
	}

	return grouped, projectOrder
}

// Init initializes the model
func (m *ImageSummaryModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m *ImageSummaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "b", "B":
			m.wentBack = true
			return m, tea.Quit

		case "up", "k":
			m.moveCursor(-1)
			return m, nil

		case "down", "j":
			m.moveCursor(1)
			return m, nil
		}
	}

	return m, nil
}

// WentBack returns true if user pressed 'b' to go back
func (m *ImageSummaryModel) WentBack() bool {
	return m.wentBack
}

// View renders the UI
func (m *ImageSummaryModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	var b strings.Builder

	// Title
	b.WriteString(summaryTitleStyle.Render("Selected Images Summary"))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("=", m.width))
	b.WriteString("\n\n")

	// Build content lines
	lines := m.buildContentLines()

	// Calculate viewport
	start, end := m.calculateViewport(len(lines))

	// Render visible lines
	for i := start; i < end && i < len(lines); i++ {
		b.WriteString(lines[i])
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Footer with total count
	footer := fmt.Sprintf("Total: %d selected image(s)", len(m.images))
	b.WriteString(summaryFooterStyle.Width(m.width).Render(footer))
	b.WriteString("\n")

	// Help text
	help := "Press 'b' to go back to selection, 'q' to quit"
	b.WriteString(summaryHelpStyle.Render(help))

	return b.String()
}

// buildContentLines builds all content lines for display
func (m *ImageSummaryModel) buildContentLines() []string {
	var lines []string

	for _, projectName := range m.projectOrder {
		images := m.grouped[projectName]

		// Project header
		projectHeader := fmt.Sprintf("Project: %s", projectName)
		lines = append(lines, summaryProjectStyle.Render(projectHeader))

		// Images under project
		for _, img := range images {
			imageLine := fmt.Sprintf("  - %s", summaryImageStyle.Render(img.ImageInfo.Name))
			if img.ImageInfo.Location != "" {
				imageLine += fmt.Sprintf(" %s", summaryLocationStyle.Render(fmt.Sprintf("(%s)", img.ImageInfo.Location)))
			}
			lines = append(lines, imageLine)
		}

		// Empty line between projects
		lines = append(lines, "")
	}

	return lines
}

// calculateViewport calculates which lines should be visible based on cursor and window size
func (m *ImageSummaryModel) calculateViewport(totalLines int) (start, end int) {
	if totalLines == 0 {
		return 0, 0
	}

	// Reserve space for title (3 lines), footer (2 lines), and help (1 line)
	availableHeight := m.height - 6
	if availableHeight < 1 {
		availableHeight = 1
	}

	// If content fits in available space, show everything from start
	if totalLines <= availableHeight {
		return 0, totalLines
	}

	// Otherwise, use cursor-based scrolling
	start = m.cursor
	end = start + availableHeight

	if end > totalLines {
		end = totalLines
		start = end - availableHeight
		if start < 0 {
			start = 0
		}
	}

	return start, end
}

// moveCursor moves the cursor up or down
func (m *ImageSummaryModel) moveCursor(delta int) {
	lines := m.buildContentLines()
	totalLines := len(lines)

	if totalLines == 0 {
		return
	}

	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	} else if m.cursor >= totalLines {
		m.cursor = totalLines - 1
	}
}
