package impl

import (
	"context"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/service"
)

type sheetCommentServiceImpl struct {
	service.BaseCommentService
}

func NewSheetCommentService(baseCommentService service.BaseCommentService) service.SheetCommentService {
	return &sheetCommentServiceImpl{
		BaseCommentService: baseCommentService,
	}
}

func (s sheetCommentServiceImpl) CountByStatus(ctx context.Context, status consts.CommentStatus) (int64, error) {
	commentDAL := dal.GetQueryByCtx(ctx).Comment
	count, err := commentDAL.WithContext(ctx).Where(commentDAL.Type.Eq(consts.CommentTypeSheet), commentDAL.Status.Eq(status)).Count()
	if err != nil {
		return 0, WrapDBErr(err)
	}
	return count, nil
}
