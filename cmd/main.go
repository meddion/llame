package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
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
	ModelEndpoint *url.URL      `type:"url" short:"e" env:"MODEL_ENDPOINT" default:"http://127.0.0.1:8080/completion" help:"URL to access a LLM."`
	Timeout       time.Duration `default:"15s" short:"t" help:"Duration for which the model should respond with results."`
	ModelType     string        `default:"mistral" short:"m" enum:"mistral,alpaca,chatml,commandr,llama2,llama3,openchat,phi3,vicuna,deepseekCoder,med42,neuralchat,nousHermes,openchatMath,orion,sauerkraut,starlingCode,yi34b,zephyr"`
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
	validateFlags(kongCtx)

	_, err := llame.NewGitRepo()
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			llame.Fatalf("Repository not found: make sure you are running this command inside a git repository.")
		}

		llame.Fatalf("Failed to open git repository: %s", err)
	}

	model := llame.NewLlamaCppModel(CLI.ModelEndpoint.String(), CLI.Timeout)

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
		Prompt:      newOneshotPrompt(CLI.ModelType, string(diff)),
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

func newOneshotPrompt(modelType, diff string) string {
	p, ok := llame.GetPromptFormats()[modelType]
	if !ok {
		panic(fmt.Errorf("model of type '%s' not found", modelType))
	}

	userContent := p.UserContent("Given the following code diff, generate a concise subject for commit message " +
		"(under 50 characters) that summarizes the change clearly and effectively:\n" + diff)

	return userContent
}

func validateFlags(kongCtx *kong.Context) {
	if kongCtx.Command() != "" {
		llame.Fatalf("unknown command: %s", kongCtx.Command())
	}

	var enumModelTypes []string
	for _, flag := range kongCtx.Flags() {
		if flag.Name == "model-type" {
			enumModelTypes = strings.Split(flag.Enum, ",")
			slices.Sort(enumModelTypes)
		}
	}

	supportedModelTypes := slices.Sorted(maps.Keys(llame.GetPromptFormats()))

	if len(enumModelTypes) != len(supportedModelTypes) {
		panic("len(enumModelTypes) != len(supportedModelTypes)")
	}

	for i := range enumModelTypes {
		if enumModelTypes[i] != supportedModelTypes[i] {
			panic(fmt.Errorf("at index %d enumModelTypes = %s, but upportedModelTypes = %s",
				i, enumModelTypes[i], supportedModelTypes[i]))
		}
	}
}
