package llame

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"text/template"
)

//go:embed prompt-formats.json
var promptFormatsContent []byte

var promptFormats PromptFormats

func init() {
	if len(promptFormatsContent) == 0 {
		panic("promptFormatsContent can't be empty")
	}

	err := json.Unmarshal(promptFormatsContent, &promptFormats)
	if err != nil {
		panic(fmt.Sprintf("failed to parse promptFormats: %s", err))
	}

	if len(promptFormats) == 0 {
		panic("promptFormats can't be empty")
	}
}

func GetPromptFormats() PromptFormats {
	// TODO: deep copy it
	return promptFormats
}

type PromptFormats map[string]PromptFormat

type PromptFormat struct {
	Template        string `json:"template"`
	HistoryTemplate string `json:"historyTemplate"`
	Char            string `json:"char"`
	CharMsgPrefix   string `json:"charMsgPrefix"`
	CharMsgSuffix   string `json:"charMsgSuffix"`
	User            string `json:"user"`
	UserMsgPrefix   string `json:"userMsgPrefix"`
	UserMsgSuffix   string `json:"userMsgSuffix"`
	Stops           string `json:"stops"`
}

type TextMessage struct {
	Name    string // User or Assistant
	Message string
}

func (p PromptFormat) UserContent(content string) string {
	return p.UserMsgPrefix + content + p.UserMsgSuffix
}

func (p PromptFormat) CharContent(content string) string {
	return p.CharMsgPrefix + content + p.CharMsgSuffix
}

func (p PromptFormat) UserMessage(content string) TextMessage {
	return TextMessage{
		Name:    p.User,
		Message: p.UserContent(content),
	}
}

func (p PromptFormat) CharMessage(content string) TextMessage {
	return TextMessage{
		Name:    p.Char,
		Message: p.CharContent(content),
	}
}

func (p PromptFormat) History(textMsgs []TextMessage) (string, error) {
	tmpl, err := template.New("history").Parse(p.HistoryTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing history template: %w", err)
	}

	buf := new(bytes.Buffer)
	for _, textMsg := range textMsgs {
		err := tmpl.Execute(buf, textMsg)
		if err != nil {
			return "", fmt.Errorf("executing history template with %v message: %w", textMsg, err)
		}
	}

	return buf.String(), nil
}

func (p PromptFormat) Prompt(system string, textMsgs ...TextMessage) (string, error) {
	type promptArgs struct {
		Prompt  string // System prompt
		Char    string // Char - name to refer to the assistant
		User    string // User name
		History string // Message history for both User and Char
	}

	history, err := p.History(textMsgs)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("template").Parse(p.Template)
	if err != nil {
		return "", fmt.Errorf("parsing prompt template: %w", err)
	}

	buf := new(bytes.Buffer)
	args := promptArgs{
		Prompt:  system,
		Char:    p.Char,
		User:    p.User,
		History: history,
	}
	err = tmpl.Execute(buf, args)
	if err != nil {
		return "", fmt.Errorf("executing prompt template with %v: %w", args, err)
	}

	return buf.String(), nil
}

func (p PromptFormat) MustPrompt(system string, textMsgs ...TextMessage) string {
	prompt, err := p.Prompt(system, textMsgs...)
	if err != nil {
		panic(err)
	}

	return prompt
}

var modelToPromptFormat = map[string]string{
	"Alpaca":            "alpaca",
	"ChatML":            "chatml",
	"Command R/+":       "commandr",
	"Llama 2":           "llama2",
	"Llama 3":           "llama3",
	"Phi-3":             "phi3",
	"OpenChat/Starling": "openchat",
	"Vicuna":            "vicuna",
	// More Prompt-Styles
	"Airoboros L2":           "vicuna",
	"BakLLaVA-1":             "vicuna",
	"Code Cherry Pop":        "alpaca",
	"Deepseek Coder":         "deepseekCoder",
	"Dolphin Mistral":        "chatml",
	"evolvedSeeker 1.3B":     "chatml",
	"Goliath 120B":           "vicuna",
	"Jordan":                 "vicuna",
	"LLaVA":                  "vicuna",
	"Leo Hessianai":          "chatml",
	"Leo Mistral":            "vicuna",
	"Marx":                   "vicuna",
	"Med42":                  "med42",
	"MetaMath":               "alpaca",
	"Mistral Instruct":       "llama2",
	"Mistral 7B OpenOrca":    "chatml",
	"MythoMax":               "alpaca",
	"Neural Chat":            "neuralchat",
	"Nous Capybara":          "vicuna",
	"Nous Hermes":            "nousHermes",
	"OpenChat Math":          "openchatMath",
	"OpenHermes 2.5-Mistral": "chatml",
	"Orca Mini v3":           "alpaca",
	"Orion":                  "orion",
	"Samantha":               "vicuna",
	"Samantha Mistral":       "chatml",
	"SauerkrautLM":           "sauerkraut",
	"Scarlett":               "vicuna",
	"Starling Coding":        "starlingCode",
	"Sydney":                 "alpaca",
	"Synthia":                "vicuna",
	"Tess":                   "vicuna",
	"Yi-6/9/34B-Chat":        "yi34b",
	"Zephyr":                 "zephyr",
}
