package service

import (
	"context"
	"errors"
	"time"

	"github.com/bubelovv/avito-internship-autumn-2025/internal/domain"
	"github.com/bubelovv/avito-internship-autumn-2025/internal/repository"
	"github.com/jackc/pgx/v5"
)

var (
	ErrTeamExists          = errors.New("team already exists")
	ErrTeamNotFound        = errors.New("team not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrPullRequestExists   = errors.New("pull request already exists")
	ErrPullRequestNotFound = errors.New("pull request not found")
	ErrPullRequestMerged   = errors.New("pull request already merged")
	ErrReviewerNotAssigned = errors.New("reviewer not assigned")
	ErrNoCandidate         = errors.New("no active replacement candidate")
)

type Service struct {
	repo *repository.Repository
	now  func() time.Time
}

func New(repo *repository.Repository) *Service {
	return &Service{
		repo: repo,
		now:  time.Now,
	}
}

func (s *Service) CreateTeam(ctx context.Context, teamName string, members []domain.TeamMember) (domain.Team, error) {
	err := s.repo.RunInTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		teamID, err := s.repo.InsertTeam(ctx, tx, teamName)
		if err != nil {
			if errors.Is(err, repository.ErrTeamExists) {
				return ErrTeamExists
			}
			return err
		}

		for _, member := range members {
			user := domain.User{
				ID:       member.UserID,
				Username: member.Username,
				IsActive: member.IsActive,
			}
			if _, err := s.repo.UpsertUser(ctx, tx, user); err != nil {
				return err
			}
			if err := s.repo.UpsertMembership(ctx, tx, teamID, member.UserID); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return domain.Team{}, err
	}

	team, err := s.repo.GetTeamByName(ctx, teamName)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return domain.Team{}, ErrTeamNotFound
		}
		return domain.Team{}, err
	}

	return team, nil
}

func (s *Service) GetTeam(ctx context.Context, teamName string) (domain.Team, error) {
	team, err := s.repo.GetTeamByName(ctx, teamName)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return domain.Team{}, ErrTeamNotFound
		}
		return domain.Team{}, err
	}
	return team, nil
}

func (s *Service) SetUserActivity(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	user, err := s.repo.SetUserActive(ctx, userID, isActive)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

func (s *Service) CreatePullRequest(ctx context.Context, prID, prName, authorID string) (domain.PullRequest, error) {
	author, err := s.repo.GetUser(ctx, authorID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return domain.PullRequest{}, ErrUserNotFound
		}
		return domain.PullRequest{}, err
	}
	if author.TeamID == nil {
		return domain.PullRequest{}, ErrTeamNotFound
	}

	err = s.repo.RunInTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		_, err := s.repo.CreatePullRequest(ctx, tx, domain.PullRequest{
			ID:       prID,
			Name:     prName,
			AuthorID: authorID,
			Status:   domain.PullRequestStatusOpen,
		})
		if err != nil {
			if errors.Is(err, repository.ErrPullRequestExists) {
				return ErrPullRequestExists
			}
			return err
		}

		reviewers, err := s.repo.ListRandomActiveTeamMembers(ctx, *author.TeamID, []string{author.ID}, 2)
		if err != nil {
			return err
		}

		reviewerIDs := make([]string, 0, len(reviewers))
		for _, reviewer := range reviewers {
			reviewerIDs = append(reviewerIDs, reviewer.UserID)
		}

		if err := s.repo.AddReviewers(ctx, tx, prID, reviewerIDs); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return domain.PullRequest{}, err
	}

	pr, err := s.repo.GetPullRequest(ctx, prID)
	if err != nil {
		if errors.Is(err, repository.ErrPullRequestNotFound) {
			return domain.PullRequest{}, ErrPullRequestNotFound
		}
		return domain.PullRequest{}, err
	}

	return pr, nil
}

func (s *Service) ReassignReviewer(ctx context.Context, prID, oldReviewerID string) (domain.PullRequest, string, error) {
	pr, err := s.repo.GetPullRequest(ctx, prID)
	if err != nil {
		if errors.Is(err, repository.ErrPullRequestNotFound) {
			return domain.PullRequest{}, "", ErrPullRequestNotFound
		}
		return domain.PullRequest{}, "", err
	}
	if pr.Status == domain.PullRequestStatusMerged {
		return domain.PullRequest{}, "", ErrPullRequestMerged
	}

	found := false
	for _, reviewer := range pr.Reviewers {
		if reviewer == oldReviewerID {
			found = true
			break
		}
	}
	if !found {
		return domain.PullRequest{}, "", ErrReviewerNotAssigned
	}

	reviewerUser, err := s.repo.GetUser(ctx, oldReviewerID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return domain.PullRequest{}, "", ErrUserNotFound
		}
		return domain.PullRequest{}, "", err
	}
	if reviewerUser.TeamID == nil {
		return domain.PullRequest{}, "", ErrNoCandidate
	}

	exclude := make([]string, 0, len(pr.Reviewers)+2)
	exclude = append(exclude, pr.AuthorID)
	exclude = append(exclude, pr.Reviewers...)

	candidates, err := s.repo.ListRandomActiveTeamMembers(ctx, *reviewerUser.TeamID, exclude, 1)
	if err != nil {
		return domain.PullRequest{}, "", err
	}
	if len(candidates) == 0 {
		return domain.PullRequest{}, "", ErrNoCandidate
	}

	replacement := candidates[0].UserID
	err = s.repo.RunInTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.ReplaceReviewer(ctx, tx, prID, oldReviewerID, replacement); err != nil {
			if errors.Is(err, repository.ErrReviewerNotAssigned) {
				return ErrReviewerNotAssigned
			}
			return err
		}
		return nil
	})
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	updated, err := s.repo.GetPullRequest(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	return updated, replacement, nil
}

func (s *Service) MergePullRequest(ctx context.Context, prID string) (domain.PullRequest, error) {
	pr, err := s.repo.GetPullRequest(ctx, prID)
	if err != nil {
		if errors.Is(err, repository.ErrPullRequestNotFound) {
			return domain.PullRequest{}, ErrPullRequestNotFound
		}
		return domain.PullRequest{}, err
	}
	if pr.Status == domain.PullRequestStatusMerged {
		return pr, nil
	}

	err = s.repo.RunInTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.repo.MarkPullRequestMerged(ctx, tx, prID, s.now().UTC()); err != nil {
			if errors.Is(err, repository.ErrPullRequestNotFound) {
				return ErrPullRequestNotFound
			}
			return err
		}
		return nil
	})
	if err != nil {
		return domain.PullRequest{}, err
	}

	return s.repo.GetPullRequest(ctx, prID)
}

func (s *Service) ListReviewerPullRequests(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	return s.repo.ListPullRequestsForReviewer(ctx, userID)
}
