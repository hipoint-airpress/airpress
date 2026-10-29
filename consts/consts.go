package consts

import (
	"time"
)

const (
	AccessTokenExpiredSeconds = 24 * 3600
	RefreshTokenExpiredDays   = 30
	TokenAccessCachePrefix    = "admin_access_token_"
	TokenRefreshCachePrefix   = "admin_refresh_token_"
	AdminTokenHeaderName      = "Admin-Authorization"
	AuthorizedUser            = "authorized_user"
	CodePrefix                = "code_"
	CodeValidDuration         = time.Second
	OneTimeTokenQueryName     = "ott"
	SessionID                 = "session_id"
	AccessPermissionKeyPrefix = "access_permission_"
)

const (
	LoginMaxFailures     = 3
	LoginFailureWindow   = 15 * time.Minute
	LoginLockDuration    = 15 * time.Minute
	LoginIPMaxFailures   = 10
	MFAVerifyMaxFailures = 3
	MFAVerifyWindow      = 10 * time.Minute
	MFALockDuration      = 10 * time.Minute
)

const (
	AirPressBackupPrefix          = "airpress-backup-"
	AirPressDataExportPrefix      = "airpress-data-export-"
	AirPressBackupMarkdownPrefix  = "airpress-backup-markdown-"
	AirPressDefaultTagColor       = "#cfd3d7"
	AirPressUploadDir             = "upload"
	AirPressLogDir                = "logs"
	AirPressTemplateDir           = "resources/template"
	AirPressAdminResourcesDir     = "resources/admin"
	AirPressThemeDir              = "resources/template/theme"
	AirPressDefaultSQLiteFileName = "airpress.db"
	// AirPressDefaultThemeDirName must keep matching the default theme submodule!!
	// repository name (see .gitmodules); renaming it breaks theme resolution.
	AirPressDefaultThemeDirName = "anatole"
)

var (
	ThemePropertyFilenames = [2]string{"theme.yaml", "theme.yml"}
	ThemeSettingFilenames  = [2]string{"settings.yaml", "settings.yml"}
)

const (
	DefaultThemeID         = "caicai_anatole"
	ThemeScreenshotsName   = "screenshot"
	ThemeCustomSheetPrefix = "sheet_"
	ThemeCustomPostPrefix  = "post_"
)

var (
	StartTime       time.Time = time.Now()
	DatabaseVersion string
	AirPressVersion = "v1.0.0"
	BuildTime       string
	BuildCommit     string
)
