package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type module int

const (
	overviewModule module = iota
	accessKeysModule
	dumpLogsModule
)

type rootModel struct {
	service  *adminService
	active   module
	overview overviewSnapshot
	access   accessKeysModel
	dumps    dumpViewerModel
	width    int
	height   int
	err      error
}

type rootRefreshMsg struct{}

func Run(cfgPath string, in io.Reader, out io.Writer) error {
	service, err := newAdminService(strings.TrimSpace(cfgPath))
	if err != nil {
		return fmt.Errorf("init admin TUI: %w", err)
	}
	defer func() { _ = service.Close() }()
	dumpsDir := "./dumps"
	if service.cfg != nil && strings.TrimSpace(service.cfg.TrafficDump.Dir) != "" {
		dumpsDir = strings.TrimSpace(service.cfg.TrafficDump.Dir)
	}
	m := newRootModel(service, dumpsDir)
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui run failed: %w", err)
	}
	return nil
}

func newRootModel(service *adminService, dumpsDir string) rootModel {
	return rootModel{service: service, active: overviewModule, access: newAccessKeysModel(service), dumps: newDumpViewerModel(dumpsDir)}
}

func (m rootModel) Init() tea.Cmd {
	return tea.Batch(overviewCmd(m.service), m.access.Init(), m.dumps.Init(), refreshTickCmd())
}

func refreshTickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(_ time.Time) tea.Msg { return rootRefreshMsg{} })
}

func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case rootRefreshMsg:
		return m, tea.Batch(overviewCmd(m.service), refreshTickCmd())
	case overviewMsg:
		m.overview = msg.snapshot
		return m, nil
	case accessKeysMsg, accessActionMsg, subjectStateMsg:
		var cmd tea.Cmd
		m.access, cmd = m.access.Update(msg)
		m.err = m.access.err
		return m, cmd
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, rootQuitKey) && m.rootQuitEnabled():
			return m, tea.Quit
		case key.Matches(keyMsg, rootNextKey) && m.moduleSwitchEnabled():
			m.active = (m.active + 1) % 3
			m.resize()
			return m, nil
		case key.Matches(keyMsg, rootPrevKey) && m.moduleSwitchEnabled():
			m.active = (m.active + 2) % 3
			m.resize()
			return m, nil
		case keyMsg.String() == "1" && m.moduleSwitchEnabled():
			m.active = overviewModule
		case keyMsg.String() == "2" && m.moduleSwitchEnabled():
			m.active = accessKeysModule
		case keyMsg.String() == "3" && m.moduleSwitchEnabled():
			m.active = dumpLogsModule
		case keyMsg.String() == "r" && m.active == overviewModule:
			return m, overviewCmd(m.service)
		}
		m.resize()
	}
	var cmd tea.Cmd
	switch m.active {
	case accessKeysModule:
		m.access, cmd = m.access.Update(msg)
		m.err = m.access.err
	case dumpLogsModule:
		var model tea.Model
		model, cmd = m.dumps.Update(msg)
		m.dumps = model.(dumpViewerModel)
	}
	return m, cmd
}

func (m rootModel) moduleSwitchEnabled() bool {
	return m.active != accessKeysModule || m.access.state == accessListState || m.access.state == accessDetailState
}

func (m rootModel) rootQuitEnabled() bool {
	if m.active != accessKeysModule {
		return true
	}
	if m.access.state == accessFormState {
		return false
	}
	if m.access.state == accessListState {
		return !m.access.filter.Focused()
	}
	return true
}

func (m rootModel) View() string {
	nav := m.navigationView()
	content := ""
	switch m.active {
	case overviewModule:
		content = renderOverview(m.overview, maxInt(40, m.width-24))
	case accessKeysModule:
		content = m.access.View()
	case dumpLogsModule:
		content = m.dumps.View()
	}
	if m.err != nil && m.active == accessKeysModule {
		content += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("error: "+m.err.Error())
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, nav, lipgloss.NewStyle().PaddingLeft(2).Render(content))
}

func (m rootModel) navigationView() string {
	items := []string{"1  Overview", "2  Access Keys", "3  Dump Logs"}
	for i := range items {
		if module(i) == m.active {
			items[i] = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")).Render("> " + items[i])
		} else {
			items[i] = "  " + items[i]
		}
	}
	return lipgloss.NewStyle().Width(18).BorderRight(true).PaddingRight(1).Render(strings.Join(items, "\n\n"))
}

func (m *rootModel) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.dumps.width = maxInt(20, m.width-22)
	m.dumps.height = m.height
	m.dumps.resize()
	m.access.width = maxInt(20, m.width-22)
	m.access.height = m.height
	m.access.table.SetWidth(maxInt(20, m.width-26))
	m.access.table.SetHeight(maxInt(5, m.height-12))
}

const refreshInterval = 5 * time.Second

var rootQuitKey = key.NewBinding(key.WithKeys("q", "ctrl+c"))
var rootNextKey = key.NewBinding(key.WithKeys("tab"))
var rootPrevKey = key.NewBinding(key.WithKeys("shift+tab"))
