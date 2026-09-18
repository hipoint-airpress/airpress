package filestorageimpl

import "github.com/hipoint-airpress/airpress/injection"

func init() {
	injection.Provide(
		NewMinIO,
		NewLocalFileStorage,
		NewAliyun,
	)
}
