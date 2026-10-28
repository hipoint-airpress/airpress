package impl

import (
	"context"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type journalServiceImpl struct {
	JournalCommentService service.JournalCommentService
	ContentURLService     service.ContentURLService
}

func (*journalServiceImpl) Page(ctx context.Context, page param.Page, sort *param.Sort) ([]*entity.Journal, int64, error) {
	if page.PageNum < 0 || page.PageSize <= 0 || page.PageSize > 100 {
		return nil, 0, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("Paging parameter error")
	}
	journalDAL := dal.GetQueryByCtx(ctx).Journal
	journalDO := journalDAL.WithContext(ctx)
	err := BuildSort(sort, &journalDAL, &journalDO)
	if err != nil {
		return nil, 0, err
	}
	journals, totalCount, err := journalDO.FindByPage(page.PageSize*page.PageNum, page.PageSize)
	if err != nil {
		return nil, 0, WrapDBErr(err)
	}
	return journals, totalCount, nil
}

func NewJournalService(journalCommentService service.JournalCommentService, contentURLService service.ContentURLService) service.JournalService {
	return &journalServiceImpl{
		JournalCommentService: journalCommentService,
		ContentURLService:     contentURLService,
	}
}

func (j *journalServiceImpl) ConvertToDTO(ctx context.Context, journal *entity.Journal) (*dto.Journal, error) {
	sourceContent, err := j.ContentURLService.Resolve(ctx, journal.SourceContent)
	if err != nil {
		return nil, err
	}
	content, err := j.ContentURLService.Resolve(ctx, journal.Content)
	if err != nil {
		return nil, err
	}
	return &dto.Journal{
		ID:            journal.ID,
		SourceContent: sourceContent,
		Content:       content,
		Likes:         journal.Likes,
		CreateTime:    journal.CreateTime.UnixMilli(),
		JournalType:   journal.Type,
	}, nil
}

func (j *journalServiceImpl) ConvertToWithCommentDTOList(ctx context.Context, journals []*entity.Journal) ([]*dto.JournalWithComment, error) {
	journalIDs := make([]int32, 0, len(journals))
	for _, journal := range journals {
		journalIDs = append(journalIDs, journal.ID)
	}
	commentCountMap, err := j.JournalCommentService.CountByStatusAndJournalID(ctx, consts.CommentStatusPublished, journalIDs)
	if err != nil {
		return nil, err
	}
	result := make([]*dto.JournalWithComment, 0, len(journals))
	for _, journal := range journals {
		journalDTO, err := j.ConvertToDTO(ctx, journal)
		if err != nil {
			return nil, err
		}
		journalWithCommentCount := &dto.JournalWithComment{
			Journal: *journalDTO,
		}
		if commentCount, ok := commentCountMap[journal.ID]; ok {
			journalWithCommentCount.CommentCount = commentCount
		}
		result = append(result, journalWithCommentCount)
	}
	return result, nil
}

func (j *journalServiceImpl) ListJournal(ctx context.Context, journalQuery param.JournalQuery) ([]*entity.Journal, int64, error) {
	if journalQuery.PageNum < 0 || journalQuery.PageSize <= 0 || journalQuery.PageSize > 100 {
		return nil, 0, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("Paging parameter error")
	}
	journalDAL := dal.GetQueryByCtx(ctx).Journal
	journalDO := journalDAL.WithContext(ctx)
	err := BuildSort(journalQuery.Sort, &journalDAL, &journalDO)
	if err != nil {
		return nil, 0, err
	}

	if journalQuery.Keyword != nil {
		journalDO.Where(journalDAL.Content.Like(*journalQuery.Keyword))
	}
	if journalQuery.JournalType != nil {
		journalDO.Where(journalDAL.Type.Eq(*journalQuery.JournalType))
	}

	posts, totalCount, err := journalDO.FindByPage(journalQuery.PageNum*journalQuery.PageSize, journalQuery.PageSize)
	if err != nil {
		return nil, 0, WrapDBErr(err)
	}
	return posts, totalCount, nil
}

func (j *journalServiceImpl) Create(ctx context.Context, journalParam *param.Journal) (*entity.Journal, error) {
	sourceContent, err := j.ContentURLService.Normalize(ctx, journalParam.SourceContent)
	if err != nil {
		return nil, err
	}
	content, err := j.ContentURLService.Normalize(ctx, journalParam.Content)
	if err != nil {
		return nil, err
	}
	journal := &entity.Journal{
		Type:          journalParam.Type,
		SourceContent: sourceContent,
		Content:       content,
	}
	journalDAL := dal.GetQueryByCtx(ctx).Journal
	err = journalDAL.WithContext(ctx).Create(journal)
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return journal, nil
}

func (j *journalServiceImpl) Update(ctx context.Context, journalID int32, journalParam *param.Journal) (*entity.Journal, error) {
	journalDAL := dal.GetQueryByCtx(ctx).Journal
	journal, err := journalDAL.WithContext(ctx).Where(journalDAL.ID.Eq(journalID)).Take()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	sourceContent, err := j.ContentURLService.Normalize(ctx, journalParam.SourceContent)
	if err != nil {
		return nil, err
	}
	content, err := j.ContentURLService.Normalize(ctx, journalParam.Content)
	if err != nil {
		return nil, err
	}
	journal.SourceContent = sourceContent
	journal.Content = content
	if err != nil {
		return nil, err
	}
	updateResult, err := journalDAL.WithContext(ctx).Where(journalDAL.ID.Eq(journalID)).UpdateSimple(journalDAL.SourceContent.Value(journal.SourceContent), journalDAL.Content.Value(journal.Content), journalDAL.Type.Value(journalParam.Type))
	if err != nil {
		return nil, WrapDBErr(err)
	}
	if updateResult.RowsAffected != 1 {
		return nil, xerr.NoType.New("").WithMsg("update journal failed")
	}
	return journal, nil
}

func (j *journalServiceImpl) Delete(ctx context.Context, journalID int32) error {
	journalDAL := dal.GetQueryByCtx(ctx).Journal
	deleteResult, err := journalDAL.WithContext(ctx).Where(journalDAL.ID.Eq(journalID)).Delete()
	if err != nil {
		return WrapDBErr(err)
	}
	if deleteResult.RowsAffected != 1 {
		return xerr.NoType.New("journalID=%v", journalID).WithMsg("delete failed")
	}
	return nil
}

func (j *journalServiceImpl) GetByJournalIDs(ctx context.Context, journalIDs []int32) (map[int32]*entity.Journal, error) {
	journalDAL := dal.GetQueryByCtx(ctx).Journal
	journals, err := journalDAL.WithContext(ctx).Where(journalDAL.ID.In(journalIDs...)).Find()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	result := make(map[int32]*entity.Journal)
	for _, journal := range journals {
		result[journal.ID] = journal
	}
	return result, nil
}

func (j *journalServiceImpl) Count(ctx context.Context) (int64, error) {
	journalDAL := dal.GetQueryByCtx(ctx).Journal
	count, err := journalDAL.WithContext(ctx).Count()
	if err != nil {
		return 0, WrapDBErr(err)
	}
	return count, nil
}

func (*journalServiceImpl) IncreaseLike(ctx context.Context, journalID int32) error {
	journalDAL := dal.GetQueryByCtx(ctx).Journal
	updateResult, err := journalDAL.WithContext(ctx).Where(journalDAL.ID.Eq(journalID)).UpdateSimple(journalDAL.Likes.Add(1))
	if err != nil {
		return WrapDBErr(err)
	}
	if updateResult.RowsAffected != 1 {
		return xerr.NoType.New("increase journal like failed id=%v", journalID).WithMsg("increase like failed").WithStatus(xerr.StatusInternalServerError)
	}
	return nil
}
