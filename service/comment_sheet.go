package service

import (
	"context"

	"github.com/hipoint-airpress/airpress/consts"
)

type SheetCommentService interface {
	BaseCommentService
	CountByStatus(ctx context.Context, status consts.CommentStatus) (int64, error)
}
