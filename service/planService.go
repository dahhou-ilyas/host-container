package service

import (
	"context"
	"fmt"
)

type Plan struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	MaxContainers   int    `json:"max_containers"`
	MaxMemoryMB     int    `json:"max_memory_mb"`
	MaxCPUNanoCores int64  `json:"max_cpu_nanocores"`
	AutoStopMinutes int    `json:"auto_stop_minutes"`
	PriceUSDCents   int    `json:"price_usd_cents"`
}

func GetPlanForUser(ctx context.Context, db DB, userID string) (Plan, error) {
	var p Plan
	err := db.QueryRow(ctx, `
		SELECT pl.id, pl.name, pl.max_containers, pl.max_memory_mb,
		       pl.max_cpu_nanocores, pl.auto_stop_minutes, pl.price_usd_cents
		FROM plans pl
		JOIN users u ON u.plan_id = pl.id
		WHERE u.id = $1::bigint
	`, userID).Scan(&p.ID, &p.Name, &p.MaxContainers, &p.MaxMemoryMB,
		&p.MaxCPUNanoCores, &p.AutoStopMinutes, &p.PriceUSDCents)
	if err != nil {
		return Plan{}, fmt.Errorf("failed to get plan for user %s: %w", userID, err)
	}
	return p, nil
}
