package db

import (
	"database/sql"
	"net"
	"net/url"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewPostgres(cfg DBConfig) (*sql.DB, error) {
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, cfg.Port),
		Path:   cfg.Database,
	}

	return openDB("pgx", dsn.String(), cfg)
}
