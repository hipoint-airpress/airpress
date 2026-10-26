package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
)

type stubAuthOptionService struct {
	service.OptionService
	installed bool
	policy    consts.MFAPolicy
}

func (s *stubAuthOptionService) GetOrByDefaultWithErr(ctx context.Context, p property.Property, defaultValue interface{}) (interface{}, error) {
	return s.installed, nil
}

func (s *stubAuthOptionService) GetMFAPolicy(ctx context.Context) (consts.MFAPolicy, error) {
	return s.policy, nil
}

type stubOneTimeTokenService struct {
	service.OneTimeTokenService
}

func (s *stubOneTimeTokenService) Get(string) (string, bool) {
	return "", false
}

type stubAuthUserService struct {
	service.UserService
	user *entity.User
}

func (s *stubAuthUserService) GetByID(ctx context.Context, id int32) (*entity.User, error) {
	return s.user, nil
}

func (s *stubAuthUserService) MustNotExpire(ctx context.Context, expireTime *time.Time) error {
	return nil
}

func setupAuthMiddleware(t *testing.T, user *entity.User, policy consts.MFAPolicy) (gin.HandlerFunc, cache.Cache) {
	t.Helper()

	c := cache.NewCache()
	c.Set(cache.BuildTokenAccessKey("valid-token"), user.ID, time.Hour)

	middleware := NewAuthMiddleware(
		&stubAuthOptionService{installed: true, policy: policy},
		&stubOneTimeTokenService{},
		c,
		&stubAuthUserService{user: user},
	)
	return middleware.GetWrapHandler(), c
}

func serveAuthRequest(t *testing.T, handler gin.HandlerFunc, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Handle(method, path, handler, func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set(consts.AdminTokenHeaderName, token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeBaseDTO(t *testing.T, recorder *httptest.ResponseRecorder) dto.BaseDTO {
	t.Helper()
	var body dto.BaseDTO
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", recorder.Body.String(), err)
	}
	return body
}

func TestAuthMiddlewareRejectsDisabledUser(t *testing.T) {
	user := &entity.User{ID: 1, Role: consts.UserRoleAuthor, Disabled: true}
	handler, _ := setupAuthMiddleware(t, user, consts.MFAPolicyNone)

	recorder := serveAuthRequest(t, handler, http.MethodGet, "/api/admin/posts", "valid-token")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	body := decodeBaseDTO(t, recorder)
	if body.Code != "ACCOUNT_DISABLED" {
		t.Fatalf("code = %q, want ACCOUNT_DISABLED", body.Code)
	}
}

func TestAuthMiddlewareMFAEnrollmentGate(t *testing.T) {
	user := &entity.User{ID: 1, Role: consts.UserRoleEditor, MfaType: consts.MFANone}

	cases := []struct {
		name     string
		method   string
		path     string
		policy   consts.MFAPolicy
		user     *entity.User
		want     int
		wantCode string
	}{
		{
			name:     "策略要求且未绑定-普通接口被拒",
			method:   http.MethodGet,
			path:     "/api/admin/posts",
			policy:   consts.MFAPolicyAll,
			user:     user,
			want:     http.StatusForbidden,
			wantCode: "MFA_ENROLLMENT_REQUIRED",
		},
		{
			name:   "策略要求且未绑定-个人资料放行",
			method: http.MethodGet,
			path:   "/api/admin/users/profiles",
			policy: consts.MFAPolicyAll,
			user:   user,
			want:   http.StatusOK,
		},
		{
			name:   "策略要求且未绑定-MFA策略可改回",
			method: http.MethodPut,
			path:   "/api/admin/manage/mfa-policy",
			policy: consts.MFAPolicyAll,
			user:   user,
			want:   http.StatusOK,
		},
		{
			name:   "策略要求且已绑定-放行",
			method: http.MethodGet,
			path:   "/api/admin/posts",
			policy: consts.MFAPolicyAll,
			user:   &entity.User{ID: 2, Role: consts.UserRoleEditor, MfaType: consts.MFATFATotp},
			want:   http.StatusOK,
		},
		{
			name:   "策略不要求-放行",
			method: http.MethodGet,
			path:   "/api/admin/posts",
			policy: consts.MFAPolicyNone,
			user:   user,
			want:   http.StatusOK,
		},
		{
			name:   "仅管理角色策略-作者放行",
			method: http.MethodGet,
			path:   "/api/admin/posts",
			policy: consts.MFAPolicyAdminEditor,
			user:   &entity.User{ID: 3, Role: consts.UserRoleAuthor, MfaType: consts.MFANone},
			want:   http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, _ := setupAuthMiddleware(t, tc.user, tc.policy)
			recorder := serveAuthRequest(t, handler, tc.method, tc.path, "valid-token")
			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d, body=%s", recorder.Code, tc.want, recorder.Body.String())
			}
			if tc.wantCode != "" {
				body := decodeBaseDTO(t, recorder)
				if body.Code != tc.wantCode {
					t.Fatalf("code = %q, want %q", body.Code, tc.wantCode)
				}
			}
		})
	}
}

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	user := &entity.User{ID: 1, Role: consts.UserRoleAuthor}
	handler, _ := setupAuthMiddleware(t, user, consts.MFAPolicyNone)

	recorder := serveAuthRequest(t, handler, http.MethodGet, "/api/admin/posts", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
