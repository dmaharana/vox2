export interface ToolDefinition {
  name: string
  description: string
  category: string
  enabled: boolean
  parameters?: any
}

export interface SkillFile {
  path: string
  category: 'script' | 'reference' | 'template' | 'asset' | 'other'
  size_bytes: number
}

export interface Skill {
  name: string
  description: string
  path: string
  directory?: string
  files?: SkillFile[]
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
  auth_type?: 'none' | 'headers' | 'oauth2' | 'oauth2_google'
  oauth_client_id?: string
  oauth_client_secret?: string
  has_oauth_client_secret?: boolean
  oauth_token_url?: string
  oauth_scopes?: string
  oauth_access_token?: string
  has_oauth_access_token?: boolean
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
  traceId?: string
  timestamp: string
}

export interface Settings {
  llm_provider?: string
  llm_base_url: string
  llm_model: string
  has_api_key: boolean
  llm_auth_type?: string
  llm_oauth_client_id?: string
  has_oauth_client_secret?: boolean
  llm_oauth_token_url?: string
  llm_oauth_scopes?: string
  llm_temperature: number
  llm_max_tokens: number
  copilot_binary?: string
  copilot_timeout?: number
  copilot_installed?: boolean
  copilot_path?: string
  log_level: string
  log_format: string
  skills_dir: string
  otel_exporter: string
  otel_endpoint: string
}
