package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/testutil"
)

func uniqueID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func TestRepository_TeamOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	t.Run("Create and Get Team", func(t *testing.T) {
		teamName := fmt.Sprintf("backend-test-%d", time.Now().UnixNano())

		err := repo.CreateTeam(ctx, teamName)
		if err != nil {
			t.Fatalf("Failed to create team: %v", err)
		}

		exists, err := repo.TeamExists(ctx, teamName)
		if err != nil {
			t.Fatalf("Failed to check team exists: %v", err)
		}
		if !exists {
			t.Error("Team should exist after creation")
		}

		users := []models.User{
			{UserID: uniqueID("u1"), Username: "Alice", TeamName: teamName, IsActive: true},
			{UserID: uniqueID("u2"), Username: "Bob", TeamName: teamName, IsActive: true},
			{UserID: uniqueID("u3"), Username: "Charlie", TeamName: teamName, IsActive: false},
		}

		for _, user := range users {
			if upsertErr := repo.UpsertUser(ctx, &user); upsertErr != nil {
				t.Fatalf("Failed to upsert user: %v", upsertErr)
			}
		}

		team, err := repo.GetTeam(ctx, teamName)
		if err != nil {
			t.Fatalf("Failed to get team: %v", err)
		}

		if team.TeamName != teamName {
			t.Errorf("Expected team name %s, got %s", teamName, team.TeamName)
		}

		if len(team.Members) != 3 {
			t.Errorf("Expected 3 members, got %d", len(team.Members))
		}
	})

	t.Run("Get Non-Existent Team", func(t *testing.T) {
		_, err := repo.GetTeam(ctx, "nonexistent")
		if err == nil {
			t.Error("Expected error for non-existent team")
		}
	})
}

func TestRepository_UserOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	teamName := fmt.Sprintf("testteam-%d", time.Now().UnixNano())
	_ = repo.CreateTeam(ctx, teamName)

	t.Run("Create and Get User", func(t *testing.T) {
		userID := uniqueID("u100")
		user := &models.User{
			UserID:   userID,
			Username: "TestUser",
			TeamName: teamName,
			IsActive: true,
		}

		err := repo.UpsertUser(ctx, user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		retrieved, err := repo.GetUser(ctx, user.UserID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		if retrieved.Username != user.Username {
			t.Errorf("Expected username %s, got %s", user.Username, retrieved.Username)
		}
	})

	t.Run("Update User Active Status", func(t *testing.T) {
		userID := uniqueID("u100")
		// Create user first
		user := &models.User{
			UserID:   userID,
			Username: "UpdateTest",
			TeamName: teamName,
			IsActive: true,
		}
		_ = repo.UpsertUser(ctx, user)

		err := repo.UpdateUserActive(ctx, userID, false)
		if err != nil {
			t.Fatalf("Failed to update user active status: %v", err)
		}

		user, err = repo.GetUser(ctx, userID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		if user.IsActive {
			t.Error("User should be inactive")
		}
	})

	t.Run("Get Active Team Members", func(t *testing.T) {
		// Create additional users
		users := []models.User{
			{UserID: uniqueID("u101"), Username: "Active1", TeamName: teamName, IsActive: true},
			{UserID: uniqueID("u102"), Username: "Active2", TeamName: teamName, IsActive: true},
			{UserID: uniqueID("u103"), Username: "Inactive", TeamName: teamName, IsActive: false},
		}

		for _, u := range users {
			_ = repo.UpsertUser(ctx, &u)
		}

		activeMembers, err := repo.GetActiveTeamMembers(ctx, teamName, "")
		if err != nil {
			t.Fatalf("Failed to get active members: %v", err)
		}

		if len(activeMembers) < 2 {
			t.Errorf("Expected at least 2 active members, got %d", len(activeMembers))
		}
	})
}

func TestRepository_PROperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	teamName := fmt.Sprintf("prteam-%d", time.Now().UnixNano())
	_ = repo.CreateTeam(ctx, teamName)

	authorID := uniqueID("author1")
	reviewer1ID := uniqueID("reviewer1")
	reviewer2ID := uniqueID("reviewer2")

	users := []models.User{
		{UserID: authorID, Username: "Author", TeamName: teamName, IsActive: true},
		{UserID: reviewer1ID, Username: "Reviewer1", TeamName: teamName, IsActive: true},
		{UserID: reviewer2ID, Username: "Reviewer2", TeamName: teamName, IsActive: true},
	}

	for _, u := range users {
		_ = repo.UpsertUser(ctx, &u)
	}

	t.Run("Create and Get PR", func(t *testing.T) {
		prID := uniqueID("pr-001")
		pr := &models.PullRequest{
			PullRequestID:     prID,
			PullRequestName:   "Test PR",
			AuthorID:          authorID,
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{reviewer1ID, reviewer2ID},
		}

		err := repo.CreatePR(ctx, pr)
		if err != nil {
			t.Fatalf("Failed to create PR: %v", err)
		}

		exists, err := repo.PRExists(ctx, pr.PullRequestID)
		if err != nil {
			t.Fatalf("Failed to check PR exists: %v", err)
		}
		if !exists {
			t.Error("PR should exist after creation")
		}

		retrieved, err := repo.GetPR(ctx, pr.PullRequestID)
		if err != nil {
			t.Fatalf("Failed to get PR: %v", err)
		}

		if retrieved.PullRequestName != pr.PullRequestName {
			t.Errorf("Expected PR name %s, got %s", pr.PullRequestName, retrieved.PullRequestName)
		}

		if len(retrieved.AssignedReviewers) != 2 {
			t.Errorf("Expected 2 reviewers, got %d", len(retrieved.AssignedReviewers))
		}
	})

	t.Run("Merge PR", func(t *testing.T) {
		prID := uniqueID("pr-merge")
		pr := &models.PullRequest{
			PullRequestID:     prID,
			PullRequestName:   "Merge Test",
			AuthorID:          authorID,
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{reviewer1ID},
		}
		_ = repo.CreatePR(ctx, pr)

		err := repo.MergePR(ctx, prID)
		if err != nil {
			t.Fatalf("Failed to merge PR: %v", err)
		}

		pr, err = repo.GetPR(ctx, prID)
		if err != nil {
			t.Fatalf("Failed to get PR: %v", err)
		}

		if pr.Status != models.PRStatusMerged {
			t.Errorf("Expected status MERGED, got %s", pr.Status)
		}

		if pr.MergedAt == nil {
			t.Error("MergedAt should be set")
		}
	})

	t.Run("Replace Reviewer", func(t *testing.T) {
		// Create new PR
		prID := uniqueID("pr-002")
		pr := &models.PullRequest{
			PullRequestID:     prID,
			PullRequestName:   "Test PR 2",
			AuthorID:          authorID,
			Status:            models.PRStatusOpen,
			AssignedReviewers: []string{reviewer1ID},
		}
		_ = repo.CreatePR(ctx, pr)

		assigned, err := repo.IsReviewerAssigned(ctx, prID, reviewer1ID)
		if err != nil {
			t.Fatalf("Failed to check reviewer assigned: %v", err)
		}
		if !assigned {
			t.Error("Reviewer1 should be assigned")
		}

		err = repo.ReplaceReviewer(ctx, prID, reviewer1ID, reviewer2ID)
		if err != nil {
			t.Fatalf("Failed to replace reviewer: %v", err)
		}

		assigned, _ = repo.IsReviewerAssigned(ctx, prID, reviewer1ID)
		if assigned {
			t.Error("Reviewer1 should not be assigned after replacement")
		}

		assigned, _ = repo.IsReviewerAssigned(ctx, prID, reviewer2ID)
		if !assigned {
			t.Error("Reviewer2 should be assigned after replacement")
		}
	})

	t.Run("Get PRs For Reviewer", func(t *testing.T) {
		prs, err := repo.GetPRsForReviewer(ctx, reviewer2ID)
		if err != nil {
			t.Fatalf("Failed to get PRs for reviewer: %v", err)
		}

		if len(prs) < 1 {
			t.Errorf("Expected at least 1 PR for reviewer2, got %d", len(prs))
		}
	})
}
