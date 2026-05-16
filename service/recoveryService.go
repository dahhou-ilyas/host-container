package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// backoffDelays defines how long to wait before each restart attempt (index = attempt number).
// Capped at the last entry for all subsequent attempts.
var backoffDelays = []time.Duration{
	5 * time.Second,
	10 * time.Second,
	20 * time.Second,
	40 * time.Second,
	60 * time.Second,
}

// RecoveryService implements the circuit-breaker pattern for container auto-restart.
// When a container crashes:
//  1. Check if circuit is already open → skip
//  2. Check restart_count vs plan limit → open circuit if exceeded
//  3. Apply exponential backoff delay
//  4. Call ContainerRestart, update DB, notify user via SSE
type RecoveryService struct {
	pool    *pgxpool.Pool
	manager *ContainerManager
	bus     *NotificationBus
}

func NewRecoveryService(pool *pgxpool.Pool, manager *ContainerManager, bus *NotificationBus) *RecoveryService {
	return &RecoveryService{pool: pool, manager: manager, bus: bus}
}

// HandleCrash is triggered by a "die" event with a non-zero exit code.
func (r *RecoveryService) HandleCrash(ctx context.Context, dockerContainerID string, exitCode int) {
	var projectID int64
	var projectName, userID string
	var restartCount, maxRestarts int
	var circuitOpen bool

	err := r.pool.QueryRow(ctx, `
		SELECT c.id, c.project_name, c.user_id::text, c.restart_count, c.circuit_open,
		       p.max_auto_restarts
		FROM containers c
		JOIN users u ON u.id = c.user_id
		JOIN plans p ON p.id = u.plan_id
		WHERE c.container_id = $1
	`, dockerContainerID).Scan(&projectID, &projectName, &userID, &restartCount, &circuitOpen, &maxRestarts)
	if err != nil {
		// Container not in our DB (already deleted, or not managed by us)
		return
	}

	if circuitOpen {
		slog.Warn("circuit open, skipping restart", "container", projectName)
		return
	}

	if restartCount >= maxRestarts {
		// Open circuit breaker — no more auto-restarts until user resets manually
		r.pool.Exec(ctx, `UPDATE containers SET circuit_open=true, status='stopped' WHERE id=$1`, projectID)
		r.persistAndNotify(userID, Notification{
			ID:        uuid.New().String(),
			Type:      "circuit_breaker_opened",
			Title:     fmt.Sprintf("Container \"%s\" permanently stopped", projectName),
			Body:      fmt.Sprintf("Restarted %d times without success. Circuit breaker opened. Fix the issue and reset manually.", restartCount),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
		slog.Warn("circuit breaker opened", "container", projectName, "restarts", restartCount)
		return
	}

	// Exponential backoff before restart attempt
	delayIdx := restartCount
	if delayIdx >= len(backoffDelays) {
		delayIdx = len(backoffDelays) - 1
	}
	delay := backoffDelays[delayIdx]
	slog.Info("scheduling container restart",
		"container", projectName,
		"attempt", restartCount+1,
		"max", maxRestarts,
		"delay", delay,
	)
	time.Sleep(delay)

	// Increment counter and mark as starting before calling Docker
	// so that if restart fails, the count is still accurate
	_, err = r.pool.Exec(ctx, `
		UPDATE containers
		SET restart_count = restart_count + 1,
		    last_restart_at = NOW(),
		    status = 'starting'
		WHERE id = $1`, projectID)
	if err != nil {
		slog.Error("failed to update restart_count", "err", err)
		return
	}

	timeout := 10
	if err := r.manager.client.ContainerRestart(ctx, dockerContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
		slog.Error("ContainerRestart failed", "container", projectName, "err", err)
		r.pool.Exec(ctx, `UPDATE containers SET status='stopped' WHERE id=$1`, projectID)
		return
	}

	r.pool.Exec(ctx, `UPDATE containers SET status='running', started_at=NOW() WHERE id=$1`, projectID)

	r.persistAndNotify(userID, Notification{
		ID:        uuid.New().String(),
		Type:      "container_restarted",
		Title:     fmt.Sprintf("Container \"%s\" restarted", projectName),
		Body:      fmt.Sprintf("Auto-restarted after crash (exit code %d). Attempt %d of %d.", exitCode, restartCount+1, maxRestarts),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// HandleOOM marks the oom_killed flag. The "die" event that follows will trigger HandleCrash.
func (r *RecoveryService) HandleOOM(ctx context.Context, dockerContainerID string) {
	r.pool.Exec(ctx, `UPDATE containers SET oom_killed=true WHERE container_id=$1`, dockerContainerID)
	slog.Warn("OOM kill detected", "container_docker_id", dockerContainerID)
}

// HandleHealthChange updates health_status and notifies on "unhealthy".
func (r *RecoveryService) HandleHealthChange(ctx context.Context, dockerContainerID, status string) {
	r.pool.Exec(ctx, `UPDATE containers SET health_status=$1 WHERE container_id=$2`, status, dockerContainerID)

	if status != "unhealthy" {
		return
	}

	var projectName, userID string
	if err := r.pool.QueryRow(ctx,
		`SELECT project_name, user_id::text FROM containers WHERE container_id=$1`,
		dockerContainerID).Scan(&projectName, &userID); err != nil {
		return
	}

	r.persistAndNotify(userID, Notification{
		ID:        uuid.New().String(),
		Type:      "container_unhealthy",
		Title:     fmt.Sprintf("Container \"%s\" is unhealthy", projectName),
		Body:      "Health check is failing. Check your application logs.",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

// ResetCircuit clears the circuit breaker and restart counter so auto-recovery can resume.
func (r *RecoveryService) ResetCircuit(ctx context.Context, userID, projectID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE containers
		SET circuit_open=false, restart_count=0, oom_killed=false
		WHERE id=$1::bigint AND user_id=$2::bigint`,
		projectID, userID)
	if err != nil || tag.RowsAffected() == 0 {
		return fmt.Errorf("container not found or access denied")
	}
	return nil
}

// persistAndNotify saves a notification to DB (survives server restarts) and
// pushes it live to the user's SSE connections.
func (r *RecoveryService) persistAndNotify(userID string, n Notification) {
	ctx := context.Background()
	r.pool.Exec(ctx,
		`INSERT INTO notifications (user_id, type, title, body) VALUES ($1::bigint, $2, $3, $4)`,
		userID, n.Type, n.Title, n.Body)
	r.bus.Publish(userID, n)
}
