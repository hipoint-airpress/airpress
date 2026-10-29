package impl

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/log"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/service/storage"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type attachmentServiceImpl struct {
	OptionService        service.OptionService
	FileStorageComposite storage.FileStorageComposite
	UserService          service.UserService
}

// fillOwners 批量填充附件归属人，避免列表逐条查询用户。
func (a *attachmentServiceImpl) fillOwners(ctx context.Context, attachments []*entity.Attachment, dtos []*dto.AttachmentDTO) error {
	userIDs := make([]int32, 0, len(attachments))
	seen := make(map[int32]struct{}, len(attachments))
	for _, attachment := range attachments {
		if attachment.UserID <= 0 {
			continue
		}
		if _, ok := seen[attachment.UserID]; ok {
			continue
		}
		seen[attachment.UserID] = struct{}{}
		userIDs = append(userIDs, attachment.UserID)
	}
	if len(userIDs) == 0 {
		return nil
	}
	users, err := a.UserService.GetByIDs(ctx, userIDs)
	if err != nil {
		return err
	}
	owners := make(map[int32]*dto.AttachmentOwner, len(users))
	for _, user := range users {
		owners[user.ID] = &dto.AttachmentOwner{
			ID:       user.ID,
			Username: user.Username,
			Nickname: user.Nickname,
		}
	}
	for i, attachment := range attachments {
		dtos[i].Owner = owners[attachment.UserID]
	}
	return nil
}

func (a *attachmentServiceImpl) ConvertToDTOs(ctx context.Context, attachments []*entity.Attachment) ([]*dto.AttachmentDTO, error) {
	dtos := make([]*dto.AttachmentDTO, 0, len(attachments))
	for _, attachment := range attachments {
		dto := &dto.AttachmentDTO{
			ID:             attachment.ID,
			Name:           attachment.Name,
			Path:           attachment.Path,
			FileKey:        attachment.FileKey,
			ThumbPath:      attachment.ThumbPath,
			MediaType:      attachment.MediaType,
			Suffix:         attachment.Suffix,
			Width:          attachment.Width,
			Height:         attachment.Height,
			Size:           attachment.Size,
			AttachmentType: attachment.Type,
		}
		dtos = append(dtos, dto)
		path, err := a.FileStorageComposite.GetFileStorage(attachment.Type).GetFilePath(ctx, attachment.Path)
		if err != nil {
			log.CtxError(ctx, "GetFilePath err", zap.Error(err))
		}
		thumbPath, err := a.getThumbURL(ctx, attachment.Type, attachment.ThumbPath)
		if err != nil {
			log.CtxError(ctx, "GetFilePath err", zap.Error(err))
		}
		dto.Path = path
		dto.ThumbPath = thumbPath
	}
	if err := a.fillOwners(ctx, attachments, dtos); err != nil {
		return nil, err
	}
	return dtos, nil
}

func NewAttachmentService(optionService service.OptionService, fileStorageComposite storage.FileStorageComposite, userService service.UserService) service.AttachmentService {
	return &attachmentServiceImpl{
		FileStorageComposite: fileStorageComposite,
		OptionService:        optionService,
		UserService:          userService,
	}
}

// getThumbURL 返回缩略图的可访问地址。空 ThumbPath 表示该附件没有缩略图（非图片附件即如此），
// 直接返回空串——把空串交给 GetFilePath 会经 url.JoinPath 拼出存储桶根地址这类无意义 URL。
func (a *attachmentServiceImpl) getThumbURL(ctx context.Context, attachmentType consts.AttachmentType, thumbPath string) (string, error) {
	if thumbPath == "" {
		return "", nil
	}
	return a.FileStorageComposite.GetFileStorage(attachmentType).GetFilePath(ctx, thumbPath)
}

func (a *attachmentServiceImpl) ConvertToDTO(ctx context.Context, attachment *entity.Attachment) (*dto.AttachmentDTO, error) {
	attachmentDTO := &dto.AttachmentDTO{
		ID:             attachment.ID,
		Name:           attachment.Name,
		Path:           attachment.Path,
		FileKey:        attachment.FileKey,
		ThumbPath:      attachment.ThumbPath,
		MediaType:      attachment.MediaType,
		Suffix:         attachment.Suffix,
		Width:          attachment.Width,
		Height:         attachment.Height,
		Size:           attachment.Size,
		AttachmentType: attachment.Type,
	}
	path, err := a.FileStorageComposite.GetFileStorage(attachment.Type).GetFilePath(ctx, attachment.Path)
	if err != nil {
		return nil, err
	}
	thumbPath, err := a.getThumbURL(ctx, attachment.Type, attachment.ThumbPath)
	if err != nil {
		return nil, err
	}
	attachmentDTO.Path = path
	attachmentDTO.ThumbPath = thumbPath
	if err := a.fillOwners(ctx, []*entity.Attachment{attachment}, []*dto.AttachmentDTO{attachmentDTO}); err != nil {
		return nil, err
	}
	return attachmentDTO, nil
}

func (a *attachmentServiceImpl) Page(ctx context.Context, queryParam *param.AttachmentQuery) ([]*entity.Attachment, int64, error) {
	attachmentDAL := dal.GetQueryByCtx(ctx).Attachment
	attachmentDo := attachmentDAL.WithContext(ctx)
	if queryParam.Keyword != "" {
		attachmentDo = attachmentDo.Where(attachmentDAL.Name.Like(queryParam.Keyword))
	}
	if queryParam.PageNum >= 0 && queryParam.PageSize >= 0 {
		if queryParam.PageSize > 50 {
			queryParam.PageSize = 50
		}
	} else {
		queryParam.PageSize = 10
		queryParam.PageNum = 0
	}
	if queryParam.Keyword != "" {
		attachmentDo = attachmentDo.Where(attachmentDAL.Name.Like(queryParam.Keyword))
	}
	if queryParam.AttachmentType != nil {
		attachmentDo = attachmentDo.Where(attachmentDAL.Type.Eq(*queryParam.AttachmentType))
	}
	if queryParam.MediaType != "" {
		attachmentDo = attachmentDo.Where(attachmentDAL.MediaType.Eq(queryParam.MediaType))
	}
	if user, ok := currentUser(ctx); ok && user.Role == consts.UserRoleAuthor {
		// 作者可见范围 = 自己上传的附件 + 自己文章关联的附件。
		// 管理员代传或历史无归属的附件不会转移 user_id，但文章转交后作者仍应能看到它们。
		query := dal.GetQueryByCtx(ctx)
		ownPostIDs := make([]int32, 0)
		if err := query.Post.WithContext(ctx).
			Where(query.Post.AuthorID.Eq(user.ID)).
			Pluck(query.Post.ID, &ownPostIDs); err != nil {
			return nil, 0, WrapDBErr(err)
		}
		linkedAttachmentIDs := make([]int32, 0)
		if len(ownPostIDs) > 0 {
			if err := query.PostAttachment.WithContext(ctx).
				Where(query.PostAttachment.PostID.In(ownPostIDs...)).
				Pluck(query.PostAttachment.AttachmentID, &linkedAttachmentIDs); err != nil {
				return nil, 0, WrapDBErr(err)
			}
		}
		authorScope := attachmentDAL.WithContext(ctx).Where(attachmentDAL.UserID.Eq(user.ID))
		if len(linkedAttachmentIDs) > 0 {
			authorScope = authorScope.Or(attachmentDAL.ID.In(linkedAttachmentIDs...))
		}
		attachmentDo = attachmentDo.Where(authorScope)
	}
	attachments, totalCount, err := attachmentDo.FindByPage(queryParam.PageNum*queryParam.PageSize, queryParam.PageSize)
	if err != nil {
		return nil, 0, WrapDBErr(err)
	}
	return attachments, totalCount, nil
}

func (a *attachmentServiceImpl) GetAttachment(ctx context.Context, attachmentID int32) (*entity.Attachment, error) {
	attachmentDAL := dal.GetQueryByCtx(ctx).Attachment
	attachment, err := attachmentDAL.WithContext(ctx).Where(attachmentDAL.ID.Eq(attachmentID)).Take()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	if err := ensureAuthorOwnsAttachment(ctx, attachment); err != nil {
		// 作者对自己文章关联的附件（可能是管理员代传或历史无归属）应可只读访问。
		linked, linkErr := canAuthorReadLinkedAttachment(ctx, attachmentID)
		if linkErr != nil {
			return nil, linkErr
		}
		if !linked {
			return nil, err
		}
	}
	return attachment, nil
}

// canAuthorReadLinkedAttachment 判断当前 Author 是否通过自己的文章关联到该附件。
func canAuthorReadLinkedAttachment(ctx context.Context, attachmentID int32) (bool, error) {
	user, ok := currentUser(ctx)
	if !ok || user.Role != consts.UserRoleAuthor {
		return false, nil
	}
	query := dal.GetQueryByCtx(ctx)
	ownPostIDs := make([]int32, 0)
	if err := query.Post.WithContext(ctx).
		Where(query.Post.AuthorID.Eq(user.ID)).
		Pluck(query.Post.ID, &ownPostIDs); err != nil {
		return false, WrapDBErr(err)
	}
	if len(ownPostIDs) == 0 {
		return false, nil
	}
	count, err := query.PostAttachment.WithContext(ctx).
		Where(query.PostAttachment.AttachmentID.Eq(attachmentID), query.PostAttachment.PostID.In(ownPostIDs...)).
		Count()
	if err != nil {
		return false, WrapDBErr(err)
	}
	return count > 0, nil
}

func (a *attachmentServiceImpl) Upload(ctx context.Context, fileHeader *multipart.FileHeader) (attachmentDTO *dto.AttachmentDTO, err error) {
	attachmentType := a.OptionService.GetAttachmentType(ctx)

	fileStorage := a.FileStorageComposite.GetFileStorage(attachmentType)
	attachmentDAL := dal.GetQueryByCtx(ctx).Attachment

	attachmentDTO, err = fileStorage.Upload(ctx, fileHeader)
	if err != nil {
		return nil, err
	}

	record, err := attachmentDAL.WithContext(ctx).Where(attachmentDAL.Path.Eq(attachmentDTO.Path)).Take()
	if record != nil && err == nil {
		return nil, xerr.BadParam.New("附件路径为 %s 已经存在", attachmentDTO.Path).
			WithStatus(xerr.StatusBadRequest).
			WithMsg("附件路径为 " + attachmentDTO.Path + " 已经存在")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, WrapDBErr(err)
	}
	attachmentEntity := &entity.Attachment{
		FileKey:   attachmentDTO.FileKey,
		Height:    attachmentDTO.Height,
		MediaType: attachmentDTO.MediaType,
		Name:      attachmentDTO.Name,
		Path:      strings.ReplaceAll(attachmentDTO.Path, string(os.PathSeparator), "/"),
		Size:      attachmentDTO.Size,
		Suffix:    attachmentDTO.Suffix,
		ThumbPath: strings.ReplaceAll(attachmentDTO.ThumbPath, string(os.PathSeparator), "/"),
		Type:      attachmentDTO.AttachmentType,
		Width:     attachmentDTO.Width,
	}
	if user, ok := currentUser(ctx); ok {
		attachmentEntity.UserID = user.ID
	}

	// change file path separator to url separator
	if err = attachmentDAL.WithContext(ctx).Create(attachmentEntity); err != nil {
		return nil, WrapDBErr(err)
	}
	attachmentDTO.ID = attachmentEntity.ID
	attachmentDTO.Path, err = fileStorage.GetFilePath(ctx, attachmentEntity.Path)
	if err != nil {
		return nil, err
	}
	attachmentDTO.ThumbPath, err = a.getThumbURL(ctx, attachmentType, attachmentEntity.ThumbPath)
	if err != nil {
		return nil, err
	}
	if err := a.fillOwners(ctx, []*entity.Attachment{attachmentEntity}, []*dto.AttachmentDTO{attachmentDTO}); err != nil {
		return nil, err
	}

	return attachmentDTO, nil
}

func (a *attachmentServiceImpl) Delete(ctx context.Context, attachmentID int32, force bool) (*entity.Attachment, error) {
	attachmentDAL := dal.GetQueryByCtx(ctx).Attachment
	attachment, err := attachmentDAL.WithContext(ctx).Where(attachmentDAL.ID.Eq(attachmentID)).Take()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	if err := ensureAttachmentDeletePermission(ctx, attachment, force); err != nil {
		return nil, err
	}
	if !force {
		postIDs, err := a.referencedPostIDs(ctx, []int32{attachmentID})
		if err != nil {
			return nil, err
		}
		if len(postIDs) > 0 {
			posts, err := a.referencedPosts(ctx, postIDs)
			if err != nil {
				return nil, err
			}
			return nil, newAttachmentReferencedError([]*entity.Attachment{attachment}, len(postIDs), posts)
		}
	}
	if err := a.deleteAttachmentRecord(ctx, attachment); err != nil {
		return nil, err
	}
	return attachment, nil
}

func (a *attachmentServiceImpl) DeleteBatch(ctx context.Context, ids []int32, force bool) (attachments []*entity.Attachment, err error) {
	uniqueIDMap := make(map[int32]struct{}, len(ids))
	uniqueIDs := make([]int32, 0, len(ids))
	for _, id := range ids {
		if _, ok := uniqueIDMap[id]; ok {
			continue
		}
		uniqueIDMap[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	if len(uniqueIDs) == 0 {
		return make([]*entity.Attachment, 0), nil
	}

	query := dal.GetQueryByCtx(ctx)
	existing, err := query.Attachment.WithContext(ctx).Where(query.Attachment.ID.In(uniqueIDs...)).Find()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	for _, attachment := range existing {
		if err := ensureAttachmentDeletePermission(ctx, attachment, force); err != nil {
			return nil, err
		}
	}
	if !force {
		postIDs, err := a.referencedPostIDs(ctx, uniqueIDs)
		if err != nil {
			return nil, err
		}
		if len(postIDs) > 0 {
			posts, err := a.referencedPosts(ctx, postIDs)
			if err != nil {
				return nil, err
			}
			return nil, newAttachmentReferencedError(existing, len(postIDs), posts)
		}
	}

	attachments = make([]*entity.Attachment, 0, len(existing))
	var globalErr error
	for _, attachment := range existing {
		if err := a.deleteAttachmentRecord(ctx, attachment); err != nil {
			globalErr = err
		} else {
			attachments = append(attachments, attachment)
		}
	}
	if globalErr != nil {
		return attachments, xerr.NoType.Wrap(globalErr).WithMsg("Failed to delete some files")
	}
	return attachments, nil
}

// deleteAttachmentRecord 删除附件关联、附件记录与物理文件，不包含权限与引用检查。
func (a *attachmentServiceImpl) deleteAttachmentRecord(ctx context.Context, attachment *entity.Attachment) error {
	err := dal.GetQueryByCtx(ctx).Transaction(func(tx *dal.Query) error {
		postAttachmentDAL := tx.PostAttachment
		if _, err := postAttachmentDAL.WithContext(ctx).Where(postAttachmentDAL.AttachmentID.Eq(attachment.ID)).Delete(); err != nil {
			return WrapDBErr(err)
		}
		result, err := tx.Attachment.WithContext(ctx).Where(tx.Attachment.ID.Eq(attachment.ID)).Delete()
		if err != nil || result.RowsAffected != 1 {
			return xerr.WithMsg(err, "delete file failed")
		}
		return nil
	})
	if err != nil {
		return err
	}
	fileStorage := a.FileStorageComposite.GetFileStorage(attachment.Type)
	if err := fileStorage.Delete(ctx, attachment.FileKey); err != nil {
		return xerr.WithMsg(err, "delete file failed")
	}
	return nil
}

// referencedPostIDs 返回引用这些附件的文章 ID。
// 直接基于关联记录判断，即使文章记录已丢失也视为存在引用。
func (a *attachmentServiceImpl) referencedPostIDs(ctx context.Context, attachmentIDs []int32) ([]int32, error) {
	postIDs := make([]int32, 0)
	if len(attachmentIDs) == 0 {
		return postIDs, nil
	}
	query := dal.GetQueryByCtx(ctx)
	if err := query.PostAttachment.WithContext(ctx).
		Where(query.PostAttachment.AttachmentID.In(attachmentIDs...)).
		Pluck(query.PostAttachment.PostID, &postIDs); err != nil {
		return nil, WrapDBErr(err)
	}
	return postIDs, nil
}

// referencedPosts 查询引用文章详情，用于删除冲突提示。
func (a *attachmentServiceImpl) referencedPosts(ctx context.Context, postIDs []int32) ([]*dto.AttachmentReferencedPost, error) {
	if len(postIDs) == 0 {
		return []*dto.AttachmentReferencedPost{}, nil
	}
	query := dal.GetQueryByCtx(ctx)
	posts, err := query.Post.WithContext(ctx).
		Where(query.Post.ID.In(postIDs...)).
		Order(query.Post.ID).
		Find()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	result := make([]*dto.AttachmentReferencedPost, 0, len(posts))
	for _, post := range posts {
		result = append(result, &dto.AttachmentReferencedPost{
			ID:    post.ID,
			Title: post.Title,
			Slug:  post.Slug,
		})
	}
	return result, nil
}

// ensureAttachmentDeletePermission 校验删除权限：作者只能删除自己的附件且不能 force。
func ensureAttachmentDeletePermission(ctx context.Context, attachment *entity.Attachment, force bool) error {
	user, ok := currentUser(ctx)
	if !ok || user.Role != consts.UserRoleAuthor {
		return nil
	}
	if attachment.UserID != user.ID {
		return xerr.WithStatus(nil, xerr.StatusForbidden).WithMsg("无权删除该附件")
	}
	if force {
		return xerr.WithStatus(nil, xerr.StatusForbidden).WithMsg("作者无权强制删除附件")
	}
	return nil
}

func newAttachmentReferencedError(attachments []*entity.Attachment, count int, posts []*dto.AttachmentReferencedPost) error {
	data := &dto.AttachmentReferenceData{Posts: posts}
	if len(attachments) == 1 {
		return xerr.NoType.New("attachment %d referenced by %d posts", attachments[0].ID, count).
			WithStatus(xerr.StatusConflict).
			WithMsg(fmt.Sprintf("该附件已被 %d 篇文章引用，强制删除将同时解除文章关联", count)).
			WithData(data)
	}
	return xerr.NoType.New("attachments referenced by posts").
		WithStatus(xerr.StatusConflict).
		WithMsg("选中的附件中有被文章引用的附件，强制删除将同时解除文章关联").
		WithData(data)
}

func (a *attachmentServiceImpl) Update(ctx context.Context, id int32, updateParam *param.AttachmentUpdate) (*entity.Attachment, error) {
	attachmentDAL := dal.GetQueryByCtx(ctx).Attachment
	attachment, err := attachmentDAL.WithContext(ctx).Where(attachmentDAL.ID.Eq(id)).Take()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	result, err := attachmentDAL.WithContext(ctx).Where(attachmentDAL.ID.Eq(id)).Updates(entity.Attachment{Name: updateParam.Name})
	if err != nil {
		return nil, WrapDBErr(err)
	}
	if result.RowsAffected != 1 {
		return nil, xerr.NoType.New("update failed")
	}
	attachment.Name = updateParam.Name
	return attachment, nil
}

func (a *attachmentServiceImpl) GetAllMediaTypes(ctx context.Context) ([]string, error) {
	attachmentDAL := dal.GetQueryByCtx(ctx).Attachment
	var allMediaTypes []string
	err := attachmentDAL.WithContext(ctx).Distinct(attachmentDAL.MediaType).Select(attachmentDAL.MediaType).Scan(&allMediaTypes)
	return allMediaTypes, WrapDBErr(err)
}

// GetAllTypes 返回可作为「存储位置」筛选项的附件类型：系统设置中当前配置的存储位置，
// 加上附件库中已经存在过的类型。前者保证刚配置完、还没有任何附件上传时也能被筛到。
func (a *attachmentServiceImpl) GetAllTypes(ctx context.Context) ([]consts.AttachmentType, error) {
	attachmentDAL := dal.GetQueryByCtx(ctx).Attachment
	var existedTypes []consts.AttachmentType
	err := attachmentDAL.WithContext(ctx).Distinct(attachmentDAL.Type).Select(attachmentDAL.Type).Scan(&existedTypes)
	if err != nil {
		return nil, WrapDBErr(err)
	}

	allTypes := make([]consts.AttachmentType, 0, len(existedTypes)+1)
	seen := make(map[consts.AttachmentType]struct{}, len(existedTypes)+1)
	add := func(attachmentType consts.AttachmentType) {
		if _, ok := seen[attachmentType]; ok {
			return
		}
		seen[attachmentType] = struct{}{}
		allTypes = append(allTypes, attachmentType)
	}

	add(a.OptionService.GetAttachmentType(ctx))
	for _, attachmentType := range existedTypes {
		add(attachmentType)
	}
	return allTypes, nil
}
