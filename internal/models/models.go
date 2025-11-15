package models

import "time"

type Team struct {
	TeamName  string       `json:"team_name"`
	Members   []TeamMember `json:"members"`
	CreatedAt time.Time    `json:"-"`
}

type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type User struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	TeamName  string    `json:"team_name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

type PRStatus string

const (
	PRStatusOpen   PRStatus = "OPEN"
	PRStatusMerged PRStatus = "MERGED"
)

type PullRequest struct {
	PullRequestID     string     `json:"pull_request_id"`
	PullRequestName   string     `json:"pull_request_name"`
	AuthorID          string     `json:"author_id"`
	Status            PRStatus   `json:"status"`
	AssignedReviewers []string   `json:"assigned_reviewers"`
	CreatedAt         *time.Time `json:"createdAt,omitempty"`
	MergedAt          *time.Time `json:"mergedAt,omitempty"`
}

type PullRequestShort struct {
	PullRequestID   string   `json:"pull_request_id"`
	PullRequestName string   `json:"pull_request_name"`
	AuthorID        string   `json:"author_id"`
	Status          PRStatus `json:"status"`
}

type ErrorCode string

const (
	ErrorTeamExists  ErrorCode = "TEAM_EXISTS"
	ErrorPRExists    ErrorCode = "PR_EXISTS"
	ErrorPRMerged    ErrorCode = "PR_MERGED"
	ErrorNotAssigned ErrorCode = "NOT_ASSIGNED"
	ErrorNoCandidate ErrorCode = "NO_CANDIDATE"
	ErrorNotFound    ErrorCode = "NOT_FOUND"
)

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type Statistics struct {
	TotalUsers      int                  `json:"total_users"`
	ActiveUsers     int                  `json:"active_users"`
	TotalTeams      int                  `json:"total_teams"`
	TotalPRs        int                  `json:"total_prs"`
	OpenPRs         int                  `json:"open_prs"`
	MergedPRs       int                  `json:"merged_prs"`
	UserAssignments []UserAssignmentStat `json:"user_assignments"`
	PRAssignments   []PRAssignmentStat   `json:"pr_assignments"`
}

type UserAssignmentStat struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	PRCount     int    `json:"pr_count"`
	OpenCount   int    `json:"open_count"`
	MergedCount int    `json:"merged_count"`
}

type PRAssignmentStat struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	ReviewerCount   int    `json:"reviewer_count"`
}
