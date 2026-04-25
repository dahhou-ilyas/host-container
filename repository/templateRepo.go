package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TemplateRow struct {
	ID           int64
	Name         string
	Description  string
	Image        string
	Category     string
	Tags         []byte
	Popular      bool
	Icon         string
	DefaultPorts []byte
	CreatedAt    time.Time
}

func GetTemplates(ctx context.Context, pool *pgxpool.Pool, category string) ([]TemplateRow, error) {
	var rows []TemplateRow
	var query string
	var args []any

	if category != "" && category != "all" {
		query = `SELECT id, name, description, image, category, tags, popular, COALESCE(icon,''), default_ports, created_at
				 FROM templates WHERE category = $1 ORDER BY popular DESC, name ASC`
		args = []any{category}
	} else {
		query = `SELECT id, name, description, image, category, tags, popular, COALESCE(icon,''), default_ports, created_at
				 FROM templates ORDER BY popular DESC, name ASC`
	}

	dbRows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query templates: %w", err)
	}
	defer dbRows.Close()

	for dbRows.Next() {
		var t TemplateRow
		if err := dbRows.Scan(&t.ID, &t.Name, &t.Description, &t.Image, &t.Category,
			&t.Tags, &t.Popular, &t.Icon, &t.DefaultPorts, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan template row: %w", err)
		}
		rows = append(rows, t)
	}
	return rows, dbRows.Err()
}

func GetTemplateByID(ctx context.Context, pool *pgxpool.Pool, id string) (*TemplateRow, error) {
	var t TemplateRow
	err := pool.QueryRow(ctx,
		`SELECT id, name, description, image, category, tags, popular, COALESCE(icon,''), default_ports, created_at
		 FROM templates WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.Description, &t.Image, &t.Category,
			&t.Tags, &t.Popular, &t.Icon, &t.DefaultPorts, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get template %s: %w", id, err)
	}
	return &t, nil
}

func GetAllowedImages(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT image FROM templates`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []string
	for rows.Next() {
		var img string
		if err := rows.Scan(&img); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}
