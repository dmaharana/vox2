package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// DB wraps the SQL database handle and manages migrations and transactions.
type DB struct {
	mu sync.RWMutex
	db *sql.DB
}

// Open initializes SQLite at dbPath with standard pragmas and schemas.
func Open(dbPath string) (*DB, error) {
	if dbPath == "" {
		dbPath = "./data/harness.db"
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// SQLite connection string with WAL and busy timeout
	connStr := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	sqlDB, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	d := &DB{db: sqlDB}
	if err := d.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	log.Info().Str("path", dbPath).Msg("SQLite database initialized successfully")
	return d, nil
}

// SQL returns the raw *sql.DB instance.
func (d *DB) SQL() *sql.DB {
	return d.db
}

// Close closes the database connection.
func (d *DB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tool_calls TEXT DEFAULT '',
		subflows TEXT DEFAULT '',
		memories_retrieved TEXT DEFAULT '',
		trace_id TEXT DEFAULT '',
		created_at DATETIME NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		memory_type TEXT NOT NULL,
		tier TEXT NOT NULL DEFAULT 'short_term',
		key TEXT NOT NULL,
		content TEXT NOT NULL,
		tags TEXT DEFAULT '',
		access_count INTEGER DEFAULT 0,
		last_used_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		id UNINDEXED,
		key,
		content,
		tags,
		tokenize='porter'
	);

	-- Trigger on memory insert to sync FTS5
	CREATE TRIGGER IF NOT EXISTS trg_memories_ai AFTER INSERT ON memories BEGIN
		INSERT INTO memories_fts(id, key, content, tags)
		VALUES (new.id, new.key, new.content, new.tags);
	END;

	-- Trigger on memory delete to sync FTS5
	CREATE TRIGGER IF NOT EXISTS trg_memories_ad AFTER DELETE ON memories BEGIN
		DELETE FROM memories_fts WHERE id = old.id;
	END;

	-- Trigger on memory update to sync FTS5
	CREATE TRIGGER IF NOT EXISTS trg_memories_au AFTER UPDATE ON memories BEGIN
		DELETE FROM memories_fts WHERE id = old.id;
		INSERT INTO memories_fts(id, key, content, tags)
		VALUES (new.id, new.key, new.content, new.tags);
	END;

	-- Settings table for persisting runtime configuration
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);

	-- MCP Servers table for persisting MCP configurations
	CREATE TABLE IF NOT EXISTS mcp_servers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		transport TEXT NOT NULL,
		command TEXT DEFAULT '',
		args TEXT DEFAULT '[]',
		env TEXT DEFAULT '{}',
		url TEXT DEFAULT '',
		headers TEXT DEFAULT '{}',
		auth_type TEXT DEFAULT '',
		oauth_client_id TEXT DEFAULT '',
		oauth_client_secret TEXT DEFAULT '',
		oauth_token_url TEXT DEFAULT '',
		oauth_scopes TEXT DEFAULT '',
		oauth_access_token TEXT DEFAULT '',
		enabled INTEGER DEFAULT 1,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	`

	_, err := d.db.Exec(schema)
	if err != nil {
		return err
	}

	// Schema column upgrades for existing databases
	_, _ = d.db.Exec(`ALTER TABLE messages ADD COLUMN subflows TEXT DEFAULT ''`)
	_, _ = d.db.Exec(`ALTER TABLE messages ADD COLUMN memories_retrieved TEXT DEFAULT ''`)
	_, _ = d.db.Exec(`ALTER TABLE messages ADD COLUMN trace_id TEXT DEFAULT ''`)

	_, _ = d.db.Exec(`ALTER TABLE mcp_servers ADD COLUMN auth_type TEXT DEFAULT ''`)
	_, _ = d.db.Exec(`ALTER TABLE mcp_servers ADD COLUMN oauth_client_id TEXT DEFAULT ''`)
	_, _ = d.db.Exec(`ALTER TABLE mcp_servers ADD COLUMN oauth_client_secret TEXT DEFAULT ''`)
	_, _ = d.db.Exec(`ALTER TABLE mcp_servers ADD COLUMN oauth_token_url TEXT DEFAULT ''`)
	_, _ = d.db.Exec(`ALTER TABLE mcp_servers ADD COLUMN oauth_scopes TEXT DEFAULT ''`)
	_, _ = d.db.Exec(`ALTER TABLE mcp_servers ADD COLUMN oauth_access_token TEXT DEFAULT ''`)

	return nil
}
