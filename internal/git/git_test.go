package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary Git repository for testing
func setupTestRepo(t *testing.T) string {
	t.Helper()

	// Create a temporary directory for the test repository
	tempDir, err := os.MkdirTemp("", "diffty-git-test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "-C", tempDir, "init")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Disable GPG signing for commits
	cmd = exec.Command("git", "-C", tempDir, "config", "--local", "commit.gpgsign", "false")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to configure git user.name: %v", err)
	}

	// Create a test file and commit it to main branch
	testFilePath := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFilePath, []byte("initial content"), 0644); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Add and commit the file
	cmd = exec.Command("git", "-C", tempDir, "add", "test.txt")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to add test file: %v", err)
	}

	cmd = exec.Command("git", "-C", tempDir, "commit", "-m", "Initial commit")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to commit test file: %v", err)
	}

	// Create a feature branch
	cmd = exec.Command("git", "-C", tempDir, "checkout", "-b", "feature")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create feature branch: %v", err)
	}

	// Modify the test file in the feature branch
	if err := os.WriteFile(testFilePath, []byte("initial content\nnew line"), 0644); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Add and commit the modified file
	cmd = exec.Command("git", "-C", tempDir, "add", "test.txt")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to add modified test file: %v", err)
	}

	cmd = exec.Command("git", "-C", tempDir, "commit", "-m", "Add new line")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to commit modified test file: %v", err)
	}

	// Switch back to main branch
	cmd = exec.Command("git", "-C", tempDir, "checkout", "main")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to switch back to main branch: %v", err)
	}

	return tempDir
}

func TestIsValidRepo(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "diffty-test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with a non-git directory
	if IsValidRepo(tempDir) {
		t.Errorf("Expected non-git directory to return false, got true")
	}

	// Create a fake .git directory
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create fake .git directory: %v", err)
	}

	// Test with a directory containing a .git subdirectory
	if !IsValidRepo(tempDir) {
		t.Errorf("Expected directory with .git subdirectory to return true, got false")
	}
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository("/path/to/repo")
	if repo.Path != "/path/to/repo" {
		t.Errorf("Expected repository path to be '/path/to/repo', got '%s'", repo.Path)
	}

	if repo.Name != "repo" {
		t.Errorf("Expected repository name to be 'repo', got '%s'", repo.Name)
	}
}

func TestGetBranches(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git command not available, skipping test")
	}

	// Create a test repository
	repoDir := setupTestRepo(t)
	defer os.RemoveAll(repoDir)

	// Create repository instance
	repo := NewRepository(repoDir)

	// Get branches
	branches, err := repo.GetBranches()
	if err != nil {
		t.Fatalf("GetBranches failed: %v", err)
	}

	// Verify branches
	expectedBranches := map[string]bool{
		"main":    true,
		"feature": true,
	}

	if len(branches) != 2 {
		t.Errorf("Expected 2 branches, got %d: %v", len(branches), branches)
	}

	for _, branch := range branches {
		if !expectedBranches[branch] {
			t.Errorf("Unexpected branch: %s", branch)
		}
	}
}

func TestGetBranchCommitHash(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git command not available, skipping test")
	}

	// Create a test repository
	repoDir := setupTestRepo(t)
	defer os.RemoveAll(repoDir)

	// Create repository instance
	repo := NewRepository(repoDir)

	// Get commit hash for main branch
	mainHash, err := repo.GetBranchCommitHash("main")
	if err != nil {
		t.Fatalf("GetBranchCommitHash for main failed: %v", err)
	}

	// Verify hash format
	if len(mainHash) != 40 || !isHexString(mainHash) {
		t.Errorf("Invalid commit hash format for main: %s", mainHash)
	}

	// Get commit hash for feature branch
	featureHash, err := repo.GetBranchCommitHash("feature")
	if err != nil {
		t.Fatalf("GetBranchCommitHash for feature failed: %v", err)
	}

	// Verify hash format
	if len(featureHash) != 40 || !isHexString(featureHash) {
		t.Errorf("Invalid commit hash format for feature: %s", featureHash)
	}

	// Hashes should be different
	if mainHash == featureHash {
		t.Errorf("Expected different commit hashes for main and feature, got same: %s", mainHash)
	}

	// Test with non-existent branch
	_, err = repo.GetBranchCommitHash("nonexistent")
	if err == nil {
		t.Errorf("Expected error for non-existent branch, got nil")
	}
}

func TestGetDiff(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git command not available, skipping test")
	}

	// Create a test repository
	repoDir := setupTestRepo(t)
	defer os.RemoveAll(repoDir)

	// Create repository instance
	repo := NewRepository(repoDir)

	// Get diff between main and feature
	diff, err := repo.GetDiff("feature", "main")
	if err != nil {
		t.Fatalf("GetDiff failed: %v", err)
	}

	// Verify diff contains expected content
	expectedParts := []string{
		"diff --git",
		"test.txt",
		"+new line",
	}

	for _, part := range expectedParts {
		if !strings.Contains(diff, part) {
			t.Errorf("Expected diff to contain '%s', but it doesn't.\nDiff: %s", part, diff)
		}
	}

	// Test with non-existent branch
	_, err = repo.GetDiff("nonexistent", "main")
	if err == nil {
		t.Errorf("Expected error for non-existent branch, got nil")
	}
}

func TestGetFileDiff(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git command not available, skipping test")
	}

	// Create a test repository
	repoDir := setupTestRepo(t)
	defer os.RemoveAll(repoDir)

	// Create repository instance
	repo := NewRepository(repoDir)

	// Get diff for specific file
	diff, err := repo.GetFileDiff("feature", "main", "test.txt")
	if err != nil {
		t.Fatalf("GetFileDiff failed: %v", err)
	}

	// Verify diff contains expected content
	expectedParts := []string{
		"diff --git",
		"test.txt",
		"+new line",
	}

	for _, part := range expectedParts {
		if !strings.Contains(diff, part) {
			t.Errorf("Expected diff to contain '%s', but it doesn't.\nDiff: %s", part, diff)
		}
	}

	// Test with non-existent file
	diff, err = repo.GetFileDiff("feature", "main", "nonexistent.txt")
	if err != nil {
		t.Fatalf("GetFileDiff for non-existent file failed: %v", err)
	}

	if diff != "" {
		t.Errorf("Expected empty diff for non-existent file, got: %s", diff)
	}
}

func TestGetFiles(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git command not available, skipping test")
	}

	// Create a test repository
	repoDir := setupTestRepo(t)
	defer os.RemoveAll(repoDir)

	// Create repository instance
	repo := NewRepository(repoDir)

	// Get files changed between main and feature
	files, err := repo.GetFiles("feature", "main")
	if err != nil {
		t.Fatalf("GetFiles failed: %v", err)
	}

	// Verify files list
	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d: %v", len(files), files)
	}

	if len(files) > 0 && files[0] != "test.txt" {
		t.Errorf("Expected 'test.txt', got '%s'", files[0])
	}

	// Test with non-existent branch
	_, err = repo.GetFiles("nonexistent", "main")
	if err == nil {
		t.Errorf("Expected error for non-existent branch, got nil")
	}
}

// Helper function to check if a string is a valid hex string
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
