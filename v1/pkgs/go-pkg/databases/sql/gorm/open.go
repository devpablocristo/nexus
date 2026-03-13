package gormdb

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type OpenOptions struct {
	DatabaseURL string
}

func Open(opts OpenOptions, gormCfg *gorm.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(opts.DatabaseURL), gormCfg)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}
