package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/service"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *service.Service
}

func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) writeError(w http.ResponseWriter, statusCode int, errCode models.ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: models.ErrorDetail{
			Code:    errCode,
			Message: message,
		},
	}); err != nil {
		log.Printf("ERROR: failed to encode error response: %v", err)
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: failed to encode JSON response: %v", err)
	}
}

func (h *Handler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	var req models.Team
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, models.ErrorNotFound, "invalid request body")
		return
	}

	team, err := h.service.CreateTeam(r.Context(), &req)
	if err != nil {
		if err.Error() == "TEAM_EXISTS" {
			h.writeError(w, http.StatusBadRequest, models.ErrorTeamExists, "team_name already exists")
			return
		}
		h.writeError(w, http.StatusInternalServerError, models.ErrorNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{"team": team})
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.writeError(w, http.StatusBadRequest, models.ErrorNotFound, "team_name is required")
		return
	}

	team, err := h.service.GetTeam(r.Context(), teamName)
	if err != nil {
		h.writeError(w, http.StatusNotFound, models.ErrorNotFound, "team not found")
		return
	}

	h.writeJSON(w, http.StatusOK, team)
}

func (h *Handler) SetUserActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, models.ErrorNotFound, "invalid request body")
		return
	}

	user, err := h.service.SetUserActive(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		h.writeError(w, http.StatusNotFound, models.ErrorNotFound, "user not found")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorID        string `json:"author_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, models.ErrorNotFound, "invalid request body")
		return
	}

	pr, err := h.service.CreatePR(r.Context(), req.PullRequestID, req.PullRequestName, req.AuthorID)
	if err != nil {
		if err.Error() == "PR_EXISTS" {
			h.writeError(w, http.StatusConflict, models.ErrorPRExists, "PR id already exists")
			return
		}
		if err.Error() == "NOT_FOUND" {
			h.writeError(w, http.StatusNotFound, models.ErrorNotFound, "author not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, models.ErrorNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, map[string]interface{}{"pr": pr})
}

func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, models.ErrorNotFound, "invalid request body")
		return
	}

	pr, err := h.service.MergePR(r.Context(), req.PullRequestID)
	if err != nil {
		if err.Error() == "NOT_FOUND" {
			h.writeError(w, http.StatusNotFound, models.ErrorNotFound, "PR not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, models.ErrorNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{"pr": pr})
}

func (h *Handler) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, models.ErrorNotFound, "invalid request body")
		return
	}

	pr, newReviewerID, err := h.service.ReassignReviewer(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		errMsg := err.Error()
		switch errMsg {
		case "NOT_FOUND":
			h.writeError(w, http.StatusNotFound, models.ErrorNotFound, "PR or user not found")
		case "PR_MERGED":
			h.writeError(w, http.StatusConflict, models.ErrorPRMerged, "cannot reassign on merged PR")
		case "NOT_ASSIGNED":
			h.writeError(w, http.StatusConflict, models.ErrorNotAssigned, "reviewer is not assigned to this PR")
		case "NO_CANDIDATE":
			h.writeError(w, http.StatusConflict, models.ErrorNoCandidate, "no active replacement candidate in team")
		default:
			h.writeError(w, http.StatusInternalServerError, models.ErrorNotFound, errMsg)
		}
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"pr":          pr,
		"replaced_by": newReviewerID,
	})
}

func (h *Handler) GetUserReviews(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.writeError(w, http.StatusBadRequest, models.ErrorNotFound, "user_id is required")
		return
	}

	prs, err := h.service.GetUserReviews(r.Context(), userID)
	if err != nil {
		if err.Error() == "NOT_FOUND" {
			h.writeError(w, http.StatusNotFound, models.ErrorNotFound, "user not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, models.ErrorNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       userID,
		"pull_requests": prs,
	})
}

func (h *Handler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetStatistics(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, models.ErrorNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) DeactivateTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TeamName string `json:"team_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, models.ErrorNotFound, "invalid request body")
		return
	}

	if req.TeamName == "" {
		h.writeError(w, http.StatusBadRequest, models.ErrorNotFound, "team_name is required")
		return
	}

	deactivatedCount, reassignedCount, err := h.service.DeactivateTeamAndReassign(r.Context(), req.TeamName)
	if err != nil {
		if err.Error() == "NOT_FOUND" {
			h.writeError(w, http.StatusNotFound, models.ErrorNotFound, "team not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, models.ErrorNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"team_name":          req.TeamName,
		"deactivated_users":  deactivatedCount,
		"reassigned_reviews": reassignedCount,
	})
}

func (h *Handler) SetupRoutes() http.Handler {
	r := chi.NewRouter()

	r.Post("/team/add", h.CreateTeam)
	r.Get("/team/get", h.GetTeam)
	r.Post("/team/deactivateAll", h.DeactivateTeam)
	r.Post("/users/setIsActive", h.SetUserActive)
	r.Post("/pullRequest/create", h.CreatePR)
	r.Post("/pullRequest/merge", h.MergePR)
	r.Post("/pullRequest/reassign", h.ReassignReviewer)
	r.Get("/users/getReview", h.GetUserReviews)
	r.Get("/statistics", h.GetStatistics)

	return r
}
