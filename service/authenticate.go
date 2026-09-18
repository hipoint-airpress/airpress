package service

import (
	"context"

	"github.com/hipoint-airpress/airpress/model/entity"
)

type AuthenticateService interface {
	PostAuthenticate(ctx context.Context, post *entity.Post, password string) (bool, error)
	CategoryAuthenticate(ctx context.Context, categoryID int32, password string) (bool, error)
}
