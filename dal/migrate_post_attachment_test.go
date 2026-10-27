package dal

import (
	"fmt"
	"sync/atomic"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
)

var migratePostAttachmentDSNSeq int64

func setupMigratePostAttachmentDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:airpress_migrate_post_attachment_%d?mode=memory&cache=shared",
		atomic.AddInt64(&migratePostAttachmentDSNSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&entity.User{}, &entity.Post{}, &entity.Attachment{}, &entity.PostAttachment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

func TestMigratePostAttachmentBackfillsReferences(t *testing.T) {
	db := setupMigratePostAttachmentDB(t)

	user := &entity.User{Username: "a", Nickname: "A", Email: "a@example.com", Role: consts.UserRoleAuthor}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	image := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: user.ID}
	if err := db.Create(image).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	used := &entity.Post{
		Title:           "used",
		Slug:            "used",
		Type:            consts.PostTypePost,
		AuthorID:        user.ID,
		OriginalContent: "![x](upload/image/2026/10/a.png)",
	}
	unused := &entity.Post{
		Title:    "unused",
		Slug:     "unused",
		Type:     consts.PostTypePost,
		AuthorID: user.ID,
	}
	if err := db.Create(used).Error; err != nil {
		t.Fatalf("create used post: %v", err)
	}
	if err := db.Create(unused).Error; err != nil {
		t.Fatalf("create unused post: %v", err)
	}

	if err := migratePostAttachment(db); err != nil {
		t.Fatalf("migratePostAttachment() error = %v", err)
	}

	var relations []entity.PostAttachment
	if err := db.Find(&relations).Error; err != nil {
		t.Fatalf("load relations: %v", err)
	}
	if len(relations) != 1 {
		t.Fatalf("relations = %d, want 1", len(relations))
	}
	if relations[0].PostID != used.ID || relations[0].AttachmentID != image.ID {
		t.Fatalf("relation = %+v, want post %d attachment %d", relations[0], used.ID, image.ID)
	}
	if relations[0].CreateTime.IsZero() {
		t.Fatal("CreateTime should be set")
	}
}

func TestMigratePostAttachmentSkipsOtherAuthorsAttachment(t *testing.T) {
	db := setupMigratePostAttachmentDB(t)

	author := &entity.User{Username: "a", Nickname: "A", Email: "a@example.com", Role: consts.UserRoleAuthor}
	other := &entity.User{Username: "b", Nickname: "B", Email: "b@example.com", Role: consts.UserRoleAuthor}
	if err := db.Create(author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	if err := db.Create(other).Error; err != nil {
		t.Fatalf("create other: %v", err)
	}

	foreign := &entity.Attachment{Name: "foreign", Path: "upload/image/2026/10/b.png", UserID: other.ID}
	if err := db.Create(foreign).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	post := &entity.Post{
		Title:           "post",
		Slug:            "post",
		Type:            consts.PostTypePost,
		AuthorID:        author.ID,
		OriginalContent: "![x](upload/image/2026/10/b.png)",
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	if err := migratePostAttachment(db); err != nil {
		t.Fatalf("migratePostAttachment() error = %v", err)
	}

	var count int64
	if err := db.Model(&entity.PostAttachment{}).Count(&count).Error; err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if count != 0 {
		t.Fatalf("relations = %d, want 0", count)
	}
}

func TestMigratePostAttachmentIsIdempotent(t *testing.T) {
	db := setupMigratePostAttachmentDB(t)

	user := &entity.User{Username: "a", Nickname: "A", Email: "a@example.com", Role: consts.UserRoleAuthor}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	image := &entity.Attachment{Name: "img", Path: "upload/image/2026/10/a.png", UserID: user.ID}
	if err := db.Create(image).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	post := &entity.Post{
		Title:           "post",
		Slug:            "post",
		Type:            consts.PostTypePost,
		AuthorID:        user.ID,
		OriginalContent: "![x](upload/image/2026/10/a.png)",
	}
	if err := db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	if err := migratePostAttachment(db); err != nil {
		t.Fatalf("first migratePostAttachment() error = %v", err)
	}
	if err := migratePostAttachment(db); err != nil {
		t.Fatalf("second migratePostAttachment() error = %v", err)
	}

	var count int64
	if err := db.Model(&entity.PostAttachment{}).Count(&count).Error; err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if count != 1 {
		t.Fatalf("relations = %d, want 1", count)
	}
}

func TestMigratePostAttachmentHandlesMultipleBatches(t *testing.T) {
	db := setupMigratePostAttachmentDB(t)

	user := &entity.User{Username: "a", Nickname: "A", Email: "a@example.com", Role: consts.UserRoleAuthor}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	image := &entity.Attachment{Name: "shared", Path: "upload/image/2026/10/shared.png", UserID: user.ID}
	if err := db.Create(image).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}

	posts := make([]*entity.Post, 0, 201)
	for i := 1; i <= 201; i++ {
		post := &entity.Post{
			Title:    fmt.Sprintf("post-%d", i),
			Slug:     fmt.Sprintf("post-%d", i),
			Type:     consts.PostTypePost,
			AuthorID: user.ID,
		}
		if i == 1 || i == 100 || i == 101 || i == 201 {
			post.OriginalContent = "![x](upload/image/2026/10/shared.png)"
		}
		posts = append(posts, post)
	}
	if err := db.CreateInBatches(posts, 100).Error; err != nil {
		t.Fatalf("create posts: %v", err)
	}

	if err := migratePostAttachment(db); err != nil {
		t.Fatalf("migratePostAttachment() error = %v", err)
	}

	var relations []entity.PostAttachment
	if err := db.Find(&relations).Error; err != nil {
		t.Fatalf("load relations: %v", err)
	}
	if len(relations) != 4 {
		t.Fatalf("relations = %d, want 4", len(relations))
	}

	relatedPostIDs := make(map[int32]struct{}, len(relations))
	for _, relation := range relations {
		if relation.AttachmentID != image.ID {
			t.Fatalf("relation attachment = %d, want %d", relation.AttachmentID, image.ID)
		}
		relatedPostIDs[relation.PostID] = struct{}{}
	}
	for _, index := range []int{0, 99, 100, 200} {
		if _, ok := relatedPostIDs[posts[index].ID]; !ok {
			t.Fatalf("missing relation for post %d (index %d)", posts[index].ID, index)
		}
	}
}

func TestMigratePostAttachmentMatchesFormatContentThumbnailAndSkipsEmptyPath(t *testing.T) {
	db := setupMigratePostAttachmentDB(t)

	user := &entity.User{Username: "a", Nickname: "A", Email: "a@example.com", Role: consts.UserRoleAuthor}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	original := &entity.Attachment{Name: "a", Path: "upload/image/2026/10/a.png", UserID: user.ID}
	formatted := &entity.Attachment{Name: "b", Path: "upload/image/2026/10/b.png", UserID: user.ID}
	thumbnail := &entity.Attachment{Name: "c", Path: "upload/image/2026/10/c.png", UserID: user.ID}
	empty := &entity.Attachment{Name: "empty", Path: "", UserID: user.ID}
	if err := db.Create(original).Error; err != nil {
		t.Fatalf("create original attachment: %v", err)
	}
	if err := db.Create(formatted).Error; err != nil {
		t.Fatalf("create formatted attachment: %v", err)
	}
	if err := db.Create(thumbnail).Error; err != nil {
		t.Fatalf("create thumbnail attachment: %v", err)
	}
	if err := db.Create(empty).Error; err != nil {
		t.Fatalf("create empty attachment: %v", err)
	}

	originalPost := &entity.Post{
		Title:           "original",
		Slug:            "original",
		Type:            consts.PostTypePost,
		AuthorID:        user.ID,
		OriginalContent: "![a](upload/image/2026/10/a.png)",
	}
	formatPost := &entity.Post{
		Title:         "format",
		Slug:          "format",
		Type:          consts.PostTypePost,
		AuthorID:      user.ID,
		FormatContent: "<p>upload/image/2026/10/b.png</p>",
	}
	thumbnailPost := &entity.Post{
		Title:     "thumbnail",
		Slug:      "thumbnail",
		Type:      consts.PostTypePost,
		AuthorID:  user.ID,
		Thumbnail: "upload/image/2026/10/c.png",
	}
	if err := db.Create(originalPost).Error; err != nil {
		t.Fatalf("create original post: %v", err)
	}
	if err := db.Create(formatPost).Error; err != nil {
		t.Fatalf("create format post: %v", err)
	}
	if err := db.Create(thumbnailPost).Error; err != nil {
		t.Fatalf("create thumbnail post: %v", err)
	}

	if err := migratePostAttachment(db); err != nil {
		t.Fatalf("migratePostAttachment() error = %v", err)
	}

	var relations []entity.PostAttachment
	if err := db.Find(&relations).Error; err != nil {
		t.Fatalf("load relations: %v", err)
	}
	if len(relations) != 3 {
		t.Fatalf("relations = %d, want 3", len(relations))
	}

	byPost := make(map[int32]int32, len(relations))
	for _, relation := range relations {
		byPost[relation.PostID] = relation.AttachmentID
	}
	want := map[int32]int32{
		originalPost.ID:  original.ID,
		formatPost.ID:    formatted.ID,
		thumbnailPost.ID: thumbnail.ID,
	}
	for postID, attachmentID := range want {
		if byPost[postID] != attachmentID {
			t.Fatalf("post %d attachment = %d, want %d", postID, byPost[postID], attachmentID)
		}
	}
}
