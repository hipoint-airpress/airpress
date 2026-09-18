package api

import "github.com/hipoint-airpress/airpress/injection"

func init() {
	injection.Provide(
		NewArchiveHandler,
		NewCategoryHandler,
		NewJournalHandler,
		NewLinkHandler,
		NewPostHandler,
		NewSheetHandler,
		NewOptionHandler,
		NewPhotoHandler,
		NewCommentHandler,
	)
}
