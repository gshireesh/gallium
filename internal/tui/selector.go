package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	choices  []string
	cursor   int
	selected bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	s := "Choose a template:\n\n"
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += cursor + " " + choice + "\n"
	}
	if m.selected {
		s += "\nGenerating project...\n"
	}
	return s
}

func SelectTemplate(templates []string) (string, error) {
	p := tea.NewProgram(model{choices: templates})
	m, err := p.Run()
	if err != nil {
		return "", err
	}
	return m.(model).choices[m.(model).cursor], nil
}
