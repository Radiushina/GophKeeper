package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiScreen int

const (
	tuiHome tuiScreen = iota
	tuiLogin
	tuiRegister
)

type authResultMsg struct {
	login string
	err   error
}

type tuiModel struct {
	ctx    context.Context
	app    *App
	screen tuiScreen
	inputs []textinput.Model
	focus  int
	busy   bool
	status string
	err    string
	width  int
	height int
}

func RunTUI(ctx context.Context, app *App) error {
	p := tea.NewProgram(newTUIModel(ctx, app), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

func newTUIModel(ctx context.Context, app *App) tuiModel {
	login := textinput.New()
	login.Placeholder = "login"
	login.CharLimit = 64
	login.Width = 32

	password := textinput.New()
	password.Placeholder = "password"
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'
	password.CharLimit = 128
	password.Width = 32

	return tuiModel{
		ctx:    ctx,
		app:    app,
		screen: tuiHome,
		inputs: []textinput.Model{login, password},
	}
}

func (m tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case authResultMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.status = "signed in as " + msg.login
		m.screen = tuiHome
		return m.blurInputs(), nil
	case tea.KeyMsg:
		if m.busy {
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			return m, nil
		}
		switch m.screen {
		case tuiHome:
			return m.updateHome(msg)
		default:
			return m.updateForm(msg)
		}
	}
	if m.screen != tuiHome {
		return m.updateInputs(msg)
	}
	return m, nil
}

func (m tuiModel) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "l":
		return m.openForm(tuiLogin)
	case "r":
		return m.openForm(tuiRegister)
	}
	return m, nil
}

func (m tuiModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.screen = tuiHome
		m.err = ""
		return m.blurInputs(), nil
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
		if msg.Type == tea.KeyUp || msg.Type == tea.KeyShiftTab {
			m.focus = (m.focus + len(m.inputs) - 1) % len(m.inputs)
		} else {
			m.focus = (m.focus + 1) % len(m.inputs)
		}
		return m.focusInputs()
	case tea.KeyEnter:
		return m.submit()
	}
	return m.updateInputs(msg)
}

func (m tuiModel) openForm(screen tuiScreen) (tea.Model, tea.Cmd) {
	m.screen = screen
	m.err = ""
	m.status = ""
	m.focus = 0
	m.inputs[0].SetValue("")
	m.inputs[1].SetValue("")
	return m.focusInputs()
}

func (m tuiModel) submit() (tea.Model, tea.Cmd) {
	login := strings.TrimSpace(m.inputs[0].Value())
	password := m.inputs[1].Value()
	if login == "" || password == "" {
		m.err = "login and password are required"
		return m, nil
	}
	m.busy = true
	m.err = ""
	register := m.screen == tuiRegister
	app := m.app
	ctx := m.ctx
	return m, func() tea.Msg {
		var err error
		if register {
			err = Register(ctx, app, login, password)
		} else {
			err = Login(ctx, app, login, password)
		}
		return authResultMsg{login: login, err: err}
	}
}

func (m tuiModel) blurInputs() tuiModel {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	return m
}

func (m tuiModel) focusInputs() (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		if i == m.focus {
			cmds[i] = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	return m, tea.Batch(cmds...)
}

func (m tuiModel) updateInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render("GophKeeper")
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("vault for notes and files")

	server := m.serverURL()
	session := "not signed in"
	if user := m.app.User(); user != "" && m.app.Token() != "" {
		session = "signed in as " + user
	}

	meta := lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Render(
		fmt.Sprintf("server  %s\nsession %s", server, session),
	)

	body := m.homeBody()
	if m.screen != tuiHome {
		body = m.formBody()
	}

	parts := []string{title, subtitle, "", meta, "", body}
	if m.status != "" {
		parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(m.status))
	}
	if m.err != "" {
		parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render(m.err))
	}
	if m.busy {
		parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("working…"))
	}
	parts = append(parts, "", m.help())

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Width(max(40, m.width-4))
	return box.Render(content)
}

func (m tuiModel) homeBody() string {
	if m.app.Token() == "" {
		return "Sign in to open the vault.\nNotes and files will show up here."
	}
	return "Vault is empty for now.\nNotes, files and cards will land in this list."
}

func (m tuiModel) formBody() string {
	heading := "Login"
	if m.screen == tuiRegister {
		heading = "Register"
	}
	return heading + "\n\n" + m.inputs[0].View() + "\n" + m.inputs[1].View()
}

func (m tuiModel) help() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	if m.screen == tuiHome {
		return style.Render("l login   r register   q quit")
	}
	return style.Render("enter submit   tab next   esc back")
}

func (m tuiModel) serverURL() string {
	if m.app != nil && m.app.Server != "" {
		return m.app.Server
	}
	return "unknown"
}
