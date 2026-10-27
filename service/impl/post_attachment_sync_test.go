package impl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/model/entity"
)

var postAttachmentDSNSeq int64

func setupPostAttachmentDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:airpress_post_attachment_%d?mode=memory&cache=shared",
		atomic.AddInt64(&postAttachmentDSNSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&entity.User{}, &entity.Post{}, &entity.Attachment{}, &entity.PostAttachment{},
		&entity.PostCategory{}, &entity.PostTag{}, &entity.Meta{}, &entity.Comment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	prev := *dal.Q
	t.Cleanup(func() {
		*dal.Q = prev
		_ = sqlDB.Close()
	})
	dal.SetDefault(db)
	return db
}

func postActorContext(user *entity.User) context.Context {
	return context.WithValue(context.Background(), consts.AuthorizedUser, user)
}

func createPostAttachmentAuthor(t *testing.T, db *gorm.DB, username string) *entity.User {
	t.Helper()
	user := &entity.User{
		Username: username,
		Nickname: username,
		Email:    username + "@example.com",
		Password: "hashed",
		Role:     consts.UserRoleAuthor,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestCreateOrUpdateSyncsPostAttachment(t *testing.T) {
	db := setupPostAttachmentDB(t)
	author := createPostAttachmentAuthor(t, db, "author")

	image := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: author.ID}
	if err := db.Create(image).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	post := &entity.Post{
		Title:           "post",
		Slug:            "post",
		Type:            consts.PostTypePost,
		Status:          consts.PostStatusDraft,
		OriginalContent: "![x](upload/image/2026/10/a.png)",
	}
	ctx := postActorContext(author)
	created, err := basePostServiceImpl{}.CreateOrUpdate(ctx, post, nil, nil, nil)
	if err != nil {
		t.Fatalf("CreateOrUpdate() error = %v", err)
	}

	var count int64
	if err := db.Model(&entity.PostAttachment{}).
		Where("post_id = ? AND attachment_id = ?", created.ID, image.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if count != 1 {
		t.Fatalf("relations = %d, want 1", count)
	}
}

func TestCreateOrUpdateReplacesStalePostAttachment(t *testing.T) {
	db := setupPostAttachmentDB(t)
	author := createPostAttachmentAuthor(t, db, "author")

	first := &entity.Attachment{Name: "first", Path: "upload/image/2026/10/a.png", UserID: author.ID}
	second := &entity.Attachment{Name: "second", Path: "upload/image/2026/10/b.png", UserID: author.ID}
	if err := db.Create(first).Error; err != nil {
		t.Fatalf("create first: %v", err)
	}
	if err := db.Create(second).Error; err != nil {
		t.Fatalf("create second: %v", err)
	}

	ctx := postActorContext(author)
	post := &entity.Post{
		Title:           "post",
		Slug:            "post",
		Type:            consts.PostTypePost,
		Status:          consts.PostStatusDraft,
		OriginalContent: "![x](upload/image/2026/10/a.png)",
	}
	created, err := basePostServiceImpl{}.CreateOrUpdate(ctx, post, nil, nil, nil)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	update := &entity.Post{
		ID:              created.ID,
		Title:           "post",
		Slug:            "post",
		Type:            consts.PostTypePost,
		Status:          consts.PostStatusDraft,
		OriginalContent: "![x](upload/image/2026/10/b.png)",
	}
	if _, err := (basePostServiceImpl{}).CreateOrUpdate(ctx, update, nil, nil, nil); err != nil {
		t.Fatalf("update post: %v", err)
	}

	var relations []entity.PostAttachment
	if err := db.Where("post_id = ?", created.ID).Find(&relations).Error; err != nil {
		t.Fatalf("load relations: %v", err)
	}
	if len(relations) != 1 || relations[0].AttachmentID != second.ID {
		t.Fatalf("relations = %+v, want only attachment %d", relations, second.ID)
	}
}

func TestUpdateDraftContentSyncsPostAttachment(t *testing.T) {
	db := setupPostAttachmentDB(t)
	author := createPostAttachmentAuthor(t, db, "author")

	image := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: author.ID}
	if err := db.Create(image).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	post := &entity.Post{
		Title:    "post",
		Slug:     "post",
		Type:     consts.PostTypePost,
		Status:   consts.PostStatusDraft,
		AuthorID: author.ID,
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	ctx := postActorContext(author)
	if _, err := (basePostServiceImpl{}).UpdateDraftContent(ctx, post.ID,
		"![x](upload/image/2026/10/a.png)", "![x](upload/image/2026/10/a.png)"); err != nil {
		t.Fatalf("UpdateDraftContent() error = %v", err)
	}

	var count int64
	if err := db.Model(&entity.PostAttachment{}).
		Where("post_id = ? AND attachment_id = ?", post.ID, image.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if count != 1 {
		t.Fatalf("relations = %d, want 1", count)
	}
}

func TestDeletePostCleansPostAttachment(t *testing.T) {
	db := setupPostAttachmentDB(t)
	author := createPostAttachmentAuthor(t, db, "author")

	post := &entity.Post{Title: "post", Slug: "post", Type: consts.PostTypePost, AuthorID: author.ID}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	relation := &entity.PostAttachment{PostID: post.ID, AttachmentID: 99}
	if err := db.Create(relation).Error; err != nil {
		t.Fatalf("create relation: %v", err)
	}

	ctx := postActorContext(author)
	if err := (basePostServiceImpl{}).Delete(ctx, post.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	var count int64
	if err := db.Model(&entity.PostAttachment{}).Where("post_id = ?", post.ID).Count(&count).Error; err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if count != 0 {
		t.Fatalf("relations = %d, want 0", count)
	}
}

func TestDeleteAttachmentCleansPostAttachment(t *testing.T) {
	db := setupPostAttachmentDB(t)

	attachment := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: 1, FileKey: "k"}
	if err := db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	relation := &entity.PostAttachment{PostID: 1, AttachmentID: attachment.ID}
	if err := db.Create(relation).Error; err != nil {
		t.Fatalf("create relation: %v", err)
	}

	svc := &attachmentServiceImpl{
		FileStorageComposite: &stubStorageComposite{storage: &stubFileStorage{}},
	}
	if _, err := svc.Delete(context.Background(), attachment.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	var count int64
	if err := db.Model(&entity.PostAttachment{}).Where("attachment_id = ?", attachment.ID).Count(&count).Error; err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if count != 0 {
		t.Fatalf("relations = %d, want 0", count)
	}
}

func TestUpdateDraftContentMatchesOriginalContent(t *testing.T) {
	db := setupPostAttachmentDB(t)
	author := createPostAttachmentAuthor(t, db, "author")

	image := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: author.ID}
	if err := db.Create(image).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	post := &entity.Post{
		Title:    "post",
		Slug:     "post",
		Type:     consts.PostTypePost,
		Status:   consts.PostStatusDraft,
		AuthorID: author.ID,
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	ctx := postActorContext(author)
	// 只出现在 Markdown 原文、未出现在渲染结果中的附件也必须建立关联。
	if _, err := (basePostServiceImpl{}).UpdateDraftContent(ctx, post.ID,
		"<p>no image</p>", "![x](upload/image/2026/10/a.png)"); err != nil {
		t.Fatalf("UpdateDraftContent() error = %v", err)
	}

	var count int64
	if err := db.Model(&entity.PostAttachment{}).
		Where("post_id = ? AND attachment_id = ?", post.ID, image.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if count != 1 {
		t.Fatalf("relations = %d, want 1", count)
	}
}
