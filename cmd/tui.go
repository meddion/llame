package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/timer"
	"github.com/charmbracelet/lipgloss"
	"github.com/meddion/llame"

	tea "github.com/charmbracelet/bubbletea"
)

const streamChanCapacity = 10

var (
	textStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render
)

// tea.Msg types:
type (
	endOfStream struct{}
	streamResp  struct {
		msg  string
		err  error
		next tea.Cmd
	}
	errMsg error
)

type keymap struct {
	commit key.Binding
	regen  key.Binding
	quit   key.Binding
}

func newKeymap() keymap {
	return keymap{
		commit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "commit"),
		),
		regen: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "regenerate"),
		),
		quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

type model struct {
	ctx             context.Context
	llm             *llame.LlamaModel
	llmTimeout      time.Duration
	completionQuery llame.CompletionQuery

	spinner   spinner.Model
	textInput textinput.Model
	help      help.Model
	timer     timer.Model

	keymap        keymap
	isStreaming   bool
	suggestions   []string
	err           error
	msgBeforeQuit string
}

func initialModel(ctx context.Context, llm *llame.LlamaModel, comp llame.CompletionQuery) model {
	ti := textinput.New()
	ti.ShowSuggestions = true
	ti.Placeholder = "Write your commit message..."
	ti.Focus()
	// ti.CharLimit = llame.GitCommitSubjectCharsMin + llame.GitCommiBodyCharsMax
	ti.CharLimit = 0
	ti.Width = llame.GitCommitSubjectCharsMin + 10

	m := model{
		ctx:             ctx,
		llm:             llm,
		llmTimeout:      llm.RequestTimeout,
		completionQuery: comp,
		textInput:       ti,
		timer:           timer.NewWithInterval(llm.RequestTimeout, time.Second),
		help:            help.New(),
		keymap:          newKeymap(),
		isStreaming:     true, // Streaming will start after m.Init()
	}

	m.resetSpinner()

	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.startStream(),
		m.spinner.Tick,
		m.timer.Init(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch tMsg := msg.(type) {
	case endOfStream:
		m.isStreaming = false
		m.suggestions = append(m.suggestions, m.textInput.Value())
		m.textInput.SetSuggestions(m.suggestions)
		return m, textinput.Blink
	case streamResp:
		if tMsg.err != nil {
			cmd = newErrMsg(tMsg.err)
		} else {
			m.textInput, cmd = m.textInputUpdate(tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune(tMsg.msg),
			})
		}
		return m, tea.Batch(tMsg.next, cmd)
	case errMsg:
		m.err = tMsg
		llame.Errorf("%s", tMsg)
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(tMsg, m.keymap.quit):
			return m, tea.Quit
		case key.Matches(tMsg, m.keymap.regen):
			if m.isStreaming {
				llame.Debugf("Stream in progress, can't restart")
				return m, nil
			}
			return m, tea.Batch(
				m.restartStream(),
				m.spinner.Tick,
			)
		case key.Matches(tMsg, m.keymap.commit):
			if m.isStreaming {
				llame.Debugf("Stream in progress, can't commit")
				return m, nil
			}

			commitMsg := m.commitMsg()
			if commitMsg == "" {
				return m, newErrMsg(errors.New("cannot commit empty message"))
			}

			if err := llame.GitCommit(commitMsg); err != nil {
				return m, newErrMsg(fmt.Errorf("failed to commit: %w", err))
			}

			m.msgBeforeQuit = "Successfully commited ;)"
			return m, tea.Quit
		}
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case timer.TickMsg:
		var cmd tea.Cmd
		m.timer, cmd = m.timer.Update(msg)
		return m, cmd
	}

	// Accept user input if don't stream LLM's response
	if !m.isStreaming {
		m.textInput, cmd = m.textInputUpdate(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() (s string) {
	if m.msgBeforeQuit != "" {
		return fmt.Sprintf("%s\n", textStyle(m.msgBeforeQuit))
	}

	if m.err != nil {
		s += fmt.Sprintf("\n%s\n", errStyle("ERROR: "+m.err.Error()))
	} else if !m.isStreaming {
		s += fmt.Sprintf("\n%s\n", textStyle("Model response:"))
	} else {
		s += fmt.Sprintf(
			textStyle("\n%s %s (%s)\n"),
			m.spinner.View(),
			"Generating response...",
			m.timer.View(),
		)
	}

	s += fmt.Sprintf(
		"\n%s\n",
		m.textInput.View(),
	)
	s += m.helpView()
	s += "\n"

	return
}

func (m model) helpView() string {
	keybindings := make([]key.Binding, 0, 3)

	if !m.isStreaming {
		if m.commitMsg() != "" {
			keybindings = append(keybindings, m.keymap.commit)
		}
		keybindings = append(keybindings, m.keymap.regen)
	}

	keybindings = append(keybindings, m.keymap.quit)

	return "\n" + m.help.ShortHelpView(keybindings)
}

func (m model) startStream() tea.Cmd {
	errCmd := func(err error) tea.Cmd {
		return tea.Batch(newErrMsg(err), newEndOfStream())
	}

	// llame.Debugf("Start stream with the following query: %v", m.completionQuery)
	llmStream, err := m.llm.ReadStream(m.ctx, m.completionQuery)
	if err != nil {
		llame.Errorf("failed to read from LLM: %w", err)

		return errCmd(fmt.Errorf("failed to read from LLM: %w", err))
	}

	streamChan := make(chan streamResp, streamChanCapacity)

	readStreamCmd := func() tea.Msg {
		msg, ok := <-streamChan
		if !ok {
			return endOfStream{}
		}

		llame.Debugf("Stream message: %v", msg)

		return msg
	}

	go func() {
		defer close(streamChan)

		for llmResp := range llmStream {
			llame.Debugf("LLM response: %v", llmResp)

			streamResp := streamResp{next: readStreamCmd}

			if llmResp.Error != nil {
				streamResp.err = llmResp.Error
			} else {
				streamResp.msg = llmResp.Content
			}

			streamChan <- streamResp
		}

		llame.Debugf("Closing stream channel...")
	}()

	return readStreamCmd
}

func (m *model) restartStream() tea.Cmd {
	m.textInput.Reset()
	m.resetSpinner()

	m.err = nil
	m.isStreaming = true

	return tea.Batch(m.startStream(), m.resetTimer())
}

func (m *model) resetSpinner() {
	m.spinner = spinner.New()
	m.spinner.Spinner = spinner.Monkey
}

func (m *model) resetTimer() tea.Cmd {
	m.timer.Timeout = m.llmTimeout
	return m.timer.Start()
}

func (m model) commitMsg() string {
	return m.textInput.Value()
}

func (m model) textInputUpdate(msg tea.Msg) (ti textinput.Model, cmd tea.Cmd) {
	ti, cmd = m.textInput.Update(msg)
	return ti, tea.Batch(newErrMsg(m.textInput.Err), cmd)
}

func newErrMsg(err error) tea.Cmd {
	return func() tea.Msg {
		return errMsg(err)
	}
}

func newEndOfStream() tea.Cmd {
	return func() tea.Msg {
		return endOfStream{}
	}
}
