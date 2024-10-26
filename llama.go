package llame

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const RecommendedCommitCharLen = 50

// CompletionQuery is a subset of all allowed options of llama-server /completion.
// Compatipable with https://github.com/openai/openai-openapi/blob/master/openapi.yaml
type CompletionQuery struct {
	Prompt      string  `json:"prompt"`
	Temperature float32 `json:"float,omitempty"`
	NPredict    int     `json:"n_predict,omitempty"` // Default: `-1`, where `-1` is infinity. Set the maximum number of tokens to predict when generating text.
	Stream      bool    `json:"stream,omitempty"`
}

type StreamData struct {
	Content    string `json:"content"`    // The content being streamed
	Stop       bool   `json:"stop"`       // Indicates whether the stream should stop
	IdSlot     int    `json:"id_slot"`    // Slot to which the task is assigned
	Multimodal bool   `json:"multimodal"` // Indicates if the response is multimodal
	Index      int    `json:"index"`      // Index of the response in the stream
}

type StreamResponse struct {
	StreamData
	Error error
}

type LlamaModel struct {
	url            string
	client         *http.Client
	RequestTimeout time.Duration
}

func NewLlamaModel(url string, reqTimeout time.Duration) *LlamaModel {
	return &LlamaModel{
		url: url,
		client: &http.Client{
			Timeout: reqTimeout,
		},
		RequestTimeout: reqTimeout,
	}
}

const streamCapacity = 10

func (this *LlamaModel) ReadStream(ctx context.Context, completion CompletionQuery) (<-chan StreamResponse, error) {
	completion.Stream = true

	payload, err := json.Marshal(completion)
	if err != nil {
		return nil, fmt.Errorf("marshal LLM query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", this.url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := this.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("bad response status: %d", resp.StatusCode)
	}

	outCh := make(chan StreamResponse, streamCapacity)

	send := func(streamResp StreamResponse) bool {
		select {
		case <-ctx.Done():
			outCh <- StreamResponse{Error: ctx.Err()}
			return false
		default:
		}

		select {
		case <-ctx.Done():
			outCh <- StreamResponse{Error: ctx.Err()}
			return false
		case outCh <- streamResp:
		}

		return streamResp.Error == nil
	}

	go func() {
		defer close(outCh)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		scanner.Split(bufio.ScanLines)

		const prefixLen = len("data: ")

		var (
			streamResp StreamResponse
			streamData StreamData
		)
		for scanner.Scan() {
			data := scanner.Bytes()
			if len(data) > prefixLen {
				err := json.Unmarshal(data[prefixLen:], &streamData)
				if err != nil {
					streamResp = StreamResponse{Error: fmt.Errorf("unmarshal error: %w", err)}
				} else {
					streamResp = StreamResponse{StreamData: streamData}
				}

				if !send(streamResp) {
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			_ = send(StreamResponse{Error: fmt.Errorf("scanner error: %w", err)})
		}
	}()

	return outCh, nil
}
