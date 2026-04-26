package config

import (
	"fmt"
	"log"
	"os"
)

type DatabaseConfig struct {
	Host         string
	Port         string
	DatabaseName string
	Password     string
	User         string
	SslMode      string
}

func (cfg *DatabaseConfig) FormatDSNString(database string) string {
	return fmt.Sprintf("%s://%s:%s@%s:%s/%s?sslmode=%s", database, cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DatabaseName, cfg.SslMode)
}

func DatabaseInit() *DatabaseConfig {
	sslmode := os.Getenv("DB_SSL")
	if sslmode == "" {
		log.Fatal("Need database ssl setting. Enviroment variable DB_SSL is empty")
	}

	host := os.Getenv("DB_HOST")
	if host == "" {
		log.Fatal("Need database host. Enviroment variable DB_HOST is empty")
	}

	port := os.Getenv("DB_PORT")
	if port == "" {
		log.Fatal("Need database port. Enviroment variable DB_PORT is empty")
	}

	databaseName := os.Getenv("DB_NAME")
	if databaseName == "" {
		log.Fatal("Need database name. Enviroment variable DB_NAME is empty")
	}

	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		log.Fatal("Need database password. Enviroment variable DB_PASSWORD is empty")
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		log.Fatal("Need database user. Enviroment variable DB_USER is empty")
	}

	return &DatabaseConfig{
		Host:         host,
		Port:         port,
		DatabaseName: databaseName,
		Password:     password,
		User:         user,
		SslMode:      sslmode,
	}
}
