package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditEntry struct {
	UserID     string
	Action     string
	Resource   string
	ResourceID string
	IP         string
	UserAgent  string
	Metadata   map[string]any
}

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) Log(ctx context.Context, e AuditEntry) error {
	var metaJSON []byte
	if e.Metadata != nil {
		var err error
		metaJSON, err = json.Marshal(e.Metadata)
		if err != nil {
			return err
		}
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit_log (user_id, action, resource, resource_id, ip, user_agent, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		e.UserID, e.Action, e.Resource, e.ResourceID, e.IP, e.UserAgent, metaJSON,
	)
	return err
}
