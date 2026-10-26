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

var migrateUserDSNSeq int64

func setupMigrateUserDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:airpress_migrate_user_%d?mode=memory&cache=shared",
		atomic.AddInt64(&migrateUserDSNSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&entity.User{}, &entity.Post{}, &entity.Attachment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

func TestMigrateUserRBACPromotesEarliestAndBackfillsContent(t *testing.T) {
	db := setupMigrateUserDB(t)

	users := []entity.User{
		{Username: "first", Email: "first@example.com", Role: consts.UserRoleAuthor},
		{Username: "second", Email: "second@example.com", Role: consts.UserRoleEditor},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}

	posts := []entity.Post{
		{Title: "legacy", Slug: "legacy", AuthorID: 0},
		{Title: "owned", Slug: "owned", AuthorID: users[1].ID},
	}
	if err := db.Create(&posts).Error; err != nil {
		t.Fatalf("create posts: %v", err)
	}

	attachments := []entity.Attachment{
		{Name: "legacy", UserID: 0},
		{Name: "owned", UserID: users[1].ID},
	}
	if err := db.Create(&attachments).Error; err != nil {
		t.Fatalf("create attachments: %v", err)
	}

	if err := migrateUserRBAC(db); err != nil {
		t.Fatalf("migrateUserRBAC() error = %v", err)
	}

	var first entity.User
	if err := db.First(&first, users[0].ID).Error; err != nil {
		t.Fatalf("load first user: %v", err)
	}
	if first.Role != consts.UserRoleSuperAdmin {
		t.Fatalf("first user role = %v, want %v", first.Role, consts.UserRoleSuperAdmin)
	}

	var second entity.User
	if err := db.First(&second, users[1].ID).Error; err != nil {
		t.Fatalf("load second user: %v", err)
	}
	if second.Role != consts.UserRoleEditor {
		t.Fatalf("second user role = %v, want %v", second.Role, consts.UserRoleEditor)
	}

	var legacyPost entity.Post
	if err := db.Where("slug = ?", "legacy").First(&legacyPost).Error; err != nil {
		t.Fatalf("load legacy post: %v", err)
	}
	if legacyPost.AuthorID != first.ID {
		t.Fatalf("legacy post author = %d, want %d", legacyPost.AuthorID, first.ID)
	}

	var ownedPost entity.Post
	if err := db.Where("slug = ?", "owned").First(&ownedPost).Error; err != nil {
		t.Fatalf("load owned post: %v", err)
	}
	if ownedPost.AuthorID != users[1].ID {
		t.Fatalf("owned post author changed to %d, want %d", ownedPost.AuthorID, users[1].ID)
	}

	var legacyAttachment entity.Attachment
	if err := db.Where("name = ?", "legacy").First(&legacyAttachment).Error; err != nil {
		t.Fatalf("load legacy attachment: %v", err)
	}
	if legacyAttachment.UserID != first.ID {
		t.Fatalf("legacy attachment user = %d, want %d", legacyAttachment.UserID, first.ID)
	}

	var ownedAttachment entity.Attachment
	if err := db.Where("name = ?", "owned").First(&ownedAttachment).Error; err != nil {
		t.Fatalf("load owned attachment: %v", err)
	}
	if ownedAttachment.UserID != users[1].ID {
		t.Fatalf("owned attachment user changed to %d, want %d", ownedAttachment.UserID, users[1].ID)
	}

	// 第二次执行必须幂等，不能把第二个用户的角色改掉。
	if err := migrateUserRBAC(db); err != nil {
		t.Fatalf("second migrateUserRBAC() error = %v", err)
	}
	if err := db.First(&second, users[1].ID).Error; err != nil {
		t.Fatalf("reload second user: %v", err)
	}
	if second.Role != consts.UserRoleEditor {
		t.Fatalf("second user role after second run = %v, want %v", second.Role, consts.UserRoleEditor)
	}
}

func TestMigrateUserRBACKeepsExistingSuperAdmin(t *testing.T) {
	db := setupMigrateUserDB(t)

	users := []entity.User{
		{Username: "first", Email: "first@example.com", Role: consts.UserRoleAuthor},
		{Username: "super", Email: "super@example.com", Role: consts.UserRoleSuperAdmin},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}

	if err := migrateUserRBAC(db); err != nil {
		t.Fatalf("migrateUserRBAC() error = %v", err)
	}

	var first entity.User
	if err := db.First(&first, users[0].ID).Error; err != nil {
		t.Fatalf("load first user: %v", err)
	}
	if first.Role != consts.UserRoleAuthor {
		t.Fatalf("first user role = %v, want %v", first.Role, consts.UserRoleAuthor)
	}
}

func TestMigrateUserRBACSkipsWhenNoUsers(t *testing.T) {
	db := setupMigrateUserDB(t)

	if err := migrateUserRBAC(db); err != nil {
		t.Fatalf("migrateUserRBAC() with no users error = %v", err)
	}
}
