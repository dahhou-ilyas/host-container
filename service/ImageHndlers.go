package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)


type ImageHandler struct {
	imageService *ImageService
}


func NewImageHandler(pool *pgxpool.Pool) *ImageHandler {
	return &ImageHandler{imageService: NewImageService(pool)}
}


func (imgHandler *ImageHandler) GetImages(ctx context.Context) ([]ImageInfo , error){
	return imgHandler.imageService.GetImages(ctx)
}