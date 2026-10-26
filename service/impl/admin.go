package impl

import (
	"bufio"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/hipoint-airpress/airpress/cache"
	"github.com/hipoint-airpress/airpress/config"
	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/event"
	"github.com/hipoint-airpress/airpress/log"
	"github.com/hipoint-airpress/airpress/model/dto"
	"github.com/hipoint-airpress/airpress/model/entity"
	"github.com/hipoint-airpress/airpress/model/param"
	"github.com/hipoint-airpress/airpress/model/property"
	"github.com/hipoint-airpress/airpress/service"
	"github.com/hipoint-airpress/airpress/util"
	"github.com/hipoint-airpress/airpress/util/xerr"
)

type adminServiceImpl struct {
	UserService         service.UserService
	OptionService       service.OptionService
	Cache               cache.Cache
	Config              *config.Config
	Event               event.Bus
	TwoFactorTOTPMFA    service.TwoFactorTOTPMFAService
	EmailService        service.EmailService
	RecoveryCodeService service.RecoveryCodeService
	LoginSecurity       *loginSecurity
}

func NewAdminService(userService service.UserService, optionService service.OptionService, cache cache.Cache, config *config.Config, event event.Bus, twoFactorMFA service.TwoFactorTOTPMFAService, emailService service.EmailService, recoveryCodeService service.RecoveryCodeService) service.AdminService {
	return &adminServiceImpl{
		UserService:         userService,
		OptionService:       optionService,
		Cache:               cache,
		Config:              config,
		Event:               event,
		TwoFactorTOTPMFA:    twoFactorMFA,
		EmailService:        emailService,
		RecoveryCodeService: recoveryCodeService,
		LoginSecurity:       newLoginSecurity(cache),
	}
}

func (a *adminServiceImpl) Authenticate(ctx context.Context, loginParam param.LoginParam) (*entity.User, error) {
	missMatchTip := "用户名或密码不正确"
	ip := util.GetClientIP(ctx)

	if err := a.LoginSecurity.CheckPasswordLock(loginParam.Username, ip); err != nil {
		return nil, err
	}

	var user *entity.User
	err := util.Validate.Var(loginParam.Username, "email")

	if err != nil {
		user, err = a.UserService.GetByUsername(ctx, loginParam.Username)
	} else {
		user, err = a.UserService.GetByEmail(ctx, loginParam.Username)
	}

	if xerr.GetType(err) == xerr.NoRecord {
		a.LoginSecurity.RecordPasswordFailure(loginParam.Username, 0, ip)
		return nil, xerr.WithMsg(err, missMatchTip).WithStatus(xerr.StatusBadRequest)
	}
	if err != nil {
		return nil, err
	}

	if user.Disabled {
		return nil, xerr.Forbidden.New("").WithStatus(xerr.StatusForbidden).WithMsg("账号已被禁用")
	}

	if err := a.UserService.MustNotExpire(ctx, user.ExpireTime); err != nil {
		return nil, err
	}

	if !a.UserService.PasswordMatch(ctx, user.Password, loginParam.Password) {
		a.LoginSecurity.RecordPasswordFailure(loginParam.Username, user.ID, ip)
		return nil, xerr.BadParam.New("").WithMsg(missMatchTip).WithStatus(xerr.StatusBadRequest)
	}
	a.LoginSecurity.ClearPasswordFailures(loginParam.Username, user.ID, ip)
	return user, nil
}

func (a *adminServiceImpl) Auth(ctx context.Context, loginParam param.LoginParam) (*dto.AuthTokenDTO, error) {
	user, err := a.Authenticate(ctx, loginParam)
	if err != nil {
		return nil, err
	}
	if a.TwoFactorTOTPMFA.UseMFA(user.MfaType) {
		if err := a.LoginSecurity.CheckMFALock(user.ID); err != nil {
			return nil, err
		}
		ok, err := a.validateSecondFactor(ctx, user, loginParam.AuthCode)
		if err != nil {
			return nil, err
		}
		if !ok {
			a.LoginSecurity.RecordMFAFailure(user.ID)
			return nil, xerr.WithStatus(nil, xerr.StatusBadRequest).WithMsg("两步验证码或恢复码验证错误")
		}
		a.LoginSecurity.ClearMFAFailures(user.ID)
	}

	enrollmentRequired, err := a.MFAEnrollmentRequired(ctx, user)
	if err != nil {
		return nil, err
	}

	a.LoginSecurity.ClearAll(loginParam.Username, user.ID, util.GetClientIP(ctx))
	a.Event.Publish(ctx, &event.LogEvent{
		LogKey:    user.Username,
		LogType:   consts.LogTypeLoggedIn,
		Content:   user.Nickname,
		IPAddress: util.GetClientIP(ctx),
	})
	token, err := a.buildAuthToken(user)
	if err != nil {
		return nil, err
	}
	token.MFAEnrollmentRequired = enrollmentRequired
	return token, nil
}

func (a *adminServiceImpl) MFAEnrollmentRequired(ctx context.Context, user *entity.User) (bool, error) {
	return mfaEnrollmentRequired(ctx, a.OptionService, user)
}

func (a *adminServiceImpl) validateSecondFactor(ctx context.Context, user *entity.User, authCode string) (bool, error) {
	code := strings.TrimSpace(authCode)
	if isSixDigitCode(code) {
		return a.TwoFactorTOTPMFA.ValidateTFACode(user.MfaKey, code), nil
	}
	if a.RecoveryCodeService == nil {
		return false, nil
	}
	return a.RecoveryCodeService.ValidateAndConsume(ctx, user.ID, code)
}

func isSixDigitCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (a *adminServiceImpl) ClearToken(ctx context.Context) error {
	user, ok := GetAuthorizedUser(ctx)
	if !ok || user == nil {
		return xerr.Forbidden.New("").WithStatus(xerr.StatusForbidden).WithMsg("未登录")
	}
	clearUserTokens(a.Cache, user.ID)
	a.Event.Publish(ctx, &event.LogEvent{
		LogKey:    user.Username,
		LogType:   consts.LogTypeLoggedOut,
		Content:   user.Nickname,
		IPAddress: util.GetClientIP(ctx),
	})
	return nil
}

func (a *adminServiceImpl) SendResetPasswordCode(ctx context.Context, resetParam param.ResetPasswordParam) error {
	user, ok := GetAuthorizedUser(ctx)
	if !ok || user == nil {
		return xerr.Forbidden.New("").WithStatus(xerr.StatusForbidden).WithMsg("未登录")
	}
	_, ok = a.Cache.Get(cache.BuildCodeCacheKey(user.ID))
	if ok {
		return xerr.NoType.New("").WithMsg("已经获取过验证码，不能重复获取").WithStatus(xerr.StatusInternalServerError)
	}
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	code := fmt.Sprintf("%06v", rnd.Int31n(1000000))

	log.CtxInfof(ctx, "重置密码验证码: %v", code)
	a.Cache.Set(cache.BuildCodeCacheKey(user.ID), code, consts.CodeValidDuration)
	emailEnabled, err := a.OptionService.GetOrByDefaultWithErr(ctx, property.EmailIsEnabled, false)
	if err != nil {
		return err
	}
	if !emailEnabled.(bool) {
		return xerr.NoType.New("未启用 SMTP 服务").WithMsg("未启用 SMTP 服务，无法发送邮件，但是你可以通过系统日志找到验证码")
	}
	content := "您正在进行密码重置操作，如不是本人操作，请尽快做好相应措施。密码重置验证码如下（五分钟有效）：\n" + code
	return a.EmailService.SendTextEmail(ctx, resetParam.Email, "找回密码验证码", content)
}

// GenerateAccessToken 生成 256 位密码学随机 token（64 位十六进制）。
func GenerateAccessToken() (string, error) {
	b := make([]byte, 32)
	if _, err := cryptorand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *adminServiceImpl) buildAuthToken(user *entity.User) (*dto.AuthTokenDTO, error) {
	// 单会话语义：同一账号重新登录时让旧设备下线。
	clearUserTokens(a.Cache, user.ID)

	accessToken, err := GenerateAccessToken()
	if err != nil {
		return nil, err
	}
	refreshToken, err := GenerateAccessToken()
	if err != nil {
		return nil, err
	}

	authToken := &dto.AuthTokenDTO{}
	authToken.AccessToken = accessToken
	authToken.ExpiredIn = consts.AccessTokenExpiredSeconds
	authToken.RefreshToken = refreshToken

	a.Cache.Set(cache.BuildTokenAccessKey(accessToken), user.ID, time.Second*consts.AccessTokenExpiredSeconds)
	a.Cache.Set(cache.BuildTokenRefreshKey(refreshToken), user.ID, consts.RefreshTokenExpiredDays*24*3600*time.Second)

	a.Cache.Set(cache.BuildAccessTokenKey(user.ID), accessToken, time.Second*consts.AccessTokenExpiredSeconds)
	a.Cache.Set(cache.BuildRefreshTokenKey(user.ID), refreshToken, consts.RefreshTokenExpiredDays*24*3600*time.Second)

	return authToken, nil
}

func (a *adminServiceImpl) RefreshToken(ctx context.Context, refreshToken string) (*dto.AuthTokenDTO, error) {
	value, ok := a.Cache.Get(cache.BuildTokenRefreshKey(refreshToken))
	if !ok {
		return nil, xerr.WithMsg(nil, "登录状态已失效，请重新登录").WithStatus(xerr.StatusBadRequest)
	}
	userID, ok := value.(int32)
	if !ok {
		return nil, xerr.WithMsg(nil, "登录状态已失效，请重新登录").WithStatus(xerr.StatusBadRequest)
	}
	user, err := a.UserService.GetByID(ctx, userID)
	if xerr.GetType(err) == xerr.NoRecord {
		return nil, xerr.WithMsg(err, "用户不存在").WithStatus(xerr.StatusUnauthorized)
	}
	if err != nil {
		return nil, err
	}
	if user.Disabled {
		return nil, xerr.Forbidden.New("").WithStatus(xerr.StatusForbidden).WithMsg("账号已被禁用")
	}
	if err := a.UserService.MustNotExpire(ctx, user.ExpireTime); err != nil {
		return nil, err
	}
	return a.buildAuthToken(user)
}

// clearUserTokens 删除某个用户当前有效的 access/refresh token 及其反向映射。
func clearUserTokens(c cache.Cache, userID int32) {
	if value, ok := c.Get(cache.BuildAccessTokenKey(userID)); ok {
		if token, ok := value.(string); ok {
			c.Delete(cache.BuildTokenAccessKey(token))
		}
		c.Delete(cache.BuildAccessTokenKey(userID))
	}
	if value, ok := c.Get(cache.BuildRefreshTokenKey(userID)); ok {
		if token, ok := value.(string); ok {
			c.Delete(cache.BuildTokenRefreshKey(token))
		}
		c.Delete(cache.BuildRefreshTokenKey(userID))
	}
}

func (a *adminServiceImpl) GetEnvironments(ctx context.Context) *dto.EnvironmentDTO {
	environments := &dto.EnvironmentDTO{
		Database:  string(dal.DBType) + " " + consts.DatabaseVersion,
		Version:   consts.AirPressVersion,
		StartTime: consts.StartTime.UnixMilli(),
		Mode:      util.IfElse(a.Config.AirPress.Mode == "", "production", a.Config.AirPress.Mode).(string),
	}
	return environments
}

func (a *adminServiceImpl) GetLogFiles(ctx context.Context, lineNum int64) (string, error) {
	errTips := "读取日志文件错误"
	lineEndByte := []byte("\n")[0]
	linesTotalByteNum := 0
	var linesCount int64

	fileName := filepath.Join(a.Config.AirPress.LogDir, a.Config.Log.FileName)
	file, err := os.Open(fileName)
	if err != nil {
		return "", xerr.WithStatus(err, xerr.StatusInternalServerError).WithMsg(errTips)
	}

	defer func() {
		err = file.Close()
		if err != nil {
			log.CtxError(ctx, "关闭日志文件错误", zap.Error(err))
		}
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		return "", xerr.WithStatus(err, xerr.StatusInternalServerError).WithMsg(errTips)
	}

	fileBytesLength := fileInfo.Size()
	position := fileBytesLength - 1

	bufReader := bufio.NewReader(file)
	curLine := bytes.Buffer{}

	globalIsPrefix := false
	lines := make([][]byte, 0, lineNum)

	for position > 0 {
		if !globalIsPrefix {
			position--

			_, err = file.Seek(position, 0)
			if err != nil {
				return "", xerr.WithMsg(err, errTips).WithStatus(xerr.StatusInternalServerError)
			}

			bufReader.Reset(file)

			curByte, err := bufReader.ReadByte()
			if err != nil {
				return "", xerr.WithMsg(err, errTips).WithStatus(xerr.StatusInternalServerError)
			}
			if curByte != lineEndByte {
				continue
			}
		}

		content, isPrefix, err := bufReader.ReadLine()
		if err != nil {
			return "", xerr.WithMsg(err, errTips).WithStatus(xerr.StatusInternalServerError)
		}

		globalIsPrefix = isPrefix

		if !isPrefix {
			curLine.Write(content)
			lines = append(lines, curLine.Bytes())
			linesTotalByteNum += len(content) + 1
			linesCount++
			curLine = bytes.Buffer{}
		} else {
			curLine.Write(content)
		}
		if linesCount == lineNum {
			break
		}
	}
	result := bytes.Buffer{}
	result.Grow(linesTotalByteNum)

	for i := len(lines) - 1; i >= 0; i-- {
		result.Write(lines[i])
		result.WriteByte(lineEndByte)
	}
	return result.String(), nil
}
