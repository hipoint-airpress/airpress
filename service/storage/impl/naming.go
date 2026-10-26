package filestorageimpl

import (
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/aidarkhanov/nanoid/v2"
)

const (
	// nameAlphabet 是随机名用的 base62 字母表；nanoid 默认字母表含 - 和 _，这里不用
	nameAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	// nameSize 是随机名长度，样本空间 62^10 ≈ 8.4e17
	nameSize = 10
)

// randomFilename 生成长度为 nameSize 的 base62 随机文件名主体（不含扩展名）。
// 它是非文档类附件未显式指定改名策略时的默认改名函数，也可用 withRename/withRenameURLOption
// 显式传入（签名与 renameFunc 相同）。
func randomFilename() (string, error) {
	return nanoid.GenerateString(nameAlphabet, nameSize)
}

const (
	// docStorageNameMaxBytes 是文档类附件存储基名的 UTF-8 字节上限。
	// ext4/NTFS 单文件名上限 255 字节，留出扩展名与截断的余量。
	docStorageNameMaxBytes = 200
	// docStorageNameFallback 是清洗后为空时的兜底基名
	docStorageNameFallback = "attachment"
)

// storageNameReplacer 替换会破坏 URL 或跨平台文件系统的字符：
// # ? % 会被 GetFilePath 的 url.JoinPath + url.PathUnescape 误解析，
// 其余是 Windows 保留字符。
var storageNameReplacer = strings.NewReplacer(
	"/", "_", "\\", "_", ":", "_", "*", "_", "?", "_",
	"\"", "_", "<", "_", ">", "_", "|", "_", "#", "_", "%", "_",
)

// docStorageBaseName 把原始文件名转成文档类附件的存储基名：
// 去路径、删空白与控制字符、替换危险字符、按字节截断、去首尾点，空结果兜底为 attachment。
// 空白包括 unicode.IsSpace 覆盖的全部字符（空格/制表/换行/NBSP/全角空格），外加零宽空格与 BOM。
func docStorageBaseName(originalName string) string {
	base, _ := splitBaseName(originalName)

	var b strings.Builder
	for _, r := range base {
		if unicode.IsSpace(r) || r == 0x200B || r == 0xFEFF || r < 0x20 || r == 0x7F {
			continue
		}
		b.WriteRune(r)
	}

	name := strings.Trim(storageNameReplacer.Replace(b.String()), ".")
	if len(name) > docStorageNameMaxBytes {
		cut := docStorageNameMaxBytes
		// 回退到字符起始字节，避免把多字节汉字切成半个
		for cut > 0 && !utf8.RuneStart(name[cut]) {
			cut--
		}
		name = name[:cut]
	}
	name = strings.TrimRight(name, ".")
	return defaultIfEmpty(name, docStorageNameFallback)
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
	_, ext := splitBaseName(filename)
	return path.Join(categoryOf(ext), t.Format("2006/01"))
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
	if dot <= 0 {
		return name, ""
	}
	return name[:dot], name[dot+1:]
}

// attachmentNameMaxLen 是原始文件名的统一长度上限（对应 attachment.name 列宽，也用于 Content-Disposition 展示名），超长按 rune 截断
const attachmentNameMaxLen = 255

// uploadBaseName 取原始上传文件名的基名（去路径、去扩展名），用于 attachment.name
func uploadBaseName(filename string) string {
	base, _ := splitBaseName(filename)
	return truncateRunes(defaultIfEmpty(base, "attachment"), attachmentNameMaxLen)
}

// uploadDisplayName 取原始上传文件名全名（去路径、含扩展名），用于 Content-Disposition 还原
func uploadDisplayName(filename string) string {
	base, ext := splitBaseName(filename)
	if ext != "" {
		base += "." + ext
	}
	return truncateRunes(defaultIfEmpty(base, "attachment"), attachmentNameMaxLen)
}

// truncateRunes 按 rune 截断，避免把多字节汉字切坏
func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// attrChars 是 RFC 5987 允许在 ext-value 中原样出现的非字母数字字符
const attrChars = "!#$&+-.^_`|~"

// contentDisposition 生成 inline 型 Content-Disposition 头值：
// filename 供只认 ASCII 的老客户端降级，filename* 用 RFC 5987 百分号编码保住中文原名。
// 一律 inline 是为了不改变浏览器直接打开图片/PDF/视频链接时的预览行为；
// 另存为/下载时浏览器同样读 filename* 里的原名。
func contentDisposition(filename string) string {
	displayName := uploadDisplayName(filename)
	return fmt.Sprintf("inline; filename=%q; filename*=UTF-8''%s",
		asciiFallback(displayName), rfc5987Encode(displayName))
}

// asciiFallback 把非 ASCII 字符、引号、反斜杠与控制字符替换为下划线
func asciiFallback(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r > 0x7E, r < 0x20, r == '"', r == '\\':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// rfc5987Encode 按 attr-char 集合逐字节百分号编码（输入按 UTF-8 字节处理，故中文原名被完整编码）
func rfc5987Encode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		alnum := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !alnum && strings.IndexByte(attrChars, c) < 0 {
			_, _ = fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
