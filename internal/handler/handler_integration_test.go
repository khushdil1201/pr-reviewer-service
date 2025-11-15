package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/repository"
	"pr-reviewer-service/internal/service"
	"pr-reviewer-service/internal/testutil"
)

func TestIntegration_CompleteWorkflow(t *testing.T) {
	db := testutil.SetupTestDB(t)

	repo := repository.NewRepository(db)
	svc := service.NewService(repo)
	h := NewHandler(svc)
	router := h.SetupRoutes()

	t.Run("Complete workflow: create team, create PR, get statistics", func(t *testing.T) {
		teamData := models.Team{
			TeamName: "engineering",
			Members: []models.TeamMember{
				{UserID: "dev1", Username: "Developer 1", IsActive: true},
				{UserID: "dev2", Username: "Developer 2", IsActive: true},
				{UserID: "dev3", Username: "Developer 3", IsActive: true},
			},
		}

		body, _ := json.Marshal(teamData)
		req := httptest.NewRequest("POST", "/team/add", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
		}

		prData := map[string]string{
			"pull_request_id":   "pr-test-1",
			"pull_request_name": "Test PR",
			"author_id":         "dev1",
		}
		body, _ = json.Marshal(prData)
		req = httptest.NewRequest("POST", "/pullRequest/create", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
		}

		var prResp struct {
			PR models.PullRequest `json:"pr"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &prResp); err != nil {
			t.Fatalf("failed to unmarshal PR response: %v", err)
		}

		if len(prResp.PR.AssignedReviewers) == 0 {
			t.Fatal("expected assigned reviewers, got none")
		}

		req = httptest.NewRequest("GET", "/statistics", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var stats models.Statistics
		if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
			t.Fatalf("failed to unmarshal statistics: %v", err)
		}

		if stats.TotalPRs != 1 {
			t.Errorf("expected 1 PR, got %d", stats.TotalPRs)
		}

		if stats.OpenPRs != 1 {
			t.Errorf("expected 1 open PR, got %d", stats.OpenPRs)
		}
	})
}

func TestIntegration_DeactivateTeamWithReassignment(t *testing.T) {
	db := testutil.SetupTestDB(t)

	repo := repository.NewRepository(db)
	svc := service.NewService(repo)
	h := NewHandler(svc)
	router := h.SetupRoutes()

	t.Run("Deactivate team and reassign open PRs", func(t *testing.T) {
		team1 := models.Team{
			TeamName: "team-alpha",
			Members: []models.TeamMember{
				{UserID: "alpha1", Username: "Alpha User 1", IsActive: true},
				{UserID: "alpha2", Username: "Alpha User 2", IsActive: true},
				{UserID: "alpha3", Username: "Alpha User 3", IsActive: true},
			},
		}

		body, _ := json.Marshal(team1)
		req := httptest.NewRequest("POST", "/team/add", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create team-alpha: %s", w.Body.String())
		}

		team2 := models.Team{
			TeamName: "team-beta",
			Members: []models.TeamMember{
				{UserID: "beta1", Username: "Beta User 1", IsActive: true},
				{UserID: "beta2", Username: "Beta User 2", IsActive: true},
			},
		}

		body, _ = json.Marshal(team2)
		req = httptest.NewRequest("POST", "/team/add", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create team-beta: %s", w.Body.String())
		}

		prData := map[string]string{
			"pull_request_id":   "pr-deactivate-test",
			"pull_request_name": "Test Deactivation",
			"author_id":         "alpha1",
		}
		body, _ = json.Marshal(prData)
		req = httptest.NewRequest("POST", "/pullRequest/create", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create PR: %s", w.Body.String())
		}

		deactivateData := map[string]string{
			"team_name": "team-alpha",
		}
		body, _ = json.Marshal(deactivateData)
		req = httptest.NewRequest("POST", "/team/deactivateAll", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var deactivateResp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &deactivateResp); err != nil {
			t.Fatalf("failed to unmarshal deactivate response: %v", err)
		}

		deactivatedCount := int(deactivateResp["deactivated_users"].(float64))
		if deactivatedCount != 3 {
			t.Errorf("expected 3 deactivated users, got %d", deactivatedCount)
		}
	})
}

func TestIntegration_ReassignReviewer(t *testing.T) {
	db := testutil.SetupTestDB(t)

	repo := repository.NewRepository(db)
	svc := service.NewService(repo)
	h := NewHandler(svc)
	router := h.SetupRoutes()

	ctx := context.Background()

	teamData := models.Team{
		TeamName: "dev-team",
		Members: []models.TeamMember{
			{UserID: "user1", Username: "User 1", IsActive: true},
			{UserID: "user2", Username: "User 2", IsActive: true},
			{UserID: "user3", Username: "User 3", IsActive: true},
			{UserID: "user4", Username: "User 4", IsActive: true},
		},
	}

	body, _ := json.Marshal(teamData)
	req := httptest.NewRequest("POST", "/team/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create team: %s", w.Body.String())
	}

	prData := map[string]string{
		"pull_request_id":   "pr-reassign",
		"pull_request_name": "Test Reassignment",
		"author_id":         "user1",
	}
	body, _ = json.Marshal(prData)
	req = httptest.NewRequest("POST", "/pullRequest/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create PR: %s", w.Body.String())
	}

	var prResp struct {
		PR models.PullRequest `json:"pr"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &prResp); err != nil {
		t.Fatalf("failed to unmarshal PR response: %v", err)
	}

	if len(prResp.PR.AssignedReviewers) == 0 {
		t.Fatal("expected assigned reviewers")
	}

	oldReviewer := prResp.PR.AssignedReviewers[0]

	reassignData := map[string]string{
		"pull_request_id": "pr-reassign",
		"old_user_id":     oldReviewer,
	}
	body, _ = json.Marshal(reassignData)
	req = httptest.NewRequest("POST", "/pullRequest/reassign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	updatedPR, err := repo.GetPR(ctx, "pr-reassign")
	if err != nil {
		t.Fatalf("failed to get updated PR: %v", err)
	}

	found := false
	for _, reviewer := range updatedPR.AssignedReviewers {
		if reviewer == oldReviewer {
			found = true
			break
		}
	}

	if found {
		t.Error("old reviewer should have been replaced")
	}
}

func TestIntegration_MergePRIdempotence(t *testing.T) {
	db := testutil.SetupTestDB(t)

	repo := repository.NewRepository(db)
	svc := service.NewService(repo)
	h := NewHandler(svc)
	router := h.SetupRoutes()

	teamData := models.Team{
		TeamName: "merge-team",
		Members: []models.TeamMember{
			{UserID: "m1", Username: "Merger 1", IsActive: true},
			{UserID: "m2", Username: "Merger 2", IsActive: true},
		},
	}

	body, _ := json.Marshal(teamData)
	req := httptest.NewRequest("POST", "/team/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	prData := map[string]string{
		"pull_request_id":   "pr-merge-test",
		"pull_request_name": "Test Merge Idempotence",
		"author_id":         "m1",
	}
	body, _ = json.Marshal(prData)
	req = httptest.NewRequest("POST", "/pullRequest/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	mergeData := map[string]string{
		"pull_request_id": "pr-merge-test",
	}
	body, _ = json.Marshal(mergeData)

	for i := 0; i < 3; i++ {
		req = httptest.NewRequest("POST", "/pullRequest/merge", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("iteration %d: expected status 200, got %d: %s", i, w.Code, w.Body.String())
		}

		var prResp struct {
			PR models.PullRequest `json:"pr"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &prResp); err != nil {
			t.Fatalf("iteration %d: failed to unmarshal response: %v", i, err)
		}

		if prResp.PR.Status != models.PRStatusMerged {
			t.Errorf("iteration %d: expected status MERGED, got %s", i, prResp.PR.Status)
		}
	}
}

func BenchmarkStatistics(b *testing.B) {
	db := testutil.SetupTestDB(nil)

	repo := repository.NewRepository(db)
	svc := service.NewService(repo)
	h := NewHandler(svc)
	router := h.SetupRoutes()

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		teamName := fmt.Sprintf("team-%d", i)
		if err := repo.CreateTeam(ctx, teamName); err != nil {
			b.Fatal(err)
		}

		for j := 0; j < 10; j++ {
			user := &models.User{
				UserID:   fmt.Sprintf("user-%d-%d", i, j),
				Username: fmt.Sprintf("User %d-%d", i, j),
				TeamName: teamName,
				IsActive: true,
			}
			if err := repo.UpsertUser(ctx, user); err != nil {
				b.Fatal(err)
			}
		}
	}

	for i := 0; i < 50; i++ {
		pr := &models.PullRequest{
			PullRequestID:   fmt.Sprintf("pr-%d", i),
			PullRequestName: fmt.Sprintf("PR %d", i),
			AuthorID:        fmt.Sprintf("user-%d-%d", i%10, i%10),
			Status:          models.PRStatusOpen,
			AssignedReviewers: []string{
				fmt.Sprintf("user-%d-%d", (i+1)%10, (i+1)%10),
				fmt.Sprintf("user-%d-%d", (i+2)%10, (i+2)%10),
			},
		}
		if err := repo.CreatePR(ctx, pr); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/statistics", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("expected status 200, got %d", w.Code)
		}
	}
}
