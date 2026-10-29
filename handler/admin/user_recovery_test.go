package admin

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type stubHandlerRecoveryCodeService struct {
	service.RecoveryCodeService
	codes     []string
	remaining int64
}

func (s *stubHandlerRecoveryCodeService) Generate(ctx context.Context, userID int32) ([]string, error) {
	return s.codes, nil
}

func (s *stubHandlerRecoveryCodeService) CountRemaining(ctx context.Context, userID int32) (int64, error) {
	return s.remaining, nil
}

type stubHandlerTOTPService struct {
	service.TwoFactorTOTPMFAService
}

func (s *stubHandlerTOTPService) ValidateTFACode(tfaKey, tfaCode string) bool {
	return tfaCode == "123456"
}

func newRecoveryCodeHandler(user *entity.User, recovery *stubHandlerRecoveryCodeService) *UserHandler {
	return NewUserHandler(nil, &stubHandlerTOTPService{}, recovery, nil)
}

func recoveryCodeContext(user *entity.User, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/admin/users/mfa/recovery-codes", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(consts.AuthorizedUser, user)
	return ctx, recorder
}

func TestGenerateRecoveryCodes(t *testing.T) {
	user := &entity.User{ID: 1, MfaType: consts.MFATFATotp, MfaKey: "key"}
	recovery := &stubHandlerRecoveryCodeService{codes: []string{"AAAA-BBBB-CCCC"}}
	handler := newRecoveryCodeHandler(user, recovery)

	ctx, _ := recoveryCodeContext(user, `{"authcode":"123456"}`)
	result, err := handler.GenerateRecoveryCodes(ctx)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes() error = %v", err)
	}
	data, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T", result)
	}
	codes, ok := data["codes"].([]string)
	if !ok || len(codes) != 1 || codes[0] != "AAAA-BBBB-CCCC" {
		t.Fatalf("codes = %v", data["codes"])
	}
}

func TestGenerateRecoveryCodesRequiresEnabledMFA(t *testing.T) {
	user := &entity.User{ID: 2, MfaType: consts.MFANone}
	handler := newRecoveryCodeHandler(user, &stubHandlerRecoveryCodeService{})

	ctx, _ := recoveryCodeContext(user, `{"authcode":"123456"}`)
	_, err := handler.GenerateRecoveryCodes(ctx)
	if err == nil {
		t.Fatal("MFA disabled should reject recovery code generation")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, xerr.StatusBadRequest)
	}
}

func TestGenerateRecoveryCodesRejectsWrongTOTP(t *testing.T) {
	user := &entity.User{ID: 3, MfaType: consts.MFATFATotp, MfaKey: "key"}
	handler := newRecoveryCodeHandler(user, &stubHandlerRecoveryCodeService{})

	ctx, _ := recoveryCodeContext(user, `{"authcode":"000000"}`)
	_, err := handler.GenerateRecoveryCodes(ctx)
	if err == nil {
		t.Fatal("wrong TOTP should reject recovery code generation")
	}
	if got := xerr.GetHTTPStatus(err); got != xerr.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, xerr.StatusBadRequest)
	}
}

func TestGetRecoveryCodeCount(t *testing.T) {
	user := &entity.User{ID: 4, MfaType: consts.MFATFATotp, MfaKey: "key"}
	handler := newRecoveryCodeHandler(user, &stubHandlerRecoveryCodeService{remaining: 7})

	ctx, _ := recoveryCodeContext(user, "")
	result, err := handler.GetRecoveryCodeCount(ctx)
	if err != nil {
		t.Fatalf("GetRecoveryCodeCount() error = %v", err)
	}
	data, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T", result)
	}
	if data["remaining"] != int64(7) {
		t.Fatalf("remaining = %v, want 7", data["remaining"])
	}
}
