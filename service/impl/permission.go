package impl

import (
	"context"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

// mfaEnrollmentRequired 判断当前策略是否要求该用户绑定 MFA，但用户尚未绑定。
func mfaEnrollmentRequired(ctx context.Context, optionService service.OptionService, user *entity.User) (bool, error) {
	policy, err := optionService.GetMFAPolicy(ctx)
	if err != nil {
		return false, err
	}
	return policy.Requires(user.Role) && user.MfaType == consts.MFANone, nil
}

func currentUser(ctx context.Context) (*entity.User, bool) {
	user, ok := GetAuthorizedUser(ctx)
	if !ok || user == nil {
		return nil, false
	}
	return user, true
}

func isAuthor(ctx context.Context) bool {
	user, ok := currentUser(ctx)
	return ok && user.Role == consts.UserRoleAuthor
}

func ensureAuthorOwnsPost(ctx context.Context, post *entity.Post) error {
	user, ok := currentUser(ctx)
	if !ok || user.Role != consts.UserRoleAuthor {
		return nil
	}
	if post.AuthorID != user.ID {
		return xerr.WithStatus(nil, xerr.StatusForbidden).WithMsg("无权操作该文章")
	}
	return nil
}

func ensureAuthorOwnsAttachment(ctx context.Context, attachment *entity.Attachment) error {
	user, ok := currentUser(ctx)
	if !ok || user.Role != consts.UserRoleAuthor {
		return nil
	}
	if attachment.UserID != user.ID {
		return xerr.WithStatus(nil, xerr.StatusForbidden).WithMsg("无权操作该附件")
	}
	return nil
}
