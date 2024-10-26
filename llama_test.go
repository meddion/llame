package llame_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	llame "github.com/meddion/llame"
	"github.com/stretchr/testify/require"
)

func TestCommitGen(t *testing.T) {
	diff := `
[main (root-commit) 2c5505c] init
 3 files changed, 377 insertions(+)
 create mode 100644 go.mod
        }

-       Debugf("tree1: %s", tree1)
+       tree0, err := latestCommit.Tree()
+       if err != nil {
+               panic(err)
+       }
+       Debugf("tree0: %s", tree0)
+
+       // tree0.Diff(
+       tree
+
+       // tree1, err := object.GetTree(repo.Storer, head.Hash())
+       // if err != nil {
+       //      panic(err)
+       // }
+
+       // Debugf("tree1: %s", tree1)

        // object.DiffTree()

@@ -93,6 +107,8 @@ func main() {
                panic(err)
        }

+       object.DecodeTree(repo.Storer, workTree)
+
        status, err := workTree.Status()
        if err != nil {
                panic(err)
`

	//TODO: make an env out of this
	const modelURL = "http://127.0.0.1:8080/completion"

	llama := llame.NewLlamaModel(modelURL, 20*time.Second)

	prompt1 := fmt.Sprintf(`%s
			Please generate a short git commit message by summarizing the git diff above. 
			Keep the commit under %d char long.`, diff, llame.RecommendedCommitCharLen)
	prompt2 := diff + "\nSummarize this git diff above into a useful, 10 words commit message."
	prompt3 := diff + "\nFor the git diff above, I want you to act as the author of a commit message in git." +
		`I'll enter a git diff, and your job is to convert it into a useful commit message in English` +
		`and make 2 options that are separated by ";".` +
		"For each option, use the present tense, return the full sentence, and use the conventional commits specification (<type in lowercase>: <subject>)"

	prompts := []string{prompt1, prompt2, prompt3}

	var wg sync.WaitGroup
	wg.Add(len(prompts))

	for _, prompt := range prompts {
		go func() {
			defer wg.Done()

			completion := llame.CompletionQuery{
				Prompt:      prompt,
				Temperature: 0.2,
				NPredict:    32,
			}
			stream, err := llama.ReadStream(context.TODO(), completion)
			require.NoError(t, err)

			var allContent string
			for resp := range stream {
				require.NoError(t, resp.Error)
				allContent += resp.Content
			}

			require.NotEmpty(t, allContent)

			t.Logf("Prompt: %s\nResponse:%s\n", prompt, allContent)
		}()
	}

	wg.Wait()
}
