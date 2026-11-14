package catalogslugs

import (
	"errors"
	"regexp"
	"strconv"
	"time"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	"github.com/dujiao-next/internal/persistence/slugregistry"
	"github.com/dujiao-next/internal/shared/jsonmap"
	catalogslug "github.com/dujiao-next/internal/shared/slug"
	"gorm.io/gorm"
)

var catalogSlugNames = map[string]string{
	"qzweb-4": "chatgpt-go-code-egypt", "qzweb-5": "chatgpt-plus-code",
	"qzweb-6": "chatgpt-pro-100-ph", "qzweb-55": "chatgpt-pro-200-ph",
	"qzweb-7": "chatgpt-pro-200-renew-ph", "qzweb-56": "chatgpt-pro-500-ph",
	"qzweb-58": "chatgpt-pro-500-egypt", "qzweb-57": "chatgpt-pro-100-egypt",
	"qzweb-52": "gemini-pro-18m-warranty", "qzweb-53": "gemini-pro-18m-no-warranty",
	"86gamestore-82": "chatgpt-pro-200-manual", "86gamestore-83": "chatgpt-pro-100-manual",
	"86gamestore-88": "chatgpt-plus-manual", "86gamestore-10010": "chatgpt-go-manual",
	"86gamestore-10004": "telegram-premium-3m", "86gamestore-10005": "telegram-premium-6m",
	"86gamestore-10006": "telegram-premium-12m",
	"qzweb-category-4":  "gpt-recharge", "qzweb-category-7": "gemini",
}

// Only catalog slugs and URL reservations change. IDs, updated_at, financial
// snapshots, inventory and supplier mappings are deliberately not rewritten.
// Once a legacy URL is an alias, a later operator rename is never undone.
func Run(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		const key = "migration/catalog_slugs_20261114"
		var marker settingsstore.SettingRecord
		if err := tx.First(&marker, "key = ?", key).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		} else if done, _ := marker.ValueJSON["done"].(bool); done {
			return nil
		}
		var products []productdomain.Product
		var categories []categorydomain.Category
		if err := tx.Select("id", "slug", "title_json", "is_active").Order("id").Find(&products).Error; err != nil {
			return err
		}
		if err := tx.Select("id", "slug", "name_json", "is_active").Order("id").Find(&categories).Error; err != nil {
			return err
		}
		for _, p := range products {
			if err := slugregistry.ReserveSlug(tx, "product_slug_aliases", "product_id", p.Slug, p.ID); err != nil {
				return err
			}
		}
		for _, c := range categories {
			if err := slugregistry.ReserveSlug(tx, "category_slug_aliases", "category_id", c.Slug, c.ID); err != nil {
				return err
			}
		}
		legacy := regexp.MustCompile(`^(qzweb-[0-9]+|qzweb-category-[0-9]+|86gamestore-[0-9]+)$`)
		migrate := func(table, aliases, owner string, id uint, old string, title jsonmap.JSON, active bool) error {
			base := catalogSlugNames[old]
			if base == "" {
				if !active || !legacy.MatchString(old) {
					return nil
				}
				for _, locale := range []string{"zh-CN", "en-US"} {
					if text, ok := title[locale].(string); ok && text != "" {
						base = catalogslug.Generate(text)
						break
					}
				}
				if base == "" || legacy.MatchString(base) {
					return nil
				}
			}
			exclude := strconv.FormatUint(uint64(id), 10)
			value, err := catalogslug.Unique(base, func(candidate string) (bool, error) {
				count, err := slugregistry.CountReservedSlugs(tx, aliases, owner, candidate, &exclude)
				return count > 0, err
			})
			if err != nil {
				return err
			}
			if err := slugregistry.ReserveSlug(tx, aliases, owner, value, id); err != nil {
				return err
			}
			return tx.Table(table).Where("id = ? AND slug = ?", id, old).UpdateColumn("slug", value).Error
		}
		for _, p := range products {
			if err := migrate("products", "product_slug_aliases", "product_id", p.ID, p.Slug, p.TitleJSON, p.IsActive); err != nil {
				return err
			}
		}
		for _, c := range categories {
			if err := migrate("categories", "category_slug_aliases", "category_id", c.ID, c.Slug, c.NameJSON, c.IsActive); err != nil {
				return err
			}
		}
		marker.Key = key
		marker.ValueJSON = jsonmap.JSON{"done": true, "migrated_at": time.Now().UTC().Format(time.RFC3339)}
		return tx.Save(&marker).Error
	})
}
