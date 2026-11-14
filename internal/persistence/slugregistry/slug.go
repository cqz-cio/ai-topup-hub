package slugregistry

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrSlugReserved = errors.New("catalog slug belongs to another entity")

// ReserveSlug is called inside the same transaction as the canonical write.
// Current names are reserved too, so two-table canonical/alias races cannot
// transfer an old URL to another entity. Table and column are internal constants.
func ReserveSlug(db *gorm.DB, table, ownerColumn, value string, id uint) error {
	row := map[string]interface{}{"slug": value, ownerColumn: id, "created_at": time.Now().UTC()}
	if err := db.Table(table).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "slug"}}, DoNothing: true}).Create(row).Error; err != nil {
		return err
	}
	var owner uint
	if err := db.Table(table).Select(ownerColumn).Where("slug = ?", value).Row().Scan(&owner); err != nil {
		return err
	}
	if owner != id {
		return ErrSlugReserved
	}
	return nil
}

func CountReservedSlugs(db *gorm.DB, table, ownerColumn, value string, excludeID *string) (int64, error) {
	var count int64
	query := db.Table(table).Where("slug = ?", value)
	if excludeID != nil {
		query = query.Where(ownerColumn+" != ?", *excludeID)
	}
	err := query.Count(&count).Error
	return count, err
}
