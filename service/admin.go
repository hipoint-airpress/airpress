package service

import (
	"context"

	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
)

type AdminService interface {
	Authenticate(ctx context.Context, loginParam param.LoginParam) (*entity.User, error)
	Auth(ctx context.Context, loginParam param.LoginParam) (*dto.AuthTokenDTO, error)
	ClearToken(ctx context.Context) error
	SendResetPasswordCode(ctx context.Context, resetParam param.ResetPasswordParam) error
	RefreshToken(ctx context.Context, refreshToken string) (*dto.AuthTokenDTO, error)
	GetEnvironments(ctx context.Context) *dto.EnvironmentDTO
	GetLogFiles(ctx context.Context, lines int64) (string, error)
}
