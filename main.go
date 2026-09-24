package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go-harness/pkg/config"
	"go-harness/pkg/cron"
	"go-harness/pkg/db"
	"go-harness/pkg/flow"
	"go-harness/pkg/llm"
	"go-harness/pkg/logger"
	"go-harness/pkg/mcp"
	"go-harness/pkg/memory"
	"go-harness/pkg/server"
	"go-harness/pkg/skills"
	"go-harness/pkg/tools"
	"go-harness/pkg/tracing"
	"go-harness/pkg/ws"

	"github.com/rs/zerolog/log"
)

//go:embed all:web/dist
var embeddedWeb embed.FS

func main() {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Parse command-line flags (takes precedence over env / .env)
	if err := cfg.ParseFlags(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize Zerolog Logger
	logger.Init(cfg.LogLevel, cfg.LogFormat)
	log.Info().Msg("Starting Go Agent Harness...")

	// Status feedback for selected LLM Provider
	snap := cfg.Clone()
	isCopilot := strings.EqualFold(snap.LLMProvider, "copilot") ||
		strings.EqualFold(snap.LLMProvider, "github-copilot") ||
		strings.EqualFold(snap.LLMProvider, "copilot-cli")

	if isCopilot {
		copilotBin := snap.CopilotBinary
		if copilotBin == "" {
			copilotBin = "copilot"
		}
		copilotPath, err := exec.LookPath(copilotBin)
		if err != nil {
			log.Warn().
				Str("binary", copilotBin).
				Msg("⚠️ GitHub Copilot CLI binary not found on PATH. Please install from https://github.com/github/copilot-cli or specify --copilot-binary.")
		} else {
			log.Info().
				Str("provider", "GitHub Copilot (CLI)").
				Str("binary_path", copilotPath).
				Str("model", snap.LLMModel).
				Msg("🤖 GitHub Copilot CLI provider active. Note: Expect higher latency per turn (~10-40s) as Copilot CLI operates as an agent.")

			// Quick authentication probe
			checkCmd := exec.Command(copilotPath, "--version")
			if out, err := checkCmd.CombinedOutput(); err != nil {
				log.Warn().
					Str("output", strings.TrimSpace(string(out))).
					Msg("⚠️ GitHub Copilot CLI authentication check failed. Run 'copilot login' or set COPILOT_GITHUB_TOKEN/GH_TOKEN.")
			} else {
				log.Info().
					Str("version", strings.TrimSpace(string(out))).
					Msg("✅ GitHub Copilot CLI verified and ready.")
			}
		}
	} else {
		log.Info().
			Str("provider", "OpenAI-compatible").
			Str("base_url", snap.LLMBaseURL).
			Str("model", snap.LLMModel).
			Str("auth_type", snap.LLMAuthType).
			Msg("🤖 OpenAI-compatible provider active.")
	}

	// 3. Initialize OpenTelemetry Tracing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err = tracing.Init(ctx, cfg)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize OpenTelemetry tracing")
	}
	defer func() {
		_ = tracing.Shutdown(context.Background())
	}()

	// 4. Open SQLite Database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open SQLite database")
	}
	defer database.Close()

	// Restore runtime settings from SQLite database if previously saved
	if err := cfg.LoadFromDB(database); err != nil {
		log.Warn().Err(err).Msg("Failed to load settings from database")
	}

	// 5. Initialize Cognitive Memory Manager
	memMgr := memory.NewManager(database)

	// 6. Initialize Skills Loader
	skillsLoader := skills.NewLoader(cfg.SkillsDir)
	if err := skillsLoader.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load skills")
	}

	// 7. Initialize Unified Tool Registry
	toolsReg := tools.NewRegistry()

	// Register built-in filesystem tools
	tools.RegisterBuiltinTools(toolsReg, ".")

	// Register skill tools (on-demand progressive disclosure)
	skillsLoader.RegisterSkillTools(toolsReg)

	// Register cognitive memory tools
	memMgr.RegisterMemoryTools(toolsReg)

	// 8. Initialize Persistent MCP Manager (official go-sdk)
	mcpMgr := mcp.NewManager(toolsReg, database)
	if err := mcpMgr.LoadFromDB(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load MCP servers from database")
	}

	// 9. Initialize WebSocket Hub & LLM Orchestrator
	hub := ws.NewHub(nil)
	go hub.Run()
	defer hub.Close()

	// Initialize Parallel Flow Engine
	flowEngine := flow.NewEngine(hub, 5)

	// Initialize LLM Orchestrator
	orchestrator := llm.NewOrchestrator(cfg, database, memMgr, skillsLoader, toolsReg, hub)

	// Initialize Background Cron Manager
	cronRunner := func(cCtx context.Context, job db.CronJob) error {
		return orchestrator.ExecuteIntent(cCtx, job.ConversationID, job.Intent)
	}
	cronMgr := cron.NewManager(database, cronRunner)
	orchestrator.SetCronManager(cronMgr)
	if err := cronMgr.LoadJobs(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to restore cron jobs from database")
	}
	cronMgr.Start()
	defer cronMgr.Stop()

	// Hook WebSocket message handler to LLM Orchestrator
	hub.SetHandler(func(ctx context.Context, client *ws.Client, msg ws.InboundMessage) {
		_ = orchestrator.HandleChatMessage(ctx, client, msg)
	})

	// Subflow runner closure for parallel sub-agents
	flowRunner := func(tCtx context.Context, task flow.Task) (string, error) {
		log.Info().Str("task_id", task.ID).Str("title", task.Title).Msg("Executing parallel sub-agent task")
		// Simulate / execute specialized sub-agent instruction
		return fmt.Sprintf("Completed sub-task [%s]: %s (Instruction: %s)", task.ID, task.Title, task.Instruction), nil
	}
	flow.RegisterFlowTools(toolsReg, flowEngine, flowRunner)

	// 10. Extract Embedded SPA Filesystem
	spaFS, err := fs.Sub(embeddedWeb, "web/dist")
	if err != nil {
		log.Warn().Err(err).Msg("Could not extract embedded web/dist filesystem")
	}

	// 11. Create and Start HTTP Server
	srv := server.New(cfg, hub, server.ServerOptions{
		DB:           database,
		MemoryMgr:    memMgr,
		SkillsLoader: skillsLoader,
		ToolsReg:     toolsReg,
		MCPMgr:       mcpMgr,
		CronMgr:      cronMgr,
		SpaFS:        spaFS,
	})

	// Graceful Shutdown Listener
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil {
			log.Info().Err(err).Msg("HTTP server stopped")
		}
	}()

	log.Info().
		Int("port", cfg.Port).
		Str("url", fmt.Sprintf("http://localhost:%d", cfg.Port)).
		Msg("🚀 Go Agent Harness server is ready!")

	<-stopChan
	log.Info().Msg("Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Error during server shutdown")
	}

	log.Info().Msg("Shutdown complete. Goodbye!")
}
