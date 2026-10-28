package impl

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/hipoint-airpress/airpress/dal"
	"github.com/hipoint-airpress/airpress/model/entity"
)

var (
	// htmlURLAttrPattern 匹配 HTML 属性中的 URL，兼容单双引号；
	// 捕获完整属性名以保留 data-src 这类前缀，避免重写时破坏属性。
	htmlURLAttrPattern = regexp.MustCompile(`(?i)\b([\w-]*(?:src|href))(\s*=\s*)("([^"]*)"|'([^']*)')`)
	// markdownURLPattern 匹配 Markdown 链接/图片目标。
	markdownURLPattern = regexp.MustCompile(`(!?\[[^\]]*\]\(\s*<?)([^)\s>]+)(>?\s*[^)]*\))`)
)

// extractContentTargets 提取正文中 HTML 属性与 Markdown 链接里的 URL 并去重。
// 扫描顺序固定为：先扫描全部 HTML 属性，再扫描全部 Markdown 链接，因此返回
// 顺序并非 URL 在文档中的真实出现顺序。返回值为属性/链接目标原文，可能包含
// HTML 实体（如 &amp;），调用方需要时自行 unescape。
// 该实现基于正则，不解析文档结构：仅处理带引号的 HTML 属性，Markdown 目标
// 不支持含 ")" 的文件名（正则会在第一个 ")" 处截断）。
func extractContentTargets(content string) []string {
	seen := make(map[string]struct{})
	targets := make([]string, 0)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		targets = append(targets, value)
	}

	for _, match := range htmlURLAttrPattern.FindAllStringSubmatch(content, -1) {
		value := match[4]
		if value == "" {
			value = match[5]
		}
		add(value)
	}
	for _, match := range markdownURLPattern.FindAllStringSubmatch(content, -1) {
		add(match[2])
	}
	return targets
}

// replaceContentTargets 只替换 HTML 属性值与 Markdown 链接目标，正文文本不受影响。
// HTML 属性值查表前会裁剪首尾空白，与 extractContentTargets 保持一致；替换后的
// 输出不保留原有首尾空白。
func replaceContentTargets(content string, replacements map[string]string) string {
	content = htmlURLAttrPattern.ReplaceAllStringFunc(content, func(match string) string {
		sub := htmlURLAttrPattern.FindStringSubmatch(match)
		quoted := sub[3]
		value := sub[4]
		if value == "" {
			value = sub[5]
		}
		value = strings.TrimSpace(value)
		replacement, ok := replacements[value]
		if !ok {
			return match
		}
		quote := string(quoted[0])
		return sub[1] + sub[2] + quote + html.EscapeString(replacement) + quote
	})

	content = markdownURLPattern.ReplaceAllStringFunc(content, func(match string) string {
		sub := markdownURLPattern.FindStringSubmatch(match)
		replacement, ok := replacements[sub[2]]
		if !ok {
			return match
		}
		return sub[1] + replacement + sub[3]
	})
	return content
}

type contentURLMode int

const (
	contentURLNormalize contentURLMode = iota
	contentURLResolve
)

// NormalizeContentURLs 把正文中指向附件库的完整 URL 归一化为 attachment.path。
// 外部链接、站内链接、无法匹配到附件的内容保持原样。可重复执行。
func NormalizeContentURLs(ctx context.Context, content string) (string, error) {
	return rewriteContentURLs(ctx, content, contentURLNormalize)
}

// normalizeCandidateKeys 生成用于匹配 attachment.path / file_key 的候选值。
// 不依赖任何域名配置，因此换域名后的旧数据同样能命中。
func normalizeCandidateKeys(raw string) []string {
	value := strings.TrimSpace(html.UnescapeString(raw))
	lower := strings.ToLower(value)
	var parsed *url.URL
	var err error
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		parsed, err = url.Parse(value)
		if err != nil {
			return nil
		}
	case strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//"):
		parsed, err = url.Parse(value)
		if err != nil {
			return nil
		}
	default:
		return nil
	}

	pathPart := strings.TrimPrefix(parsed.Path, "/")
	if pathPart == "" {
		return nil
	}

	keys := []string{pathPart}
	// MinIO 的 URL 路径包含 bucket 前缀，附件 path 不含 bucket。
	if i := strings.IndexByte(pathPart, '/'); i > 0 {
		keys = append(keys, pathPart[i+1:])
	}
	// OSS 样式参数保存在 attachment.path 的查询串里。
	if parsed.RawQuery != "" {
		withQuery := make([]string, 0, len(keys))
		for _, key := range keys {
			withQuery = append(withQuery, key+"?"+parsed.RawQuery)
		}
		keys = append(withQuery, keys...)
	}
	return keys
}

func findAttachmentsByKeys(ctx context.Context, keys []string) (map[string]*entity.Attachment, error) {
	result := make(map[string]*entity.Attachment, len(keys)*2)
	if len(keys) == 0 {
		return result, nil
	}
	attachmentDAL := dal.GetQueryByCtx(ctx).Attachment
	attachments, err := attachmentDAL.WithContext(ctx).
		Where(attachmentDAL.Path.In(keys...)).
		Or(attachmentDAL.FileKey.In(keys...)).
		Order(attachmentDAL.ID).
		Find()
	if err != nil {
		return nil, WrapDBErr(err)
	}
	// Path 优先于 FileKey：先写 Path 并保留首个（ID 最小）记录，再写 FileKey，
	// 已存在的 key 不覆盖，保证同一 file_key 多行时匹配结果确定。
	for _, attachment := range attachments {
		if _, ok := result[attachment.Path]; !ok {
			result[attachment.Path] = attachment
		}
	}
	for _, attachment := range attachments {
		if _, ok := result[attachment.FileKey]; !ok {
			result[attachment.FileKey] = attachment
		}
	}
	return result, nil
}

// urlFragment 返回 URL 中的 fragment（不含 "#"），解析失败或没有 fragment 时返回空串。
func urlFragment(raw string) string {
	value := strings.TrimSpace(html.UnescapeString(raw))
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	return parsed.Fragment
}

func rewriteContentURLs(ctx context.Context, content string, mode contentURLMode) (string, error) {
	targets := extractContentTargets(content)
	if len(targets) == 0 {
		return content, nil
	}

	keysByTarget := make(map[string][]string, len(targets))
	allKeys := make([]string, 0, len(targets))
	seenKeys := make(map[string]struct{})
	for _, target := range targets {
		var keys []string
		if mode == contentURLNormalize {
			keys = normalizeCandidateKeys(target)
		}
		keysByTarget[target] = keys
		for _, key := range keys {
			if _, ok := seenKeys[key]; ok {
				continue
			}
			seenKeys[key] = struct{}{}
			allKeys = append(allKeys, key)
		}
	}
	if len(allKeys) == 0 {
		return content, nil
	}

	attachments, err := findAttachmentsByKeys(ctx, allKeys)
	if err != nil {
		return content, err
	}

	replacements := make(map[string]string, len(targets))
	for _, target := range targets {
		for _, key := range keysByTarget[target] {
			attachment, ok := attachments[key]
			if !ok {
				continue
			}
			// 通过 file_key 命中时统一使用 attachment.path，原 URL 的查询串会被丢弃，
			// 以当前存储配置/样式参数为准；fragment 属于文档定位，需要保留。
			replacement := attachment.Path
			if fragment := urlFragment(target); fragment != "" {
				replacement += "#" + fragment
			}
			replacements[target] = replacement
			break
		}
	}
	if len(replacements) == 0 {
		return content, nil
	}
	return replaceContentTargets(content, replacements), nil
}
