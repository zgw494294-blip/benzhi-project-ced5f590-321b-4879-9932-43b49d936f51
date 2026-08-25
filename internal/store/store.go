package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db               *sql.DB
	permitSerialMu   sync.Mutex
	nextPermitSerial int64
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		path = "restoration.db"
	}
	dsn := path
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	dsn += separator + "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	store := &Store{db: db, nextPermitSerial: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接 SQLite: %w", err)
	}
	if err := store.Migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }
