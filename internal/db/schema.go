/**
 * 按方言创建 codex_accounts；去掉未参与查询的二级索引以降低 upsert 写放大；
 * PostgreSQL 使用 fillfactor 缓解高频 UPDATE 的页分裂。
 */
package db

import (
	"database/sql"
	"fmt"
	"strings"
)

func SetupSchema(db *sql.DB, d Dialect) error {
	if db == nil {
		return nil
	}
	var ddl string
	switch d {
	case DialectMySQL:
		ddl = `
CREATE TABLE IF NOT EXISTS codex_accounts (
	id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
	account_id VARCHAR(768) NULL,
	email VARCHAR(768) NULL,
	id_token MEDIUMTEXT,
	access_token MEDIUMTEXT,
	refresh_token MEDIUMTEXT,
	expire VARCHAR(128),
	plan_type VARCHAR(128),
	last_refresh DATETIME(6) NULL,
	updated_at DATETIME(6) NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
	UNIQUE KEY uk_codex_accounts_account_id (account_id),
	UNIQUE KEY uk_codex_accounts_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`
	case DialectSQLite:
		ddl = `
CREATE TABLE IF NOT EXISTS codex_accounts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id TEXT UNIQUE,
	email TEXT UNIQUE,
	id_token TEXT,
	access_token TEXT,
	refresh_token TEXT,
	expire TEXT,
	plan_type TEXT,
	last_refresh TEXT,
	updated_at TEXT DEFAULT CURRENT_TIMESTAMP
)`
	default:
		ddl = `
CREATE TABLE IF NOT EXISTS codex_accounts (
	id SERIAL PRIMARY KEY,
	account_id TEXT UNIQUE,
	email TEXT UNIQUE,
	id_token TEXT,
	access_token TEXT,
	refresh_token TEXT,
	expire TEXT,
	plan_type TEXT,
	last_refresh TIMESTAMPTZ,
	updated_at TIMESTAMPTZ DEFAULT NOW()
) WITH (fillfactor=90)`
	}
	if _, err := db.Exec(ddl); err != nil {
		return err
	}
	if err := dropLegacySecondaryIndexes(db, d); err != nil {
		return err
	}
	return nil
}

/* 旧版本曾建 refresh_token / updated_at 二级索引，业务查询未使用，删除以降低写放大 */
func dropLegacySecondaryIndexes(db *sql.DB, d Dialect) error {
	switch d {
	case DialectMySQL:
		for _, idx := range []string{"idx_codex_accounts_refresh_token", "idx_codex_accounts_updated_at"} {
			_, err := db.Exec(fmt.Sprintf("ALTER TABLE codex_accounts DROP INDEX %s", mysqlBacktickIdent(idx)))
			if err != nil && !mysqlDropIndexMissing(err) {
				return fmt.Errorf("drop index %s: %w", idx, err)
			}
		}
	case DialectSQLite:
		for _, idx := range []string{"idx_codex_accounts_refresh_token", "idx_codex_accounts_updated_at"} {
			if _, err := db.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", idx)); err != nil {
				return fmt.Errorf("drop index %s: %w", idx, err)
			}
		}
	default:
		for _, idx := range []string{"idx_codex_accounts_refresh_token", "idx_codex_accounts_updated_at"} {
			if _, err := db.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", pqQuoteIdent(idx))); err != nil {
				return fmt.Errorf("drop index %s: %w", idx, err)
			}
		}
	}
	return nil
}

func mysqlDropIndexMissing(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "1091") || strings.Contains(s, "check that column/key exists") || strings.Contains(s, "doesn't exist")
}
