package impl

import (
	"context"

	"github.com/hipoint-airpress/airpress/dal"
)

const contentURLMigrateBatchSize = 200

// MigrateContentURLs 把存量文章/页面/日志正文中的附件完整 URL 归一化为相对 path。
// 返回实际更新的记录数。可重复执行。
func MigrateContentURLs(ctx context.Context) (int64, error) {
	updated, err := migratePostsContentURLs(ctx)
	if err != nil {
		return updated, err
	}
	journalUpdated, err := migrateJournalsContentURLs(ctx)
	if err != nil {
		return updated + journalUpdated, err
	}
	return updated + journalUpdated, nil
}

func migratePostsContentURLs(ctx context.Context) (int64, error) {
	var updated int64
	var lastID int32
	for {
		postDAL := dal.GetQueryByCtx(ctx).Post
		posts, err := postDAL.WithContext(ctx).
			Where(postDAL.ID.Gt(lastID)).
			Order(postDAL.ID).
			Limit(contentURLMigrateBatchSize).
			Find()
		if err != nil {
			return updated, WrapDBErr(err)
		}
		if len(posts) == 0 {
			return updated, nil
		}
		for _, post := range posts {
			originalContent, err := NormalizeContentURLs(ctx, post.OriginalContent)
			if err != nil {
				return updated, err
			}
			formatContent, err := NormalizeContentURLs(ctx, post.FormatContent)
			if err != nil {
				return updated, err
			}
			if originalContent == post.OriginalContent && formatContent == post.FormatContent {
				continue
			}
			_, err = postDAL.WithContext(ctx).Where(postDAL.ID.Eq(post.ID)).UpdateSimple(
				postDAL.OriginalContent.Value(originalContent),
				postDAL.FormatContent.Value(formatContent),
			)
			if err != nil {
				return updated, WrapDBErr(err)
			}
			updated++
		}
		lastID = posts[len(posts)-1].ID
	}
}

func migrateJournalsContentURLs(ctx context.Context) (int64, error) {
	var updated int64
	var lastID int32
	for {
		journalDAL := dal.GetQueryByCtx(ctx).Journal
		journals, err := journalDAL.WithContext(ctx).
			Where(journalDAL.ID.Gt(lastID)).
			Order(journalDAL.ID).
			Limit(contentURLMigrateBatchSize).
			Find()
		if err != nil {
			return updated, WrapDBErr(err)
		}
		if len(journals) == 0 {
			return updated, nil
		}
		for _, journal := range journals {
			sourceContent, err := NormalizeContentURLs(ctx, journal.SourceContent)
			if err != nil {
				return updated, err
			}
			content, err := NormalizeContentURLs(ctx, journal.Content)
			if err != nil {
				return updated, err
			}
			if sourceContent == journal.SourceContent && content == journal.Content {
				continue
			}
			_, err = journalDAL.WithContext(ctx).Where(journalDAL.ID.Eq(journal.ID)).UpdateSimple(
				journalDAL.SourceContent.Value(sourceContent),
				journalDAL.Content.Value(content),
			)
			if err != nil {
				return updated, WrapDBErr(err)
			}
			updated++
		}
		lastID = journals[len(journals)-1].ID
	}
}
