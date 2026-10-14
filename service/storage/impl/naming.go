package filestorageimpl

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/aidarkhanov/nanoid/v2"
)

const (
	// nameAlphabet 是随机名用的 base62 字母表；nanoid 默认字母表含 - 和 _，这里不用
	nameAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// nameSize 是随机名长度，样本空间 62^10 ≈ 8.4e17
	nameSize = 10
)

// storageName 生成长度为 nameSize 的 base62 随机文件名主体（不含扩展名）。
// 签名与 filestorageimpl.renameFunc 一致，驱动可直接 as 改名策略传入文件描述符。
func storageName() (string, error) {
	return nanoid.GenerateString(nameAlphabet, nameSize)
}

// categoryFile 是未知扩展名的兜底类型目录
const categoryFile = "file"

// categoryOf 按扩展名返回类型目录名，大小写与前置点号均不敏感，未知扩展名归 file
func categoryOf(ext string) string {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "ico", "tif", "tiff", "heic", "avif":
		return "img"
	case "mp4", "mov", "avi", "mkv", "webm", "flv", "wmv", "m4v":
		return "video"
	case "mp3", "wav", "flac", "aac", "ogg", "m4a", "opus":
		return "audio"
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "md", "csv":
		return "doc"
	case "zip", "rar", "7z", "tar", "gz", "bz2", "xz":
		return "zip"
	default:
		return categoryFile
	}
}

// storageSubPath 返回形如 "img/2026/10" 的类型+年月目录（以 / 分隔，不含存储前缀）
func storageSubPath(t time.Time, filename string) string {
	year, month, _ := t.Date()
	_, ext := splitBaseName(filename)
	return path.Join(categoryOf(ext), strconv.Itoa(year), fmt.Sprintf("%02d", int(month)))
}

// cloudSubPath 拼接云存储的目录前缀与类型/年月目录；前缀为空时不产生多余层级
func cloudSubPath(source string, t time.Time, filename string) string {
	return path.Join(source, storageSubPath(t, filename))
}

// splitBaseName 取上传文件名的基名与扩展名（不含点）。
// 浏览器上传时可能带上完整路径（历史行为是 Windows 反斜杠路径），这里两种分隔符都剥掉；
// 以点开头的隐藏文件（如 .htaccess）视为无扩展名。
func splitBaseName(filename string) (base string, ext string) {
	name := filename
	if i := strings.LastIndexByte(name, '\\'); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	dot := strings.LastIndexByte(name, '.')
	switch {
	case dot <= 0:
		return name, ""
	default:
		return name[:dot], name[dot+1:]
	}
}
