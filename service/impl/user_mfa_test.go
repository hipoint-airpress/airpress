package impl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/event"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

var userMFADSNSeq int64

type stubUserMFAOptionService struct {
	service.OptionService
	policy consts.MFAPolicy
}

func (s *stubUserMFAOptionService) GetMFAPolicy(ctx context.Context) (consts.MFAPolicy, error) {
	return s.policy, nil
}

type stubUserMFARecoveryService struct {
	service.RecoveryCodeService
	deleted int
}

func (s *stubUserMFARecoveryService) DeleteAll(ctx context.Context, userID int32) error {
	s.deleted++
	return nil
}

func setupUserMFA(t *testing.T, user *entity.User, policy consts.MFAPolicy) (*userServiceImpl, *stubUserMFARecoveryService) {
	t.Helper()

	dsn := fmt.Sprintf("file:airpress_user_mfa_%d?mode=memory&cache=shared",
		atomic.AddInt64(&userMFADSNSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&entity.User{}); err != nil {
		t.Fatalf("auto migrate user: %v", err)
	}
	prev := *dal.Q
	t.Cleanup(func() {
		*dal.Q = prev
		_ = sqlDB.Close()
	})
	dal.SetDefault(db)
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	recovery := &stubUserMFARecoveryService{}
	svc := &userServiceImpl{
		TwoFactorMFAService: &stubTOTPService{},
		Event:               event.NewSyncEventBus(zap.NewNop()),
		OptionService:       &stubUserMFAOptionService{policy: policy},
		RecoveryCodeService: recovery,
	}
	return svc, recovery
}

func userMFAContext(user *entity.User) context.Context {
	return context.WithValue(context.Background(), consts.AuthorizedUser, user)
}

func reloadUserMFA(t *testing.T, userID int32) *entity.User {
	t.Helper()
	userDAL := dal.GetQueryByCtx(context.Background()).User
	user, err := userDAL.WithContext(context.Background()).Where(userDAL.ID.Eq(userID)).First()
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return user
}

func TestUpdateMFACannotDisableUnderPolicy(t *testing.T) {
	user := &entity.User{
		ID: 1, Username: "editor", Nickname: "editor",
		Role: consts.UserRoleEditor, MfaType: consts.MFATFATotp, MfaKey: "key",
	}
	svc, recovery := setupUserMFA(t, user, consts.MFAPolicyAdminEditor)

	err := svc.UpdateMFA(userMFAContext(user), "", consts.MFANone, "123456")
	if err == nil {
		t.Fatal("disabling MFA under policy should fail")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, xerr.StatusBadRequest)
	}
	if recovery.deleted != 0 {
		t.Fatal("recovery codes should not be touched when update is rejected")
	}
	if got := reloadUserMFA(t, user.ID); got.MfaType != consts.MFATFATotp {
		t.Fatalf("MFA type changed to %v", got.MfaType)
	}
}

func TestUpdateMFADisableClearsRecoveryCodes(t *testing.T) {
	user := &entity.User{
		ID: 2, Username: "author", Nickname: "author",
		Role: consts.UserRoleAuthor, MfaType: consts.MFATFATotp, MfaKey: "key",
	}
	svc, recovery := setupUserMFA(t, user, consts.MFAPolicyNone)

	if err := svc.UpdateMFA(userMFAContext(user), "", consts.MFANone, "123456"); err != nil {
		t.Fatalf("UpdateMFA() error = %v", err)
	}
	if recovery.deleted != 1 {
		t.Fatalf("recovery DeleteAll calls = %d, want 1", recovery.deleted)
	}
	got := reloadUserMFA(t, user.ID)
	if got.MfaType != consts.MFANone || got.MfaKey != "" {
		t.Fatalf("MFA not disabled: type=%v key=%q", got.MfaType, got.MfaKey)
	}
}

func TestUpdateMFAEnableValidatesCodeAndClearsOldCodes(t *testing.T) {
	user := &entity.User{
		ID: 3, Username: "author", Nickname: "author",
		Role: consts.UserRoleAuthor, MfaType: consts.MFANone,
	}
	svc, recovery := setupUserMFA(t, user, consts.MFAPolicyNone)

	if err := svc.UpdateMFA(userMFAContext(user), "new-key", consts.MFATFATotp, "000000"); err == nil {
		t.Fatal("invalid code should fail")
	}
	if recovery.deleted != 0 {
		t.Fatal("recovery codes should not be cleared on invalid code")
	}

	if err := svc.UpdateMFA(userMFAContext(user), "new-key", consts.MFATFATotp, "123456"); err != nil {
		t.Fatalf("UpdateMFA() error = %v", err)
	}
	if recovery.deleted != 1 {
		t.Fatalf("recovery DeleteAll calls = %d, want 1", recovery.deleted)
	}
	got := reloadUserMFA(t, user.ID)
	if got.MfaType != consts.MFATFATotp || got.MfaKey != "new-key" {
		t.Fatalf("MFA not enabled: type=%v key=%q", got.MfaType, got.MfaKey)
	}
}
