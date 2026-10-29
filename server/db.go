package main

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id         INTEGER PRIMARY KEY,
	google_sub TEXT NOT NULL UNIQUE,
	email      TEXT NOT NULL,
	name       TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS enroll_codes (
	code       TEXT PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	expires_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS hosts (
	id         INTEGER PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name       TEXT NOT NULL,
	os         TEXT NOT NULL,
	key_hash   TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL,
	last_seen  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS hosts_user ON hosts(user_id);
CREATE TABLE IF NOT EXISTS metrics (
	host_id    INTEGER NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
	ts         INTEGER NOT NULL,
	cpu        REAL NOT NULL,
	mem_used   REAL NOT NULL,
	mem_total  REAL NOT NULL,
	disk_used  REAL NOT NULL,
	disk_total REAL NOT NULL,
	net_rx     REAL NOT NULL,
	net_tx     REAL NOT NULL,
	load1      REAL NOT NULL,
	PRIMARY KEY (host_id, ts)
) WITHOUT ROWID;
`

func openDB(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// A single writer connection avoids SQLITE_BUSY under concurrent ingest.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return db, nil
}
