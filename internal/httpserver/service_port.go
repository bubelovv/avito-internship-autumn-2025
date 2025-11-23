package httpserver

import (
	"context"

	"github.com/bubelovv/avito-internship-autumn-2025/internal/domain"
)

type Service interface {
	CreateTeam(ctx context.Context, teamName string, members []domain.TeamMember) (domain.Team, error)
	GetTeam(ctx context.Context, teamName string) (domain.Team, error)
	SetUserActivity(ctx context.Context, userID string, isActive bool) (domain.User, error)
	CreatePullRequest(ctx context.Context, prID, prName, authorID string) (domain.PullRequest, error)
	MergePullRequest(ctx context.Context, prID string) (domain.PullRequest, error)
	ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (domain.PullRequest, string, error)
	ListReviewerPullRequests(ctx context.Context, userID string) ([]domain.PullRequestShort, error)
}
