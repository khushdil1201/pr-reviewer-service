package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/repository"
	"pr-reviewer-service/internal/service"
	"pr-reviewer-service/internal/testutil"
)

func setupHandler(t *testing.T) *Handler {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewRepository(pool)
	svc := service.NewService(repo)
	return NewHandler(svc)
}

func uniqueID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func TestHandler_CreateTeam(t *testing.T) {
	handler := setupHandler(t)

	t.Run("Create Team Successfully", func(t *testing.T) {
		teamName := uniqueID("engineering")
		reqBody := models.Team{
			TeamName: teamName,
			Members: []models.TeamMember{
				{UserID: uniqueID("e1"), Username: "Engineer1", IsActive: true},
				{UserID: uniqueID("e2"), Username: "Engineer2", IsActive: true},
			},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/team/add", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateTeam(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)

		team, ok := response["team"].(map[string]interface{})
		if !ok {
			t.Fatal("Response should contain 'team' field")
		}

		if team["team_name"] != teamName {
			t.Errorf("Expected team_name '%s', got %v", teamName, team["team_name"])
		}
	})

	t.Run("Create Duplicate Team", func(t *testing.T) {
		teamName := uniqueID("dupteam")
		reqBody := models.Team{
			TeamName: teamName,
			Members:  []models.TeamMember{},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/team/add", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreateTeam(w, req)

		body2, _ := json.Marshal(reqBody)
		req2 := httptest.NewRequest(http.MethodPost, "/team/add", bytes.NewReader(body2))
		w2 := httptest.NewRecorder()
		handler.CreateTeam(w2, req2)

		if w2.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w2.Code)
		}
	})

	t.Run("Invalid Request Body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/team/add", bytes.NewReader([]byte("invalid json")))
		w := httptest.NewRecorder()

		handler.CreateTeam(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})
}

func TestHandler_GetTeam(t *testing.T) {
	handler := setupHandler(t)
	ctx := context.Background()

	teamName := uniqueID("design")
	team := &models.Team{
		TeamName: teamName,
		Members: []models.TeamMember{
			{UserID: uniqueID("d1"), Username: "Designer1", IsActive: true},
		},
	}
	_, _ = handler.service.CreateTeam(ctx, team)

	t.Run("Get Existing Team", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/team/get?team_name=%s", teamName), nil)
		w := httptest.NewRecorder()

		handler.GetTeam(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response models.Team
		json.NewDecoder(w.Body).Decode(&response)

		if response.TeamName != teamName {
			t.Errorf("Expected team_name '%s', got %s", teamName, response.TeamName)
		}
	})

	t.Run("Get Non-Existent Team", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/team/get?team_name=%s", uniqueID("nonexistent")), nil)
		w := httptest.NewRecorder()

		handler.GetTeam(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("Missing team_name Parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/team/get", nil)
		w := httptest.NewRecorder()

		handler.GetTeam(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})
}

func TestHandler_CreatePR(t *testing.T) {
	handler := setupHandler(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	team := &models.Team{
		TeamName: fmt.Sprintf("prteam-%s", suffix),
		Members: []models.TeamMember{
			{UserID: fmt.Sprintf("pa1-%s", suffix), Username: "Author", IsActive: true},
			{UserID: fmt.Sprintf("pr1-%s", suffix), Username: "Reviewer1", IsActive: true},
			{UserID: fmt.Sprintf("pr2-%s", suffix), Username: "Reviewer2", IsActive: true},
		},
	}
	_, _ = handler.service.CreateTeam(ctx, team)

	t.Run("Create PR Successfully", func(t *testing.T) {
		reqBody := map[string]string{
			"pull_request_id":   fmt.Sprintf("pr-500-%s", suffix),
			"pull_request_name": "New Feature",
			"author_id":         fmt.Sprintf("pa1-%s", suffix),
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreatePR(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, w.Code, w.Body.String())
		}

		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)

		pr, ok := response["pr"].(map[string]interface{})
		if !ok {
			t.Fatal("Response should contain 'pr' field")
		}

		if pr["status"] != "OPEN" {
			t.Errorf("Expected status OPEN, got %v", pr["status"])
		}

		reviewers, ok := pr["assigned_reviewers"].([]interface{})
		if !ok || len(reviewers) != 2 {
			t.Errorf("Expected 2 reviewers, got %v", reviewers)
		}
	})

	t.Run("Create PR with Non-Existent Author", func(t *testing.T) {
		reqBody := map[string]string{
			"pull_request_id":   "pr-501",
			"pull_request_name": "Invalid PR",
			"author_id":         "nonexistent",
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.CreatePR(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})
}

func TestHandler_MergePR(t *testing.T) {
	handler := setupHandler(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	team := &models.Team{
		TeamName: fmt.Sprintf("mergeteam2-%s", suffix),
		Members: []models.TeamMember{
			{UserID: fmt.Sprintf("ma1-%s", suffix), Username: "Author", IsActive: true},
			{UserID: fmt.Sprintf("mr1-%s", suffix), Username: "Reviewer", IsActive: true},
		},
	}
	_, _ = handler.service.CreateTeam(ctx, team)
	prID := fmt.Sprintf("pr-600-%s", suffix)
	_, _ = handler.service.CreatePR(ctx, prID, "Merge Test", fmt.Sprintf("ma1-%s", suffix))

	t.Run("Merge PR Successfully", func(t *testing.T) {
		reqBody := map[string]string{
			"pull_request_id": prID,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/pullRequest/merge", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.MergePR(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)

		pr, ok := response["pr"].(map[string]interface{})
		if !ok {
			t.Fatal("Response should contain 'pr' field")
		}

		if pr["status"] != "MERGED" {
			t.Errorf("Expected status MERGED, got %v", pr["status"])
		}
	})

	t.Run("Merge Already Merged PR (Idempotent)", func(t *testing.T) {
		reqBody := map[string]string{
			"pull_request_id": prID,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/pullRequest/merge", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.MergePR(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d for idempotent merge, got %d", http.StatusOK, w.Code)
		}
	})
}

func TestHandler_SetUserActive(t *testing.T) {
	handler := setupHandler(t)
	ctx := context.Background()

	userID := uniqueID("au1")
	team := &models.Team{
		TeamName: uniqueID("activeteam"),
		Members: []models.TeamMember{
			{UserID: userID, Username: "ActiveUser", IsActive: true},
		},
	}
	_, _ = handler.service.CreateTeam(ctx, team)

	t.Run("Deactivate User", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"user_id":   userID,
			"is_active": false,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/users/setIsActive", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.SetUserActive(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)

		user, ok := response["user"].(map[string]interface{})
		if !ok {
			t.Fatal("Response should contain 'user' field")
		}

		if user["is_active"] != false {
			t.Errorf("Expected is_active to be false, got %v", user["is_active"])
		}
	})
}

func TestHandler_ReassignReviewer(t *testing.T) {
	handler := setupHandler(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	team1 := &models.Team{
		TeamName: fmt.Sprintf("reassignteam1-%s", suffix),
		Members: []models.TeamMember{
			{UserID: fmt.Sprintf("ra1-%s", suffix), Username: "Author", IsActive: true},
			{UserID: fmt.Sprintf("rr1-%s", suffix), Username: "Rev1", IsActive: true},
			{UserID: fmt.Sprintf("rr2-%s", suffix), Username: "Rev2", IsActive: true},
			{UserID: fmt.Sprintf("rr3-%s", suffix), Username: "Rev3", IsActive: true},
		},
	}
	_, _ = handler.service.CreateTeam(ctx, team1)

	prID := fmt.Sprintf("pr-700-%s", suffix)
	pr, err := handler.service.CreatePR(ctx, prID, "Reassign Test", fmt.Sprintf("ra1-%s", suffix))
	if err != nil || pr == nil || len(pr.AssignedReviewers) == 0 {
		t.Fatalf("Failed to create PR for test: %v", err)
	}
	oldReviewerID := pr.AssignedReviewers[0]

	t.Run("Reassign Reviewer Successfully", func(t *testing.T) {
		reqBody := map[string]string{
			"pull_request_id": prID,
			"old_user_id":     oldReviewerID,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/pullRequest/reassign", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ReassignReviewer(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
		}

		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)

		if response["replaced_by"] == nil {
			t.Error("Response should contain 'replaced_by' field")
		}
	})

	t.Run("Reassign on Merged PR", func(t *testing.T) {
		pr2ID := fmt.Sprintf("pr-701-%s", suffix)
		pr2, _ := handler.service.CreatePR(ctx, pr2ID, "Merged PR", fmt.Sprintf("ra1-%s", suffix))
		_, _ = handler.service.MergePR(ctx, pr2ID)

		reqBody := map[string]string{
			"pull_request_id": pr2ID,
			"old_user_id":     pr2.AssignedReviewers[0],
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/pullRequest/reassign", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ReassignReviewer(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("Expected status %d, got %d", http.StatusConflict, w.Code)
		}
	})
}

func TestHandler_GetUserReviews(t *testing.T) {
	handler := setupHandler(t)
	ctx := context.Background()

	// Setup: create team and PR
	authorID := uniqueID("va1")
	rev1ID := uniqueID("vr1")
	rev2ID := uniqueID("vr2")

	team := &models.Team{
		TeamName: uniqueID("reviewsteam"),
		Members: []models.TeamMember{
			{UserID: authorID, Username: "Author", IsActive: true},
			{UserID: rev1ID, Username: "Reviewer", IsActive: true},
			{UserID: rev2ID, Username: "Reviewer2", IsActive: true},
		},
	}
	_, _ = handler.service.CreateTeam(ctx, team)
	prID := uniqueID("pr-800")
	_, _ = handler.service.CreatePR(ctx, prID, "Review Test", authorID)

	t.Run("Get Reviews for Reviewer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/users/getReview?user_id=%s", rev1ID), nil)
		w := httptest.NewRecorder()

		handler.GetUserReviews(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response map[string]interface{}
		json.NewDecoder(w.Body).Decode(&response)

		if response["user_id"] != rev1ID {
			t.Errorf("Expected user_id 'vr1', got %v", response["user_id"])
		}

		if response["pull_requests"] == nil {
			t.Error("Response should contain 'pull_requests' field")
		}
	})

	t.Run("Missing user_id Parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/getReview", nil)
		w := httptest.NewRecorder()

		handler.GetUserReviews(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})
}
