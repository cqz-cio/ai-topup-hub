// catalog-seo-migrate applies only the public URL migration. It never starts
// application jobs or reads credentials. Dry-run rolls back all schema/data writes.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/dujiao-next/internal/bootstrap/database/migrations/catalogslugs"
	category "github.com/dujiao-next/internal/modules/catalog/category/domain"
	product "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type entry struct {
	ID   uint   `json:"id"`
	Slug string `json:"slug"`
}
type change struct {
	Table  string `json:"table"`
	ID     uint   `json:"id"`
	Before string `json:"before"`
	After  string `json:"after"`
}

func main() {
	database := flag.String("database", "", "existing SQLite database")
	apply := flag.Bool("apply", false, "commit the migration (default: roll back)")
	flag.Parse()
	if err := run(*database, *apply); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path string, apply bool) error {
	if path == "" {
		return errors.New("-database is required")
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return errors.New("database must be an existing file")
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	conn, err := db.DB()
	if err != nil {
		return err
	}
	defer conn.Close()
	changes := []change{}
	dryRun := errors.New("dry run complete")
	err = db.Transaction(func(tx *gorm.DB) error {
		before := map[string][]entry{}
		for _, table := range []string{"products", "categories"} {
			var rows []entry
			if err := tx.Table(table).Select("id, slug").Order("id").Find(&rows).Error; err != nil {
				return err
			}
			before[table] = rows
		}
		if err := tx.AutoMigrate(&product.ProductSlugAlias{}, &category.CategorySlugAlias{}); err != nil {
			return err
		}
		if !tx.Migrator().HasColumn(&category.Category{}, "SeoMetaJSON") {
			if err := tx.Migrator().AddColumn(&category.Category{}, "SeoMetaJSON"); err != nil {
				return err
			}
		}
		if err := catalogslugs.Run(tx); err != nil {
			return err
		}
		for table, old := range before {
			var rows []entry
			if err := tx.Table(table).Select("id, slug").Order("id").Find(&rows).Error; err != nil {
				return err
			}
			if len(rows) != len(old) {
				return errors.New("catalog row count changed")
			}
			for i, row := range rows {
				if row.ID != old[i].ID {
					return errors.New("catalog identity changed")
				}
				if row.Slug != old[i].Slug {
					changes = append(changes, change{table, row.ID, old[i].Slug, row.Slug})
				}
			}
		}
		if !apply {
			return dryRun
		}
		return nil
	})
	if err != nil && !errors.Is(err, dryRun) {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Applied bool     `json:"applied"`
		Changes []change `json:"changes"`
	}{apply, changes})
}
