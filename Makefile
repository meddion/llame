TOPD := $(if $(TOPD),$(TOPD),$(shell git rev-parse --show-toplevel))

mistral:
	llama-server -m ~/models/Mistral-7B-v0.1/Mistral-7B-Instruct-v0.3.fp16.gguf


llama:
	llama-server -m ~/models/Llama-3.2-3B-Instruct/Llama-3.2-3B-Instruct-IQ3_M.gguf

gemma:
	llama-server -m ~/models/Gemma-7B/gemma-7b-it.Q8_0-v2.gguf

run:
	go run cmd/*.go

build:
	go build -o llame cmd/*.go

