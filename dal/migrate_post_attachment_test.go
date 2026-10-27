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
