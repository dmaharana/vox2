package db

import (
	"encoding/json"
	"time"
)

// MCPServerRecord represents an MCP server stored in SQLite.
type MCPServerRecord struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// SaveMCPServer stores or updates an MCP server configuration in SQLite.
func (d *DB) SaveMCPServer(s MCPServerRecord) error {
	argsJSON, _ := json.Marshal(s.Args)
	envJSON, _ := json.Marshal(s.Env)
	headersJSON, _ := json.Marshal(s.Headers)

	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	s.UpdatedAt = time.Now().UTC()

	enabledInt := 0
	if s.Enabled {
		enabledInt = 1
	}

	query := `INSERT INTO mcp_servers (id, name, transport, command, args, env, url, headers, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			transport = excluded.transport,
			command = excluded.command,
			args = excluded.args,
			env = excluded.env,
			url = excluded.url,
			headers = excluded.headers,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at`

	_, err := d.db.Exec(query,
		s.ID, s.Name, s.Transport, s.Command, string(argsJSON), string(envJSON), s.URL, string(headersJSON),
		enabledInt, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

// GetMCPServers returns all saved MCP servers from SQLite.
func (d *DB) GetMCPServers() ([]MCPServerRecord, error) {
	query := `SELECT id, name, transport, command, args, env, url, headers, enabled, created_at, updated_at FROM mcp_servers ORDER BY created_at ASC`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]MCPServerRecord, 0)
	for rows.Next() {
		var s MCPServerRecord
		var argsJSON, envJSON, headersJSON string
		var enabledInt int

		if err := rows.Scan(&s.ID, &s.Name, &s.Transport, &s.Command, &argsJSON, &envJSON, &s.URL, &headersJSON, &enabledInt, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}

		s.Enabled = enabledInt == 1
		_ = json.Unmarshal([]byte(argsJSON), &s.Args)
		_ = json.Unmarshal([]byte(envJSON), &s.Env)
		_ = json.Unmarshal([]byte(headersJSON), &s.Headers)
		if s.Args == nil {
			s.Args = []string{}
		}
		if s.Env == nil {
			s.Env = make(map[string]string)
		}
		if s.Headers == nil {
			s.Headers = make(map[string]string)
		}

		records = append(records, s)
	}

	return records, nil
}

// DeleteMCPServer removes an MCP server from SQLite.
func (d *DB) DeleteMCPServer(id string) error {
	_, err := d.db.Exec(`DELETE FROM mcp_servers WHERE id = ?`, id)
	return err
}
