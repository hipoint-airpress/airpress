package dto

type UserContentCount struct {
	Posts       int64 `json:"posts"`
	Sheets      int64 `json:"sheets"`
	Attachments int64 `json:"attachments"`
}
