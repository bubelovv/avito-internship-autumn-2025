package domain

import "time"

type Team struct {
	ID      int64
	Name    string
	Members []TeamMember
}

type TeamMember struct {
	UserID   string
	Username string
	IsActive bool
}

type User struct {
	ID       string
	Username string
	IsActive bool
	TeamID   *int64
	TeamName *string
}

type PullRequestStatus string

const (
	PullRequestStatusOpen   PullRequestStatus = "OPEN"
	PullRequestStatusMerged PullRequestStatus = "MERGED"
)

type PullRequest struct {
	ID        string
	Name      string
	AuthorID  string
	Status    PullRequestStatus
	CreatedAt time.Time
	MergedAt  *time.Time
	Reviewers []string
}

type PullRequestShort struct {
	ID       string
	Name     string
	AuthorID string
	Status   PullRequestStatus
}
