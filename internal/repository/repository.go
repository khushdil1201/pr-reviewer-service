package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"pr-reviewer-service/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateTeam(ctx context.Context, teamName string) error {
	query := `INSERT INTO teams (team_name) VALUES ($1)`
	_, err := r.db.Exec(ctx, query, teamName)
	return err
}

func (r *Repository) TeamExists(ctx context.Context, teamName string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)`
	err := r.db.QueryRow(ctx, query, teamName).Scan(&exists)
	return exists, err
}

func (r *Repository) GetTeam(ctx context.Context, teamName string) (*models.Team, error) {
	teamExists, err := r.TeamExists(ctx, teamName)
	if err != nil {
		return nil, err
	}
	if !teamExists {
		return nil, errors.New("team not found")
	}

	query := `
                SELECT u.user_id, u.username, u.is_active
                FROM users u
                WHERE u.team_name = $1
                ORDER BY u.user_id
        `

	rows, err := r.db.Query(ctx, query, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []models.TeamMember{}
	for rows.Next() {
		var member models.TeamMember
		if err := rows.Scan(&member.UserID, &member.Username, &member.IsActive); err != nil {
			return nil, err
		}
		members = append(members, member)
	}

	return &models.Team{
		TeamName: teamName,
		Members:  members,
	}, nil
}

func (r *Repository) UpsertUser(ctx context.Context, user *models.User) error {
	query := `
                INSERT INTO users (user_id, username, team_name, is_active, updated_at)
                VALUES ($1, $2, $3, $4, $5)
                ON CONFLICT (user_id) 
                DO UPDATE SET username = $2, team_name = $3, is_active = $4, updated_at = $5
        `
	_, err := r.db.Exec(ctx, query, user.UserID, user.Username, user.TeamName, user.IsActive, time.Now())
	return err
}

func (r *Repository) GetUser(ctx context.Context, userID string) (*models.User, error) {
	query := `
                SELECT user_id, username, team_name, is_active, created_at, updated_at
                FROM users
                WHERE user_id = $1
        `

	var user models.User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&user.UserID, &user.Username, &user.TeamName, &user.IsActive,
		&user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (r *Repository) UpdateUserActive(ctx context.Context, userID string, isActive bool) error {
	query := `UPDATE users SET is_active = $1, updated_at = $2 WHERE user_id = $3`
	result, err := r.db.Exec(ctx, query, isActive, time.Now(), userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("user not found")
	}

	return nil
}

func (r *Repository) GetActiveTeamMembers(ctx context.Context, teamName string, excludeUserID string) ([]models.User, error) {
	query := `
                SELECT user_id, username, team_name, is_active
                FROM users
                WHERE team_name = $1 AND is_active = true AND user_id != $2
                ORDER BY user_id
        `

	rows, err := r.db.Query(ctx, query, teamName, excludeUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []models.User{}
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (r *Repository) CreatePR(ctx context.Context, pr *models.PullRequest) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				// Логируем ошибку отката, но возвращаем оригинальную ошибку
				fmt.Printf("Warning: failed to rollback transaction: %v\n", rollbackErr)
			}
		}
	}()

	query := `
                INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, created_at)
                VALUES ($1, $2, $3, $4, $5)
        `
	_, err = tx.Exec(ctx, query, pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, time.Now())
	if err != nil {
		return err
	}

	if len(pr.AssignedReviewers) > 0 {
		for _, reviewerID := range pr.AssignedReviewers {
			reviewerQuery := `
                                INSERT INTO pr_reviewers (pull_request_id, user_id)
                                VALUES ($1, $2)
                        `
			_, err = tx.Exec(ctx, reviewerQuery, pr.PullRequestID, reviewerID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) PRExists(ctx context.Context, prID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)`
	err := r.db.QueryRow(ctx, query, prID).Scan(&exists)
	return exists, err
}

func (r *Repository) GetPR(ctx context.Context, prID string) (*models.PullRequest, error) {
	query := `
                SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
                FROM pull_requests
                WHERE pull_request_id = $1
        `

	var pr models.PullRequest
	err := r.db.QueryRow(ctx, query, prID).Scan(
		&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status,
		&pr.CreatedAt, &pr.MergedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("PR not found")
		}
		return nil, err
	}

	reviewersQuery := `SELECT user_id FROM pr_reviewers WHERE pull_request_id = $1 ORDER BY user_id`
	rows, err := r.db.Query(ctx, reviewersQuery, prID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pr.AssignedReviewers = []string{}
	for rows.Next() {
		var reviewerID string
		if err := rows.Scan(&reviewerID); err != nil {
			return nil, err
		}
		pr.AssignedReviewers = append(pr.AssignedReviewers, reviewerID)
	}

	return &pr, nil
}

func (r *Repository) MergePR(ctx context.Context, prID string) error {
	query := `
                UPDATE pull_requests
                SET status = $1, merged_at = $2
                WHERE pull_request_id = $3 AND status = 'OPEN'
        `
	_, err := r.db.Exec(ctx, query, models.PRStatusMerged, time.Now(), prID)
	return err
}

func (r *Repository) IsReviewerAssigned(ctx context.Context, prID, userID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM pr_reviewers WHERE pull_request_id = $1 AND user_id = $2)`
	err := r.db.QueryRow(ctx, query, prID, userID).Scan(&exists)
	return exists, err
}

func (r *Repository) ReplaceReviewer(ctx context.Context, prID, oldUserID, newUserID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				fmt.Printf("Warning: failed to rollback transaction: %v\n", rollbackErr)
			}
		}
	}()

	deleteQuery := `DELETE FROM pr_reviewers WHERE pull_request_id = $1 AND user_id = $2`
	result, err := tx.Exec(ctx, deleteQuery, prID, oldUserID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("reviewer not assigned")
	}

	insertQuery := `INSERT INTO pr_reviewers (pull_request_id, user_id) VALUES ($1, $2)`
	_, err = tx.Exec(ctx, insertQuery, prID, newUserID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetPRsForReviewer(ctx context.Context, userID string) ([]models.PullRequestShort, error) {
	query := `
                SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
                FROM pull_requests pr
                INNER JOIN pr_reviewers r ON pr.pull_request_id = r.pull_request_id
                WHERE r.user_id = $1
                ORDER BY pr.created_at DESC
        `

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prs := []models.PullRequestShort{}
	for rows.Next() {
		var pr models.PullRequestShort
		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status); err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

func (r *Repository) GetStatistics(ctx context.Context) (*models.Statistics, error) {
	stats := &models.Statistics{}

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers); err != nil {
		return nil, err
	}

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE is_active = true`).Scan(&stats.ActiveUsers); err != nil {
		return nil, err
	}

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM teams`).Scan(&stats.TotalTeams); err != nil {
		return nil, err
	}

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM pull_requests`).Scan(&stats.TotalPRs); err != nil {
		return nil, err
	}

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM pull_requests WHERE status = 'OPEN'`).Scan(&stats.OpenPRs); err != nil {
		return nil, err
	}

	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM pull_requests WHERE status = 'MERGED'`).Scan(&stats.MergedPRs); err != nil {
		return nil, err
	}

	userAssignmentsQuery := `
                SELECT 
                        u.user_id, 
                        u.username, 
                        COUNT(pr.pull_request_id) as pr_count,
                        COUNT(CASE WHEN pr.status = 'OPEN' THEN 1 END) as open_count,
                        COUNT(CASE WHEN pr.status = 'MERGED' THEN 1 END) as merged_count
                FROM users u
                LEFT JOIN pr_reviewers r ON u.user_id = r.user_id
                LEFT JOIN pull_requests pr ON r.pull_request_id = pr.pull_request_id
                GROUP BY u.user_id, u.username
                HAVING COUNT(pr.pull_request_id) > 0
                ORDER BY pr_count DESC
        `

	rows, err := r.db.Query(ctx, userAssignmentsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats.UserAssignments = []models.UserAssignmentStat{}
	for rows.Next() {
		var ua models.UserAssignmentStat
		if err := rows.Scan(&ua.UserID, &ua.Username, &ua.PRCount, &ua.OpenCount, &ua.MergedCount); err != nil {
			return nil, err
		}
		stats.UserAssignments = append(stats.UserAssignments, ua)
	}

	prAssignmentsQuery := `
                SELECT 
                        pr.pull_request_id, 
                        pr.pull_request_name, 
                        COUNT(r.user_id) as reviewer_count
                FROM pull_requests pr
                LEFT JOIN pr_reviewers r ON pr.pull_request_id = r.pull_request_id
                GROUP BY pr.pull_request_id, pr.pull_request_name
                ORDER BY reviewer_count DESC, pr.pull_request_id
        `

	rows2, err := r.db.Query(ctx, prAssignmentsQuery)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	stats.PRAssignments = []models.PRAssignmentStat{}
	for rows2.Next() {
		var pa models.PRAssignmentStat
		if err := rows2.Scan(&pa.PullRequestID, &pa.PullRequestName, &pa.ReviewerCount); err != nil {
			return nil, err
		}
		stats.PRAssignments = append(stats.PRAssignments, pa)
	}

	return stats, nil
}

func (r *Repository) DeactivateTeamUsers(ctx context.Context, teamName string) ([]string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
				fmt.Printf("Warning: failed to rollback transaction: %v\n", rollbackErr)
			}
		}
	}()

	query := `
                UPDATE users 
                SET is_active = false, updated_at = $1 
                WHERE team_name = $2 AND is_active = true
                RETURNING user_id
        `

	rows, err := tx.Query(ctx, query, time.Now(), teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deactivatedIDs := []string{}
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		deactivatedIDs = append(deactivatedIDs, userID)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return deactivatedIDs, nil
}

func (r *Repository) GetOpenPRsWithReviewer(ctx context.Context, userID string) ([]string, error) {
	query := `
                SELECT DISTINCT pr.pull_request_id
                FROM pull_requests pr
                INNER JOIN pr_reviewers r ON pr.pull_request_id = r.pull_request_id
                WHERE pr.status = 'OPEN' AND r.user_id = $1
                ORDER BY pr.pull_request_id
        `

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prIDs := []string{}
	for rows.Next() {
		var prID string
		if err := rows.Scan(&prID); err != nil {
			return nil, err
		}
		prIDs = append(prIDs, prID)
	}

	return prIDs, nil
}
