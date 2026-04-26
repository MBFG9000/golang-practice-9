package database

import (
	"log"

	_ "github.com/lib/pq"

	"github.com/MBFG9000/golang-practice-9/internal/config"
	"github.com/jmoiron/sqlx"
)

type Dialect struct {
	DB *sqlx.DB
}

func GetConnection(config *config.DatabaseConfig) *Dialect {

	db, err := sqlx.Connect("postgres", config.FormatDSNString("postgres"))

	if err != nil {
		log.Fatal("Database connection not initialized", err)
	}

	return &Dialect{
		DB: db,
	}
}
