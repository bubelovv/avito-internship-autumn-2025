package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bubelovv/avito-internship-autumn-2025/internal/domain"
	"github.com/bubelovv/avito-internship-autumn-2025/internal/service"
	"go.uber.org/zap"
)

type handler struct {
	svc    Service
	logger *zap.Logger
}

func (h *handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *handler) handleTeamAdd(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TeamName string `json:"team_name"`
		Members  []struct {
			UserID   string `json:"user_id"`
			Username string `json:"username"`
			IsActive bool   `json:"is_active"`
		} `json:"members"`
	}

	if err := decodeJSON(r.Context(), r.Body, &req); err != nil {
		writeValidationError(w, err)
		return
	}
	if req.TeamName == "" {
		writeValidationError(w, errors.New("team_name is required"))
		return
	}

	members := make([]domain.TeamMember, 0, len(req.Members))
	for _, m := range req.Members {
		if m.UserID == "" || m.Username == "" {
			writeValidationError(w, errors.New("members.user_id and members.username are required"))
			return
		}
		members = append(members, domain.TeamMember{
			UserID:   m.UserID,
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}

	team, err := h.svc.CreateTeam(r.Context(), req.TeamName, members)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"team": mapTeam(team),
	})
}

func (h *handler) handleTeamGet(w http.ResponseWriter, r *http.Request) {
	teamName := strings.TrimSpace(r.URL.Query().Get("team_name"))
	if teamName == "" {
		writeValidationError(w, errors.New("team_name query parameter is required"))
		return
	}

	team, err := h.svc.GetTeam(r.Context(), teamName)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapTeam(team))
}

func (h *handler) handleUserSetActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := decodeJSON(r.Context(), r.Body, &req); err != nil {
		writeValidationError(w, err)
		return
	}
	if req.UserID == "" {
		writeValidationError(w, errors.New("user_id is required"))
		return
	}

	user, err := h.svc.SetUserActivity(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": mapUser(user),
	})
}

func (h *handler) handlePullRequestCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"pull_request_id"`
		Name     string `json:"pull_request_name"`
		AuthorID string `json:"author_id"`
	}
	if err := decodeJSON(r.Context(), r.Body, &req); err != nil {
		writeValidationError(w, err)
		return
	}
	if req.ID == "" || req.Name == "" || req.AuthorID == "" {
		writeValidationError(w, errors.New("pull_request_id, pull_request_name and author_id are required"))
		return
	}

	pr, err := h.svc.CreatePullRequest(r.Context(), req.ID, req.Name, req.AuthorID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"pr": mapPullRequest(pr),
	})
}

func (h *handler) handlePullRequestMerge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"pull_request_id"`
	}
	if err := decodeJSON(r.Context(), r.Body, &req); err != nil {
		writeValidationError(w, err)
		return
	}
	if req.ID == "" {
		writeValidationError(w, errors.New("pull_request_id is required"))
		return
	}

	pr, err := h.svc.MergePullRequest(r.Context(), req.ID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pr": mapPullRequest(pr),
	})
}

func (h *handler) handlePullRequestReassign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID            string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
		OldReviewerID string `json:"old_reviewer_id"`
	}
	if err := decodeJSON(r.Context(), r.Body, &req); err != nil {
		writeValidationError(w, err)
		return
	}
	oldReviewer := req.OldUserID
	if oldReviewer == "" {
		oldReviewer = req.OldReviewerID
	}
	if req.ID == "" || oldReviewer == "" {
		writeValidationError(w, errors.New("pull_request_id and old_user_id are required"))
		return
	}

	pr, replacedBy, err := h.svc.ReassignReviewer(r.Context(), req.ID, oldReviewer)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"pr":          mapPullRequest(pr),
		"replaced_by": replacedBy,
	})
}

func (h *handler) handleUserGetReview(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeValidationError(w, errors.New("user_id query parameter is required"))
		return
	}

	prs, err := h.svc.ListReviewerPullRequests(r.Context(), userID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":       userID,
		"pull_requests": mapPullRequestShortList(prs),
	})
}

func (h *handler) writeServiceError(w http.ResponseWriter, err error) {
	status, code := mapServiceError(err)
	if status >= http.StatusInternalServerError {
		h.logger.Error("service error", zap.Error(err))
	}
	writeError(w, status, code, err.Error())
}

func mapServiceError(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrTeamExists):
		return http.StatusBadRequest, "TEAM_EXISTS"
	case errors.Is(err, service.ErrTeamNotFound),
		errors.Is(err, service.ErrUserNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, service.ErrPullRequestExists):
		return http.StatusConflict, "PR_EXISTS"
	case errors.Is(err, service.ErrPullRequestNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, service.ErrPullRequestMerged):
		return http.StatusConflict, "PR_MERGED"
	case errors.Is(err, service.ErrReviewerNotAssigned):
		return http.StatusConflict, "NOT_ASSIGNED"
	case errors.Is(err, service.ErrNoCandidate):
		return http.StatusConflict, "NO_CANDIDATE"
	default:
		return http.StatusInternalServerError, "NOT_FOUND"
	}
}

func mapTeam(team domain.Team) map[string]any {
	members := make([]map[string]any, 0, len(team.Members))
	for _, m := range team.Members {
		members = append(members, map[string]any{
			"user_id":   m.UserID,
			"username":  m.Username,
			"is_active": m.IsActive,
		})
	}
	return map[string]any{
		"team_name": team.Name,
		"members":   members,
	}
}

func mapUser(u domain.User) map[string]any {
	teamName := ""
	if u.TeamName != nil {
		teamName = *u.TeamName
	}
	return map[string]any{
		"user_id":   u.ID,
		"username":  u.Username,
		"team_name": teamName,
		"is_active": u.IsActive,
	}
}

func mapPullRequest(pr domain.PullRequest) map[string]any {
	resp := map[string]any{
		"pull_request_id":    pr.ID,
		"pull_request_name":  pr.Name,
		"author_id":          pr.AuthorID,
		"status":             string(pr.Status),
		"assigned_reviewers": pr.Reviewers,
	}
	if !pr.CreatedAt.IsZero() {
		resp["createdAt"] = formatTime(pr.CreatedAt)
	}
	if pr.MergedAt != nil {
		resp["mergedAt"] = formatTime(*pr.MergedAt)
	}
	return resp
}

func mapPullRequestShortList(prs []domain.PullRequestShort) []map[string]any {
	result := make([]map[string]any, 0, len(prs))
	for _, pr := range prs {
		result = append(result, map[string]any{
			"pull_request_id":   pr.ID,
			"pull_request_name": pr.Name,
			"author_id":         pr.AuthorID,
			"status":            string(pr.Status),
		})
	}
	return result
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func decodeJSON(ctx context.Context, body io.ReadCloser, dst any) error {
	defer body.Close()
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected extra JSON input")
		}
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func writeValidationError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusBadRequest, "NOT_FOUND", err.Error())
}
