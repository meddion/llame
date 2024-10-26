package llame

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-errors/errors"
)

//TODO: improve log formatting

const DebugLogFileName = "llame_debug.log"

var DefaultDebugDirs = []string{"/tmp/", "/tmp/var/"}

func InitDiscardLogging() {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func InitFileLogging(dirs ...string) error {
	if len(dirs) == 0 {
		dirs = DefaultDebugDirs
	}

	var logFile *os.File
	for i, dir := range dirs {
		var err error
		logFile, err = tea.LogToFile(dir+DebugLogFileName, "debug")
		if err != nil {
			if os.IsPermission(err) && i != len(dirs)-1 {
				continue
			}

			return err
		}
		break
	}

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: false,
	}))
	slog.SetDefault(logger)

	return nil
}

func Printf(msg string, args ...any) {
	if len(args) > 0 {
		fmt.Printf(msg, args...)
	} else {
		fmt.Print(msg)
	}
}

func Fatalf(msg string, args ...any) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, msg, args...)
	} else {
		fmt.Fprint(os.Stderr, msg)
	}

	fmt.Fprint(os.Stderr, "\n")

	os.Exit(1)
}

var (
	Errorf = logFuncWithStack(slog.Error)
	Debugf = logFuncWithStack(slog.Debug)
)

type logFunc func(msg string, args ...any)

func logFuncWithStack(f logFunc) logFunc {
	return func(msg string, args ...any) {
		for i, arg := range args {
			if err, ok := arg.(error); ok {
				args[i] = errors.Wrap(err, 1).ErrorStack()
			}
		}

		if len(args) > 0 {
			f(fmt.Sprintf(msg, args...))
		} else {
			f(msg)
		}
	}
}
