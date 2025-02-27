package models

// FileReview represents the review state of a file
type FileReview struct {
	Repo  string            `json:"repo"`
	Path  string            `json:"path"`
	Lines map[string]string `json:"lines"` // line number or range -> state (approved, skipped, rejected)
}

// ReviewState represents the overall review state
type ReviewState struct {
	ReviewedFiles []FileReview `json:"reviewed_files"`
	SourceBranch  string       `json:"source_branch"`
	TargetBranch  string       `json:"target_branch"`
	SourceCommit  string       `json:"source_commit"`
	TargetCommit  string       `json:"target_commit"`
}

// LineState constants
const (
	StateApproved = "approved"
	StateRejected = "rejected"
	StateSkipped  = "skipped"
)

// DiffFile represents a file diff
type DiffFile struct {
	Path      string     `json:"path"`
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	Sections  []DiffHunk `json:"sections"`
}

// DiffHunk represents a section of a diff
type DiffHunk struct {
	StartLine   int      `json:"start_line"`
	LineCount   int      `json:"line_count"`
	Context     string   `json:"context"`
	Lines       []string `json:"lines"`
	LineNumbers struct {
		Left  []int `json:"left"`
		Right []int `json:"right"`
	} `json:"line_numbers"`
}

// BranchCompare represents a comparison between two branches
type BranchCompare struct {
	SourceBranch string     `json:"source_branch"`
	TargetBranch string     `json:"target_branch"`
	Files        []DiffFile `json:"files"`
}
