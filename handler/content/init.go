package content

import "github.com/hipoint-airpress/airpress/injection"

func init() {
	injection.Provide(
		NewIndexHandler,
		NewFeedHandler,
		NewArchiveHandler,
		NewViewHandler,
		NewCategoryHandler,
		NewSheetHandler,
		NewTagHandler,
		NewLinkHandler,
		NewPhotoHandler,
		NewJournalHandler,
		NewSearchHandler,
	)
}
