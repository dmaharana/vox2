package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"go-harness/pkg/config"
	"go-harness/pkg/db"
	"go-harness/pkg/memory"
	"go-harness/pkg/skills"
	"go-harness/pkg/tools"
	"go-harness/pkg/ws"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog/log"
)

// Server encapsulates the HTTP router, config, WebSocket hub, database, and subsystems.
type Server struct {
	cfg          *config.Config
	router       *chi.Mux
	hub          *ws.Hub
	db           *db.DB
	memoryMgr    *memory.Manager
	skillsLoader *skills.Loader
	toolsReg     *tools.Registry
	httpServer   *http.Server
	spaFS        fs.FS
}

// ServerOptions allows optional injection of subsystems.
type ServerOptions struct {
	DB           *db.DB
	MemoryMgr    *memory.Manager
	SkillsLoader *skills.Loader
	ToolsReg     *tools.Registry
	SpaFS        fs.FS
}

// New creates and configures a new Server instance.
func New(cfg *config.Config, hub *ws.Hub, opts ...ServerOptions) *Server {
	s := &Server{
		cfg:    cfg,
		router: chi.NewRouter(),
		hub:    hub,
	}

	if len(opts) > 0 {
		opt := opts[0]
		s.db = opt.DB
		s.memoryMgr = opt.MemoryMgr
		s.skillsLoader = opt.SkillsLoader
		s.toolsReg = opt.ToolsReg
		s.spaFS = opt.SpaFS
	}

	s.setupMiddlewares()
	s.setupRoutes()
	return s
}

// Router returns the underlying Chi router.
func (s *Server) Router() *chi.Mux {
	return s.router
}

func (s *Server) setupMiddlewares() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(120 * time.Second))

	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

func (s *Server) setupRoutes() {
	s.router.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/settings", s.handleGetSettings)
		r.Post("/settings", s.handlePostSettings)

		// Conversations
		r.Get("/conversations", s.handleListConversations)
		r.Get("/conversations/{id}", s.handleGetConversation)
		r.Delete("/conversations/{id}", s.handleDeleteConversation)
		r.Get("/conversations/{id}/export", s.handleExportConversationCSV)

		// Memories
		r.Get("/memories", s.handleSearchMemories)
		r.Post("/memories", s.handleSaveMemory)
		r.Post("/memories/promote", s.handlePromoteMemories)

		// Tools & Skills
		r.Get("/tools", s.handleListTools)
		r.Post("/tools/{name}/toggle", s.handleToggleTool)
		r.Get("/skills", s.handleListSkills)
		r.Post("/skills/{name}/toggle", s.handleToggleSkill)
	})

	if s.hub != nil {
		s.router.Get("/ws", s.hub.ServeWS)
	}

	if s.spaFS != nil {
		s.router.Mount("/", NewSPAHandler(s.spaFS))
	} else {
		s.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":  "online",
				"message": "Go Agent Harness API server running",
			})
		})
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	})
}

// SettingsResponse returns sanitized settings to the frontend.
type SettingsResponse struct {
	LLMBaseURL     string  `json:"llm_base_url"`
	LLMModel       string  `json:"llm_model"`
	HasAPIKey      bool    `json:"has_api_key"`
	LLMTemperature float64 `json:"llm_temperature"`
	LLMMaxTokens   int     `json:"llm_max_tokens"`
	LogLevel       string  `json:"log_level"`
	LogFormat      string  `json:"log_format"`
	SkillsDir      string  `json:"skills_dir"`
	OTelExporter   string  `json:"otel_exporter"`
	OTelEndpoint   string  `json:"otel_endpoint"`
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	current := s.cfg.Clone()
	resp := SettingsResponse{
		LLMBaseURL:     current.LLMBaseURL,
		LLMModel:       current.LLMModel,
		HasAPIKey:      current.LLMAPIKey != "",
		LLMTemperature: current.LLMTemperature,
		LLMMaxTokens:   current.LLMMaxTokens,
		LogLevel:       current.LogLevel,
		LogFormat:      current.LogFormat,
		SkillsDir:      current.SkillsDir,
		OTelExporter:   current.OTelExporter,
		OTelEndpoint:   current.OTelEndpoint,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type UpdateSettingsRequest struct {
	LLMBaseURL     *string  `json:"llm_base_url,omitempty"`
	LLMModel       *string  `json:"llm_model,omitempty"`
	LLMAPIKey      *string  `json:"llm_api_key,omitempty"`
	LLMTemperature *float64 `json:"llm_temperature,omitempty"`
	LLMMaxTokens   *int     `json:"llm_max_tokens,omitempty"`
	LogLevel       *string  `json:"log_level,omitempty"`
	SkillsDir      *string  `json:"skills_dir,omitempty"`
}

func (s *Server) handlePostSettings(w http.ResponseWriter, r *http.Request) {
	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	current := s.cfg.Clone()
	if req.LLMBaseURL != nil {
		current.LLMBaseURL = *req.LLMBaseURL
	}
	if req.LLMModel != nil {
		current.LLMModel = *req.LLMModel
	}
	if req.LLMAPIKey != nil {
		current.LLMAPIKey = *req.LLMAPIKey
	}
	if req.LLMTemperature != nil {
		current.LLMTemperature = *req.LLMTemperature
	}
	if req.LLMMaxTokens != nil {
		current.LLMMaxTokens = *req.LLMMaxTokens
	}
	if req.LogLevel != nil {
		current.LogLevel = *req.LogLevel
	}
	if req.SkillsDir != nil {
		current.SkillsDir = *req.SkillsDir
	}

	s.cfg.Update(current)
	s.handleGetSettings(w, r)
}

// -------------------------------------------------------------
// Conversation Handlers
// -------------------------------------------------------------

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	list, err := s.db.ListConversations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []db.Conversation{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	conv, err := s.db.GetConversation(id)
	if err != nil {
		http.Error(w, "Conversation not found", http.StatusNotFound)
		return
	}

	messages, _ := s.db.GetMessages(id)
	if messages == nil {
		messages = []db.Message{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"conversation": conv,
		"messages":     messages,
	})
}

func (s *Server) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.db.DeleteConversation(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleExportConversationCSV(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	csvContent, err := s.db.ExportConversationCSV(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=conversation_%s.csv", id))
	_, _ = w.Write([]byte(csvContent))
}

// -------------------------------------------------------------
// Memory Handlers
// -------------------------------------------------------------

func (s *Server) handleSearchMemories(w http.ResponseWriter, r *http.Request) {
	if s.memoryMgr == nil {
		http.Error(w, "Memory manager not configured", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query().Get("q")
	mType := r.URL.Query().Get("type")
	tier := r.URL.Query().Get("tier")
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
		}
	}

	items, err := s.memoryMgr.Search(q, mType, tier, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []memory.MemoryItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

func (s *Server) handleSaveMemory(w http.ResponseWriter, r *http.Request) {
	if s.memoryMgr == nil {
		http.Error(w, "Memory manager not configured", http.StatusServiceUnavailable)
		return
	}

	var item memory.MemoryItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if item.Key == "" || item.Content == "" {
		http.Error(w, "key and content are required", http.StatusBadRequest)
		return
	}

	if err := s.memoryMgr.Save(&item); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}

func (s *Server) handlePromoteMemories(w http.ResponseWriter, r *http.Request) {
	if s.memoryMgr == nil {
		http.Error(w, "Memory manager not configured", http.StatusServiceUnavailable)
		return
	}
	count, err := s.memoryMgr.AutoPromoteAging(2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"promoted_count": count,
		"success":        true,
	})
}

// -------------------------------------------------------------
// Tools & Skills Handlers
// -------------------------------------------------------------

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	if s.toolsReg == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]tools.ToolDefinition{})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.toolsReg.List())
}

func (s *Server) handleToggleTool(w http.ResponseWriter, r *http.Request) {
	if s.toolsReg == nil {
		http.Error(w, "Tools registry not configured", http.StatusServiceUnavailable)
		return
	}
	name := chi.URLParam(r, "name")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	ok := s.toolsReg.SetEnabled(name, req.Enabled)
	if !ok {
		http.Error(w, "Tool not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	if s.skillsLoader == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]skills.Skill{})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.skillsLoader.List())
}

func (s *Server) handleToggleSkill(w http.ResponseWriter, r *http.Request) {
	if s.skillsLoader == nil {
		http.Error(w, "Skills loader not configured", http.StatusServiceUnavailable)
		return
	}
	name := chi.URLParam(r, "name")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	ok := s.skillsLoader.SetEnabled(name, req.Enabled)
	if !ok {
		http.Error(w, "Skill not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	log.Info().Str("addr", addr).Msg("Starting Go Agent Harness HTTP server")
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
