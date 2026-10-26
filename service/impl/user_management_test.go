package impl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/event"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

var userManagementDSNSeq int64

type stubUserManagementOptionService struct {
	service.OptionService
	policy consts.MFAPolicy
}

func (s *stubUserManagementOptionService) GetMFAPolicy(ctx context.Context) (consts.MFAPolicy, error) {
	return s.policy, nil
}

func (s *stubUserManagementOptionService) Save(ctx context.Context, optionMap map[string]string) error {
	if value, ok := optionMap[property.MFAPolicy.KeyValue]; ok {
		s.policy = consts.MFAPolicy(value)
	}
	return nil
}

type userManagementFixture struct {
	db      *gorm.DB
	cache   cache.Cache
	service service.UserManagementService
	options *stubUserManagementOptionService
}

func setupUserManagement(t *testing.T) *userManagementFixture {
	t.Helper()

	dsn := fmt.Sprintf("file:airpress_user_management_%d?mode=memory&cache=shared",
		atomic.AddInt64(&userManagementDSNSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&entity.User{}, &entity.Post{}, &entity.Attachment{}, &entity.MfaRecoveryCode{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	prev := *dal.Q
	t.Cleanup(func() {
		*dal.Q = prev
		_ = sqlDB.Close()
	})
	dal.SetDefault(db)

	c := cache.NewCache()
	options := &stubUserManagementOptionService{policy: consts.MFAPolicyNone}
	eventBus := event.NewSyncEventBus(zap.NewNop())
	userService := NewUserService(&stubTOTPService{}, eventBus, options, NewRecoveryCodeService())
	svc := NewUserManagementService(userService, c, options)

	return &userManagementFixture{db: db, cache: c, service: svc, options: options}
}

func (f *userManagementFixture) createUser(t *testing.T, username string, role consts.UserRole) *entity.User {
	t.Helper()
	user := &entity.User{
		Username: username,
		Nickname: username,
		Email:    username + "@example.com",
		Password: "hashed",
		Role:     role,
	}
	if err := f.db.Create(user).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return user
}

func actorContext(user *entity.User) context.Context {
	return context.WithValue(context.Background(), consts.AuthorizedUser, user)
}

func TestUserManagementCreateUser(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)

	created, err := f.service.Create(actorContext(admin), param.CreateUser{
		Username: "writer",
		Nickname: "Writer",
		Email:    "writer@example.com",
		Password: "Password123!",
		Role:     consts.UserRoleAuthor,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Role != consts.UserRoleAuthor {
		t.Fatalf("role = %v, want %v", created.Role, consts.UserRoleAuthor)
	}

	var stored entity.User
	if err := f.db.First(&stored, created.ID).Error; err != nil {
		t.Fatalf("load created user: %v", err)
	}
	if stored.Password == "Password123!" {
		t.Fatal("password should be hashed")
	}
}

func TestUserManagementCreateRejectsDuplicateIdentity(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)
	f.createUser(t, "writer", consts.UserRoleAuthor)

	_, err := f.service.Create(actorContext(admin), param.CreateUser{
		Username: "writer",
		Nickname: "Writer",
		Email:    "other@example.com",
		Password: "Password123!",
		Role:     consts.UserRoleAuthor,
	})
	if err == nil {
		t.Fatal("duplicate username should fail")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, xerr.StatusBadRequest)
	}
}

func TestUserManagementCreateRejectsDuplicateEmail(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)
	f.createUser(t, "writer", consts.UserRoleAuthor)

	_, err := f.service.Create(actorContext(admin), param.CreateUser{
		Username: "other",
		Nickname: "Other",
		Email:    "writer@example.com",
		Password: "Password123!",
		Role:     consts.UserRoleAuthor,
	})
	if err == nil {
		t.Fatal("duplicate email should fail")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, xerr.StatusBadRequest)
	}
}

func TestUserManagementCreateRejectsWeakPassword(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)

	_, err := f.service.Create(actorContext(admin), param.CreateUser{
		Username: "weak",
		Nickname: "Weak",
		Email:    "weak@example.com",
		Password: "password123",
		Role:     consts.UserRoleAuthor,
	})
	if err == nil {
		t.Fatal("weak password should fail")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, xerr.StatusBadRequest)
	}
}

func TestUserManagementCannotDemoteLastSuperAdmin(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)

	err := f.service.UpdateRole(actorContext(admin), admin.ID, consts.UserRoleEditor)
	if err == nil {
		t.Fatal("demoting the last super admin should fail")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusForbidden {
		t.Fatalf("status = %d, want %d", got, xerr.StatusForbidden)
	}
}

func TestUserManagementUpdateRole(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)
	author := f.createUser(t, "writer", consts.UserRoleAuthor)

	if err := f.service.UpdateRole(actorContext(admin), author.ID, consts.UserRoleEditor); err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	var stored entity.User
	if err := f.db.First(&stored, author.ID).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if stored.Role != consts.UserRoleEditor {
		t.Fatalf("role = %v, want %v", stored.Role, consts.UserRoleEditor)
	}
}

func TestUserManagementDisableClearsTokens(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)
	author := f.createUser(t, "writer", consts.UserRoleAuthor)

	const accessToken = "author-access-token"
	f.cache.Set(cache.BuildTokenAccessKey(accessToken), author.ID, time.Hour)
	f.cache.Set(cache.BuildAccessTokenKey(author.ID), accessToken, time.Hour)

	if err := f.service.UpdateStatus(actorContext(admin), author.ID, true); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	var stored entity.User
	if err := f.db.First(&stored, author.ID).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if !stored.Disabled {
		t.Fatal("user should be disabled")
	}
	if _, ok := f.cache.Get(cache.BuildTokenAccessKey(accessToken)); ok {
		t.Fatal("access token should be cleared")
	}
	if _, ok := f.cache.Get(cache.BuildAccessTokenKey(author.ID)); ok {
		t.Fatal("user token pointer should be cleared")
	}
}

func TestUserManagementCannotDisableLastSuperAdmin(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)

	err := f.service.UpdateStatus(actorContext(admin), admin.ID, true)
	if err == nil {
		t.Fatal("disabling the last super admin should fail")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusForbidden {
		t.Fatalf("status = %d, want %d", got, xerr.StatusForbidden)
	}
}

func TestUserManagementDeleteRequiresTransferAndMovesContent(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)
	author := f.createUser(t, "writer", consts.UserRoleAuthor)

	post := &entity.Post{Title: "post", Slug: "post", AuthorID: author.ID, Type: consts.PostTypePost}
	sheet := &entity.Post{Title: "sheet", Slug: "sheet", AuthorID: author.ID, Type: consts.PostTypeSheet}
	attachment := &entity.Attachment{Name: "file", UserID: author.ID}
	if err := f.db.Create(post).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := f.db.Create(sheet).Error; err != nil {
		t.Fatalf("create sheet: %v", err)
	}
	if err := f.db.Create(attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	recovery := &entity.MfaRecoveryCode{UserID: author.ID, CodeHash: "hash"}
	if err := f.db.Create(recovery).Error; err != nil {
		t.Fatalf("create recovery code: %v", err)
	}

	count, err := f.service.ContentCount(actorContext(admin), author.ID)
	if err != nil {
		t.Fatalf("ContentCount() error = %v", err)
	}
	if count.Posts != 1 || count.Sheets != 1 || count.Attachments != 1 {
		t.Fatalf("content count = %+v", count)
	}

	_, err = f.service.Delete(actorContext(admin), author.ID, 0)
	if err == nil {
		t.Fatal("deleting a user with content without transfer target should fail")
	}

	moved, err := f.service.Delete(actorContext(admin), author.ID, admin.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if moved.Posts != 1 || moved.Sheets != 1 || moved.Attachments != 1 {
		t.Fatalf("moved count = %+v", moved)
	}

	var storedPost entity.Post
	if err := f.db.First(&storedPost, post.ID).Error; err != nil {
		t.Fatalf("load post: %v", err)
	}
	if storedPost.AuthorID != admin.ID {
		t.Fatalf("post author = %d, want %d", storedPost.AuthorID, admin.ID)
	}
	var storedAttachment entity.Attachment
	if err := f.db.First(&storedAttachment, attachment.ID).Error; err != nil {
		t.Fatalf("load attachment: %v", err)
	}
	if storedAttachment.UserID != admin.ID {
		t.Fatalf("attachment user = %d, want %d", storedAttachment.UserID, admin.ID)
	}

	var userCount int64
	if err := f.db.Model(&entity.User{}).Where("id = ?", author.ID).Count(&userCount).Error; err != nil {
		t.Fatalf("count user: %v", err)
	}
	if userCount != 0 {
		t.Fatal("user should be deleted")
	}
	var codeCount int64
	if err := f.db.Model(&entity.MfaRecoveryCode{}).Where("user_id = ?", author.ID).Count(&codeCount).Error; err != nil {
		t.Fatalf("count recovery codes: %v", err)
	}
	if codeCount != 0 {
		t.Fatal("recovery codes should be deleted")
	}
}

func TestUserManagementCannotDeleteSelf(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)

	_, err := f.service.Delete(actorContext(admin), admin.ID, 0)
	if err == nil {
		t.Fatal("deleting self should fail")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusForbidden {
		t.Fatalf("status = %d, want %d", got, xerr.StatusForbidden)
	}
}

func TestUserManagementListFilters(t *testing.T) {
	f := setupUserManagement(t)
	f.createUser(t, "admin", consts.UserRoleSuperAdmin)
	author := f.createUser(t, "writer", consts.UserRoleAuthor)
	f.createUser(t, "editor", consts.UserRoleEditor)

	users, total, err := f.service.List(context.Background(), param.UserQuery{
		Page:    param.Page{PageNum: 0, PageSize: 10},
		Keyword: "e",
		Role:    &author.Role,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 || len(users) != 1 {
		t.Fatalf("List() total=%d len=%d, want 1/1", total, len(users))
	}
	if users[0].ID != author.ID {
		t.Fatalf("user id = %d, want %d", users[0].ID, author.ID)
	}

	allUsers, allTotal, err := f.service.List(context.Background(), param.UserQuery{
		Page: param.Page{PageNum: 0, PageSize: 10},
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if allTotal != 3 || len(allUsers) != 3 {
		t.Fatalf("List() total=%d len=%d, want 3/3", allTotal, len(allUsers))
	}
}

func TestUserManagementResetPasswordClearsTokens(t *testing.T) {
	f := setupUserManagement(t)
	admin := f.createUser(t, "admin", consts.UserRoleSuperAdmin)
	author := f.createUser(t, "writer", consts.UserRoleAuthor)

	const accessToken = "author-access-token"
	f.cache.Set(cache.BuildTokenAccessKey(accessToken), author.ID, time.Hour)
	f.cache.Set(cache.BuildAccessTokenKey(author.ID), accessToken, time.Hour)

	if err := f.service.ResetPassword(actorContext(admin), author.ID, "newPassword123!"); err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}

	var stored entity.User
	if err := f.db.First(&stored, author.ID).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if stored.Password == "newPassword123!" {
		t.Fatal("password should be hashed")
	}
	if _, ok := f.cache.Get(cache.BuildTokenAccessKey(accessToken)); ok {
		t.Fatal("access token should be cleared")
	}

	if err := f.service.ResetPassword(actorContext(admin), author.ID, "short"); err == nil {
		t.Fatal("short password should be rejected")
	}
}

func TestUserManagementMFAPolicy(t *testing.T) {
	f := setupUserManagement(t)

	policy, err := f.service.GetMFAPolicy(context.Background())
	if err != nil {
		t.Fatalf("GetMFAPolicy() error = %v", err)
	}
	if policy != consts.MFAPolicyNone {
		t.Fatalf("policy = %v, want %v", policy, consts.MFAPolicyNone)
	}

	if err := f.service.SaveMFAPolicy(context.Background(), consts.MFAPolicyAll); err != nil {
		t.Fatalf("SaveMFAPolicy() error = %v", err)
	}
	policy, err = f.service.GetMFAPolicy(context.Background())
	if err != nil {
		t.Fatalf("GetMFAPolicy() error = %v", err)
	}
	if policy != consts.MFAPolicyAll {
		t.Fatalf("policy = %v, want %v", policy, consts.MFAPolicyAll)
	}

	if err := f.service.SaveMFAPolicy(context.Background(), consts.MFAPolicy("BOGUS")); err == nil {
		t.Fatal("invalid policy should fail")
	}
}
