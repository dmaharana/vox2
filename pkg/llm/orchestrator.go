package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"go-harness/pkg/config"
	"go-harness/pkg/db"
	"go-harness/pkg/memory"
	"go-harness/pkg/skills"
	"go-harness/pkg/tools"
	"go-harness/pkg/tracing"
	"go-harness/pkg/ws"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Orchestrator coordinates LLM streaming, memory retrieval, skills injection, tool execution, and client messaging.
type Orchestrator struct {
	cfg          *config.Config
	client       *Client
	db           *db.DB
	memoryMgr    *memory.Manager
	skillsLoader *skills.Loader
	toolsReg     *tools.Registry
	hub          *ws.Hub
}

// NewOrchestrator creates a new Orchestrator instance.
func NewOrchestrator(
	cfg *config.Config,
	database *db.DB,
	memMgr *memory.Manager,
	skillsLoader *skills.Loader,
	toolsReg *tools.Registry,
	hub *ws.Hub,
) *Orchestrator {
	return &Orchestrator{
		cfg:          cfg,
		client:       NewClient(cfg),
		db:           database,
		memoryMgr:    memMgr,
		skillsLoader: skillsLoader,
		toolsReg:     toolsReg,
		hub:          hub,
	}
}

// HandleChatMessage processes an incoming chat message from a WebSocket client.
func (o *Orchestrator) HandleChatMessage(parentCtx context.Context, wsClient *ws.Client, msg ws.InboundMessage) {
	convID := msg.ConversationID
	if convID == "" {
		convID = uuid.New().String()
	}

	// Setup cancellable context
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	if wsClient != nil {
		wsClient.RegisterCancel(convID, cancel)
		defer wsClient.RemoveCancel(convID)
	}

	// Tracing parent span for chat turn
	ctx, span := tracing.StartSpan(ctx, "agent.turn",
		trace.WithAttributes(
			attribute.String("conversation.id", convID),
			attribute.String("user.message", msg.Content),
		),
	)
	defer span.End()

	// 1. Save user message to database
	if o.db != nil && msg.Content != "" {
		_ = o.db.AddMessage(db.Message{
			ConversationID: convID,
			Role:           "user",
			Content:        msg.Content,
		})
	}

	// 2. Intercept Slash Commands (e.g., /help, /skills, /skill <name>, /skill:<name>, /<skill_name>)
	trimmedContent := strings.TrimSpace(msg.Content)
	var directSkillSection string

	if strings.HasPrefix(trimmedContent, "/") {
		parts := strings.Fields(trimmedContent)
		cmd := strings.TrimPrefix(parts[0], "/")
		promptArg := strings.TrimSpace(strings.TrimPrefix(trimmedContent, parts[0]))

		switch {
		case strings.EqualFold(cmd, "help"):
			helpText := "### ⚡ Slash Commands Reference\n\n" +
				"- `/<skill_name> [prompt]` or `/skill <name> [prompt]` or `/skill:<name> [prompt]` — Directly invoke a skill with its runbook.\n" +
				"- `/<tool_name> [args]` or `/tool <name> [args]` — Directly trigger a specific tool.\n" +
				"- `/skills` — List all discovered skills and their statuses.\n" +
				"- `/tools` — List all registered tools and their categories.\n" +
				"- `/help` — Show this slash command reference.\n"
			o.sendDirectResponse(convID, wsClient, helpText)
			return

		case strings.EqualFold(cmd, "skills"):
			var sb strings.Builder
			sb.WriteString("### 🛠️ Loaded Skills\n\n")
			if o.skillsLoader == nil || len(o.skillsLoader.List()) == 0 {
				sb.WriteString("No skills currently discovered in configured skill locations.\n")
			} else {
				sb.WriteString("| Skill Name | Status | Description |\n|---|:---:|---|\n")
				for _, s := range o.skillsLoader.List() {
					status := "🟢 Active"
					if !s.Enabled {
						status = "🔴 Disabled"
					}
					sb.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", s.Name, status, s.Description))
				}
				sb.WriteString("\n*Type `/<skill_name> [prompt]` or `/skill:<name>` to invoke a skill directly, or use the `read_skill` tool.*")
			}
			o.sendDirectResponse(convID, wsClient, sb.String())
			return

		case strings.EqualFold(cmd, "tools"):
			var sb strings.Builder
			sb.WriteString("### 🔧 Registered Agent Tools\n\n")
			if o.toolsReg == nil || len(o.toolsReg.List()) == 0 {
				sb.WriteString("No tools currently registered in the registry.\n")
			} else {
				sb.WriteString("| Tool Name | Category | Status | Description |\n|---|:---:|:---:|---|\n")
				for _, t := range o.toolsReg.List() {
					status := "🟢 Active"
					if !t.Enabled {
						status = "🔴 Disabled"
					}
					sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n", t.Name, t.Category, status, t.Description))
				}
				sb.WriteString("\n*Type `/<tool_name> [args]` to trigger a tool directly.*")
			}
			o.sendDirectResponse(convID, wsClient, sb.String())
			return

		case strings.EqualFold(cmd, "skill") || strings.HasPrefix(strings.ToLower(cmd), "skill:"):
			var skillName string
			if strings.HasPrefix(strings.ToLower(cmd), "skill:") {
				skillName = strings.TrimPrefix(cmd, "skill:")
			} else if len(parts) >= 2 {
				skillName = parts[1]
				promptArg = strings.TrimSpace(strings.TrimPrefix(trimmedContent, parts[0]+" "+parts[1]))
			}

			if skillName != "" {
				skill, ok := o.findSkill(skillName)
				if !ok {
					o.sendDirectResponse(convID, wsClient, fmt.Sprintf("❌ Skill `%s` not found. Type `/skills` to see available skills.", skillName))
					return
				}
				if !skill.Enabled {
					o.sendDirectResponse(convID, wsClient, fmt.Sprintf("⚠️ Skill `%s` is currently disabled. Enable it in the Tools & Skills modal first.", skill.Name))
					return
				}
				promptSuffix := ""
				if promptArg != "" {
					promptSuffix = fmt.Sprintf("\n\n**User Task:** %s", promptArg)
				}
				directSkillSection = fmt.Sprintf("\n## Direct Invocation: Skill '%s'\n**Description:** %s\n\n%s%s\n", skill.Name, skill.Description, skill.Content, promptSuffix)
			} else {
				o.sendDirectResponse(convID, wsClient, "Usage: `/skill <skill_name> [prompt...]` or `/skill:<name> [prompt...]` (e.g. `/skill:code-review Check this function`)")
				return
			}

		case strings.EqualFold(cmd, "tool"):
			if len(parts) >= 2 {
				toolName := parts[1]
				t, ok := o.toolsReg.Get(toolName)
				if !ok {
					o.sendDirectResponse(convID, wsClient, fmt.Sprintf("❌ Tool `%s` not found. Type `/tools` to see available tools.", toolName))
					return
				}
				if !t.Enabled {
					o.sendDirectResponse(convID, wsClient, fmt.Sprintf("⚠️ Tool `%s` is currently disabled. Enable it in the Tools & Skills modal first.", t.Name))
					return
				}
				directSkillSection = fmt.Sprintf("\n## Direct Tool Invocation: '%s'\n**Directive:** The user explicitly triggered the `%s` tool (%s). Prioritize executing the `%s` tool to fulfill this turn's request.\n", t.Name, t.Name, t.Description, t.Name)
			} else {
				o.sendDirectResponse(convID, wsClient, "Usage: `/tool <tool_name> [args...]` (e.g. `/tool read_file main.go`)")
				return
			}

		default:
			// Check if the command matches any skill name directly (e.g. /code-review ...)
			if skill, ok := o.findSkill(cmd); ok {
				if !skill.Enabled {
					o.sendDirectResponse(convID, wsClient, fmt.Sprintf("⚠️ Skill `%s` is currently disabled. Enable it in the Tools & Skills modal first.", skill.Name))
					return
				}
				promptSuffix := ""
				if promptArg != "" {
					promptSuffix = fmt.Sprintf("\n\n**User Task:** %s", promptArg)
				}
				directSkillSection = fmt.Sprintf("\n## Direct Invocation: Skill '%s'\n**Description:** %s\n\n%s%s\n", skill.Name, skill.Description, skill.Content, promptSuffix)
			} else if t, ok := o.toolsReg.Get(cmd); ok {
				// Check if command matches a tool directly (e.g. /read_file ...)
				if !t.Enabled {
					o.sendDirectResponse(convID, wsClient, fmt.Sprintf("⚠️ Tool `%s` is currently disabled. Enable it in the Tools & Skills modal first.", t.Name))
					return
				}
				directSkillSection = fmt.Sprintf("\n## Direct Tool Invocation: '%s'\n**Directive:** The user explicitly triggered the `%s` tool (%s). Prioritize executing the `%s` tool to fulfill this turn's request.\n", t.Name, t.Name, t.Description, t.Name)
			} else {
				o.sendDirectResponse(convID, wsClient, fmt.Sprintf("❓ Unknown command `/%s`. Type `/help`, `/skills`, or `/tools` to see available commands.", cmd))
				return
			}
		}
	}

	// 3. Query relevant memory context (FTS5 search)
	var memorySection string
	var retrievedMemories []memory.MemoryItem
	if o.memoryMgr != nil && msg.Content != "" {
		memories, err := o.memoryMgr.Search(msg.Content, "", "", 5)
		if err == nil && len(memories) > 0 {
			retrievedMemories = memories
			var sb strings.Builder
			sb.WriteString("\n## Relevant Memories & Context\n")
			for _, m := range memories {
				sb.WriteString(fmt.Sprintf("- **[%s] %s**: %s\n", m.MemoryType, m.Key, m.Content))
			}
			memorySection = sb.String()

			if wsClient != nil {
				wsClient.Send(ws.OutboundMessage{
					Type:           ws.TypeMemoryEvent,
					ConversationID: convID,
					Payload: ws.MemoryPayload{
						Action: "retrieved",
						Query:  msg.Content,
						Count:  len(memories),
						Items:  memories,
					},
				})
			}
		}
	}

	// 4. Build system prompt (base + skills + direct skill invocation + memories)
	var skillsSection string
	if o.skillsLoader != nil {
		skillsSection = o.skillsLoader.BuildPromptSection()
	}

	systemPrompt := fmt.Sprintf(
		"You are Vox2, an advanced, intelligent AI engineering assistant and agent harness.\n"+
			"Follow user instructions thoroughly. Use the provided tools when appropriate to read/write/edit files, execute local scripts, execute parallel subflows, search/save memory, or interact with MCP servers.\n"+
			"Always be accurate, direct, and helpful. Format your responses in clean Markdown.\n\n"+
			"FILE EDITING & MUTATION RULES:\n"+
			"1. Use the `edit` (or `edit_file`) tool for precise targeted modifications in existing files. `oldText` must match a unique block of text in the target file.\n"+
			"2. When changing multiple separate locations in one file, use a single `edit` call with multiple items in `edits: [{oldText, newText}]` rather than making sequential single edits.\n"+
			"3. Keep `oldText` as small as possible while remaining unique. Do not include large unchanged regions just to pad context.\n"+
			"4. Use `write_file` when creating new files or completely rewriting entire file contents from scratch.\n\n"+
			"LOCAL SCRIPT EXECUTION RULES:\n"+
			"1. When data analysis, calculation, script execution, or code verification is requested or needed, use the `execute_script` tool.\n"+
			"2. You can generate and execute Python (`language: 'python'`), Node.js/JavaScript (`language: 'javascript'`), PowerShell (`language: 'powershell'`), Windows Batch/CMD (`language: 'bat'`), or Bash (`language: 'bash'`) scripts.\n"+
			"3. Always check the tool's returned `stdout`, `stderr`, and `exit_code` to interpret the script output accurately.\n\n"+
			"COGNITIVE MEMORY & SEARCH CACHE RULES:\n"+
			"1. Actively identify important user preferences, developer conventions, project architecture rules, tech stack facts, or reusable procedures shared during the conversation.\n"+
			"2. When the user explicitly asks you to remember something (e.g. 'remember that ...', 'my preference is ...', 'save this ...'), or when you discover crucial project facts, synthesis from key searches, or instructions that should persist across sessions, proactively invoke the `save_memory` tool.\n"+
			"3. External search tools (e.g. web search, file search, scrapers) are automatically indexed into the short-term `semantic_cache` tier. For important research discoveries, explicitly call `save_memory` with `memory_type: 'semantic'` or `'procedural'` to persist the high-level conclusions into long-term memory.\n"+
			"4. Use `search_memory` if you need to query past memories, cached search outcomes, or context beyond what was automatically injected.\n\n"+
			"CITATION & SOURCE REFERENCE RULES:\n"+
			"1. Whenever you reference, explain, or extract code/data from a file, MCP tool, memory item, or URL, include an inline clickable markdown link pointing directly to the specific source (e.g. `[filename.go](file:///path/to/filename.go#L10-L25)` or `[ToolName](tool://tool_name)` or `[API URL](http://...)`).\n"+
			"2. At the end of every response where external files, tools, skills, or documentation are used, conclude with a structured `### 📚 References` section listing all referenced sources with clickable links and brief 1-line context.\n\n"+
			"%s\n%s\n%s",
		skillsSection,
		directSkillSection,
		memorySection,
	)

	// 4. Assemble message history from SQLite DB
	var openAIMessages []openai.ChatCompletionMessage
	openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})

	if o.db != nil {
		history, err := o.db.GetMessages(convID)
		if err == nil {
			for _, m := range history {
				switch m.Role {
				case "user":
					openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: m.Content,
					})
				case "assistant":
					openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleAssistant,
						Content: m.Content,
					})
				case "tool":
					openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleTool,
						Content: m.Content,
					})
				}
			}
		}
	}

	// 5. Multi-turn Agent loop
	maxTurns := 10
	currentTurn := 0
	var finalAssistantText strings.Builder
	var turnToolCallStates []ws.ToolCallPayload
	var turnSubflowStates []ws.SubflowPayload

	for currentTurn < maxTurns {
		currentTurn++

		select {
		case <-ctx.Done():
			log.Warn().Str("conversation_id", convID).Msg("Chat execution cancelled by user")
			if wsClient != nil {
				wsClient.Send(ws.OutboundMessage{
					Type:           ws.TypeError,
					ConversationID: convID,
					Error:          "Execution cancelled by user",
				})
			}
			return
		default:
		}

		availableTools := o.toolsReg.ToOpenAITools()

		stream, err := o.client.StreamChat(ctx, openAIMessages, availableTools)
		if err != nil {
			log.Error().Err(err).Msg("Failed to start chat completion stream")
			if wsClient != nil {
				wsClient.Send(ws.OutboundMessage{
					Type:           ws.TypeError,
					ConversationID: convID,
					Error:          err.Error(),
				})
			}
			return
		}

		var turnText strings.Builder
		toolCallMap := make(map[int]*openai.ToolCall)

		// Read tokens
		for {
			delta, readErr := ReadStreamChunk(stream, toolCallMap)
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					break
				}
				log.Warn().Err(readErr).Msg("Error reading stream chunk")
				break
			}

			if delta.Content != "" {
				turnText.WriteString(delta.Content)
				finalAssistantText.WriteString(delta.Content)

				if wsClient != nil {
					wsClient.Send(ws.OutboundMessage{
						Type:           ws.TypeToken,
						ConversationID: convID,
						Payload: ws.TokenPayload{
							Delta:    delta.Content,
							FullText: finalAssistantText.String(),
						},
					})
				}
			}
		}
		stream.Close()

		// Check if tools were called
		if len(toolCallMap) == 0 {
			// No tool calls - final turn finished
			break
		}

		// Collect tool calls
		var toolCalls []openai.ToolCall
		for i := 0; i < len(toolCallMap); i++ {
			if tc, ok := toolCallMap[i]; ok {
				toolCalls = append(toolCalls, *tc)
			}
		}

		// Append assistant message with tool calls to context
		assistantMsg := openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   turnText.String(),
			ToolCalls: toolCalls,
		}
		openAIMessages = append(openAIMessages, assistantMsg)

		// Execute each tool call
		for _, tc := range toolCalls {
			toolCallID := tc.ID
			toolName := tc.Function.Name
			argsJSON := json.RawMessage(tc.Function.Arguments)

			if wsClient != nil {
				wsClient.Send(ws.OutboundMessage{
					Type:           ws.TypeToolCall,
					ConversationID: convID,
					Payload: ws.ToolCallPayload{
						ID:        toolCallID,
						Tool:      toolName,
						Arguments: string(argsJSON),
						Status:    "started",
					},
				})
			}

			// Execute tool
			execRes, execErr := o.toolsReg.Execute(ctx, toolName, argsJSON)

			var resStr string
			if execErr != nil {
				resStr = fmt.Sprintf("Error executing tool %s: %v", toolName, execErr)
				turnToolCallStates = append(turnToolCallStates, ws.ToolCallPayload{
					ID:        toolCallID,
					Tool:      toolName,
					Arguments: string(argsJSON),
					Status:    "failed",
					Error:     execErr.Error(),
				})
				if wsClient != nil {
					wsClient.Send(ws.OutboundMessage{
						Type:           ws.TypeToolCall,
						ConversationID: convID,
						Payload: ws.ToolCallPayload{
							ID:     toolCallID,
							Tool:   toolName,
							Status: "failed",
							Error:  execErr.Error(),
						},
					})
				}
			} else {
				resBytes, _ := json.Marshal(execRes)
				resStr = string(resBytes)
				turnToolCallStates = append(turnToolCallStates, ws.ToolCallPayload{
					ID:        toolCallID,
					Tool:      toolName,
					Arguments: string(argsJSON),
					Status:    "completed",
					Result:    execRes,
				})
				if wsClient != nil {
					wsClient.Send(ws.OutboundMessage{
						Type:           ws.TypeToolCall,
						ConversationID: convID,
						Payload: ws.ToolCallPayload{
							ID:     toolCallID,
							Tool:   toolName,
							Status: "completed",
							Result: execRes,
						},
					})

					if toolName == "save_memory" {
						wsClient.Send(ws.OutboundMessage{
							Type:           ws.TypeMemoryEvent,
							ConversationID: convID,
							Payload: ws.MemoryPayload{
								Action: "saved",
								Count:  1,
							},
						})
					}
				}

				// Auto-save key searches to semantic cache tier
				if o.memoryMgr != nil && execErr == nil && toolName != "search_memory" &&
					(strings.Contains(strings.ToLower(toolName), "search") || strings.Contains(strings.ToLower(toolName), "scraper") || strings.Contains(strings.ToLower(toolName), "tavily")) {
					var queryKey string
					var inArgs map[string]any
					if err := json.Unmarshal(argsJSON, &inArgs); err == nil {
						if q, ok := inArgs["query"].(string); ok && q != "" {
							queryKey = q
						} else if q, ok := inArgs["pattern"].(string); ok && q != "" {
							queryKey = q
						} else if q, ok := inArgs["url"].(string); ok && q != "" {
							queryKey = q
						}
					}
					if queryKey == "" {
						queryKey = toolName
					}
					contentSummary := resStr
					if len(contentSummary) > 800 {
						contentSummary = contentSummary[:800] + "..."
					}
					_ = o.memoryMgr.Save(&memory.MemoryItem{
						Key:        fmt.Sprintf("Search: %s", queryKey),
						Content:    contentSummary,
						MemoryType: memory.TypeSemanticCache,
						Tier:       memory.TierShortTerm,
						Tags:       fmt.Sprintf("search,%s", toolName),
					})
				}
			}

			// Append tool response message to history
			openAIMessages = append(openAIMessages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    resStr,
				ToolCallID: toolCallID,
			})
		}
	}

	// 6. Save final assistant response to DB with all traces, tool calls, and subflows
	if o.db != nil && (finalAssistantText.Len() > 0 || len(turnToolCallStates) > 0) {
		var toolCallsJSON, memoriesJSON, subflowsJSON string
		if len(turnToolCallStates) > 0 {
			if b, err := json.Marshal(turnToolCallStates); err == nil {
				toolCallsJSON = string(b)
			}
		}
		if len(retrievedMemories) > 0 {
			if b, err := json.Marshal(retrievedMemories); err == nil {
				memoriesJSON = string(b)
			}
		}
		if len(turnSubflowStates) > 0 {
			if b, err := json.Marshal(turnSubflowStates); err == nil {
				subflowsJSON = string(b)
			}
		}

		traceID := ""
		if span.SpanContext().HasTraceID() {
			traceID = span.SpanContext().TraceID().String()
		}

		_ = o.db.AddMessage(db.Message{
			ConversationID:    convID,
			Role:              "assistant",
			Content:           finalAssistantText.String(),
			ToolCalls:         toolCallsJSON,
			Subflows:          subflowsJSON,
			MemoriesRetrieved: memoriesJSON,
			TraceID:           traceID,
		})
	}

	// 7. Emit Done event
	if wsClient != nil {
		wsClient.Send(ws.OutboundMessage{
			Type:           ws.TypeDone,
			ConversationID: convID,
			Payload: map[string]any{
				"conversation_id": convID,
				"content":         finalAssistantText.String(),
				"disclaimer":      "AI can make mistakes, so double-check responses",
			},
		})
	}
}

func (o *Orchestrator) findSkill(name string) (*skills.Skill, bool) {
	if o.skillsLoader == nil {
		return nil, false
	}
	s, ok := o.skillsLoader.Get(name)
	if ok {
		return s, true
	}
	// Case-insensitive lookup
	cleanName := strings.ToLower(strings.TrimSpace(name))
	for _, item := range o.skillsLoader.List() {
		if strings.ToLower(item.Name) == cleanName {
			sCopy := item
			return &sCopy, true
		}
	}
	return nil, false
}

func (o *Orchestrator) sendDirectResponse(convID string, wsClient *ws.Client, content string) {
	if wsClient != nil {
		wsClient.Send(ws.OutboundMessage{
			Type:           ws.TypeToken,
			ConversationID: convID,
			Payload: ws.TokenPayload{
				Delta:    content,
				FullText: content,
			},
		})
	}

	if o.db != nil {
		_ = o.db.AddMessage(db.Message{
			ConversationID: convID,
			Role:           "assistant",
			Content:        content,
		})
	}

	if wsClient != nil {
		wsClient.Send(ws.OutboundMessage{
			Type:           ws.TypeDone,
			ConversationID: convID,
			Payload: map[string]any{
				"conversation_id": convID,
				"content":         content,
				"disclaimer":      "AI can make mistakes, so double-check responses",
			},
		})
	}
}
