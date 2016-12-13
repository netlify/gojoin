package models

import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"

	"github.com/jinzhu/gorm"

	"github.com/netlify/netlify-subscriptions/conf"
)

// Namespace puts all tables names under a common
// namespace. This is useful if you want to use
// the same database for several services and don't
// want table names to collide.
var Namespace string

func Connect(config *conf.DBConfig) (*gorm.DB, error) {
	db, err := gorm.Open(config.Driver, config.ConnURL)
	if err != nil {
		return nil, errors.Wrap(err, "opening database connection")
	}

	err = db.DB().Ping()
	if err != nil {
		return nil, errors.Wrap(err, "checking database connection")
	}

	if config.Automigrate {
		if err := AutoMigrate(db); err != nil {
			return nil, errors.Wrap(err, "migrating tables")
		}
	}

	return db, nil
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(Subscription{}, User{}).Error
}
func tableName(defaultName string) string {
	if Namespace != "" {
		return Namespace + "_" + defaultName
	}
	return defaultName
}
