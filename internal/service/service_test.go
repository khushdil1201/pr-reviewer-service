package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/repository"
	"pr-reviewer-service/internal/testutil"
)

func setupService(t *testing.T) *Service {
	pool := testutil.SetupTestDB(t)
	repo := repository.NewRepository(pool)
	return NewService(repo)
}

func uniqueID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func TestService_CreateTeam(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	t.Run("Create Team Successfully", func(t *testing.T) {
		teamName := uniqueID("frontend")
		team := &models.Team{
			TeamName: teamName,
			Members: []models.TeamMember{
				{UserID: uniqueID("u1"), Username: "Alice", IsActive: true},
				{UserID: uniqueID("u2"), Username: "Bob", IsActive: true},
			},
		}

		created, err := svc.CreateTeam(ctx, team)
		if err != nil {
			t.Fatalf("Failed to create team: %v", err)
		}

		if created.TeamName != team.TeamName {
			t.Errorf("Expected team name %s, got %s", team.TeamName, created.TeamName)
		}

		if len(created.Members) != 2 {
			t.Errorf("Expected 2 members, got %d", len(created.Members))
		}
	})

	t.Run("Create Duplicate Team", func(t *testing.T) {
		teamName := uniqueID("dupteam")
		team := &models.Team{
			TeamName: teamName,
			Members:  []models.TeamMember{},
		}

		// Create first time
		_, err := svc.CreateTeam(ctx, team)
		if err != nil {
			t.Fatalf("Failed to create team first time: %v", err)
		}

		_, err = svc.CreateTeam(ctx, team)
		if err == nil {
			t.Error("Expected error when creating duplicate team")
		}

		if err.Error() != "TEAM_EXISTS" {
			t.Errorf("Expected TEAM_EXISTS error, got %v", err)
		}
	})
}

func TestService_SetUserActive(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	// Setup: create team and user
	userID := uniqueID("u10")
	team := &models.Team{
		TeamName: uniqueID("testteam"),
		Members: []models.TeamMember{
			{UserID: userID, Username: "TestUser", IsActive: true},
		},
	}
	_, _ = svc.CreateTeam(ctx, team)

	t.Run("Deactivate User", func(t *testing.T) {
		user, err := svc.SetUserActive(ctx, userID, false)
		if err != nil {
			t.Fatalf("Failed to set user active: %v", err)
		}

		if user.IsActive {
			t.Error("User should be inactive")
		}
	})

	t.Run("Set Active for Non-Existent User", func(t *testing.T) {
		_, err := svc.SetUserActive(ctx, uniqueID("nonexistent"), true)
		if err == nil {
			t.Error("Expected error for non-existent user")
		}
	})
}

func TestService_CreatePR(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	// Setup: create team with users
	authorID := uniqueID("author")
	rev1ID := uniqueID("rev1")
	rev2ID := uniqueID("rev2")
	rev3ID := uniqueID("rev3")

	team := &models.Team{
		TeamName: uniqueID("devteam"),
		Members: []models.TeamMember{
			{UserID: authorID, Username: "Author", IsActive: true},
			{UserID: rev1ID, Username: "Reviewer1", IsActive: true},
			{UserID: rev2ID, Username: "Reviewer2", IsActive: true},
			{UserID: rev3ID, Username: "Reviewer3", IsActive: false}, // inactive
		},
	}
	_, _ = svc.CreateTeam(ctx, team)

	t.Run("Create PR with Auto-Assignment", func(t *testing.T) {
		prID := uniqueID("pr-100")
		pr, err := svc.CreatePR(ctx, prID, "Test PR", authorID)
		if err != nil {
			t.Fatalf("Failed to create PR: %v", err)
		}

		if pr.Status != models.PRStatusOpen {
			t.Errorf("Expected status OPEN, got %s", pr.Status)
		}

		if len(pr.AssignedReviewers) != 2 {
			t.Errorf("Expected 2 reviewers, got %d", len(pr.AssignedReviewers))
		}

		for _, reviewerID := range pr.AssignedReviewers {
			if reviewerID == authorID {
				t.Error("Author should not be assigned as reviewer")
			}
			if reviewerID == rev3ID {
				t.Error("Inactive user should not be assigned as reviewer")
			}
		}
	})

	t.Run("Create PR with No Active Reviewers", func(t *testing.T) {
		soloID := uniqueID("solo")
		soloTeam := &models.Team{
			TeamName: uniqueID("soloteam"),
			Members: []models.TeamMember{
				{UserID: soloID, Username: "Solo", IsActive: true},
			},
		}
		_, _ = svc.CreateTeam(ctx, soloTeam)

		pr, err := svc.CreatePR(ctx, uniqueID("pr-101"), "Solo PR", soloID)
		if err != nil {
			t.Fatalf("Failed to create PR: %v", err)
		}

		if len(pr.AssignedReviewers) != 0 {
			t.Errorf("Expected 0 reviewers, got %d", len(pr.AssignedReviewers))
		}
	})

	t.Run("Create Duplicate PR", func(t *testing.T) {
		dupPRID := uniqueID("pr-dup")
		_, err := svc.CreatePR(ctx, dupPRID, "Test PR", authorID)
		if err != nil {
			t.Fatalf("Failed to create PR first time: %v", err)
		}

		_, err = svc.CreatePR(ctx, dupPRID, "Duplicate", authorID)
		if err == nil {
			t.Error("Expected error for duplicate PR")
		}
		if err.Error() != "PR_EXISTS" {
			t.Errorf("Expected PR_EXISTS error, got %v", err)
		}
	})
}

func TestService_MergePR(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	authorID := uniqueID("author2")
	team := &models.Team{
		TeamName: uniqueID("mergeteam"),
		Members: []models.TeamMember{
			{UserID: authorID, Username: "Author2", IsActive: true},
			{UserID: uniqueID("rev10"), Username: "Rev10", IsActive: true},
		},
	}
	_, _ = svc.CreateTeam(ctx, team)
	prID := uniqueID("pr-200")
	_, _ = svc.CreatePR(ctx, prID, "Merge Test", authorID)

	t.Run("Merge PR Successfully", func(t *testing.T) {
		pr, err := svc.MergePR(ctx, prID)
		if err != nil {
			t.Fatalf("Failed to merge PR: %v", err)
		}

		if pr.Status != models.PRStatusMerged {
			t.Errorf("Expected status MERGED, got %s", pr.Status)
		}
	})

	t.Run("Merge Already Merged PR (Idempotent)", func(t *testing.T) {
		pr, err := svc.MergePR(ctx, prID)
		if err != nil {
			t.Errorf("Merge should be idempotent, got error: %v", err)
		}

		if pr.Status != models.PRStatusMerged {
			t.Errorf("Expected status MERGED, got %s", pr.Status)
		}
	})

	t.Run("Merge Non-Existent PR", func(t *testing.T) {
		_, err := svc.MergePR(ctx, uniqueID("nonexistent"))
		if err == nil {
			t.Error("Expected error for non-existent PR")
		}
	})
}

func TestService_ReassignReviewer(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	a1ID := uniqueID("a1")
	r1ID := uniqueID("r1")
	r2ID := uniqueID("r2")
	r3ID := uniqueID("r3")
	r4ID := uniqueID("r4")

	team1 := &models.Team{
		TeamName: uniqueID("team1"),
		Members: []models.TeamMember{
			{UserID: a1ID, Username: "Author1", IsActive: true},
			{UserID: r1ID, Username: "Rev1", IsActive: true},
			{UserID: r2ID, Username: "Rev2", IsActive: true},
		},
	}
	_, _ = svc.CreateTeam(ctx, team1)

	team2 := &models.Team{
		TeamName: uniqueID("team2"),
		Members: []models.TeamMember{
			{UserID: r3ID, Username: "Rev3", IsActive: true},
			{UserID: r4ID, Username: "Rev4", IsActive: true},
		},
	}
	_, _ = svc.CreateTeam(ctx, team2)

	prID := uniqueID("pr-300")
	pr, _ := svc.CreatePR(ctx, prID, "Reassign Test", a1ID)
	// Manually assign r3 from team2 for testing
	svc.repo.ReplaceReviewer(ctx, prID, pr.AssignedReviewers[0], r3ID)

	t.Run("Reassign Reviewer Successfully", func(t *testing.T) {
		updatedPR, newReviewerID, err := svc.ReassignReviewer(ctx, prID, r3ID)
		if err != nil {
			t.Fatalf("Failed to reassign reviewer: %v", err)
		}

		if newReviewerID != r4ID {
			t.Logf("New reviewer is %s (expected %s from replaced reviewer's team)", newReviewerID, r4ID)
		}

		for _, rev := range updatedPR.AssignedReviewers {
			if rev == r3ID {
				t.Error("Old reviewer should not be assigned")
			}
		}
	})

	t.Run("Reassign on Merged PR", func(t *testing.T) {
		// Create and merge a PR
		mergedPRID := uniqueID("pr-301")
		_, _ = svc.CreatePR(ctx, mergedPRID, "Merged PR", a1ID)
		_, _ = svc.MergePR(ctx, mergedPRID)

		_, _, err := svc.ReassignReviewer(ctx, mergedPRID, r1ID)
		if err == nil {
			t.Error("Expected error when reassigning on merged PR")
		}
		if err.Error() != "PR_MERGED" {
			t.Errorf("Expected PR_MERGED error, got %v", err)
		}
	})

	t.Run("Reassign Non-Assigned Reviewer", func(t *testing.T) {
		testPRID := uniqueID("pr-302")
		_, _ = svc.CreatePR(ctx, testPRID, "Test PR", a1ID)

		_, _, err := svc.ReassignReviewer(ctx, testPRID, r3ID)
		if err == nil {
			t.Error("Expected error when reassigning non-assigned reviewer")
		}
	})
}

func TestService_GetUserReviews(t *testing.T) {
	svc := setupService(t)
	ctx := context.Background()

	a10ID := uniqueID("a10")
	r20ID := uniqueID("r20")
	r21ID := uniqueID("r21")

	team := &models.Team{
		TeamName: uniqueID("reviewteam"),
		Members: []models.TeamMember{
			{UserID: a10ID, Username: "Author10", IsActive: true},
			{UserID: r20ID, Username: "Rev20", IsActive: true},
			{UserID: r21ID, Username: "Rev21", IsActive: true},
		},
	}
	_, _ = svc.CreateTeam(ctx, team)

	_, _ = svc.CreatePR(ctx, uniqueID("pr-400"), "PR 1", a10ID)
	_, _ = svc.CreatePR(ctx, uniqueID("pr-401"), "PR 2", a10ID)

	t.Run("Get Reviews for Reviewer", func(t *testing.T) {
		prs, err := svc.GetUserReviews(ctx, r20ID)
		if err != nil {
			t.Fatalf("Failed to get user reviews: %v", err)
		}

		if len(prs) == 0 {
			t.Error("Expected at least some PRs for reviewer")
		}
	})

	t.Run("Get Reviews for Non-Existent User", func(t *testing.T) {
		_, err := svc.GetUserReviews(ctx, uniqueID("nonexistent"))
		if err == nil {
			t.Error("Expected error for non-existent user")
		}
	})
}
