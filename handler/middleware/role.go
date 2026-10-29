package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
)

// RequireAnyRole 做模块级角色门禁，资源级所有权仍由 service 层校验。
func RequireAnyRole(roles ...consts.UserRole) gin.HandlerFunc {
	allowed := make(map[consts.UserRole]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(ctx *gin.Context) {
		value, exists := ctx.Get(consts.AuthorizedUser)
		if !exists {
			abortWithStatusJSON(ctx, http.StatusForbidden, "无权访问")
			return
		}
		user, ok := value.(*entity.User)
		if !ok || user == nil {
			abortWithStatusJSON(ctx, http.StatusForbidden, "无权访问")
			return
		}
		if _, ok := allowed[user.Role]; !ok {
			abortWithStatusJSON(ctx, http.StatusForbidden, "无权访问")
			return
		}
	}
}
