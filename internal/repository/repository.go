package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bubelovv/avito-internship-autumn-2025/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTeamExists          = errors.New("team already exists")
	ErrTeamNotFound        = errors.New("team not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrPullRequestExists   = errors.New("pull request already exists")
	ErrPullRequestNotFound = errors.New("pull request not found")
	ErrReviewerNotAssigned = errors.New("reviewer not assigned to pull request")

	errTxRequired = errors.New("transaction is required")
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

func (r *Repository) RunInTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback tx: %v (original err: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (r *Repository) InsertTeam(ctx context.Context, tx pgx.Tx, teamName string) (int64, error) {
	if tx == nil {
		return 0, errTxRequired
	}

	var id int64
	err := tx.QueryRow(ctx, `INSERT INTO teams (team_name) VALUES ($1) RETURNING team_id`, teamName).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrTeamExists
		}
		return 0, fmt.Errorf("insert team: %w", err)
	}

	return id, nil
}

func (r *Repository) GetTeamByName(ctx context.Context, teamName string) (domain.Team, error) {
	var team domain.Team
	err := r.pool.QueryRow(ctx, `SELECT team_id, team_name FROM teams WHERE team_name = $1`, teamName).
		Scan(&team.ID, &team.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Team{}, ErrTeamNotFound
	}
	if err != nil {
		return domain.Team{}, fmt.Errorf("select team: %w", err)
	}

	members, err := r.listTeamMembersByTeamID(ctx, team.ID)
	if err != nil {
		return domain.Team{}, err
	}
	team.Members = members

	return team, nil
}

func (r *Repository) listTeamMembersByTeamID(ctx context.Context, teamID int64) ([]domain.TeamMember, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.user_id, u.username, u.is_active
		FROM team_memberships tm
		JOIN users u ON u.user_id = tm.user_id
		WHERE tm.team_id = $1
		ORDER BY u.username
	`, teamID)
	if err != nil {
		return nil, fmt.Errorf("select team members: %w", err)
	}
	defer rows.Close()

	var members []domain.TeamMember
	for rows.Next() {
		var m domain.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team members: %w", err)
	}

	return members, nil
}

func (r *Repository) UpsertUser(ctx context.Context, tx pgx.Tx, user domain.User) (domain.User, error) {
	if tx == nil {
		return domain.User{}, errTxRequired
	}

	var stored domain.User
	if err := tx.QueryRow(ctx, `
		INSERT INTO users (user_id, username, is_active)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id)
		DO UPDATE SET username = EXCLUDED.username,
		              is_active = EXCLUDED.is_active,
		              updated_at = NOW()
		RETURNING user_id, username, is_active
	`, user.ID, user.Username, user.IsActive).Scan(&stored.ID, &stored.Username, &stored.IsActive); err != nil {
		return domain.User{}, fmt.Errorf("upsert user: %w", err)
	}

	return stored, nil
}

func (r *Repository) UpsertMembership(ctx context.Context, tx pgx.Tx, teamID int64, userID string) error {
	if tx == nil {
		return errTxRequired
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO team_memberships (team_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id)
		DO UPDATE SET team_id = EXCLUDED.team_id,
		             joined_at = NOW()
	`, teamID, userID); err != nil {
		return fmt.Errorf("upsert membership: %w", err)
	}

	return nil
}

func (r *Repository) GetUser(ctx context.Context, userID string) (domain.User, error) {
	var user domain.User
	var teamID sql.NullInt64
	var teamName sql.NullString

	err := r.pool.QueryRow(ctx, `
		SELECT u.user_id, u.username, u.is_active, tm.team_id, t.team_name
		FROM users u
		LEFT JOIN team_memberships tm ON tm.user_id = u.user_id
		LEFT JOIN teams t ON t.team_id = tm.team_id
		WHERE u.user_id = $1
	`, userID).Scan(&user.ID, &user.Username, &user.IsActive, &teamID, &teamName)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("select user: %w", err)
	}

	if teamID.Valid {
		id := teamID.Int64
		user.TeamID = &id
	}
	if teamName.Valid {
		name := teamName.String
		user.TeamName = &name
	}

	return user, nil
}

func (r *Repository) SetUserActive(ctx context.Context, userID string, isActive bool) (domain.User, error) {
	var user domain.User
	var teamID sql.NullInt64
	var teamName sql.NullString

	err := r.pool.QueryRow(ctx, `
		WITH updated AS (
			UPDATE users
			SET is_active = $2,
			    updated_at = NOW()
			WHERE user_id = $1
			RETURNING user_id, username, is_active
		)
		SELECT u.user_id, u.username, u.is_active, tm.team_id, t.team_name
		FROM updated u
		LEFT JOIN team_memberships tm ON tm.user_id = u.user_id
		LEFT JOIN teams t ON t.team_id = tm.team_id
	`, userID, isActive).Scan(&user.ID, &user.Username, &user.IsActive, &teamID, &teamName)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("update user activity: %w", err)
	}

	if teamID.Valid {
		id := teamID.Int64
		user.TeamID = &id
	}
	if teamName.Valid {
		name := teamName.String
		user.TeamName = &name
	}

	return user, nil
}

func (r *Repository) CreatePullRequest(ctx context.Context, tx pgx.Tx, pr domain.PullRequest) (domain.PullRequest, error) {
	if tx == nil {
		return domain.PullRequest{}, errTxRequired
	}

	var createdAt time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`, pr.ID, pr.Name, pr.AuthorID, string(pr.Status)).Scan(&createdAt); err != nil {
		if isUniqueViolation(err) {
			return domain.PullRequest{}, ErrPullRequestExists
		}
		return domain.PullRequest{}, fmt.Errorf("insert pull request: %w", err)
	}

	pr.CreatedAt = createdAt
	return pr, nil
}

func (r *Repository) GetPullRequest(ctx context.Context, prID string) (domain.PullRequest, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
		FROM pull_requests
		WHERE pull_request_id = $1
	`, prID)

	var pr domain.PullRequest
	var status string
	var mergedAt sql.NullTime
	if err := row.Scan(&pr.ID, &pr.Name, &pr.AuthorID, &status, &pr.CreatedAt, &mergedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.PullRequest{}, ErrPullRequestNotFound
		}
		return domain.PullRequest{}, fmt.Errorf("select pull request: %w", err)
	}
	pr.Status = domain.PullRequestStatus(status)

	if mergedAt.Valid {
		t := mergedAt.Time
		pr.MergedAt = &t
	}

	reviewers, err := r.ListReviewers(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, err
	}
	pr.Reviewers = reviewers

	return pr, nil
}

func (r *Repository) ListReviewers(ctx context.Context, prID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT reviewer_id
		FROM pr_reviewers
		WHERE pull_request_id = $1
		ORDER BY assigned_at
	`, prID)
	if err != nil {
		return nil, fmt.Errorf("select reviewers: %w", err)
	}
	defer rows.Close()

	var reviewers []string
	for rows.Next() {
		var reviewerID string
		if err := rows.Scan(&reviewerID); err != nil {
			return nil, fmt.Errorf("scan reviewer: %w", err)
		}
		reviewers = append(reviewers, reviewerID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reviewers: %w", err)
	}

	return reviewers, nil
}

func (r *Repository) AddReviewers(ctx context.Context, tx pgx.Tx, prID string, reviewerIDs []string) error {
	if tx == nil {
		return errTxRequired
	}
	if len(reviewerIDs) == 0 {
		return nil
	}

	for _, reviewerID := range reviewerIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO pr_reviewers (pull_request_id, reviewer_id)
			VALUES ($1, $2)
		`, prID, reviewerID); err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("reviewer already assigned: %w", err)
			}
			return fmt.Errorf("insert reviewer: %w", err)
		}
	}

	return nil
}

func (r *Repository) ReplaceReviewer(ctx context.Context, tx pgx.Tx, prID, oldReviewerID, newReviewerID string) error {
	if tx == nil {
		return errTxRequired
	}

	tag, err := tx.Exec(ctx, `
		DELETE FROM pr_reviewers
		WHERE pull_request_id = $1 AND reviewer_id = $2
	`, prID, oldReviewerID)
	if err != nil {
		return fmt.Errorf("delete reviewer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrReviewerNotAssigned
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO pr_reviewers (pull_request_id, reviewer_id)
		VALUES ($1, $2)
	`, prID, newReviewerID); err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("reviewer already assigned: %w", err)
		}
		return fmt.Errorf("insert reviewer: %w", err)
	}

	return nil
}

func (r *Repository) MarkPullRequestMerged(ctx context.Context, tx pgx.Tx, prID string, mergedAt time.Time) error {
	if tx == nil {
		return errTxRequired
	}

	tag, err := tx.Exec(ctx, `
		UPDATE pull_requests
		SET status = $2,
		    merged_at = COALESCE(merged_at, $3)
		WHERE pull_request_id = $1
	`, prID, string(domain.PullRequestStatusMerged), mergedAt)
	if err != nil {
		return fmt.Errorf("update pull request status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPullRequestNotFound
	}

	return nil
}

func (r *Repository) ListPullRequestsForReviewer(ctx context.Context, userID string) ([]domain.PullRequestShort, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
		FROM pr_reviewers rr
		JOIN pull_requests pr ON pr.pull_request_id = rr.pull_request_id
		WHERE rr.reviewer_id = $1
		ORDER BY pr.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("select reviewer pull requests: %w", err)
	}
	defer rows.Close()

	var result []domain.PullRequestShort
	for rows.Next() {
		var pr domain.PullRequestShort
		var status string
		if err := rows.Scan(&pr.ID, &pr.Name, &pr.AuthorID, &status); err != nil {
			return nil, fmt.Errorf("scan pull request short: %w", err)
		}
		pr.Status = domain.PullRequestStatus(status)
		result = append(result, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pull requests short: %w", err)
	}

	return result, nil
}

func (r *Repository) ListRandomActiveTeamMembers(ctx context.Context, teamID int64, exclude []string, limit int) ([]domain.TeamMember, error) {
	if exclude == nil {
		exclude = []string{}
	}

	rows, err := r.pool.Query(ctx, `
		SELECT u.user_id, u.username, u.is_active
		FROM team_memberships tm
		JOIN users u ON u.user_id = tm.user_id
		WHERE tm.team_id = $1
		  AND u.is_active = TRUE
		  AND u.user_id <> ALL($2::text[])
		ORDER BY random()
		LIMIT $3
	`, teamID, exclude, limit)
	if err != nil {
		return nil, fmt.Errorf("select random team members: %w", err)
	}
	defer rows.Close()

	var members []domain.TeamMember
	for rows.Next() {
		var m domain.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return nil, fmt.Errorf("scan random member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate random members: %w", err)
	}

	return members, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
