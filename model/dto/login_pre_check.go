package dto

type LoginPreCheckDTO struct {
	NeedMFACode           bool `json:"needMFACode"`
	MFAEnrollmentRequired bool `json:"mfaEnrollmentRequired"`
}
