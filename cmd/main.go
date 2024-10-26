package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/go-git/go-git/v5"
	"github.com/meddion/llame"

	tea "github.com/charmbracelet/bubbletea"
)

var CLI struct {
	Log           bool          `short:"l" help:"Enable logs."`
	LogDirectory  string        `short:"d" type:"path" help:"Directory where to write logs. By default /tmp and /tmp/var are tried."`
	ModelEndpoint *url.URL      `type:"url" short:"m" env:"MODEL_ENDPOINT" default:"http://127.0.0.1:8080/completion" help:"URL to access a LLM."`
	Timeout       time.Duration `default:"15s" short:"t" help:"Duration for which the model should respond with results."`
}

func main() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-signalChan
		cancel()
	}()

	kongCtx := kong.Parse(&CLI)
	initLogging()

	if kongCtx.Command() != "" {
		llame.Fatalf("unknown command: %s", kongCtx.Command())
	}

	_, err := llame.NewGitRepo()
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			llame.Fatalf("Repository not found: make sure you are running this command inside a git repository.")
		}

		llame.Fatalf("Failed to open git repository: %s", err)
	}

	model := llame.NewLlamaModel(CLI.ModelEndpoint.String(), CLI.Timeout)

	diff, err := llame.GitDiffStaged(rootCtx)
	if err != nil {
		if errors.Is(err, llame.NoStagedFilesErr) {
			llame.Printf("No staged files found. 'git add' one of these files to proceed:\n")
			llame.Printf(llame.MustGitStatus())
			return
		}

		llame.Fatalf("failed to get 'git diff': %s", err)
	}

	comp := llame.CompletionQuery{
		Prompt:      newMistralPrompt(string(diff)),
		NPredict:    512,
		Temperature: 0.5,
	}
	llame.Debugf("Completion query: %#v", comp)

	p := tea.NewProgram(initialModel(rootCtx, model, comp))
	if _, err := p.Run(); err != nil {
		llame.Fatalf("%s", err)
	}
}

func initLogging() {
	if CLI.Log {
		var err error
		if CLI.LogDirectory != "" {
			err = llame.InitFileLogging(CLI.LogDirectory)
		} else {
			err = llame.InitFileLogging()
		}

		if err != nil {
			llame.Fatalf("Failed to init file logging: %s", err)
		}

		llame.Debugf("Connecting to %q...", CLI.ModelEndpoint)
		d, _ := json.MarshalIndent(CLI, "", "\t")
		llame.Debugf("CLI arguments: %s", d)
	} else {
		llame.InitDiscardLogging()
	}
}

func newMistralPrompt(diff string) string {
	p, ok := llame.GetPromptFormats()["llama2"]
	if !ok {
		panic("llama2 not found")
	}

	// const system = "This is a conversation between a user and a git chatbot. The chatbot is helpful, kind, honest, good at writing, and never fails to answer any requests immediately and with precision"
	const system = ""

	userContent := p.UserContent("Given the following code diff, generate a concise subject for commit message " +
		"(under 50 characters) that summarizes the change clearly and effectively:\n" + diff)

	// userMsg := p.UserMessage("Given the following code diff, generate a concise subject for commit message " +
	// 	"(under 50 characters) that summarizes the change clearly and effectively:\n" + diff)

	// return p.MustPrompt(system, userMsg)
	return userContent
}
