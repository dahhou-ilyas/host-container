package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("container not found")

type ContainerRepo struct {
	pool *pgxpool.Pool
}

func NewContainerRepo(pool *pgxpool.Pool) *ContainerRepo {
	return &ContainerRepo{pool: pool}
}

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *ContainerRepo) CreateContainer(ctx context.Context, db DB, info ContainerInfo) (string, error) {
	var id string
	err := db.QueryRow(ctx, `INSERT INTO container(container_id, project_name, folder_path, port, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text
	`, info.ContainerID, info.ProjectName, info.FolderPath, info.Port, info.Status).Scan(&id)

	return id, err
}

func (r *ContainerRepo) GetContainerByID(ctx context.Context, db DB, id string) (ContainerInfo, error) {
	var c ContainerInfo
	err := db.QueryRow(ctx, `
		SELECT id::text, container_id, project_name, folder_path, port, status
		FROM container
		WHERE id = $1::bigint
	`, id).Scan(&c.ProjectID, &c.ContainerID, &c.ProjectName, &c.FolderPath, &c.Port, &c.Status)

	if errors.Is(err, pgx.ErrNoRows) {
		return ContainerInfo{}, ErrNotFound
	}
	return c, err
}

func (r *ContainerRepo) UpdateContainer(ctx context.Context, db DB, id string, info ContainerInfo) (ContainerInfo, error) {
	var updated ContainerInfo
	err := db.QueryRow(ctx, `
		UPDATE container
		SET container_id = $1, project_name = $2, folder_path = $3, port = $4, status = $5
		WHERE id = $6::bigint
		RETURNING id::text, container_id, project_name, folder_path, port, status
	`, info.ContainerID, info.ProjectName, info.FolderPath, info.Port, info.Status, id).
		Scan(&updated.ProjectID, &updated.ContainerID, &updated.ProjectName, &updated.FolderPath, &updated.Port, &updated.Status)

	if errors.Is(err, pgx.ErrNoRows) {
		return ContainerInfo{}, ErrNotFound
	}
	return updated, err
}

func (r *ContainerRepo) DeleteContainer(ctx context.Context, db DB, id string) error {
	tag, err := db.Exec(ctx, `DELETE FROM container WHERE id = $1::bigint`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
