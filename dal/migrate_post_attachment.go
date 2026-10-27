package dal

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	airpressLog "github.com/hipoint-airpress/airpress/log"
	"github.com/hipoint-airpress/airpress/model/entity"
)

const postAttachmentMigrateBatchSize = 100

// migratePostAttachment 为历史文章建立附件关联，只在 post_attachment 表首次创建后执行一次。
func migratePostAttachment(db *gorm.DB) error {
	var lastID int32
	processed := 0
	for {
		posts := make([]entity.Post, 0, postAttachmentMigrateBatchSize)
		if err := db.Select("id", "author_id", "original_content", "format_content", "thumbnail").
			Where("id > ?", lastID).
			Order("id ASC").
			Limit(postAttachmentMigrateBatchSize).
			Find(&posts).Error; err != nil {
			return err
		}
		if len(posts) == 0 {
			airpressLog.Infof("post attachment backfill finished: processed %d posts", processed)
			return nil
		}
		lastID = posts[len(posts)-1].ID
		if err := backfillPostAttachmentBatch(db, posts); err != nil {
			return fmt.Errorf("backfill post attachment up to post %d: %w", lastID, err)
		}
		processed += len(posts)
		airpressLog.Infof("post attachment backfill: processed posts up to id %d", lastID)
	}
}

func backfillPostAttachmentBatch(db *gorm.DB, posts []entity.Post) error {
	authorIDs := make([]int32, 0, len(posts))
	seenAuthor := make(map[int32]struct{}, len(posts))
	for _, post := range posts {
		if _, ok := seenAuthor[post.AuthorID]; ok {
			continue
		}
		seenAuthor[post.AuthorID] = struct{}{}
		authorIDs = append(authorIDs, post.AuthorID)
	}

	attachments := make([]entity.Attachment, 0)
	if err := db.Select("id", "user_id", "path").
		Where("user_id IN ?", authorIDs).
		Find(&attachments).Error; err != nil {
		return err
	}
	byAuthor := make(map[int32][]entity.Attachment, len(authorIDs))
	for _, attachment := range attachments {
		byAuthor[attachment.UserID] = append(byAuthor[attachment.UserID], attachment)
	}

	records := make([]*entity.PostAttachment, 0)
	for _, post := range posts {
		content := post.OriginalContent + "\n" + post.FormatContent + "\n" + post.Thumbnail
		for _, attachment := range byAuthor[post.AuthorID] {
			if attachment.Path == "" || !strings.Contains(content, attachment.Path) {
				continue
			}
			records = append(records, &entity.PostAttachment{
				PostID:       post.ID,
				AttachmentID: attachment.ID,
			})
		}
	}
	if len(records) == 0 {
		return nil
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(records, 500).Error
}
