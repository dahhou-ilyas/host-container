package db_config

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pool *pgxpool.Pool
	once sync.Once
)

func Init(dsn string) error {
	var initErr error
	once.Do(func() {
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			initErr = err
			return
		}
		cfg.MaxConns = 10
		cfg.MaxConnLifetime = time.Hour

		p, err := pgxpool.NewWithConfig(context.Background(), cfg)
		if err != nil {
			initErr = err
			return
		}
		if err := p.Ping(context.Background()); err != nil {
			p.Close()
			initErr = err
			return
		}
		pool = p
	})
	return initErr
}

func Pool() *pgxpool.Pool {
	if pool == nil {
		log.Panic("db not initialized: call db.Init first")
	}
	return pool
}

func Close() {
	if pool != nil {
		pool.Close()
	}
}
