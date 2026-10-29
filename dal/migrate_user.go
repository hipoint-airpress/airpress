package dal

import (
	"gorm.io/gorm"

	"github.com/hipoint-airpress/airpress/consts"
	"github.com/hipoint-airpress/airpress/model/entity"
)

// migrateUserRBAC 为老库补齐角色与内容归属，逻辑幂等，可在每次启动时执行。
func migrateUserRBAC(db *gorm.DB) error {
	var userCount int64
	if err := db.Model(&entity.User{}).Count(&userCount).Error; err != nil {
		return err
	}
	if userCount == 0 {
		return nil
	}

	var earliest entity.User
	if err := db.Order("id ASC").First(&earliest).Error; err != nil {
		return err
	}

	// 老库升级后没有任何超管时，把最早的用户提升为超管，避免后台无人可管理。
	var superAdminCount int64
	if err := db.Model(&entity.User{}).
		Where("role = ?", consts.UserRoleSuperAdmin).
		Count(&superAdminCount).Error; err != nil {
		return err
	}
	if superAdminCount == 0 {
		if err := db.Model(&entity.User{}).
			Where("id = ?", earliest.ID).
			Update("role", consts.UserRoleSuperAdmin).Error; err != nil {
			return err
		}
	}

	// 历史文章/页面/附件没有归属人，统一回填给最早的用户。
	if err := db.Model(&entity.Post{}).
		Where("author_id = 0").
		Update("author_id", earliest.ID).Error; err != nil {
		return err
	}
	if err := db.Model(&entity.Attachment{}).
		Where("user_id = 0").
		Update("user_id", earliest.ID).Error; err != nil {
		return err
	}
	return nil
}
