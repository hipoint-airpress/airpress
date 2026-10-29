package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
)

func TestRequireAnyRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		user interface{}
		role []consts.UserRole
		want int
	}{
		{
			name: "未登录",
			user: nil,
			role: []consts.UserRole{consts.UserRoleEditor, consts.UserRoleSuperAdmin},
			want: http.StatusForbidden,
		},
		{
			name: "作者访问编辑接口",
			user: &entity.User{Role: consts.UserRoleAuthor},
			role: []consts.UserRole{consts.UserRoleEditor, consts.UserRoleSuperAdmin},
			want: http.StatusForbidden,
		},
		{
			name: "编辑访问编辑接口",
			user: &entity.User{Role: consts.UserRoleEditor},
			role: []consts.UserRole{consts.UserRoleEditor, consts.UserRoleSuperAdmin},
			want: http.StatusOK,
		},
		{
			name: "超管访问超管接口",
			user: &entity.User{Role: consts.UserRoleSuperAdmin},
			role: []consts.UserRole{consts.UserRoleSuperAdmin},
			want: http.StatusOK,
		},
		{
			name: "超管访问编辑接口",
			user: &entity.User{Role: consts.UserRoleSuperAdmin},
			role: []consts.UserRole{consts.UserRoleEditor, consts.UserRoleSuperAdmin},
			want: http.StatusOK,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(ctx *gin.Context) {
				if tc.user != nil {
					ctx.Set(consts.AuthorizedUser, tc.user)
				}
				ctx.Next()
			})
			router.GET("/test", RequireAnyRole(tc.role...), func(ctx *gin.Context) {
				ctx.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != tc.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.want)
			}
		})
	}
}

func TestRequireAnyRoleRejectsWrongContextType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set(consts.AuthorizedUser, "not-a-user")
		ctx.Next()
	})
	router.GET("/test", RequireAnyRole(consts.UserRoleSuperAdmin), func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}
