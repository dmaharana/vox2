package db

import (
	"encoding/json"
	"time"

	"go-harness/pkg/crypto"
)

// MCPServerRecord represents an MCP server stored in SQLite.
type MCPServerRecord struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Transport         string            `json:"transport"`
	Command           string            `json:"command"`
	Args              []string          `json:"args"`
	Env               map[string]string `json:"env"`
	URL               string            `json:"url"`
	Headers           map[string]string `json:"headers"`
	AuthType          string            `json:"auth_type,omitempty"` // "none", "headers", "oauth2"
	OAuthClientID     string            `json:"oauth_client_id,omitempty"`
	OAuthClientSecret string            `json:"oauth_client_secret,omitempty"` // encrypted in DB
	OAuthTokenURL     string            `json:"oauth_token_url,omitempty"`
	OAuthScopes       string            `json:"oauth_scopes,omitempty"`
	OAuthAccessToken  string            `json:"oauth_access_token,omitempty"` // encrypted in DB
	Enabled           bool              `json:"enabled"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// SaveMCPServer stores or updates an MCP server configuration in SQLite.
func (d *DB) SaveMCPServer(s MCPServerRecord) error {
	argsJSON, _ := json.Marshal(s.Args)
	envJSON, _ := json.Marshal(s.Env)

	// Encrypt sensitive headers if present (e.g. Authorization)
	storedHeaders := make(map[string]string)
	for k, v := range s.Headers {
		if k == "Authorization" || k == "X-API-Key" || k == "api-key" {
			enc, err := crypto.Encrypt(v)
			if err == nil {
				storedHeaders[k] = enc
			} else {
				storedHeaders[k] = v
			}
		} else {
			storedHeaders[k] = v
		}
	}
	headersJSON, _ := json.Marshal(storedHeaders)

	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	s.UpdatedAt = time.Now().UTC()

	enabledInt := 0
	if s.Enabled {
		enabledInt = 1
	}

	encryptedClientSecret := s.OAuthClientSecret
	if s.OAuthClientSecret != "" {
		if enc, err := crypto.Encrypt(s.OAuthClientSecret); err == nil {
			encryptedClientSecret = enc
		}
	}

	encryptedAccessToken := s.OAuthAccessToken
	if s.OAuthAccessToken != "" {
		if enc, err := crypto.Encrypt(s.OAuthAccessToken); err == nil {
			encryptedAccessToken = enc
		}
	}

	query := `INSERT INTO mcp_servers (id, name, transport, command, args, env, url, headers, auth_type, oauth_client_id, oauth_client_secret, oauth_token_url, oauth_scopes, oauth_access_token, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			transport = excluded.transport,
			command = excluded.command,
			args = excluded.args,
			env = excluded.env,
			url = excluded.url,
			headers = excluded.headers,
			auth_type = excluded.auth_type,
			oauth_client_id = excluded.oauth_client_id,
			oauth_client_secret = excluded.oauth_client_secret,
			oauth_token_url = excluded.oauth_token_url,
			oauth_scopes = excluded.oauth_scopes,
			oauth_access_token = excluded.oauth_access_token,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at`

	_, err := d.db.Exec(query,
		s.ID, s.Name, s.Transport, s.Command, string(argsJSON), string(envJSON), s.URL, string(headersJSON),
		s.AuthType, s.OAuthClientID, encryptedClientSecret, s.OAuthTokenURL, s.OAuthScopes, encryptedAccessToken,
		enabledInt, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

// GetMCPServers returns all saved MCP servers from SQLite with decrypted secrets.
func (d *DB) GetMCPServers() ([]MCPServerRecord, error) {
	query := `SELECT id, name, transport, command, args, env, url, headers, auth_type, oauth_client_id, oauth_client_secret, oauth_token_url, oauth_scopes, oauth_access_token, enabled, created_at, updated_at FROM mcp_servers ORDER BY created_at ASC`
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

		if err := rows.Scan(
			&s.ID, &s.Name, &s.Transport, &s.Command, &argsJSON, &envJSON, &s.URL, &headersJSON,
			&s.AuthType, &s.OAuthClientID, &s.OAuthClientSecret, &s.OAuthTokenURL, &s.OAuthScopes, &s.OAuthAccessToken,
			&enabledInt, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
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
		} else {
			// Decrypt sensitive headers if encrypted
			for k, v := range s.Headers {
				if dec, err := crypto.Decrypt(v); err == nil {
					s.Headers[k] = dec
				}
			}
		}

		// Decrypt OAuth secrets
		if s.OAuthClientSecret != "" {
			if dec, err := crypto.Decrypt(s.OAuthClientSecret); err == nil {
				s.OAuthClientSecret = dec
			}
		}
		if s.OAuthAccessToken != "" {
			if dec, err := crypto.Decrypt(s.OAuthAccessToken); err == nil {
				s.OAuthAccessToken = dec
			}
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
