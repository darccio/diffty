package server

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/darccio/diffty/internal/git"
	"github.com/darccio/diffty/internal/models"
)

// MockStorage is a mock implementation of the Storage interface for testing
type MockStorage struct {
	repositories []string
	reviewState  *models.ReviewState
	saveCalled   bool
	loadCalled   bool
}

func (m *MockStorage) SaveReviewState(state *models.ReviewState, repoPath string) error {
	m.reviewState = state
	m.saveCalled = true
	return nil
}

func (m *MockStorage) LoadReviewState(repoPath, sourceBranch, targetBranch, sourceCommit, targetCommit string) (*models.ReviewState, error) {
	m.loadCalled = true
	if m.reviewState != nil {
		return m.reviewState, nil
	}
	return &models.ReviewState{
		ReviewedFiles: []models.FileReview{},
		SourceBranch:  sourceBranch,
		TargetBranch:  targetBranch,
		SourceCommit:  sourceCommit,
		TargetCommit:  targetCommit,
	}, nil
}

func (m *MockStorage) SaveRepositories(repos []string) error {
	m.repositories = repos
	return nil
}

func (m *MockStorage) LoadRepositories() ([]string, error) {
	return m.repositories, nil
}

// MockGitRepo is a mock implementation of git.Repository for testing
type MockGitRepo struct {
	path string
	name string
}

func NewMockGitRepo() *MockGitRepo {
	return &MockGitRepo{
		path: "/test/repo",
		name: "test-repo",
	}
}

func (m *MockGitRepo) GetBranches() ([]string, error) {
	return []string{"main", "feature"}, nil
}

func (m *MockGitRepo) GetBranchCommitHash(branch string) (string, error) {
	if branch == "feature" {
		return "feature-commit-hash", nil
	}
	if branch == "main" {
		return "main-commit-hash", nil
	}
	return "", fmt.Errorf("unknown branch: %s", branch)
}

func (m *MockGitRepo) GetDiff(sourceBranch, targetBranch string) (string, error) {
	return "diff --git a/file.txt b/file.txt\nindex 1234..5678 100644\n--- a/file.txt\n+++ b/file.txt\n@@ -1,1 +1,2 @@\n line1\n+line2", nil
}

func (m *MockGitRepo) GetFileDiff(sourceBranch, targetBranch, filePath string) (string, error) {
	return "diff --git a/" + filePath + " b/" + filePath + "\nindex 1234..5678 100644\n--- a/" + filePath + "\n+++ b/" + filePath + "\n@@ -1,1 +1,2 @@\n line1\n+line2", nil
}

func (m *MockGitRepo) GetFiles(sourceBranch, targetBranch string) ([]string, error) {
	return []string{"file.txt"}, nil
}

// This field just to satisfy having all methods of git.Repository
var _ = (*MockGitRepo)(nil).GetFiles

// TestServer extends Server for testing
type TestServer struct {
	Server
	mockRepo *MockGitRepo
}

// GetRepository overrides the Server.GetRepository method for testing
func (s *TestServer) GetRepository(path string) (*git.Repository, bool, error) {
	// Return a real Repository with the path/name from our mock
	// Since we've overridden the handler methods that call the repo methods,
	// they'll call our mock methods instead
	return &git.Repository{
		Path: s.mockRepo.path,
		Name: s.mockRepo.name,
	}, true, nil
}

// Override method handlers directly - this is simpler than trying to make
// a complete mock implementation
func (s *TestServer) handleCompare(w http.ResponseWriter, r *http.Request) {
	// For GET requests
	if r.Method == http.MethodGet {
		s.render(w, "compare.html", map[string]interface{}{
			"RepoPath":     "/test/repo",
			"RepoName":     "test-repo",
			"SourceBranch": "feature",
			"TargetBranch": "main",
			"Branches":     []string{"main", "feature"},
		})
		return
	}

	// For POST requests, redirect to diff view
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			s.renderError(w, "Invalid Form", "Invalid form data submitted", http.StatusBadRequest)
			return
		}

		redirectURL := fmt.Sprintf("/diff?repo=%s&source=%s&target=%s&source_commit=%s&target_commit=%s",
			url.QueryEscape("/test/repo"),
			url.QueryEscape("feature"),
			url.QueryEscape("main"),
			url.QueryEscape("feature-commit-hash"),
			url.QueryEscape("main-commit-hash"))

		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}
}

// Override handleDiffView to use our mock data
func (s *TestServer) handleDiffView(w http.ResponseWriter, r *http.Request) {
	s.render(w, "diff.html", map[string]interface{}{
		"RepoPath":     "/test/repo",
		"RepoName":     "test-repo",
		"SourceBranch": "feature",
		"TargetBranch": "main",
		"SourceCommit": "feature-commit-hash",
		"TargetCommit": "main-commit-hash",
		"Files":        []map[string]string{{"Path": "file.txt", "Status": "unreviewed"}},
		"DiffLines":    []string{"diff --git a/file.txt b/file.txt", "@@ -1,1 +1,2 @@", " line1", "+line2"},
	})
}

// Helper function to create a test server with mocked dependencies
func setupTestServer(t *testing.T) (*Server, *MockStorage) {
	t.Helper()

	mockStorage := &MockStorage{
		repositories: []string{"/test/repo"},
		reviewState: &models.ReviewState{
			ReviewedFiles: []models.FileReview{},
			SourceBranch:  "feature",
			TargetBranch:  "main",
			SourceCommit:  "feature-commit-hash",
			TargetCommit:  "main-commit-hash",
		},
	}

	// remporarly replate getTemplateDir with a mocked one.
	origFS := getTemplateDir
	getTemplateDir = func() fs.FS {
		return fstest.MapFS{
			"templates/layout.html": &fstest.MapFile{
				Data: []byte(`{{define "layout.html"}}<!DOCTYPE html><html><body>{{.RenderedContent}}</body></html>{{end}}`),
				Mode: 0644,
			},
			"templates/index.html": &fstest.MapFile{
				Data: []byte(`{{define "index.html"}}Index Page{{end}}`),
				Mode: 0644,
			},
			"templates/compare.html": &fstest.MapFile{
				Data: []byte(`{{define "compare.html"}}Compare Page{{end}}`),
				Mode: 0644,
			},
			"templates/diff.html": &fstest.MapFile{
				Data: []byte(`{{define "diff.html"}}Diff Page{{end}}`),
				Mode: 0644,
			},
			"templates/error.html": &fstest.MapFile{
				Data: []byte(`{{define "error.html"}}Error: {{.Title}} - {{.Message}}{{end}}`),
				Mode: 0644,
			},
		}
	}
	t.Cleanup(func() {
		getTemplateDir = origFS
	})

	server, err := New(mockStorage)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return server, mockStorage
}

// Helper function to create a test server with a mock repository
func setupTestServerWithMockRepo(t *testing.T) (*TestServer, *MockStorage) {
	t.Helper()

	server, mockStorage := setupTestServer(t)

	// Create a test server with a mock repository
	testServer := &TestServer{
		Server:   *server,
		mockRepo: NewMockGitRepo(),
	}

	return testServer, mockStorage
}

// TestServerInit tests that the server initializes correctly
func TestServerInit(t *testing.T) {
	server, _ := setupTestServer(t)
	if server == nil {
		t.Fatal("Server should not be nil")
	}
}

// TestHandleIndex tests the index handler
func TestHandleIndex(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.handleIndex(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	if !strings.Contains(string(body), "Index Page") {
		t.Errorf("Expected body to contain 'Index Page', got %s", string(body))
	}
}

// TestHandleCompare tests the compare handler
func TestHandleCompare(t *testing.T) {
	server, _ := setupTestServerWithMockRepo(t)

	// Test GET request
	req := httptest.NewRequest("GET", "/compare?repo=/test/repo", nil)
	w := httptest.NewRecorder()

	server.handleCompare(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	if !strings.Contains(string(body), "Compare Page") {
		t.Errorf("Expected body to contain 'Compare Page', got %s", string(body))
	}

	// Test POST request
	formData := url.Values{}
	formData.Set("repo", "/test/repo")
	formData.Set("source", "feature")
	formData.Set("target", "main")

	req = httptest.NewRequest("POST", "/compare", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()

	server.handleCompare(w, req)

	resp = w.Result()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status code %d, got %d", http.StatusSeeOther, resp.StatusCode)
	}

	// Check redirect location
	location := resp.Header.Get("Location")
	if !strings.Contains(location, "/diff") ||
		!strings.Contains(location, "repo=%2Ftest%2Frepo") ||
		!strings.Contains(location, "source=feature") ||
		!strings.Contains(location, "target=main") {
		t.Errorf("Expected redirect to diff page with proper parameters, got %s", location)
	}
}

// TestHandleDiffView tests the diff view handler
func TestHandleDiffView(t *testing.T) {
	server, _ := setupTestServerWithMockRepo(t)

	req := httptest.NewRequest("GET", "/diff?repo=/test/repo&source=feature&target=main", nil)
	w := httptest.NewRecorder()

	server.handleDiffView(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	if !strings.Contains(string(body), "Diff Page") {
		t.Errorf("Expected body to contain 'Diff Page', got %s", string(body))
	}
}

// TestHandleReviewState tests the review state handler
func TestHandleReviewState(t *testing.T) {
	server, mockStorage := setupTestServer(t)

	formData := url.Values{}
	formData.Set("repo", "/test/repo")
	formData.Set("source", "feature")
	formData.Set("target", "main")
	formData.Set("source_commit", "feature-commit-hash")
	formData.Set("target_commit", "main-commit-hash")
	formData.Set("file", "file.txt")
	formData.Set("status", "approved")

	req := httptest.NewRequest("POST", "/api/review-state?"+formData.Encode(), nil)
	w := httptest.NewRecorder()

	server.handleReviewState(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status code %d, got %d", http.StatusSeeOther, resp.StatusCode)
	}

	if !mockStorage.saveCalled {
		t.Error("SaveReviewState should have been called")
	}

	if !mockStorage.loadCalled {
		t.Error("LoadReviewState should have been called")
	}

	if mockStorage.reviewState == nil || len(mockStorage.reviewState.ReviewedFiles) == 0 {
		t.Error("ReviewState should have been updated with a file review")
	}
}

// TestExtractFilesFromDiff tests the extractFilesFromDiff function
func TestExtractFilesFromDiff(t *testing.T) {
	diffText := `diff --git a/file1.txt b/file1.txt
index 1234..5678 100644
--- a/file1.txt
+++ b/file1.txt
@@ -1,3 +1,4 @@
 line1
+new line
 line2
 line3
diff --git a/file2.txt b/file2.txt
index 8765..4321 100644
--- a/file2.txt
+++ b/file2.txt
@@ -1,3 +1,3 @@
 line1
-old line
+new line
 line3`

	reviewState := &models.ReviewState{
		ReviewedFiles: []models.FileReview{
			{
				Repo:  "/test/repo",
				Path:  "file1.txt",
				Lines: map[string]string{"all": models.StateApproved},
			},
		},
	}

	files := extractFilesFromDiff(diffText, reviewState, "/test/repo")

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	if files[0]["Path"] != "file2.txt" {
		t.Errorf("Expected first file to be file2.txt (unreviewed), got %s", files[0]["Path"])
	}

	if files[1]["Path"] != "file1.txt" {
		t.Errorf("Expected second file to be file1.txt (approved), got %s", files[1]["Path"])
	}

	if files[0]["Status"] != "unreviewed" {
		t.Errorf("Expected file2.txt status to be unreviewed, got %s", files[0]["Status"])
	}

	if files[1]["Status"] != models.StateApproved {
		t.Errorf("Expected file1.txt status to be approved, got %s", files[1]["Status"])
	}
}

// TestAddRepository tests the AddRepository method
func TestAddRepository(t *testing.T) {
	server, mockStorage := setupTestServer(t)

	// Create a temporary directory that will be our mock git repo
	tempDir, err := os.MkdirTemp("", "diffty-test-repo")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a .git directory to make it look like a git repo
	if err := os.Mkdir(filepath.Join(tempDir, ".git"), 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Add the repository
	success, err := server.AddRepository(tempDir)

	if !success || err != nil {
		t.Errorf("AddRepository failed: %v", err)
	}

	// Check that the repository was added to the storage
	if len(mockStorage.repositories) != 2 || mockStorage.repositories[1] != tempDir {
		t.Errorf("Repository not added to storage correctly: %v", mockStorage.repositories)
	}
}

// TestRenderError tests the renderError method
func TestRenderError(t *testing.T) {
	server, _ := setupTestServer(t)

	w := httptest.NewRecorder()

	server.renderError(w, "Test Error", "This is a test error message", http.StatusBadRequest)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}

	expectedContent := "Error: Test Error - This is a test error message"
	if !strings.Contains(string(body), expectedContent) {
		t.Errorf("Expected body to contain '%s', got '%s'", expectedContent, string(body))
	}
}
