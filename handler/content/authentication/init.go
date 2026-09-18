package authentication

import "github.com/hipoint-airpress/airpress/injection"

func init() {
	injection.Provide(
		NewCategoryAuthentication,
		NewPostAuthentication,
	)
}
