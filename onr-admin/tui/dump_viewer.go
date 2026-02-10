package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/r9s-ai/open-next-router/onr-admin/store"
)

type dumpViewerState int

const (
	dumpViewerStateList dumpViewerState = iota
	dumpViewerStateDetail
)

type dumpKeyMap struct {
	Open   key.Binding
	Back   key.Binding
	Reload key.Binding
	Quit   key.Binding
}

func (k dumpKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Reload, k.Quit}
}

func (k dumpKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.Back, k.Reload},
		{k.Quit},
	}
}

var dumpKeys = dumpKeyMap{
	Open: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "b"),
		key.WithHelp("esc/b", "back"),
	),
	Reload: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reload"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

type dumpItem struct {
	sum store.DumpSummary
}

func (i dumpItem) Title() string {
	ts := i.sum.Time
	if ts.IsZero() {
		ts = i.sum.ModTime
	}
	timeText := "-"
	if !ts.IsZero() {
		timeText = ts.Format("2006-01-02 15:04:05")
	}
	status := "-"
	if i.sum.ProxyStatus != 0 {
		status = fmt.Sprintf("%d", i.sum.ProxyStatus)
	}
	provider := strings.TrimSpace(i.sum.Provider)
	if provider == "" {
		provider = "-"
	}
	model := strings.TrimSpace(i.sum.Model)
	if model == "" {
		model = "-"
	}
	return fmt.Sprintf("%s  %s  %s  %s", timeText, status, provider, model)
}

func (i dumpItem) Description() string {
	path := strings.TrimSpace(i.sum.URLPath)
	if path == "" {
		path = "-"
	}
	rid := strings.TrimSpace(i.sum.RequestID)
	if rid == "" {
		rid = strings.TrimSuffix(i.sum.FileName, filepath.Ext(i.sum.FileName))
	}
	return fmt.Sprintf("path=%s rid=%s file=%s", path, rid, i.sum.FileName)
}

func (i dumpItem) FilterValue() string {
	parts := []string{
		strings.TrimSpace(i.sum.Provider),
		strings.TrimSpace(i.sum.Model),
		strings.TrimSpace(i.sum.URLPath),
		strings.TrimSpace(i.sum.Method),
		strings.TrimSpace(i.sum.RequestID),
	}
	if i.sum.ProxyStatus != 0 {
		parts = append(parts, fmt.Sprintf("%d", i.sum.ProxyStatus))
	}
	if i.sum.Stream != nil {
		if *i.sum.Stream {
			parts = append(parts, "stream=true")
		} else {
			parts = append(parts, "stream=false")
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

type dumpViewerModel struct {
	dumpsDir string
	limit    int

	state dumpViewerState
	list  list.Model
	vp    viewport.Model
	help  help.Model
	keys  dumpKeyMap

	width  int
	height int

	selectedPath string
	lastLoaded   time.Time
	err          error
}

type dumpListMsg struct {
	items []store.DumpSummary
	err   error
}

type dumpFileMsg struct {
	path    string
	content string
	err     error
}

func newDumpViewerProgram(dumpsDir string, in io.Reader, out io.Writer) *tea.Program {
	m := newDumpViewerModel(dumpsDir)
	p := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen())
	return p
}

func newDumpViewerModel(dumpsDir string) dumpViewerModel {
	d := list.NewDefaultDelegate()
	d.ShowDescription = true
	d.SetSpacing(0)

	l := list.New(nil, d, 0, 0)
	l.Title = "Dump Logs"
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowFilter(true)
	l.DisableQuitKeybindings()

	h := help.New()
	h.ShowAll = false

	return dumpViewerModel{
		dumpsDir: strings.TrimSpace(dumpsDir),
		limit:    200,
		state:    dumpViewerStateList,
		list:     l,
		vp:       viewport.New(0, 0),
		help:     h,
		keys:     dumpKeys,
	}
}

func (m dumpViewerModel) Init() tea.Cmd {
	return m.loadDumpsCmd()
}

func (m dumpViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case dumpListMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		items := make([]list.Item, 0, len(msg.items))
		for _, s := range msg.items {
			items = append(items, dumpItem{sum: s})
		}
		m.list.SetItems(items)
		m.lastLoaded = time.Now()
		m.err = nil
		return m, nil

	case dumpFileMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.selectedPath = msg.path
		m.vp.SetContent(msg.content)
		m.vp.GotoTop()
		m.state = dumpViewerStateDetail
		m.resize()
		m.err = nil
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case m.state == dumpViewerStateDetail && key.Matches(msg, m.keys.Back):
			m.state = dumpViewerStateList
			m.resize()
			return m, nil
		case m.state == dumpViewerStateList && key.Matches(msg, m.keys.Reload):
			return m, m.loadDumpsCmd()
		case m.state == dumpViewerStateDetail && key.Matches(msg, m.keys.Reload):
			// Reload list in background; keep detail view untouched.
			return m, m.loadDumpsCmd()
		case m.state == dumpViewerStateList && key.Matches(msg, m.keys.Open):
			it, ok := m.list.SelectedItem().(dumpItem)
			if !ok {
				return m, nil
			}
			return m, m.readDumpFileCmd(it.sum.Path)
		}
	}

	switch m.state {
	case dumpViewerStateList:
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	case dumpViewerStateDetail:
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	default:
		return m, nil
	}
}

func (m dumpViewerModel) View() string {
	var b strings.Builder
	switch m.state {
	case dumpViewerStateList:
		header := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Dump Logs  dir=%s  limit=%d", m.dumpsDir, m.limit))
		b.WriteString(header)
		b.WriteString("\n")
		if !m.lastLoaded.IsZero() {
			b.WriteString(lipgloss.NewStyle().Faint(true).Render("loaded: " + m.lastLoaded.Format(time.RFC3339)))
			b.WriteString("\n")
		}
		if m.err != nil {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("error: " + m.err.Error()))
			b.WriteString("\n\n")
		}
		b.WriteString(m.list.View())
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Faint(true).Render("Tip: press / to filter (provider/model/path/status/rid), esc to clear filter"))
		b.WriteString("\n")
		b.WriteString(m.help.View(m.keys))
		return b.String()

	case dumpViewerStateDetail:
		title := fmt.Sprintf("Dump File  %s", m.selectedPath)
		b.WriteString(lipgloss.NewStyle().Bold(true).Render(title))
		b.WriteString("\n")
		if m.err != nil {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("error: " + m.err.Error()))
			b.WriteString("\n\n")
		}
		b.WriteString(m.vp.View())
		b.WriteString("\n")
		b.WriteString(m.help.View(m.keys))
		return b.String()
	default:
		return ""
	}
}

func (m *dumpViewerModel) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	helpHeight := 1
	if m.help.ShowAll {
		helpHeight = 3
	}

	switch m.state {
	case dumpViewerStateList:
		// header(2-4) + list + tip(1) + help(1)
		headerLines := 2
		if !m.lastLoaded.IsZero() {
			headerLines++
		}
		if m.err != nil {
			headerLines += 2
		}
		avail := m.height - headerLines - 1 - helpHeight
		if avail < 5 {
			avail = 5
		}
		m.list.SetSize(m.width, avail)
	case dumpViewerStateDetail:
		headerLines := 1
		if m.err != nil {
			headerLines += 2
		}
		avail := m.height - headerLines - helpHeight
		if avail < 5 {
			avail = 5
		}
		m.vp.Width = m.width
		m.vp.Height = avail
	}
}

func (m dumpViewerModel) loadDumpsCmd() tea.Cmd {
	dir := strings.TrimSpace(m.dumpsDir)
	limit := m.limit
	return func() tea.Msg {
		items, err := store.ListDumpSummaries(store.DumpListOptions{
			Dir:   dir,
			Limit: limit,
		})
		return dumpListMsg{items: items, err: err}
	}
}

func (m dumpViewerModel) readDumpFileCmd(path string) tea.Cmd {
	p := strings.TrimSpace(path)
	return func() tea.Msg {
		b, err := os.ReadFile(p) // #nosec G304 -- admin tool reads user-selected dump file.
		if err != nil {
			return dumpFileMsg{path: p, err: err}
		}
		return dumpFileMsg{path: p, content: string(b)}
	}
}
