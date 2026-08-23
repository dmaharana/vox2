package db

import (
	"database/sql"
	"errors"
	"time"
)

// GetSetting retrieves a setting value by key.
func (d *DB) GetSetting(key string) (string, error) {
	var val string
	err := d.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// SetSetting stores or updates a setting key-value pair.
func (d *DB) SetSetting(key, val string) error {
	now := time.Now().UTC()
	query := `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	_, err := d.db.Exec(query, key, val, now)
	return err
}

// GetAllSettings returns all key-value settings.
func (d *DB) GetAllSettings() (map[string]string, error) {
	rows, err := d.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, nil
}
