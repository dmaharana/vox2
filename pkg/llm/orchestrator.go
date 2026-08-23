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

	// 2. Query relevant memory context (FTS5 search)
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

	// 3. Build system prompt (base + skills + memories)
	var skillsSection string
	if o.skillsLoader != nil {
		skillsSection = o.skillsLoader.BuildPromptSection()
	}

	systemPrompt := fmt.Sprintf(
		"You are Vox2, an advanced, intelligent AI engineering assistant and agent harness.\n"+
			"Follow user instructions thoroughly. Use the provided tools when appropriate to read/write/edit files, execute local scripts, execute parallel subflows, search/save memory, or interact with MCP servers.\n"+
			"Always be accurate, direct, and helpful. Format your responses in clean Markdown.\n\n"+
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
			"%s\n%s",
		skillsSection,
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
