package server

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darccio/diffty/internal/git"
	"github.com/darccio/diffty/internal/models"
	"github.com/darccio/diffty/internal/storage"
)

// Server represents the HTTP server
type Server struct {
	storage storage.Storage
	tmpl    *template.Template
	mux     *http.ServeMux
}

// New creates a new Server instance
func New(storage storage.Storage, templateDir string) (*Server, error) {
	// Check if template directory exists
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("template directory does not exist: %s", templateDir)
	}

	// Make sure the layout template exists
	layoutPath := filepath.Join(templateDir, "layout.html")
	if _, err := os.Stat(layoutPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("layout template does not exist: %s", layoutPath)
	}

	// Create template functions map
	funcMap := template.FuncMap{
		"hasPrefix": strings.HasPrefix, // Used to check if a string starts with a prefix
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
		"index":     func(arr []map[string]string, i int) map[string]string { return arr[i] },
		"len":       func(arr []map[string]string) int { return len(arr) },
	}

	// Parse all templates with the function map
	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	// Create server
	server := &Server{
		storage: storage,
		tmpl:    tmpl,
		mux:     http.NewServeMux(),
	}

	return server, nil
}

// AddRepository adds a new repository to the server and persists it
func (s *Server) AddRepository(path string) (bool, error) {
	// Validate the repository path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	// Check if it's a valid git repository
	if !git.IsValidRepo(absPath) {
		return false, fmt.Errorf("not a valid git repository: %s", absPath)
	}

	// Get current repositories
	repos, err := s.storage.LoadRepositories()
	if err != nil {
		return false, fmt.Errorf("failed to load repositories: %w", err)
	}

	// Check if repository already exists
	for _, existingPath := range repos {
		if existingPath == absPath {
			// Repository already exists, nothing to do
			return true, nil
		}
	}

	// Add new repository path
	repos = append(repos, absPath)

	// Save updated list
	if err := s.storage.SaveRepositories(repos); err != nil {
		return false, fmt.Errorf("failed to save repositories: %w", err)
	}

	return true, nil
}

// GetRepository returns a repository by path
func (s *Server) GetRepository(path string) (*git.Repository, bool, error) {
	repos, err := s.storage.LoadRepositories()
	if err != nil {
		return nil, false, fmt.Errorf("failed to load repositories: %w", err)
	}

	// Check if repository exists
	for _, repo := range repos {
		if repo == path {
			return git.NewRepository(path), true, nil
		}
	}

	return nil, false, nil
}

// GetRepositories returns all repositories
func (s *Server) GetRepositories() (map[string]*git.Repository, error) {
	repos, err := s.storage.LoadRepositories()
	if err != nil {
		return nil, fmt.Errorf("failed to load repositories: %w", err)
	}

	// Create a map of repositories
	reposMap := make(map[string]*git.Repository)
	for _, path := range repos {
		reposMap[path] = git.NewRepository(path)
	}

	return reposMap, nil
}

// Router sets up and returns the HTTP router
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Static files
	staticDir := filepath.Join("internal", "server", "static")
	fileServer := http.FileServer(http.Dir(staticDir))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))

	// API routes
	mux.HandleFunc("POST /api/repository/add", s.handleAddRepository)
	mux.HandleFunc("POST /api/review-state", s.handleReviewState)

	// HTML routes
	mux.HandleFunc("GET /compare", s.handleCompare)
	mux.HandleFunc("POST /compare", s.handleCompare)
	mux.HandleFunc("GET /diff", s.handleDiffView)
	mux.HandleFunc("GET /", s.handleIndex)

	return mux
}

// handleIndex renders the index page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	repos, err := s.GetRepositories()
	if err != nil {
		s.renderError(w, "Repository Error", fmt.Sprintf("Error loading repositories: %v", err), http.StatusInternalServerError)
		return
	}

	// Check if we have any repositories
	hasRepos := len(repos) > 0

	data := map[string]interface{}{
		"Repositories": repos,
		"HasRepos":     hasRepos,
	}

	s.render(w, "index.html", data)
}

// handleCompare renders the comparison page
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")
	sourceBranch := r.URL.Query().Get("source")
	targetBranch := r.URL.Query().Get("target")
	// Handle form submission
	if r.Method == http.MethodPost {
		// Parse form data
		if err := r.ParseForm(); err != nil {
			s.renderError(w, "Invalid Form", "Invalid form data submitted", http.StatusBadRequest)
			return
		}

		// Get repository path from form data (in case of POST)
		formRepoPath := r.FormValue("repo")
		formSourceBranch := r.FormValue("source")
		formTargetBranch := r.FormValue("target")

		if formRepoPath != "" {
			repoPath = formRepoPath
		}

		// Make sure we have a repository path
		if repoPath == "" {
			s.renderError(w, "Missing Repository", "Repository path is required", http.StatusBadRequest)
			return
		}

		// Only update if non-empty values provided
		if formSourceBranch != "" {
			sourceBranch = formSourceBranch
		}

		if formTargetBranch != "" {
			targetBranch = formTargetBranch
		}

		// Make sure we have source and target branches
		if sourceBranch == "" || targetBranch == "" {
			s.renderError(w, "Missing Branches", "Source and target branches are required", http.StatusBadRequest)
			return
		}

		// Check if the repository exists
		repo, exists, err := s.GetRepository(repoPath)
		if err != nil {
			s.renderError(w, "Repository Error", fmt.Sprintf("Error loading repository: %v", err), http.StatusInternalServerError)
			return
		}
		if !exists {
			s.renderError(w, "Not Found", "Repository not found", http.StatusNotFound)
			return
		}

		// Get commit hashes for the branches
		sourceCommit, err := repo.GetBranchCommitHash(sourceBranch)
		if err != nil {
			s.renderError(w, "Branch Error", fmt.Sprintf("Failed to get commit hash for source branch '%s': %v", sourceBranch, err), http.StatusInternalServerError)
			return
		}

		targetCommit, err := repo.GetBranchCommitHash(targetBranch)
		if err != nil {
			s.renderError(w, "Branch Error", fmt.Sprintf("Failed to get commit hash for target branch '%s': %v", targetBranch, err), http.StatusInternalServerError)
			return
		}

		// Redirect to diff view with commit hashes
		redirectURL := fmt.Sprintf("/diff?repo=%s&source=%s&target=%s&source_commit=%s&target_commit=%s",
			url.QueryEscape(repoPath),
			url.QueryEscape(sourceBranch),
			url.QueryEscape(targetBranch),
			url.QueryEscape(sourceCommit),
			url.QueryEscape(targetCommit))

		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Handle GET request
	if repoPath == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Check if the repository exists
	repo, exists, err := s.GetRepository(repoPath)
	if err != nil {
		s.renderError(w, "Repository Error", fmt.Sprintf("Error loading repository: %v", err), http.StatusInternalServerError)
		return
	}
	if !exists {
		s.renderError(w, "Not Found", "Repository not found", http.StatusNotFound)
		return
	}

	// Get repository name from path for display
	repoName := filepath.Base(repoPath)

	// Load branches from the repository
	branches, err := repo.GetBranches()
	if err != nil {
		s.renderError(w, "Branch Error", fmt.Sprintf("Failed to load branches: %v", err), http.StatusInternalServerError)
		return
	}

	// Pre-select branches if not specified
	if sourceBranch == "" && len(branches) > 0 {
		// Try to use the second branch (usually a feature branch) as source
		if len(branches) > 1 {
			sourceBranch = branches[1]
		} else {
			sourceBranch = branches[0]
		}
	}

	if targetBranch == "" && len(branches) > 0 {
		// Usually main/master is the first branch
		targetBranch = branches[0]
	}

	data := map[string]interface{}{
		"RepoPath":     repoPath,
		"RepoName":     repoName,
		"SourceBranch": sourceBranch,
		"TargetBranch": targetBranch,
		"Branches":     branches,
	}

	s.render(w, "compare.html", data)
}

// handleAddRepository adds a new repository
func (s *Server) handleAddRepository(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.renderError(w, "Method Not Allowed", "This method is not allowed for this endpoint", http.StatusMethodNotAllowed)
		return
	}

	// Parse the form data
	if err := r.ParseForm(); err != nil {
		s.renderError(w, "Invalid Form", "Invalid form data submitted", http.StatusBadRequest)
		return
	}

	repoPath := r.Form.Get("path")
	if repoPath == "" {
		s.renderError(w, "Missing Path", "Repository path is required", http.StatusBadRequest)
		return
	}

	// Add the repository
	success, err := s.AddRepository(repoPath)
	if !success {
		if err != nil {
			s.renderError(w, "Repository Error", err.Error(), http.StatusInternalServerError)
		} else {
			s.renderError(w, "Repository Error", "Failed to add repository", http.StatusInternalServerError)
		}
		return
	}

	// Redirect to the index page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleReviewState handles saving and loading review state
func (s *Server) handleReviewState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.renderError(w, "Method Not Allowed", "This method is not allowed for this endpoint", http.StatusMethodNotAllowed)
		return
	}

	// Get required parameters
	repoPath := r.URL.Query().Get("repo")
	sourceBranch := r.URL.Query().Get("source")
	targetBranch := r.URL.Query().Get("target")
	sourceCommit := r.URL.Query().Get("source_commit")
	targetCommit := r.URL.Query().Get("target_commit")
	filePath := r.URL.Query().Get("file")
	status := r.URL.Query().Get("status")
	nextFilePath := r.URL.Query().Get("next")

	if repoPath == "" || sourceBranch == "" || targetBranch == "" || sourceCommit == "" || targetCommit == "" || filePath == "" || status == "" {
		s.renderError(w, "Missing Parameters", "Missing required parameters for updating review state", http.StatusBadRequest)
		return
	}

	// Validate status value
	if status != models.StateApproved && status != models.StateRejected && status != models.StateSkipped {
		s.renderError(w, "Invalid Status", "Invalid status value for file review", http.StatusBadRequest)
		return
	}

	// Load existing review state
	existingState, err := s.storage.LoadReviewState(repoPath, sourceBranch, targetBranch, sourceCommit, targetCommit)
	if err != nil {
		s.renderError(w, "Review State Error", fmt.Sprintf("Failed to load review state: %v", err), http.StatusInternalServerError)
		return
	}

	// Look for the file in the existing review state
	fileFound := false
	for i := range existingState.ReviewedFiles {
		if existingState.ReviewedFiles[i].Path == filePath && existingState.ReviewedFiles[i].Repo == repoPath {
			// Update existing file review
			if existingState.ReviewedFiles[i].Lines == nil {
				existingState.ReviewedFiles[i].Lines = make(map[string]string)
			}
			existingState.ReviewedFiles[i].Lines["all"] = status
			fileFound = true
			break
		}
	}

	// If file not found, add it to the review state
	if !fileFound {
		existingState.ReviewedFiles = append(existingState.ReviewedFiles, models.FileReview{
			Repo:  repoPath,
			Path:  filePath,
			Lines: map[string]string{"all": status},
		})
	}

	// Save updated review state
	if err := s.storage.SaveReviewState(existingState, repoPath); err != nil {
		s.renderError(w, "Review State Error", fmt.Sprintf("Failed to save review state: %v", err), http.StatusInternalServerError)
		return
	}

	// Determine where to redirect
	redirectPath := fmt.Sprintf("/diff?repo=%s&source=%s&target=%s&source_commit=%s&target_commit=%s",
		url.QueryEscape(repoPath),
		url.QueryEscape(sourceBranch),
		url.QueryEscape(targetBranch),
		url.QueryEscape(sourceCommit),
		url.QueryEscape(targetCommit))

	// If next file specified and this was approved, rejected, or skipped, go to next file
	if nextFilePath != "" && (status == models.StateApproved || status == models.StateRejected || status == models.StateSkipped) {
		redirectPath += "&file=" + url.QueryEscape(nextFilePath)
	} else if filePath != "" {
		// Otherwise stay on current file
		redirectPath += "&file=" + url.QueryEscape(filePath)
	}

	// Redirect to the appropriate diff view
	http.Redirect(w, r, redirectPath, http.StatusSeeOther)
}

// handleDiffView renders the diff visualization page
func (s *Server) handleDiffView(w http.ResponseWriter, r *http.Request) {
	repoPath := r.URL.Query().Get("repo")
	sourceBranch := r.URL.Query().Get("source")
	targetBranch := r.URL.Query().Get("target")
	filePath := r.URL.Query().Get("file")

	if repoPath == "" || sourceBranch == "" || targetBranch == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Check if the repository exists
	repo, exists, err := s.GetRepository(repoPath)
	if err != nil {
		s.renderError(w, "Repository Error", fmt.Sprintf("Error loading repository: %v", err), http.StatusInternalServerError)
		return
	}
	if !exists {
		s.renderError(w, "Not Found", "Repository not found", http.StatusNotFound)
		return
	}

	// Get repository name from path for display
	repoName := filepath.Base(repoPath)

	// Get commit hashes for the branches
	sourceCommit, err := repo.GetBranchCommitHash(sourceBranch)
	if err != nil {
		s.renderError(w, "Branch Error", fmt.Sprintf("Failed to get commit hash for source branch: %v", err), http.StatusInternalServerError)
		return
	}

	targetCommit, err := repo.GetBranchCommitHash(targetBranch)
	if err != nil {
		s.renderError(w, "Branch Error", fmt.Sprintf("Failed to get commit hash for target branch: %v", err), http.StatusInternalServerError)
		return
	}

	// Load review state
	var reviewState *models.ReviewState
	reviewState, err = s.storage.LoadReviewState(repoPath, sourceBranch, targetBranch, sourceCommit, targetCommit)
	if err != nil {
		reviewState = &models.ReviewState{
			ReviewedFiles: []models.FileReview{},
			SourceBranch:  sourceBranch,
			TargetBranch:  targetBranch,
			SourceCommit:  sourceCommit,
			TargetCommit:  targetCommit,
		}
	}

	// Data to pass to the template
	data := map[string]interface{}{
		"RepoPath":     repoPath,
		"RepoName":     repoName,
		"SourceBranch": sourceBranch,
		"TargetBranch": targetBranch,
		"SourceCommit": sourceCommit,
		"TargetCommit": targetCommit,
		"Error":        "",
		"NoDiff":       false,
		"ReviewState":  reviewState,
	}

	// Get the diff
	var diffText string
	var err2 error
	var files []map[string]string

	// Always get full diff to extract file list (needed for navigation)
	fullDiffText, fullDiffErr := repo.GetDiff(sourceBranch, targetBranch)
	if fullDiffErr != nil {
		data["Error"] = fmt.Sprintf("Failed to load diff: %v", fullDiffErr)
	} else if fullDiffText == "" {
		data["NoDiff"] = true
	} else {
		// Extract file paths from diff
		files = extractFilesFromDiff(fullDiffText, reviewState, repoPath)
		data["Files"] = files
	}

	if filePath == "" {
		s.render(w, "diff.html", data)
		return
	}

	// If a specific file is requested, load its diff
	diffText, err2 = repo.GetFileDiff(sourceBranch, targetBranch, filePath)
	if err2 != nil {
		data["Error"] = fmt.Sprintf("Failed to load diff: %v", err2)
	} else {
		data["SelectedFile"] = filePath
		data["DiffLines"] = strings.Split(diffText, "\n")

		// Determine the file status for display in the UI
		fileStatus := "unreviewed"
		for _, review := range reviewState.ReviewedFiles {
			if review.Path == filePath && review.Repo == repoPath {
				// Check if all lines have the same status
				statuses := make(map[string]bool)
				for _, status := range review.Lines {
					statuses[status] = true
				}

				if len(statuses) == 1 {
					for status := range statuses {
						fileStatus = status
					}
				} else if len(statuses) > 1 {
					fileStatus = "mixed"
				}
				break
			}
		}
		data["FileStatus"] = fileStatus

		// Find next file for navigation
		if len(files) > 0 {
			currentIndex := -1
			for i, file := range files {
				if file["Path"] == filePath {
					currentIndex = i
					break
				}
			}

			if currentIndex != -1 && currentIndex < len(files)-1 {
				data["NextFilePath"] = files[currentIndex+1]["Path"]
			}
		}
	}

	s.render(w, "diff.html", data)
}

// extractFilesFromDiff extracts file paths from a diff output
func extractFilesFromDiff(diffText string, reviewState *models.ReviewState, repoPath string) []map[string]string {
	var files []map[string]string
	lines := strings.Split(diffText, "\n")

	// Map to store file status
	fileStatusMap := make(map[string]string)

	// Process review state to determine file status
	for _, review := range reviewState.ReviewedFiles {
		if review.Repo != repoPath {
			continue
		}

		// Determine file status based on line statuses
		var approved, rejected, skipped bool
		for _, status := range review.Lines {
			switch status {
			case models.StateApproved:
				approved = true
			case models.StateRejected:
				rejected = true
			case models.StateSkipped:
				skipped = true
			}
		}

		// Prioritize rejection, then approval, then skipped
		status := "unreviewed"
		if rejected {
			status = models.StateRejected
		} else if approved {
			status = models.StateApproved
		} else if skipped {
			status = models.StateSkipped
		}

		fileStatusMap[review.Path] = status
	}

	// Extract files from diff
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			// Extract file path from the diff line
			// Format is typically: diff --git a/path/to/file b/path/to/file
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				bPath := parts[3]
				// Remove the "b/" prefix
				if strings.HasPrefix(bPath, "b/") {
					filePath := bPath[2:] // Skip the "b/" prefix

					// Get status, default to "unreviewed"
					status, exists := fileStatusMap[filePath]
					if !exists {
						status = "unreviewed"
					}

					files = append(files, map[string]string{
						"Path":   filePath,
						"Status": status,
					})
				}
			}
		}
	}

	// Sort files by status and then alphabetically
	sort.Slice(files, func(i, j int) bool {
		// First sort by status
		iStatus := files[i]["Status"]
		jStatus := files[j]["Status"]

		// Priority order: unreviewed > skipped > rejected > approved
		statusPriority := map[string]int{
			"unreviewed":         0,
			models.StateSkipped:  1,
			models.StateRejected: 2,
			models.StateApproved: 3,
		}

		iPriority := statusPriority[iStatus]
		jPriority := statusPriority[jStatus]

		if iPriority != jPriority {
			return iPriority < jPriority
		}

		// Then sort alphabetically
		return files[i]["Path"] < files[j]["Path"]
	})

	return files
}

// render renders a template with the given data
func (s *Server) render(w http.ResponseWriter, templateName string, data interface{}) {
	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// First render the content template to a buffer
	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, templateName, data); err != nil {
		// We can't use renderError here as it would cause an infinite loop if the error is in error.html
		log.Printf("Error rendering content template %s: %v", templateName, err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("<html><body><h1>Internal Server Error</h1><p>Failed to render page. Please try again later.</p></body></html>"))
		return
	}

	// Then render the layout with the pre-rendered content
	layoutData := map[string]interface{}{
		"Content":         templateName,
		"ContentData":     data,
		"RenderedContent": template.HTML(contentBuf.String()),
	}

	if err := s.tmpl.ExecuteTemplate(w, "layout.html", layoutData); err != nil {
		// We can't use renderError here as it would cause an infinite loop if the error is in layout.html
		log.Printf("Error rendering layout template: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("<html><body><h1>Internal Server Error</h1><p>Failed to render page layout. Please try again later.</p></body></html>"))
		return
	}
}

// renderError renders an error page with the given status code and message
func (s *Server) renderError(w http.ResponseWriter, title string, message string, statusCode int) {
	// Set the HTTP status code
	w.WriteHeader(statusCode)

	// Prepare error data
	errorData := map[string]interface{}{
		"Title":   title,
		"Message": message,
	}

	// Render the error template
	s.render(w, "error.html", errorData)
}
