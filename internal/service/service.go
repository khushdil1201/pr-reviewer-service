package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/repository"
)

type Service struct {
	repo *repository.Repository
	rand *rand.Rand
}

func NewService(repo *repository.Repository) *Service {
	return &Service{
		repo: repo,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Service) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	exists, err := s.repo.TeamExists(ctx, team.TeamName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("TEAM_EXISTS")
	}

	if err := s.repo.CreateTeam(ctx, team.TeamName); err != nil {
		return nil, err
	}

	for _, member := range team.Members {
		user := &models.User{
			UserID:   member.UserID,
			Username: member.Username,
			TeamName: team.TeamName,
			IsActive: member.IsActive,
		}
		if err := s.repo.UpsertUser(ctx, user); err != nil {
			return nil, err
		}
	}

	return s.repo.GetTeam(ctx, team.TeamName)
}

func (s *Service) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	team, err := s.repo.GetTeam(ctx, teamName)
	if err != nil {
		return nil, errors.New("NOT_FOUND")
	}
	return team, nil
}

func (s *Service) SetUserActive(ctx context.Context, userID string, isActive bool) (*models.User, error) {
	if err := s.repo.UpdateUserActive(ctx, userID, isActive); err != nil {
		return nil, errors.New("NOT_FOUND")
	}
	return s.repo.GetUser(ctx, userID)
}

func (s *Service) CreatePR(ctx context.Context, prID, prName, authorID string) (*models.PullRequest, error) {
	exists, err := s.repo.PRExists(ctx, prID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("PR_EXISTS")
	}

	author, err := s.repo.GetUser(ctx, authorID)
	if err != nil {
		return nil, errors.New("NOT_FOUND")
	}

	activeMembers, err := s.repo.GetActiveTeamMembers(ctx, author.TeamName, authorID)
	if err != nil {
		return nil, err
	}

	reviewers := s.selectReviewers(activeMembers, 2)

	pr := &models.PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            models.PRStatusOpen,
		AssignedReviewers: reviewers,
	}

	if err := s.repo.CreatePR(ctx, pr); err != nil {
		return nil, err
	}

	return s.repo.GetPR(ctx, prID)
}

func (s *Service) selectReviewers(candidates []models.User, maxCount int) []string {
	if len(candidates) == 0 {
		return []string{}
	}

	count := maxCount
	if len(candidates) < count {
		count = len(candidates)
	}

	shuffled := make([]models.User, len(candidates))
	copy(shuffled, candidates)

	for i := len(shuffled) - 1; i > 0; i-- {
		j := s.rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	reviewers := make([]string, count)
	for i := 0; i < count; i++ {
		reviewers[i] = shuffled[i].UserID
	}

	return reviewers
}

func (s *Service) MergePR(ctx context.Context, prID string) (*models.PullRequest, error) {
	pr, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		return nil, errors.New("NOT_FOUND")
	}

	if pr.Status == models.PRStatusMerged {
		return pr, nil
	}

	if err := s.repo.MergePR(ctx, prID); err != nil {
		return nil, err
	}

	return s.repo.GetPR(ctx, prID)
}

func (s *Service) ReassignReviewer(ctx context.Context, prID, oldUserID string) (*models.PullRequest, string, error) {
	pr, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		return nil, "", errors.New("NOT_FOUND")
	}

	if pr.Status == models.PRStatusMerged {
		return nil, "", errors.New("PR_MERGED")
	}

	isAssigned, err := s.repo.IsReviewerAssigned(ctx, prID, oldUserID)
	if err != nil {
		return nil, "", err
	}
	if !isAssigned {
		return nil, "", errors.New("NOT_ASSIGNED")
	}

	oldUser, err := s.repo.GetUser(ctx, oldUserID)
	if err != nil {
		return nil, "", errors.New("NOT_FOUND")
	}

	candidates, err := s.repo.GetActiveTeamMembers(ctx, oldUser.TeamName, "")
	if err != nil {
		return nil, "", err
	}

	validCandidates := []models.User{}
	for _, candidate := range candidates {
		isCurrentlyAssigned := false
		for _, reviewerID := range pr.AssignedReviewers {
			if reviewerID == candidate.UserID {
				isCurrentlyAssigned = true
				break
			}
		}
		if !isCurrentlyAssigned && candidate.UserID != pr.AuthorID {
			validCandidates = append(validCandidates, candidate)
		}
	}

	if len(validCandidates) == 0 {
		return nil, "", errors.New("NO_CANDIDATE")
	}

	newReviewers := s.selectReviewers(validCandidates, 1)
	if len(newReviewers) == 0 {
		return nil, "", errors.New("NO_CANDIDATE")
	}

	newReviewerID := newReviewers[0]

	if replaceErr := s.repo.ReplaceReviewer(ctx, prID, oldUserID, newReviewerID); replaceErr != nil {
		return nil, "", replaceErr
	}

	updatedPR, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		return nil, "", err
	}

	return updatedPR, newReviewerID, nil
}

func (s *Service) GetUserReviews(ctx context.Context, userID string) ([]models.PullRequestShort, error) {
	_, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return nil, errors.New("NOT_FOUND")
	}

	prs, err := s.repo.GetPRsForReviewer(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PRs: %w", err)
	}

	return prs, nil
}

func (s *Service) GetStatistics(ctx context.Context) (*models.Statistics, error) {
	return s.repo.GetStatistics(ctx)
}

func (s *Service) DeactivateTeamAndReassign(ctx context.Context, teamName string) (int, int, error) {
	teamExists, err := s.repo.TeamExists(ctx, teamName)
	if err != nil {
		return 0, 0, err
	}
	if !teamExists {
		return 0, 0, errors.New("NOT_FOUND")
	}

	deactivatedIDs, err := s.repo.DeactivateTeamUsers(ctx, teamName)
	if err != nil {
		return 0, 0, err
	}

	reassignedCount := 0
	for _, userID := range deactivatedIDs {
		prIDs, err := s.repo.GetOpenPRsWithReviewer(ctx, userID)
		if err != nil {
			continue
		}

		for _, prID := range prIDs {
			pr, err := s.repo.GetPR(ctx, prID)
			if err != nil || pr.Status != models.PRStatusOpen {
				continue
			}

			author, err := s.repo.GetUser(ctx, pr.AuthorID)
			if err != nil {
				continue
			}

			activeMembers, err := s.repo.GetActiveTeamMembers(ctx, author.TeamName, "")
			if err != nil {
				continue
			}

			validCandidates := []models.User{}
			for _, candidate := range activeMembers {
				isAlreadyAssigned := false
				for _, reviewerID := range pr.AssignedReviewers {
					if reviewerID == candidate.UserID {
						isAlreadyAssigned = true
						break
					}
				}
				if !isAlreadyAssigned && candidate.UserID != pr.AuthorID {
					validCandidates = append(validCandidates, candidate)
				}
			}

			if len(validCandidates) > 0 {
				newReviewers := s.selectReviewers(validCandidates, 1)
				if len(newReviewers) > 0 {
					if err := s.repo.ReplaceReviewer(ctx, prID, userID, newReviewers[0]); err == nil {
						reassignedCount++
					}
				}
			}
		}
	}

	return len(deactivatedIDs), reassignedCount, nil
}
