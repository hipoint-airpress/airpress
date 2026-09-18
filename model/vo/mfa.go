package vo

import "github.com/hipoint-airpress/airpress/consts"

type MFAFactorAuth struct {
	QRImage    string         `json:"qrImage"`
	OptAuthURL string         `json:"optAuthUrl"`
	MFAKey     string         `json:"mfaKey"`
	MFAType    consts.MFAType `json:"mfaType"`
}
