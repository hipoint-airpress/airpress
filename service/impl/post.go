package impl

import (
	"context"
	"database/sql/driver"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/event"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type postServiceImpl struct {
	service.BasePostService
	CategoryService service.CategoryService
	OptionService   service.OptionService
	Event           event.Bus
	Cache           cache.Cache
}

func NewPostService(basePostService service.BasePostService,
	categoryService service.CategoryService,
	optionService service.OptionService,
	event event.Bus,
	cache cache.Cache,
) service.PostService {
	return &postServiceImpl{
		BasePostService: basePostService,
		CategoryService: categoryService,
		OptionService:   optionService,
		Event:           event,
		Cache:           cache,
	}
}

func (p postServiceImpl) Page(ctx context.Context, postQuery param.PostQuery) ([]*entity.Post, int64, error) {
	if postQuery.PageNum < 0 || postQuery.PageSize <= 0 {
		return nil, 0, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("Paging parameter error")
	}
	postDAL := dal.GetQueryByCtx(ctx).Post
	postCategoryDAL := dal.GetQueryByCtx(ctx).PostCategory
	postDo := postDAL.WithContext(ctx).Where(postDAL.Type.Eq(consts.PostTypePost))
	err := BuildSort(postQuery.Sort, &postDAL, &postDo)
	if err != nil {
		return nil, 0, err
	}

	if postQuery.Keyword != nil {
		postDo.Where(postDAL.Title.Like("%" + *postQuery.Keyword + "%")).Or(postDAL.OriginalContent.Like("%" + *postQuery.Keyword + "%"))
	}

	if postQuery.WithPassword != nil && !*postQuery.WithPassword {
		postDo.Where(postDAL.Password.Neq(""))
	}
	if len(postQuery.Statuses) > 0 {
		statuesValue := make([]driver.Valuer, len(postQuery.Statuses))
		for i, status := range postQuery.Statuses {
			statuesValue[i] = driver.Valuer(status)
		}
		postDo = postDo.Where(postDAL.Status.In(statuesValue...))
	}
	if postQuery.CategoryID != nil {
		postDo.Join(&entity.PostCategory{}, postDAL.ID.EqCol(postCategoryDAL.PostID)).Where(postCategoryDAL.CategoryID.Eq(*postQuery.CategoryID))
	}
	if user, ok := currentUser(ctx); ok && user.Role == consts.UserRoleAuthor {
		postDo = postDo.Where(postDAL.AuthorID.Eq(user.ID))
	}

	posts, totalCount, err := postDo.FindByPage(postQuery.PageNum*postQuery.PageSize, postQuery.PageSize)
	if err != nil {
		return nil, 0, WrapDBErr(err)
	}
	return posts, totalCount, nil
}

func (p postServiceImpl) IncreaseLike(ctx context.Context, postID int32) error {
	postDAL := dal.GetQueryByCtx(ctx).Post
	info, err := postDAL.WithContext(ctx).Where(postDAL.ID.Eq(postID)).UpdateSimple(postDAL.Likes.Add(1))
	if err != nil {
		return WrapDBErr(err)
	}
	if info.RowsAffected != 1 {
		return xerr.NoType.New("increase post like failed postID=%v", postID).WithStatus(xerr.StatusBadRequest).WithMsg("failed to like post")
	}
	return nil
}

func (p postServiceImpl) Create(ctx context.Context, postParam *param.Post) (*entity.Post, error) {
	post, err := p.ConvertParam(ctx, postParam)
	if err != nil {
		return nil, err
	}
	needEncrypt, err := p.CategoryService.IsCategoriesEncrypt(ctx, postParam.CategoryIDs...)
	if err != nil {
		return nil, nil
	}
	if post.Status != consts.PostStatusDraft && (post.Password != "" || needEncrypt) {
		post.Status = consts.PostStatusIntimate
	}

	post, err = p.CreateOrUpdate(ctx, post, postParam.CategoryIDs, postParam.TagIDs, postParam.MetaParam)
	if err != nil {
		return nil, err
	}
	// Todo delete authorization
	p.Event.Publish(ctx, &event.LogEvent{
		LogKey:    strconv.Itoa(int(post.ID)),
		LogType:   consts.LogTypePostPublished,
		Content:   post.Title,
		IPAddress: util.GetClientIP(ctx),
	})
	return post, nil
}

func (p postServiceImpl) Update(ctx context.Context, postID int32, postParam *param.Post) (*entity.Post, error) {
	postDAL := dal.GetQueryByCtx(ctx).Post
	post, err := postDAL.WithContext(ctx).Where(postDAL.ID.Eq(postID)).First()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	if err := ensureAuthorOwnsPost(ctx, post); err != nil {
		return nil, err
	}
	postToUpdate, err := p.ConvertParam(ctx, postParam)
	if err != nil {
		return nil, err
	}
	if postToUpdate.CreateTime.Equal(time.Time{}) {
		postToUpdate.CreateTime = post.CreateTime
	}
	postToUpdate.ID = post.ID
	post, err = p.CreateOrUpdate(ctx, postToUpdate, postParam.CategoryIDs, postParam.TagIDs, postParam.MetaParam)
	if err != nil {
		return nil, err
	}
	// TODO should use transcation
	p.Event.Publish(ctx, &event.PostUpdateEvent{
		PostID: post.ID,
	})
	p.Event.Publish(ctx, &event.LogEvent{
		LogKey:    strconv.Itoa(int(post.ID)),
		LogType:   consts.LogTypePostEdited,
		Content:   post.Title,
		IPAddress: util.GetClientIP(ctx),
	})
	return post, nil
}

func (p postServiceImpl) ConvertParam(ctx context.Context, postParam *param.Post) (*entity.Post, error) {
	post := &entity.Post{
		Type:            consts.PostTypePost,
		DisallowComment: postParam.DisallowComment,
		OriginalContent: postParam.OriginalContent,
		Password:        postParam.Password,
		MetaDescription: postParam.MetaDescription,
		MetaKeywords:    postParam.MetaKeywords,
		// 存短名（去 .tmpl 后缀），与 ListCustomTemplates 的下拉值约定一致
		Template:    strings.TrimSuffix(postParam.Template, ".tmpl"),
		Thumbnail:   postParam.Thumbnail,
		Title:       postParam.Title,
		TopPriority: postParam.TopPriority,
		Status:      postParam.Status,
		EditTime:    util.TimePtr(time.Now()),
		Summary:     postParam.Summary,
	}
	if postParam.EditorType != nil {
		post.EditorType = *postParam.EditorType
	} else {
		post.EditorType = consts.EditorTypeMarkdown
	}
	// 服务端统一渲染:markdown 类型的 FormatContent 由 OriginalContent 转 HTML,
	// 不信任客户端提交的 Content;富文本或原文为空时原样透传。
	if post.EditorType == consts.EditorTypeMarkdown && postParam.OriginalContent != "" {
		htmlContent, err := util.MarkdownToHTML(postParam.OriginalContent)
		if err != nil {
			return nil, xerr.BadParam.Wrapf(err, "convert markdown err").WithStatus(xerr.StatusBadRequest)
		}
		post.FormatContent = htmlContent
	} else {
		post.FormatContent = postParam.Content
	}
	if postParam.EditTime != nil {
		post.EditTime = util.TimePtr(time.UnixMilli(*postParam.EditTime))
	}
	if postParam.UpdateTime != nil {
		post.UpdateTime = util.TimePtr(time.UnixMilli(*postParam.UpdateTime))
	}

	post.WordCount = util.HTMLFormatWordCount(post.FormatContent)
	if postParam.Slug == "" {
		post.Slug = util.Slug(postParam.Title)
	} else {
		post.Slug = util.Slug(postParam.Slug)
	}
	if postParam.CreateTime != nil {
		post.CreateTime = time.UnixMilli(*postParam.CreateTime)
	} else {
		post.CreateTime = time.Now()
	}
	return post, nil
}

func (p postServiceImpl) CountByStatus(ctx context.Context, status consts.PostStatus) (int64, error) {
	postDAL := dal.GetQueryByCtx(ctx).Post
	count, err := postDAL.WithContext(ctx).Where(postDAL.Type.Eq(consts.PostTypePost), postDAL.Status.Eq(status)).Count()
	if err != nil {
		return 0, WrapDBErr(err)
	}
	return count, nil
}

func (p postServiceImpl) Preview(ctx context.Context, postID int32) (string, error) {
	post, err := p.GetByPostID(ctx, postID)
	if err != nil {
		return "", err
	}
	token := util.GenUUIDWithOutDash()
	p.Cache.Set(token, token, time.Minute*10)

	previewURL := strings.Builder{}

	isEnabledAbsolutePath, err := p.OptionService.IsEnabledAbsolutePath(ctx)
	if err != nil {
		return "", err
	}
	if isEnabledAbsolutePath {
		blogBaseURL, err := p.OptionService.GetBlogBaseURL(ctx)
		if err != nil {
			return "", err
		}
		previewURL.WriteString(blogBaseURL)
	}
	post.Slug = url.QueryEscape(post.Slug)
	fullPath, err := p.BuildFullPath(ctx, post)
	if err != nil {
		return "", err
	}
	previewURL.WriteString("/admin_preview")
	previewURL.WriteString(fullPath)
	previewURL.WriteString("?token=")
	previewURL.WriteString(token)
	return previewURL.String(), nil
}

func (p postServiceImpl) CountVisit(ctx context.Context) (int64, error) {
	var count float64
	postDAL := dal.GetQueryByCtx(ctx).Post
	err := postDAL.WithContext(ctx).Select(postDAL.Visits.Sum().IfNull(0)).Where(postDAL.Type.Eq(consts.PostTypePost), postDAL.Status.Eq(consts.PostStatusPublished)).Scan(&count)
	if err != nil {
		return 0, WrapDBErr(err)
	}
	return int64(count), nil
}

func (p postServiceImpl) CountLike(ctx context.Context) (int64, error) {
	var count float64
	postDAL := dal.GetQueryByCtx(ctx).Post
	err := postDAL.WithContext(ctx).Select(postDAL.Likes.Sum().IfNull(0)).Where(postDAL.Type.Eq(consts.PostTypePost), postDAL.Status.Eq(consts.PostStatusPublished)).Scan(&count)
	if err != nil {
		return 0, WrapDBErr(err)
	}
	return int64(count), nil
}

func (p postServiceImpl) GetPrevPosts(ctx context.Context, post *entity.Post, size int) ([]*entity.Post, error) {
	postSort := p.OptionService.GetOrByDefault(ctx, property.IndexSort)
	postDAL := dal.GetQueryByCtx(ctx).Post
	postDO := postDAL.WithContext(ctx).Where(postDAL.Status.Eq(consts.PostStatusPublished), postDAL.Type.Eq(consts.PostTypePost))

	switch postSort {
	case "createTime":
		postDO = postDO.Where(postDAL.CreateTime.Gt(post.CreateTime)).Order(postDAL.CreateTime)
	case "editTime":
		editTime := post.CreateTime
		if post.EditTime != nil {
			editTime = *post.EditTime
		}
		postDO = postDO.Where(postDAL.EditTime.Gt(editTime)).Order(postDAL.EditTime)
	case "visits":
		postDO = postDO.Where(postDAL.Visits.Gt(post.Visits)).Order(postDAL.EditTime)
	default:
		return nil, nil
	}

	posts, err := postDO.Find()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, WrapDBErr(err)
	}
	return posts, nil
}

func (p postServiceImpl) GetNextPosts(ctx context.Context, post *entity.Post, size int) ([]*entity.Post, error) {
	postSort := p.OptionService.GetOrByDefault(ctx, property.IndexSort)
	postDAL := dal.GetQueryByCtx(ctx).Post
	postDO := postDAL.WithContext(ctx).Where(postDAL.Status.Eq(consts.PostStatusPublished), postDAL.Type.Eq(consts.PostTypePost))

	switch postSort {
	case "createTime":
		postDO = postDO.Where(postDAL.CreateTime.Lt(post.CreateTime)).Order(postDAL.CreateTime.Desc())
	case "editTime":
		editTime := post.CreateTime
		if post.EditTime != nil {
			editTime = *post.EditTime
		}
		postDO = postDO.Where(postDAL.EditTime.Lt(editTime)).Order(postDAL.EditTime.Desc())
	case "visits":
		postDO = postDO.Where(postDAL.Visits.Lt(post.Visits)).Order(postDAL.EditTime.Desc())
	default:
		return nil, nil
	}

	posts, err := postDO.Find()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return posts, nil
}

const maxTransferPostCount = 100

func (p postServiceImpl) TransferAuthor(ctx context.Context, postIDs []int32, authorID int32) (*dto.TransferPostAuthorResult, error) {
	actor, ok := currentUser(ctx)
	if !ok || (actor.Role != consts.UserRoleEditor && actor.Role != consts.UserRoleSuperAdmin) {
		return nil, xerr.WithStatus(nil, xerr.StatusForbidden).WithMsg("无权转交文章")
	}
	if authorID <= 0 {
		return nil, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("目标作者参数错误")
	}

	uniqueIDs := make([]int32, 0, len(postIDs))
	seen := make(map[int32]struct{}, len(postIDs))
	for _, postID := range postIDs {
		if postID <= 0 {
			continue
		}
		if _, exists := seen[postID]; exists {
			continue
		}
		seen[postID] = struct{}{}
		uniqueIDs = append(uniqueIDs, postID)
	}
	if len(uniqueIDs) == 0 {
		return nil, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("请选择文章")
	}
	if len(uniqueIDs) > maxTransferPostCount {
		return nil, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("单次最多转交 100 篇文章")
	}

	userDAL := dal.GetQueryByCtx(ctx).User
	target, err := userDAL.WithContext(ctx).Where(userDAL.ID.Eq(authorID)).Take()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("目标作者不存在")
		}
		return nil, WrapDBErr(err)
	}
	if target.Disabled {
		return nil, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("目标作者已被禁用")
	}

	postDAL := dal.GetQueryByCtx(ctx).Post
	posts, err := postDAL.WithContext(ctx).
		Where(postDAL.ID.In(uniqueIDs...), postDAL.Type.Eq(consts.PostTypePost)).
		Find()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	if len(posts) == 0 {
		return nil, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("没有可转交的文章")
	}

	postIDsToUpdate := make([]int32, 0, len(posts))
	authorIDs := make([]int32, 0, len(posts))
	seenAuthor := make(map[int32]struct{}, len(posts))
	for _, post := range posts {
		postIDsToUpdate = append(postIDsToUpdate, post.ID)
		if _, exists := seenAuthor[post.AuthorID]; exists {
			continue
		}
		seenAuthor[post.AuthorID] = struct{}{}
		authorIDs = append(authorIDs, post.AuthorID)
	}

	attachmentCount := int64(0)
	err = dal.GetQueryByCtx(ctx).Transaction(func(tx *dal.Query) error {
		postAttachmentDAL := tx.PostAttachment
		relations, err := postAttachmentDAL.WithContext(ctx).Where(postAttachmentDAL.PostID.In(postIDsToUpdate...)).Find()
		if err != nil {
			return WrapDBErr(err)
		}
		attachmentIDs := make([]int32, 0, len(relations))
		seenAttachment := make(map[int32]struct{}, len(relations))
		for _, relation := range relations {
			if _, exists := seenAttachment[relation.AttachmentID]; exists {
				continue
			}
			seenAttachment[relation.AttachmentID] = struct{}{}
			attachmentIDs = append(attachmentIDs, relation.AttachmentID)
		}

		if _, err := tx.Post.WithContext(ctx).Where(tx.Post.ID.In(postIDsToUpdate...)).
			UpdateSimple(tx.Post.AuthorID.Value(authorID)); err != nil {
			return WrapDBErr(err)
		}
		if len(attachmentIDs) == 0 {
			return nil
		}
		updateResult, err := tx.Attachment.WithContext(ctx).
			Where(tx.Attachment.ID.In(attachmentIDs...), tx.Attachment.UserID.In(authorIDs...)).
			UpdateSimple(tx.Attachment.UserID.Value(authorID))
		if err != nil {
			return WrapDBErr(err)
		}
		attachmentCount = updateResult.RowsAffected
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &dto.TransferPostAuthorResult{
		PostCount:       int64(len(postIDsToUpdate)),
		AttachmentCount: attachmentCount,
	}, nil
}
