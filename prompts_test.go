package llame

import (
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestPromptTemplates(t *testing.T) {
	promptFormats := GetPromptFormats()

	t.Run("consistency", func(t *testing.T) {
		for _, v := range modelToPromptFormat {
			_, ok := promptFormats[v]
			assert.True(t, ok, v)
		}

		for model, prompt := range promptFormats {
			tmpl, err := template.New("template").Parse(prompt.Template)
			assert.NoError(t, err, model)
			assert.NotNil(t, tmpl, model)
			histTmpl, err := template.New("history").Parse(prompt.HistoryTemplate)
			assert.NoError(t, err, model)
			assert.NotNil(t, histTmpl, model)
		}
	})

	const system = "This is a conversation between a user and a friendly chatbot. The chatbot is helpful, kind, honest, good at writing, and never fails to answer any requests immediately and with precision"

	t.Run("llama2", func(t *testing.T) {
		p := promptFormats["llama2"]
		userMsg := p.UserMessage("Hello to you!")
		charMsg := p.CharMessage("Hello friend :)")

		prompt := p.MustPrompt(system, userMsg, charMsg)
		assert.Equal(t, "<s>[INST] <<SYS>>\nThis is a conversation between a user and a friendly chatbot. The chatbot is helpful, kind, honest, good at writing, and never fails to answer any requests immediately and with precision\n<</SYS>>\n\nTest Message [/INST] Test Successfull </s>User: <s>[INST] Hello to you! [/INST]Assistant: Hello friend :)</s>Assistant", prompt)
	})

	t.Run("model prompts", func(t *testing.T) {
		for model, p := range promptFormats {
			userMsg := p.UserMessage("Hello to you!")
			charMsg := p.CharMessage("Hello friend :)")
			prompt, err := p.Prompt(system, userMsg, charMsg)
			assert.NoError(t, err, model)
			assert.NotEmpty(t, prompt, model)
		}
	})
}
