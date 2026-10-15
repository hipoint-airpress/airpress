package filestorageimpl

import (
	"testing"
	"time"
)

func TestGetUploadPath(t *testing.T) {
	oct := time.Date(2026, 10, 13, 8, 0, 0, 0, time.Local)
	mar := time.Date(2026, 3, 5, 0, 0, 0, 0, time.Local)
	cases := []struct {
		name     string
		at       time.Time
		filename string
		want     string
	}{
		{"图片落在类型年月目录", oct, "a.JPG", "upload/img/2026/10"},
		{"未知类型落进 file 目录", oct, "c.dat", "upload/file/2026/10"},
		{"浏览器带的 Windows 路径", oct, `C:\fakepath\e.png`, "upload/img/2026/10"},
		{"月份补零", mar, "d.mp3", "upload/audio/2026/03"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := &LocalFileStorage{}
			if got := l.getUploadPath(c.at, c.filename); got != c.want {
				t.Errorf("getUploadPath() = %q, want %q", got, c.want)
			}
		})
	}
}
