package impl

import (
	"context"
	"testing"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

func TestEnsureAuthorOwnsPost(t *testing.T) {
	post := &entity.Post{ID: 1, AuthorID: 10}

	cases := []struct {
		name string
		ctx  context.Context
		want int
	}{
		{
			name: "前台请求无登录用户时放行",
			ctx:  context.Background(),
			want: 0,
		},
		{
			name: "作者操作自己的文章",
			ctx:  context.WithValue(context.Background(), consts.AuthorizedUser, &entity.User{ID: 10, Role: consts.UserRoleAuthor}),
			want: 0,
		},
		{
			name: "作者操作他人文章",
			ctx:  context.WithValue(context.Background(), consts.AuthorizedUser, &entity.User{ID: 11, Role: consts.UserRoleAuthor}),
			want: xerr.StatusForbidden,
		},
		{
			name: "编辑操作他人文章",
			ctx:  context.WithValue(context.Background(), consts.AuthorizedUser, &entity.User{ID: 11, Role: consts.UserRoleEditor}),
			want: 0,
		},
		{
			name: "超管操作他人文章",
			ctx:  context.WithValue(context.Background(), consts.AuthorizedUser, &entity.User{ID: 12, Role: consts.UserRoleSuperAdmin}),
			want: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ensureAuthorOwnsPost(tc.ctx, post)
			if tc.want == 0 {
				if err != nil {
					t.Fatalf("ensureAuthorOwnsPost() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ensureAuthorOwnsPost() should fail")
			}
			if got := xerr.GetHTTPStatus(err); got != tc.want {
				t.Fatalf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestEnsureAuthorOwnsAttachment(t *testing.T) {
	attachment := &entity.Attachment{ID: 1, UserID: 10}

	authorOther := context.WithValue(context.Background(), consts.AuthorizedUser, &entity.User{ID: 11, Role: consts.UserRoleAuthor})
	if err := ensureAuthorOwnsAttachment(authorOther, attachment); err == nil {
		t.Fatal("author should not access another user's attachment")
	}

	owner := context.WithValue(context.Background(), consts.AuthorizedUser, &entity.User{ID: 10, Role: consts.UserRoleAuthor})
	if err := ensureAuthorOwnsAttachment(owner, attachment); err != nil {
		t.Fatalf("owner should access attachment: %v", err)
	}

	editor := context.WithValue(context.Background(), consts.AuthorizedUser, &entity.User{ID: 11, Role: consts.UserRoleEditor})
	if err := ensureAuthorOwnsAttachment(editor, attachment); err != nil {
		t.Fatalf("editor should access attachment: %v", err)
	}
}
