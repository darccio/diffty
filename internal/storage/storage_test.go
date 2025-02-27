package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darccio/diffty/internal/models"
)

func TestJSONStorage(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "diffty-test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test .diffty directory
	difftyDir := filepath.Join(tempDir, ".diffty")
	if err := os.MkdirAll(difftyDir, 0755); err != nil {
		t.Fatalf("Failed to create .diffty directory: %v", err)
	}

	// Create a test storage instance with a custom path
	storage := &JSONStorage{
		baseStoragePath: difftyDir,
		reposPath:       filepath.Join(difftyDir, "repositories.json"),
	}

	// Test SaveReviewState and LoadReviewState
	t.Run("ReviewState", func(t *testing.T) {
		// Create a test review state
		testState := &models.ReviewState{
			ReviewedFiles: []models.FileReview{
				{
					Repo: "/path/to/repo",
					Path: "test/file.go",
					Lines: map[string]string{
						"1":   models.StateApproved,
						"2":   models.StateRejected,
						"3-5": models.StateSkipped,
					},
				},
			},
			SourceBranch: "feature",
			TargetBranch: "main",
			SourceCommit: "abc123",
			TargetCommit: "def456",
		}

		// Save the test state
		if err := storage.SaveReviewState(testState, "/path/to/repo"); err != nil {
			t.Fatalf("Failed to save review state: %v", err)
		}

		// Load the test state
		loadedState, err := storage.LoadReviewState("/path/to/repo", "feature", "main", "abc123", "def456")
		if err != nil {
			t.Fatalf("Failed to load review state: %v", err)
		}

		// Verify the loaded state
		if len(loadedState.ReviewedFiles) != 1 {
			t.Fatalf("Expected 1 reviewed file, got %d", len(loadedState.ReviewedFiles))
		}

		if loadedState.ReviewedFiles[0].Repo != "/path/to/repo" {
			t.Errorf("Expected repository path to be '/path/to/repo', got '%s'", loadedState.ReviewedFiles[0].Repo)
		}

		if loadedState.ReviewedFiles[0].Path != "test/file.go" {
			t.Errorf("Expected file path to be 'test/file.go', got '%s'", loadedState.ReviewedFiles[0].Path)
		}

		if len(loadedState.ReviewedFiles[0].Lines) != 3 {
			t.Errorf("Expected 3 lines, got %d", len(loadedState.ReviewedFiles[0].Lines))
		}

		if loadedState.ReviewedFiles[0].Lines["1"] != models.StateApproved {
			t.Errorf("Expected line 1 to be '%s', got '%s'", models.StateApproved, loadedState.ReviewedFiles[0].Lines["1"])
		}

		if loadedState.ReviewedFiles[0].Lines["2"] != models.StateRejected {
			t.Errorf("Expected line 2 to be '%s', got '%s'", models.StateRejected, loadedState.ReviewedFiles[0].Lines["2"])
		}

		if loadedState.ReviewedFiles[0].Lines["3-5"] != models.StateSkipped {
			t.Errorf("Expected lines 3-5 to be '%s', got '%s'", models.StateSkipped, loadedState.ReviewedFiles[0].Lines["3-5"])
		}

		if loadedState.SourceBranch != "feature" {
			t.Errorf("Expected source branch to be 'feature', got '%s'", loadedState.SourceBranch)
		}

		if loadedState.TargetBranch != "main" {
			t.Errorf("Expected target branch to be 'main', got '%s'", loadedState.TargetBranch)
		}

		if loadedState.SourceCommit != "abc123" {
			t.Errorf("Expected source commit to be 'abc123', got '%s'", loadedState.SourceCommit)
		}

		if loadedState.TargetCommit != "def456" {
			t.Errorf("Expected target commit to be 'def456', got '%s'", loadedState.TargetCommit)
		}
	})

	// Test LoadReviewState with missing file
	t.Run("LoadMissingReviewState", func(t *testing.T) {
		// Load a non-existent review state
		loadedState, err := storage.LoadReviewState("/nonexistent/repo", "feature", "main", "abc123", "def456")
		if err != nil {
			t.Fatalf("Failed to load non-existent review state: %v", err)
		}

		// Verify we get an empty review state
		if len(loadedState.ReviewedFiles) != 0 {
			t.Errorf("Expected 0 reviewed files, got %d", len(loadedState.ReviewedFiles))
		}

		if loadedState.SourceBranch != "feature" {
			t.Errorf("Expected source branch to be 'feature', got '%s'", loadedState.SourceBranch)
		}

		if loadedState.TargetBranch != "main" {
			t.Errorf("Expected target branch to be 'main', got '%s'", loadedState.TargetBranch)
		}

		if loadedState.SourceCommit != "abc123" {
			t.Errorf("Expected source commit to be 'abc123', got '%s'", loadedState.SourceCommit)
		}

		if loadedState.TargetCommit != "def456" {
			t.Errorf("Expected target commit to be 'def456', got '%s'", loadedState.TargetCommit)
		}
	})

	// Test SaveReviewState with missing commit hashes
	t.Run("MissingCommitHashes", func(t *testing.T) {
		testState := &models.ReviewState{
			ReviewedFiles: []models.FileReview{
				{
					Repo: "/path/to/repo",
					Path: "test/file.go",
					Lines: map[string]string{
						"1": models.StateApproved,
					},
				},
			},
			SourceBranch: "feature",
			TargetBranch: "main",
			// Missing commit hashes
		}

		err := storage.SaveReviewState(testState, "/path/to/repo")
		if err == nil {
			t.Errorf("Expected error for missing commit hashes, got nil")
		}
	})

	// Test SaveRepositories and LoadRepositories
	t.Run("Repositories", func(t *testing.T) {
		// Save repositories
		testRepos := []string{"/path/to/repo1", "/path/to/repo2"}
		if err := storage.SaveRepositories(testRepos); err != nil {
			t.Fatalf("Failed to save repositories: %v", err)
		}

		// Load repositories
		loadedRepos, err := storage.LoadRepositories()
		if err != nil {
			t.Fatalf("Failed to load repositories: %v", err)
		}

		// Verify loaded repositories
		if len(loadedRepos) != 2 {
			t.Fatalf("Expected 2 repositories, got %d", len(loadedRepos))
		}

		if loadedRepos[0] != "/path/to/repo1" {
			t.Errorf("Expected repository 1 to be '/path/to/repo1', got '%s'", loadedRepos[0])
		}

		if loadedRepos[1] != "/path/to/repo2" {
			t.Errorf("Expected repository 2 to be '/path/to/repo2', got '%s'", loadedRepos[1])
		}
	})

	// Test LoadRepositories with no file
	t.Run("LoadEmptyRepositories", func(t *testing.T) {
		// Create a new storage instance with a different path
		emptyStorage := &JSONStorage{
			baseStoragePath: difftyDir,
			reposPath:       filepath.Join(difftyDir, "nonexistent.json"),
		}

		// Load repositories
		loadedRepos, err := emptyStorage.LoadRepositories()
		if err != nil {
			t.Fatalf("Failed to load repositories: %v", err)
		}

		// Verify we get an empty slice
		if len(loadedRepos) != 0 {
			t.Errorf("Expected 0 repositories, got %d", len(loadedRepos))
		}
	})
}

func TestNewJSONStorage(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "diffty-test-home")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set temporary home directory
	t.Setenv("HOME", tempDir)

	// Create new storage
	storage, err := NewJSONStorage()
	if err != nil {
		t.Fatalf("Failed to create JSON storage: %v", err)
	}

	// Verify storage creation
	if storage == nil {
		t.Fatal("Storage should not be nil")
	}

	// Verify .diffty directory was created
	difftyPath := filepath.Join(tempDir, ".diffty")
	if _, err := os.Stat(difftyPath); os.IsNotExist(err) {
		t.Errorf(".diffty directory was not created")
	}

	// Verify repositories path
	expectedReposPath := filepath.Join(difftyPath, "repositories.json")
	if storage.reposPath != expectedReposPath {
		t.Errorf("Expected reposPath to be '%s', got '%s'", expectedReposPath, storage.reposPath)
	}
}
