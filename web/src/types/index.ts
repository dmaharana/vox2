export interface ToolDefinition {
  name: string
  description: string
  category: string
  enabled: boolean
  parameters?: any
}

export interface Skill {
  name: string
  description: string
  path: string
  content: string
  enabled: boolean
}

export interface MCPServer {
  id: string
  name: string
  transport: 'stdio' | 'sse' | 'http'
  command?: string
  args?: string[]
  env?: Record<string, string>
  url?: string
  headers?: Record<string, string>
  enabled: boolean
  status: 'connected' | 'disconnected' | 'error'
  last_error?: string
}

export interface MemoryItem {
  id: string
  memory_type: 'working' | 'semantic_cache' | 'semantic' | 'episodic' | 'procedural'
  tier: 'short_term' | 'long_term'
  key: string
  content: string
  tags?: string
  access_count: number
  last_used_at: string
  created_at: string
}

export interface Conversation {
  id: string
  title: string
  created_at: string
  updated_at: string
}

export interface ToolCallState {
  id: string
  tool: string
  arguments: any
  status: 'started' | 'completed' | 'failed'
  result?: any
  error?: string
}

export interface SubflowState {
  flow_id: string
  task_index: number
  total_tasks: number
  task_name: string
  status: 'running' | 'completed' | 'failed'
  summary?: string
  duration_ms?: number
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  toolCalls?: ToolCallState[]
  subflows?: SubflowState[]
  memoriesRetrieved?: MemoryItem[]
  timestamp: string
}

export interface Settings {
  llm_base_url: string
  llm_model: string
  has_api_key: boolean
  llm_temperature: number
  llm_max_tokens: number
  log_level: string
  log_format: string
  skills_dir: string
  otel_exporter: string
  otel_endpoint: string
}
