package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)


type ImageInfo struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Tag       string    `json:"tag" db:"tag"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}


type ImageService struct {
	repo *ImageRepo
}



func NewImageService(pool *pgxpool.Pool) *ImageService {
	return &ImageService{repo: NewImageRepo(pool)}
}


func (img *ImageService) GetImages(ctx context.Context) ([]ImageInfo , error){
	return img.repo.GetImages(ctx,img.repo.GetDB())
}