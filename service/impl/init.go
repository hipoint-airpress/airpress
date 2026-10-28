package impl

import (
	"github.com/hipoint-airpress/airpress/injection"
	"github.com/hipoint-airpress/airpress/service/storage"
)

func init() {
	injection.Provide(
		NewAdminService,
		NewAttachmentService,
		NewContentURLService,
		NewAuthenticateService,
		NewBackUpService,
		NewBaseCommentService,
		NewBasePostService,
		NewCategoryService,
		NewEmailService,
		NewInstallService,
		NewJournalService,
		NewLinkService,
		NewJournalCommentService,
		NewLogService,
		NewMenuService,
		NewMetaService,
		NewBaseMFAService,
		NewTwoFactorTOTPMFAService,
		NewOneTimeTokenService,
		NewOptionService,
		NewClientOptionService,
		NewPhotoService,
		NewPostService,
		NewPostCategoryService,
		NewPostCommentService,
		NewPostTagService,
		NewRecoveryCodeService,
		NewSheetService,
		NewSheetCommentService,
		NewStatisticService,
		NewTagService,
		NewThemeService,
		NewUserService,
		NewUserManagementService,
		NewExportImport,
		storage.NewFileStorageComposite,
	)
}
