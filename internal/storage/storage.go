package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darccio/diffty/internal/models"
)

// Storage interface defines methods for persisting and retrieving data
type Storage interface {
	SaveReviewState(state *models.ReviewState, repoPath string) error
	LoadReviewState(repoPath, sourceBranch, targetBranch, sourceCommit, targetCommit string) (*models.ReviewState, error)
	SaveRepositories(repos []string) error
	LoadRepositories() ([]string, error)
}

// JSONStorage implements Storage using JSON files
type JSONStorage struct {
	baseStoragePath string
	reposPath       string
}

// NewJSONStorage creates a new JSONStorage instance
func NewJSONStorage() (*JSONStorage, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Ensure .diffty directory exists
	storageDir := filepath.Join(homeDir, ".diffty")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &JSONStorage{
		baseStoragePath: storageDir,
		reposPath:       filepath.Join(storageDir, "repositories.json"),
	}, nil
}

// getReviewStatePath returns the path to the review state file
func (s *JSONStorage) getReviewStatePath(repoPath, sourceCommit, targetCommit string) string {
	// Create a safe repository path by replacing special characters
	safeRepoPath := strings.ReplaceAll(repoPath, string(os.PathSeparator), "_")
	safeRepoPath = strings.ReplaceAll(safeRepoPath, ":", "_")

	// Create directory structure: .diffty/repository/first-branch-commit-hash/second-branch-commit-hash
	reviewDir := filepath.Join(s.baseStoragePath, safeRepoPath, sourceCommit, targetCommit)

	// Ensure the directory exists
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		// Just log error, don't fail
		fmt.Printf("Warning: failed to create review directory: %v\n", err)
	}

	return filepath.Join(reviewDir, "review-state.json")
}

// SaveReviewState saves the review state to a JSON file
func (s *JSONStorage) SaveReviewState(state *models.ReviewState, repoPath string) error {
	if state.SourceCommit == "" || state.TargetCommit == "" {
		return fmt.Errorf("source and target commit hashes are required")
	}

	storagePath := s.getReviewStatePath(repoPath, state.SourceCommit, state.TargetCommit)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal review state: %w", err)
	}

	if err := os.WriteFile(storagePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write review state: %w", err)
	}

	return nil
}

// LoadReviewState loads the review state from a JSON file
func (s *JSONStorage) LoadReviewState(repoPath, sourceBranch, targetBranch, sourceCommit, targetCommit string) (*models.ReviewState, error) {
	if sourceCommit == "" || targetCommit == "" {
		return &models.ReviewState{
			ReviewedFiles: []models.FileReview{},
			SourceBranch:  sourceBranch,
			TargetBranch:  targetBranch,
			SourceCommit:  sourceCommit,
			TargetCommit:  targetCommit,
		}, nil
	}

	storagePath := s.getReviewStatePath(repoPath, sourceCommit, targetCommit)

	// Check if the file exists
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		// Return empty state if file doesn't exist
		return &models.ReviewState{
			ReviewedFiles: []models.FileReview{},
			SourceBranch:  sourceBranch,
			TargetBranch:  targetBranch,
			SourceCommit:  sourceCommit,
			TargetCommit:  targetCommit,
		}, nil
	}

	data, err := os.ReadFile(storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read review state: %w", err)
	}

	var state models.ReviewState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal review state: %w", err)
	}

	return &state, nil
}

// SaveRepositories saves the repository paths to a JSON file
func (s *JSONStorage) SaveRepositories(repos []string) error {
	data, err := json.MarshalIndent(repos, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal repositories: %w", err)
	}

	if err := os.WriteFile(s.reposPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write repositories: %w", err)
	}

	return nil
}

// LoadRepositories loads the repository paths from a JSON file
func (s *JSONStorage) LoadRepositories() ([]string, error) {
	// Check if the file exists
	if _, err := os.Stat(s.reposPath); os.IsNotExist(err) {
		// Return empty slice if file doesn't exist
		return []string{}, nil
	}

	data, err := os.ReadFile(s.reposPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read repositories: %w", err)
	}

	var repos []string
	if err := json.Unmarshal(data, &repos); err != nil {
		return nil, fmt.Errorf("failed to unmarshal repositories: %w", err)
	}

	return repos, nil
}
