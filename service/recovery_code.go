package service

import "context"

type RecoveryCodeService interface {
	// Generate 生成一批新的恢复码并作废旧码，明文只在这里返回一次。
	Generate(ctx context.Context, userID int32) ([]string, error)
	ValidateAndConsume(ctx context.Context, userID int32, code string) (bool, error)
	CountRemaining(ctx context.Context, userID int32) (int64, error)
	DeleteAll(ctx context.Context, userID int32) error
}
