package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repository represents a git repository
type Repository struct {
	Name string
	Path string
}

// IsValidRepo checks if the given path is a valid git repository
func IsValidRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	_, err := os.Stat(gitPath)
	return err == nil
}

// NewRepository creates a new Repository instance
func NewRepository(path string) *Repository {
	return &Repository{
		Name: filepath.Base(path),
		Path: path,
	}
}

// GetBranches returns a list of all branches in the repository
func (r *Repository) GetBranches() ([]string, error) {
	cmd := exec.Command("git", "-C", r.Path, "branch", "--format=%(refname:short)")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	branches := strings.Split(strings.TrimSpace(out.String()), "\n")
	return branches, nil
}

// GetBranchCommitHash returns the commit hash for a branch
func (r *Repository) GetBranchCommitHash(branch string) (string, error) {
	cmd := exec.Command("git", "-C", r.Path, "rev-parse", branch)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash for branch %s: %w", branch, err)
	}

	return strings.TrimSpace(out.String()), nil
}

// GetDiff returns the diff between two branches
// targetBranch is the base branch (what we're merging INTO, e.g. main)
// sourceBranch is the feature branch (what we're merging FROM, e.g. feature-branch)
func (r *Repository) GetDiff(sourceBranch, targetBranch string) (string, error) {
	cmd := exec.Command("git", "-C", r.Path, "diff", "--no-color", targetBranch, sourceBranch)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return out.String(), nil
}

// GetFileDiff returns the diff for a specific file between two branches
// targetBranch is the base branch (what we're merging INTO, e.g. main)
// sourceBranch is the feature branch (what we're merging FROM, e.g. feature-branch)
func (r *Repository) GetFileDiff(sourceBranch, targetBranch, filePath string) (string, error) {
	cmd := exec.Command("git", "-C", r.Path, "diff", "--no-color", targetBranch, sourceBranch, "--", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get file diff: %w", err)
	}

	return out.String(), nil
}

// GetFiles returns a list of files that have changed between two branches
// targetBranch is the base branch (what we're merging INTO, e.g. main)
// sourceBranch is the feature branch (what we're merging FROM, e.g. feature-branch)
func (r *Repository) GetFiles(sourceBranch, targetBranch string) ([]string, error) {
	cmd := exec.Command("git", "-C", r.Path, "diff", "--name-only", targetBranch, sourceBranch)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(out.String()), "\n")
	// Handle empty diff case
	if len(files) == 1 && files[0] == "" {
		return []string{}, nil
	}
	return files, nil
}
