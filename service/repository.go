package service

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("container not found")
var UserNotFound = errors.New("User Not Found")

type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)

}




// -----------------------------------------  CONTAINER REPOSITORY -----------------------------------------
type ContainerRepo struct {
	pool *pgxpool.Pool
}

func NewContainerRepo(pool *pgxpool.Pool) *ContainerRepo {
	return &ContainerRepo{pool: pool}
}

func (c *ContainerRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return c.pool.Begin(ctx)
}

func (c *ContainerRepo) GetDB() DB {
	return c.pool
}

func (r *ContainerRepo) CreateContainer(ctx context.Context, db DB, info ContainerInfo) (string, error) {
	var id string
	err := db.QueryRow(ctx, `INSERT INTO containers(container_id, project_name, folder_path, port, status, user_id, started_at, image_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id::text
	`, info.ContainerID, info.ProjectName, info.FolderPath, info.Port, info.Status, info.UserId, info.StartedAt, info.ImageName).Scan(&id)

	return id, err
}

func (r *ContainerRepo) GetContainerByID(ctx context.Context, db DB, id string) (ContainerInfo, error) {
	var c ContainerInfo
	err := db.QueryRow(ctx, `
		SELECT id::text, container_id, project_name, folder_path, port, status, started_at, image_name, created_at
		FROM containers
		WHERE id = $1::bigint
	`, id).Scan(&c.ProjectID, &c.ContainerID, &c.ProjectName, &c.FolderPath, &c.Port, &c.Status, &c.StartedAt, &c.ImageName, &c.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ContainerInfo{}, ErrNotFound
	}
	return c, err
}

func (r *ContainerRepo) UpdateContainer(ctx context.Context, db DB, id string, info ContainerInfo) (ContainerInfo, error) {
	var updated ContainerInfo
	err := db.QueryRow(ctx, `
		UPDATE containers
		SET container_id = $1, project_name = $2, folder_path = $3, port = $4, status = $5, started_at = $7, image_name = $8
		WHERE id = $6::bigint
		RETURNING id::text, container_id, project_name, folder_path, port, status, started_at, image_name, created_at
	`, info.ContainerID, info.ProjectName, info.FolderPath, info.Port, info.Status, id, info.StartedAt, info.ImageName).
		Scan(&updated.ProjectID, &updated.ContainerID, &updated.ProjectName, &updated.FolderPath, &updated.Port, &updated.Status, &updated.StartedAt, &updated.ImageName, &updated.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return ContainerInfo{}, ErrNotFound
	}
	return updated, err
}

func (r *ContainerRepo) DeleteContainer(ctx context.Context, db DB, id string) error {
	tag, err := db.Exec(ctx, `DELETE FROM containers WHERE id = $1::bigint`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ContainerRepo) GetRunningContainersOlderThan(ctx context.Context, db DB, maxAge time.Duration) ([]ContainerInfo, error) {
	cutoff := time.Now().Add(-maxAge)
	rows, err := db.Query(ctx, `
		SELECT id::text, container_id, project_name, folder_path, port, status, started_at, image_name, created_at
		FROM containers
		WHERE status = 'running' AND started_at IS NOT NULL AND started_at < $1
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []ContainerInfo
	for rows.Next() {
		var c ContainerInfo
		if err := rows.Scan(&c.ProjectID, &c.ContainerID, &c.ProjectName, &c.FolderPath, &c.Port, &c.Status, &c.StartedAt, &c.ImageName, &c.CreatedAt); err != nil {
			return nil, err
		}
		containers = append(containers, c)
	}
	return containers, rows.Err()
}



// -----------------------------------------  USER REPOSITORY -----------------------------------------


type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (u *UserRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return u.pool.Begin(ctx)
}

func (u *UserRepo) GetDB() DB {
	return u.pool
}



func (u *UserRepo) CreateUser(ctx context.Context, db DB, user User) (string, error) {
	var id string
	err := db.QueryRow(ctx, `INSERT INTO users(name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id::text
	`, user.Name, user.Email, user.Password).Scan(&id)

	return id, err
}



func (u *UserRepo) GetUserByID(ctx context.Context, db DB, id string) (User, error) {
	var user User
	err := db.QueryRow(ctx, `
		SELECT id::text, name, email
		FROM users
		WHERE id = $1::bigint
	`, id).Scan(&user.Id, &user.Name, &user.Email)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, UserNotFound
	}
	return user, err
}

func (u *UserRepo) GetUserByEmail(ctx context.Context, db DB, email string) (User, error) {
	var user User

	err := db.QueryRow(ctx, `
		SELECT id::text, name, email
		FROM users
		WHERE email = $1
	`, email).Scan(&user.Id, &user.Name, &user.Email)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, UserNotFound
	}
	return user, err
}

func (u *UserRepo) GetUserByEmailWithPassword(ctx context.Context, db DB, email string) (User, error) {
	var user User

	err := db.QueryRow(ctx, `
		SELECT id::text, name, email, password_hash
		FROM users
		WHERE email = $1
	`, email).Scan(&user.Id, &user.Name, &user.Email, &user.Password)

	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, UserNotFound
	}
	return user, err
}


func (u *UserRepo) GetContainersForUserByID(ctx context.Context, db DB, userId string) (User, []ContainerInfo, error) {
	var user User
	containers := make([]ContainerInfo, 0)

	rows, err := db.Query(ctx, `
		SELECT
			u.id::text, u.name, u.email,
			c.container_id, c.id::text, c.project_name, c.folder_path, c.port::text, c.status, c.image_name, c.created_at
		FROM users u
		LEFT JOIN containers c ON c.user_id = u.id
		WHERE u.id = $1::bigint
		ORDER BY c.id DESC
	`, userId)
	if err != nil {
		return User{}, nil, err
	}
	defer rows.Close()

	foundUser := false

	for rows.Next() {
		foundUser = true

		var uid, name, email string

		var containerID, projectID, projectName, folderPath, port, status, imageName *string
		var createdAt *time.Time

		if err := rows.Scan(
			&uid, &name, &email,
			&containerID, &projectID, &projectName, &folderPath, &port, &status, &imageName, &createdAt,
		); err != nil {
			return User{}, nil, err
		}

		if user.Id == "" {
			user.Id = uid
			user.Name = name
			user.Email = email
		}

		// (LEFT JOIN => colonnes NULL), on skip
		if containerID == nil {
			continue
		}

		c := ContainerInfo{
			ContainerID: *containerID,
			UserId:      uid,
			CreatedAt:   createdAt,
		}
		if projectID != nil {
			c.ProjectID = *projectID
		}
		if projectName != nil {
			c.ProjectName = *projectName
		}
		if folderPath != nil {
			c.FolderPath = *folderPath
		}
		if port != nil {
			c.Port = *port
		}
		if status != nil {
			c.Status = *status
		}
		if imageName != nil {
			c.ImageName = *imageName
		}

		containers = append(containers, c)
	}

	if err := rows.Err(); err != nil {
		return User{}, nil, err
	}

	if !foundUser {
		return User{}, nil, UserNotFound
	}

	user.Containers = &containers

	return user, containers, nil
}


func (u *UserRepo) DeleteUserByID(ctx context.Context, db DB, userId string) error {
	if _, err := db.Exec(ctx, `DELETE FROM containers WHERE user_id = $1::bigint`, userId); err != nil {
		return err
	}

	tag, err := db.Exec(ctx, `DELETE FROM users WHERE id = $1::bigint`, userId)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return UserNotFound
	}
	return nil
}



// -----------------------------------------  IMAGES REPOSITORY -----------------------------------------

type ImageRepo struct {
	pool *pgxpool.Pool
}

func NewImageRepo(pool *pgxpool.Pool) *ImageRepo {
	return &ImageRepo{pool: pool}
}

func (i *ImageRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return i.pool.Begin(ctx)
}

func (i *ImageRepo) GetDB() DB {
	return i.pool
}

func (i *ImageRepo) GetImages(ctx context.Context,db DB) ([]ImageInfo, error) {
	rows, err := db.Query(ctx, `
		SELECT id, name, tag, created_at
		FROM images
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]ImageInfo, 0);
	for rows.Next() {
		var img ImageInfo
		if err := rows.Scan(&img.ID, &img.Name, &img.Tag, &img.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}