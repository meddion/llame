package llame

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/go-git/go-git/v5"
)

// The 50/72 rule is a guideline for writing clear and concise Git commit messages:
const (
	GitCommitSubjectCharsMin = 50
	GitCommiBodyCharsMax     = 72
)

var NoStagedFilesErr = errors.New("no staged files")

// GitDiffStaged gets a diff for all staged files (if only called with context) or for the specified ones.
func GitDiffStaged(ctx context.Context, files ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"diff", "--staged", "HEAD"}, files...)...)
	var changes bytes.Buffer
	cmd.Stdout = &changes
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	diffs := changes.Bytes()
	if len(diffs) == 0 {
		return nil, NoStagedFilesErr
	}

	return diffs, nil
}

type GitFiles struct {
	Tracked   []string
	Untracked []string
}

func FilesInCommit() (*GitFiles, error) {
	repo, err := NewGitRepo()
	if err != nil {
		return nil, err
	}

	return stagedFiles(repo)
}

func NewGitRepo() (*git.Repository, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	Debugf("Opening git repo at %s", wd)

	repo, err := git.PlainOpenWithOptions(wd, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func MustGitStatus() string {
	repo, err := NewGitRepo()
	if err != nil {
		panic(err)
	}

	workTree, err := repo.Worktree()
	if err != nil {
		panic(err)
	}

	status, err := workTree.Status()
	if err != nil {
		panic(err)
	}

	return status.String()
}

func stagedFiles(repo *git.Repository) (*GitFiles, error) {
	workTree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := workTree.Status()
	if err != nil {
		return nil, err
	}

	untrackedFiles := make([]string, 0, len(status)/2)
	trackedFiles := make([]string, 0, len(status))

	for fileName, st := range status {
		switch st.Staging {
		case git.Untracked:
			untrackedFiles = append(untrackedFiles, fileName)
			continue
		}

		trackedFiles = append(trackedFiles, fileName)
	}

	return &GitFiles{
		Tracked:   trackedFiles,
		Untracked: untrackedFiles,
	}, nil
}

func GitCommit(commitMsg string) error {
	repo, err := NewGitRepo()
	if err != nil {
		return err
	}

	workTree, err := repo.Worktree()
	if err != nil {
		return err
	}

	// TODO: perhaps add some commit options
	_, err = workTree.Commit(commitMsg, &git.CommitOptions{})

	return err
}

// TODO: rm
func logCommitFiles() {
	gitFiles, err := FilesInCommit()
	if err != nil {
		Debugf("%s", err)
		Fatalf("Failed to get files staged for commit")
	}

	sort.Strings(gitFiles.Tracked)
	for i, file := range gitFiles.Tracked {
		if i == 0 {
			fmt.Printf("Files to be commited:\n")
		}

		fmt.Printf("\t%s\n", file)
	}

	sort.Strings(gitFiles.Untracked)
	for i, file := range gitFiles.Untracked {
		if i == 0 {
			fmt.Printf("Untracked:\n")
		}
		fmt.Printf("\t%s\n", file)
	}
}
