package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type AuthMiddleware struct {
	OptionService       service.OptionService
	OneTimeTokenService service.OneTimeTokenService
	UserService         service.UserService
	Cache               cache.Cache
}

func NewAuthMiddleware(optionService service.OptionService, oneTimeTokenService service.OneTimeTokenService, cache cache.Cache, userService service.UserService) *AuthMiddleware {
	authMiddleware := &AuthMiddleware{
		OptionService:       optionService,
		OneTimeTokenService: oneTimeTokenService,
		Cache:               cache,
		UserService:         userService,
	}
	return authMiddleware
}

func (a *AuthMiddleware) GetWrapHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		isInstalled, err := a.OptionService.GetOrByDefaultWithErr(ctx, property.IsInstalled, false)
		if err != nil {
			abortWithStatusJSON(ctx, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
		if !isInstalled.(bool) {
			abortWithStatusJSON(ctx, http.StatusBadRequest, "Blog is not initialized")
			return
		}

		oneTimeToken, ok := ctx.GetQuery(consts.OneTimeTokenQueryName)
		if ok {
			allowedURL, ok := a.OneTimeTokenService.Get(oneTimeToken)
			if !ok {
				abortWithStatusJSON(ctx, http.StatusBadRequest, "OneTimeToken is not exist or expired")
				return
			}
			currentURL := ctx.Request.URL.Path
			if currentURL != allowedURL {
				abortWithStatusJSON(ctx, http.StatusBadRequest, "The one-time token does not correspond the request uri")
				return
			}
			return
		}

		token := ctx.GetHeader(consts.AdminTokenHeaderName)
		if token == "" {
			abortWithStatusJSON(ctx, http.StatusUnauthorized, "未登录，请登录后访问")
			return
		}
		value, ok := a.Cache.Get(cache.BuildTokenAccessKey(token))

		if !ok || value == nil {
			abortWithStatusJSON(ctx, http.StatusUnauthorized, "Token 已过期或不存在")
			return
		}
		userID, ok := value.(int32)
		if !ok {
			abortWithStatusJSON(ctx, http.StatusUnauthorized, "Token 已过期或不存在")
			return
		}

		user, err := a.UserService.GetByID(ctx, userID)
		if xerr.GetType(err) == xerr.NoRecord {
			_ = ctx.Error(err)
			abortWithStatusJSON(ctx, http.StatusUnauthorized, "用户不存在")
			return
		}
		if err != nil {
			_ = ctx.Error(err)
			abortWithStatusJSON(ctx, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
		if user.Disabled {
			abortWithCodeJSON(ctx, http.StatusForbidden, "ACCOUNT_DISABLED", "账号已被禁用")
			return
		}
		if err := a.UserService.MustNotExpire(ctx, user.ExpireTime); err != nil {
			abortWithStatusJSON(ctx, xerr.GetHTTPStatus(err), xerr.GetMessage(err))
			return
		}

		policy, err := a.OptionService.GetMFAPolicy(ctx)
		if err != nil {
			_ = ctx.Error(err)
			abortWithStatusJSON(ctx, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
		if policy.Requires(user.Role) && user.MfaType == consts.MFANone &&
			!isMFAEnrollmentAllowed(ctx.Request.Method, ctx.Request.URL.Path) {
			abortWithCodeJSON(ctx, http.StatusForbidden, "MFA_ENROLLMENT_REQUIRED", "请先绑定两步验证")
			return
		}

		ctx.Set(consts.AuthorizedUser, user)
	}
}

// 强制绑定 MFA 期间只允许访问完成绑定所必需的接口。
var mfaEnrollmentAllowedPaths = map[string]struct{}{
	"GET /api/admin/users/profiles":           {},
	"PUT /api/admin/users/profiles":           {},
	"PUT /api/admin/users/profiles/password":  {},
	"PUT /api/admin/users/mfa/generate":       {},
	"PUT /api/admin/users/mfa/update":         {},
	"GET /api/admin/users/mfa/recovery-codes": {},
	"POST /api/admin/logout":                  {},
	"GET /api/admin/statistics/user":          {},
	"POST /api/admin/options/map_view/keys":   {},
	// 让超管在策略误设为强制后仍能改回，避免把自己锁死。
	"GET /api/admin/manage/mfa-policy": {},
	"PUT /api/admin/manage/mfa-policy": {},
}

func isMFAEnrollmentAllowed(method, path string) bool {
	_, ok := mfaEnrollmentAllowedPaths[method+" "+path]
	return ok
}

func abortWithStatusJSON(ctx *gin.Context, status int, message string) {
	ctx.AbortWithStatusJSON(status, &dto.BaseDTO{
		Status:  status,
		Message: message,
	})
}

func abortWithCodeJSON(ctx *gin.Context, status int, code, message string) {
	ctx.AbortWithStatusJSON(status, &dto.BaseDTO{
		Status:  status,
		Code:    code,
		Message: message,
	})
}
