package impl

import (
	"context"

	"gorm.io/gen/field"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type userManagementServiceImpl struct {
	UserService   service.UserService
	Cache         cache.Cache
	OptionService service.OptionService
}

func NewUserManagementService(userService service.UserService, cache cache.Cache, optionService service.OptionService) service.UserManagementService {
	return &userManagementServiceImpl{
		UserService:   userService,
		Cache:         cache,
		OptionService: optionService,
	}
}

func (s *userManagementServiceImpl) List(ctx context.Context, query param.UserQuery) ([]*dto.User, int64, error) {
	userDAL := dal.GetQueryByCtx(ctx).User
	userDo := userDAL.WithContext(ctx)
	if query.Keyword != "" {
		keyword := "%" + query.Keyword + "%"
		userDo = userDo.Where(field.Or(
			userDAL.Username.Like(keyword),
			userDAL.Nickname.Like(keyword),
			userDAL.Email.Like(keyword),
		))
	}
	if query.Role != nil {
		if !query.Role.IsValid() {
			return nil, 0, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("角色参数错误")
		}
		userDo = userDo.Where(userDAL.Role.Eq(*query.Role))
	}
	if query.Disabled != nil {
		userDo = userDo.Where(userDAL.Disabled.Eq(*query.Disabled))
	}

	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageNum < 0 {
		query.PageNum = 0
	}
	users, total, err := userDo.Order(userDAL.ID).FindByPage(query.PageNum*query.PageSize, query.PageSize)
	if err != nil {
		return nil, 0, WrapDBErr(err)
	}
	result := make([]*dto.User, 0, len(users))
	for _, user := range users {
		result = append(result, s.UserService.ConvertToDTO(ctx, user))
	}
	return result, total, nil
}

func (s *userManagementServiceImpl) Get(ctx context.Context, id int32) (*dto.User, error) {
	user, err := s.getUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.UserService.ConvertToDTO(ctx, user), nil
}

func (s *userManagementServiceImpl) Create(ctx context.Context, userParam param.CreateUser) (*dto.User, error) {
	if !userParam.Role.IsValid() {
		return nil, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("角色参数错误")
	}
	created, err := s.UserService.CreateByParam(ctx, param.User{
		Username:    userParam.Username,
		Nickname:    userParam.Nickname,
		Email:       userParam.Email,
		Password:    userParam.Password,
		Avatar:      userParam.Avatar,
		Description: userParam.Description,
		Role:        userParam.Role,
	})
	if err != nil {
		return nil, err
	}
	return s.UserService.ConvertToDTO(ctx, created), nil
}

func (s *userManagementServiceImpl) Update(ctx context.Context, id int32, userParam param.UpdateUser) (*dto.User, error) {
	user, err := s.getUser(ctx, id)
	if err != nil {
		return nil, err
	}
	userDAL := dal.GetQueryByCtx(ctx).User
	_, err = userDAL.WithContext(ctx).Where(userDAL.ID.Eq(id)).UpdateSimple(
		userDAL.Username.Value(userParam.Username),
		userDAL.Nickname.Value(userParam.Nickname),
		userDAL.Email.Value(userParam.Email),
		userDAL.Avatar.Value(userParam.Avatar),
		userDAL.Description.Value(userParam.Description),
	)
	if err != nil {
		return nil, WrapDBErr(err)
	}
	user.Username = userParam.Username
	user.Nickname = userParam.Nickname
	user.Email = userParam.Email
	user.Avatar = userParam.Avatar
	user.Description = userParam.Description
	return s.UserService.ConvertToDTO(ctx, user), nil
}

func (s *userManagementServiceImpl) UpdateRole(ctx context.Context, id int32, role consts.UserRole) error {
	if !role.IsValid() {
		return xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("角色参数错误")
	}
	user, err := s.getUser(ctx, id)
	if err != nil {
		return err
	}
	if role != consts.UserRoleSuperAdmin {
		if err := s.ensureNotRemovingLastActiveSuperAdmin(ctx, user); err != nil {
			return err
		}
	}
	if user.Role == role {
		return nil
	}

	userDAL := dal.GetQueryByCtx(ctx).User
	if _, err := userDAL.WithContext(ctx).Where(userDAL.ID.Eq(id)).UpdateSimple(userDAL.Role.Value(role)); err != nil {
		return WrapDBErr(err)
	}
	clearUserTokens(s.Cache, id)
	return nil
}

func (s *userManagementServiceImpl) UpdateStatus(ctx context.Context, id int32, disabled bool) error {
	user, err := s.getUser(ctx, id)
	if err != nil {
		return err
	}
	if disabled {
		if err := s.ensureNotRemovingLastActiveSuperAdmin(ctx, user); err != nil {
			return err
		}
	}

	userDAL := dal.GetQueryByCtx(ctx).User
	if _, err := userDAL.WithContext(ctx).Where(userDAL.ID.Eq(id)).UpdateSimple(userDAL.Disabled.Value(disabled)); err != nil {
		return WrapDBErr(err)
	}
	if disabled {
		clearUserTokens(s.Cache, id)
	}
	return nil
}

func (s *userManagementServiceImpl) ResetPassword(ctx context.Context, id int32, newPassword string) error {
	if err := validatePasswordStrength(newPassword); err != nil {
		return err
	}
	if _, err := s.getUser(ctx, id); err != nil {
		return err
	}

	userDAL := dal.GetQueryByCtx(ctx).User
	if _, err := userDAL.WithContext(ctx).Where(userDAL.ID.Eq(id)).
		UpdateSimple(userDAL.Password.Value(s.UserService.EncryptPassword(ctx, newPassword))); err != nil {
		return WrapDBErr(err)
	}
	clearUserTokens(s.Cache, id)
	return nil
}

func (s *userManagementServiceImpl) ContentCount(ctx context.Context, id int32) (*dto.UserContentCount, error) {
	if _, err := s.getUser(ctx, id); err != nil {
		return nil, err
	}
	return s.countContent(ctx, id)
}

func (s *userManagementServiceImpl) Delete(ctx context.Context, id int32, transferToID int32) (*dto.UserContentCount, error) {
	actor, ok := currentUser(ctx)
	if !ok {
		return nil, xerr.WithStatus(nil, xerr.StatusForbidden).WithMsg("未登录")
	}
	if actor.ID == id {
		return nil, xerr.WithStatus(nil, xerr.StatusForbidden).WithMsg("不能删除自己")
	}
	target, err := s.getUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureNotRemovingLastActiveSuperAdmin(ctx, target); err != nil {
		return nil, err
	}

	counts, err := s.countContent(ctx, id)
	if err != nil {
		return nil, err
	}
	hasContent := counts.Posts > 0 || counts.Sheets > 0 || counts.Attachments > 0
	if hasContent {
		if transferToID <= 0 || transferToID == id {
			return nil, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("请选择内容接手用户")
		}
		transferUser, err := s.getUser(ctx, transferToID)
		if err != nil {
			return nil, err
		}
		if transferUser.Disabled {
			return nil, xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("接手用户已被禁用")
		}
	}

	err = dal.GetQueryByCtx(ctx).Transaction(func(tx *dal.Query) error {
		if hasContent {
			postDAL := tx.Post
			if _, err := postDAL.WithContext(ctx).Where(postDAL.AuthorID.Eq(id)).
				UpdateSimple(postDAL.AuthorID.Value(transferToID)); err != nil {
				return WrapDBErr(err)
			}
			attachmentDAL := tx.Attachment
			if _, err := attachmentDAL.WithContext(ctx).Where(attachmentDAL.UserID.Eq(id)).
				UpdateSimple(attachmentDAL.UserID.Value(transferToID)); err != nil {
				return WrapDBErr(err)
			}
		}
		recoveryDAL := tx.MfaRecoveryCode
		if _, err := recoveryDAL.WithContext(ctx).Where(recoveryDAL.UserID.Eq(id)).Delete(); err != nil {
			return WrapDBErr(err)
		}
		userDAL := tx.User
		result, err := userDAL.WithContext(ctx).Where(userDAL.ID.Eq(id)).Delete()
		if err != nil {
			return WrapDBErr(err)
		}
		if result.RowsAffected != 1 {
			return xerr.NoType.New("").WithMsg("删除用户失败")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	clearUserTokens(s.Cache, id)
	return counts, nil
}

func (s *userManagementServiceImpl) GetMFAPolicy(ctx context.Context) (consts.MFAPolicy, error) {
	return s.OptionService.GetMFAPolicy(ctx)
}

func (s *userManagementServiceImpl) SaveMFAPolicy(ctx context.Context, policy consts.MFAPolicy) error {
	if !policy.IsValid() {
		return xerr.BadParam.New("").WithStatus(xerr.StatusBadRequest).WithMsg("MFA 策略参数错误")
	}
	return s.OptionService.Save(ctx, map[string]string{property.MFAPolicy.KeyValue: string(policy)})
}

func (s *userManagementServiceImpl) ListSelectable(ctx context.Context) ([]*dto.SelectableUser, error) {
	userDAL := dal.GetQueryByCtx(ctx).User
	users, err := userDAL.WithContext(ctx).Where(userDAL.Disabled.Eq(false)).Order(userDAL.ID).Find()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	result := make([]*dto.SelectableUser, 0, len(users))
	for _, user := range users {
		result = append(result, &dto.SelectableUser{
			ID:       user.ID,
			Username: user.Username,
			Nickname: user.Nickname,
			Role:     user.Role,
		})
	}
	return result, nil
}

func (s *userManagementServiceImpl) getUser(ctx context.Context, id int32) (*entity.User, error) {
	userDAL := dal.GetQueryByCtx(ctx).User
	user, err := userDAL.WithContext(ctx).Where(userDAL.ID.Eq(id)).First()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return user, nil
}

func (s *userManagementServiceImpl) countContent(ctx context.Context, id int32) (*dto.UserContentCount, error) {
	postDAL := dal.GetQueryByCtx(ctx).Post
	postCount, err := postDAL.WithContext(ctx).
		Where(postDAL.AuthorID.Eq(id), postDAL.Type.Eq(consts.PostTypePost)).Count()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	sheetCount, err := postDAL.WithContext(ctx).
		Where(postDAL.AuthorID.Eq(id), postDAL.Type.Eq(consts.PostTypeSheet)).Count()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	attachmentDAL := dal.GetQueryByCtx(ctx).Attachment
	attachmentCount, err := attachmentDAL.WithContext(ctx).Where(attachmentDAL.UserID.Eq(id)).Count()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	return &dto.UserContentCount{
		Posts:       postCount,
		Sheets:      sheetCount,
		Attachments: attachmentCount,
	}, nil
}

func (s *userManagementServiceImpl) ensureNotRemovingLastActiveSuperAdmin(ctx context.Context, user *entity.User) error {
	if user.Role != consts.UserRoleSuperAdmin || user.Disabled {
		return nil
	}
	userDAL := dal.GetQueryByCtx(ctx).User
	count, err := userDAL.WithContext(ctx).
		Where(userDAL.Role.Eq(consts.UserRoleSuperAdmin), userDAL.Disabled.Eq(false)).Count()
	if err != nil {
		return WrapDBErr(err)
	}
	if count <= 1 {
		return xerr.WithStatus(nil, xerr.StatusForbidden).WithMsg("不能移除最后一个有效的超级管理员")
	}
	return nil
}
