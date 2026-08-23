package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-harness/pkg/db"
	"go-harness/pkg/tools"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// MemoryType represents the cognitive memory category.
type MemoryType string

const (
	TypeWorking       MemoryType = "working"        // Short-term: active turn/scratchpad
	TypeSemanticCache MemoryType = "semantic_cache" // Short-term: cached prompt-response pairs
	TypeSemantic      MemoryType = "semantic"       // Long-term: general facts, user profile, entity info
	TypeEpisodic      MemoryType = "episodic"       // Long-term: past interaction logs and task outcomes
	TypeProcedural    MemoryType = "procedural"     // Long-term: encoded rules, workflows, skills
)

// MemoryTier indicates memory retention level.
type MemoryTier string

const (
	TierShortTerm MemoryTier = "short_term"
	TierLongTerm  MemoryTier = "long_term"
)

// MemoryItem represents a memory entry in SQLite.
type MemoryItem struct {
	ID          string     `json:"id"`
	MemoryType  MemoryType `json:"memory_type"`
	Tier        MemoryTier `json:"tier"`
	Key         string     `json:"key"`
	Content     string     `json:"content"`
	Tags        string     `json:"tags,omitempty"`
	AccessCount int        `json:"access_count"`
	LastUsedAt  time.Time  `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Manager handles memory persistence, FTS5 retrieval, and tier promotion.
type Manager struct {
	db *db.DB
}

// NewManager creates a new memory manager with the underlying DB.
func NewManager(database *db.DB) *Manager {
	return &Manager{db: database}
}

// Save inserts or updates a memory item.
func (m *Manager) Save(item *MemoryItem) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.Tier == "" {
		if item.MemoryType == TypeWorking || item.MemoryType == TypeSemanticCache {
			item.Tier = TierShortTerm
		} else {
			item.Tier = TierLongTerm
		}
	}
	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.LastUsedAt = now

	query := `
	INSERT INTO memories (id, memory_type, tier, key, content, tags, access_count, last_used_at, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		memory_type = excluded.memory_type,
		tier = excluded.tier,
		key = excluded.key,
		content = excluded.content,
		tags = excluded.tags,
		last_used_at = excluded.last_used_at;
	`
	_, err := m.db.SQL().Exec(query,
		item.ID,
		string(item.MemoryType),
		string(item.Tier),
		item.Key,
		item.Content,
		item.Tags,
		item.AccessCount,
		item.LastUsedAt,
		item.CreatedAt,
	)
	return err
}

// Get fetches a memory by its ID.
func (m *Manager) Get(id string) (*MemoryItem, error) {
	query := `SELECT id, memory_type, tier, key, content, tags, access_count, last_used_at, created_at FROM memories WHERE id = ?`
	row := m.db.SQL().QueryRow(query, id)

	var item MemoryItem
	var mType, mTier string
	if err := row.Scan(&item.ID, &mType, &mTier, &item.Key, &item.Content, &item.Tags, &item.AccessCount, &item.LastUsedAt, &item.CreatedAt); err != nil {
		return nil, err
	}
	item.MemoryType = MemoryType(mType)
	item.Tier = MemoryTier(mTier)
	return &item, nil
}

// Search queries memories using FTS5 keyword matching and relevance ranking.
func (m *Manager) Search(query string, memoryType string, tier string, limit int) ([]MemoryItem, error) {
	if limit <= 0 {
		limit = 10
	}

	sanitized := sanitizeFTS5Query(query)
	var rows *sql.Rows
	var err error

	if sanitized == "" {
		// Return most recent if empty query
		sqlQuery := `SELECT id, memory_type, tier, key, content, tags, access_count, last_used_at, created_at FROM memories ORDER BY last_used_at DESC LIMIT ?`
		rows, err = m.db.SQL().Query(sqlQuery, limit)
	} else {
		// FTS5 MATCH with join
		sqlQuery := `
		SELECT m.id, m.memory_type, m.tier, m.key, m.content, m.tags, m.access_count, m.last_used_at, m.created_at
		FROM memories m
		JOIN memories_fts fts ON m.id = fts.id
		WHERE memories_fts MATCH ?
		ORDER BY rank
		LIMIT ?;
		`
		rows, err = m.db.SQL().Query(sqlQuery, sanitized, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("memory search failed: %w", err)
	}
	defer rows.Close()

	results := make([]MemoryItem, 0)
	now := time.Now().UTC()

	for rows.Next() {
		var item MemoryItem
		var mType, mTier string
		if err := rows.Scan(&item.ID, &mType, &mTier, &item.Key, &item.Content, &item.Tags, &item.AccessCount, &item.LastUsedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.MemoryType = MemoryType(mType)
		item.Tier = MemoryTier(mTier)

		// Filter memoryType and tier if specified
		if memoryType != "" && string(item.MemoryType) != memoryType {
			continue
		}
		if tier != "" && string(item.Tier) != tier {
			continue
		}

		item.AccessCount++
		item.LastUsedAt = now
		results = append(results, item)

		_, _ = m.db.SQL().Exec(`UPDATE memories SET access_count = access_count + 1, last_used_at = ? WHERE id = ?`, now, item.ID)
	}

	return results, nil
}

// AutoPromoteAging moves short-term memories accessed frequently (or aged) to long-term memory.
func (m *Manager) AutoPromoteAging(minAccessCount int) (int64, error) {
	if minAccessCount <= 0 {
		minAccessCount = 3
	}

	query := `
	UPDATE memories 
	SET tier = 'long_term' 
	WHERE tier = 'short_term' AND (access_count >= ? OR memory_type IN ('semantic', 'episodic', 'procedural'));
	`
	res, err := m.db.SQL().Exec(query, minAccessCount)
	if err != nil {
		return 0, err
	}
	count, _ := res.RowsAffected()
	if count > 0 {
		log.Info().Int64("promoted", count).Msg("Promoted short-term memories to long-term memory")
	}
	return count, nil
}

// RegisterMemoryTools adds memory search & save tools to the agent tool registry.
func (m *Manager) RegisterMemoryTools(reg *tools.Registry) {
	reg.Register(m.newSaveMemoryTool())
	reg.Register(m.newSearchMemoryTool())
}

func (m *Manager) newSaveMemoryTool() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "save_memory",
		Description: "Saves a fact, user preference, workflow procedure, or key-value memory into searchable long-term/short-term storage.",
		Category:    "memory",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"key": {
					Type:        jsonschema.String,
					Description: "Short descriptive identifier or subject for this memory",
				},
				"content": {
					Type:        jsonschema.String,
					Description: "The detailed knowledge, instruction, or event content to store",
				},
				"memory_type": {
					Type:        jsonschema.String,
					Description: "Type: 'semantic' (facts), 'episodic' (events), 'procedural' (workflows), 'working' (short-term)",
					Enum:        []string{"semantic", "episodic", "procedural", "working", "semantic_cache"},
				},
				"tags": {
					Type:        jsonschema.String,
					Description: "Comma-separated keywords or tags for search indexing",
				},
			},
			Required: []string{"key", "content"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in struct {
				Key        string `json:"key"`
				Content    string `json:"content"`
				MemoryType string `json:"memory_type"`
				Tags       string `json:"tags"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}
			if in.MemoryType == "" {
				in.MemoryType = "semantic"
			}

			item := &MemoryItem{
				Key:        in.Key,
				Content:    in.Content,
				MemoryType: MemoryType(in.MemoryType),
				Tags:       in.Tags,
			}
			if err := m.Save(item); err != nil {
				return nil, fmt.Errorf("failed to save memory: %w", err)
			}

			return map[string]any{
				"success": true,
				"id":      item.ID,
				"key":     item.Key,
				"message": "Memory saved successfully",
			}, nil
		},
	}
}

func (m *Manager) newSearchMemoryTool() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "search_memory",
		Description: "Searches through cognitive agent memory (semantic facts, past experiences, procedures) using full-text keyword search.",
		Category:    "memory",
		Enabled:     true,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {
					Type:        jsonschema.String,
					Description: "Search terms or keywords to locate relevant memories",
				},
				"memory_type": {
					Type:        jsonschema.String,
					Description: "Optional filter: 'semantic', 'episodic', 'procedural', 'working'",
				},
				"limit": {
					Type:        jsonschema.Integer,
					Description: "Max number of memories to return (default 5)",
				},
			},
			Required: []string{"query"},
		},
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			var in struct {
				Query      string `json:"query"`
				MemoryType string `json:"memory_type"`
				Limit      int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return nil, fmt.Errorf("invalid arguments: %w", err)
			}
			if in.Limit <= 0 {
				in.Limit = 5
			}

			items, err := m.Search(in.Query, in.MemoryType, "", in.Limit)
			if err != nil {
				return nil, err
			}

			return map[string]any{
				"query": in.Query,
				"count": len(items),
				"items": items,
			}, nil
		},
	}
}

// sanitizeFTS5Query converts input text to safe FTS5 query tokens.
func sanitizeFTS5Query(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Remove special FTS5 operators like quotes, asterisks, parens
	replacer := strings.NewReplacer(`"`, " ", `'`, " ", `*`, " ", `(`, " ", `)`, " ", `^`, " ", `:`, " ", `+`, " ", `-`, " ")
	cleaned := replacer.Replace(raw)
	words := strings.Fields(cleaned)
	if len(words) == 0 {
		return ""
	}

	// Append '*' wildcard for prefix matching
	var tokens []string
	for _, w := range words {
		if len(w) > 0 {
			tokens = append(tokens, fmt.Sprintf(`"%s"*`, w))
		}
	}
	return strings.Join(tokens, " OR ")
}
