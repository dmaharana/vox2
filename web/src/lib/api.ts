import { Settings, ToolDefinition, Skill, MCPServer, MemoryItem, Conversation } from '../types'

const BASE_URL = '' // Relative for embedded SPA

export async function fetchSettings(): Promise<Settings> {
  const res = await fetch(`${BASE_URL}/api/settings`)
  if (!res.ok) throw new Error('Failed to fetch settings')
  return res.json()
}

export async function updateSettings(updates: Partial<Settings> & { llm_api_key?: string }): Promise<Settings> {
  const res = await fetch(`${BASE_URL}/api/settings`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  })
  if (!res.ok) throw new Error('Failed to update settings')
  return res.json()
}

export async function fetchTools(): Promise<ToolDefinition[]> {
  const res = await fetch(`${BASE_URL}/api/tools`)
  if (!res.ok) return []
  const data = await res.json()
  return Array.isArray(data) ? data : []
}

export async function toggleTool(name: string, enabled: boolean): Promise<void> {
  await fetch(`${BASE_URL}/api/tools/${encodeURIComponent(name)}/toggle`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
}

export async function fetchSkills(): Promise<Skill[]> {
  const res = await fetch(`${BASE_URL}/api/skills`)
  if (!res.ok) return []
  const data = await res.json()
  return Array.isArray(data) ? data : []
}

export async function refreshSkills(): Promise<Skill[]> {
  const res = await fetch(`${BASE_URL}/api/skills/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) throw new Error('Failed to refresh skills from disk')
  const data = await res.json()
  return Array.isArray(data) ? data : []
}

export async function toggleSkill(name: string, enabled: boolean): Promise<void> {
  await fetch(`${BASE_URL}/api/skills/${encodeURIComponent(name)}/toggle`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
}

export async function fetchMCPServers(): Promise<MCPServer[]> {
  const res = await fetch(`${BASE_URL}/api/mcp/servers`)
  if (!res.ok) return []
  const data = await res.json()
  return Array.isArray(data) ? data : []
}

export async function addMCPServer(server: Partial<MCPServer>): Promise<MCPServer> {
  const res = await fetch(`${BASE_URL}/api/mcp/servers`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(server),
  })
  if (!res.ok) {
    const err = await res.text()
    throw new Error(err || 'Failed to add MCP server')
  }
  return res.json()
}

export async function updateMCPServer(id: string, server: Partial<MCPServer>): Promise<void> {
  const res = await fetch(`${BASE_URL}/api/mcp/servers/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(server),
  })
  if (!res.ok) {
    const err = await res.text()
    throw new Error(err || 'Failed to update MCP server')
  }
}

export async function connectMCPServer(id: string): Promise<void> {
  const res = await fetch(`${BASE_URL}/api/mcp/servers/${id}/connect`, { method: 'POST' })
  if (!res.ok) {
    const err = await res.text()
    throw new Error(err || 'Failed to connect MCP server')
  }
}

export async function disconnectMCPServer(id: string): Promise<void> {
  await fetch(`${BASE_URL}/api/mcp/servers/${id}/disconnect`, { method: 'POST' })
}

export async function deleteMCPServer(id: string): Promise<void> {
  await fetch(`${BASE_URL}/api/mcp/servers/${id}`, { method: 'DELETE' })
}

export async function fetchConversations(): Promise<Conversation[]> {
  const res = await fetch(`${BASE_URL}/api/conversations`)
  if (!res.ok) return []
  const data = await res.json()
  return Array.isArray(data) ? data : []
}

export async function fetchConversation(id: string): Promise<{ conversation: Conversation; messages: any[] }> {
  const res = await fetch(`${BASE_URL}/api/conversations/${id}`)
  if (!res.ok) throw new Error('Conversation not found')
  return res.json()
}

export async function deleteConversation(id: string): Promise<void> {
  await fetch(`${BASE_URL}/api/conversations/${id}`, { method: 'DELETE' })
}

export function getExportCSVUrl(id: string): string {
  return `${BASE_URL}/api/conversations/${id}/export`
}

export async function searchMemories(query?: string, type?: string, tier?: string): Promise<MemoryItem[]> {
  const params = new URLSearchParams()
  if (query) params.set('q', query)
  if (type) params.set('type', type)
  if (tier) params.set('tier', tier)

  const res = await fetch(`${BASE_URL}/api/memories?${params.toString()}`)
  if (!res.ok) return []
  const data = await res.json()
  return Array.isArray(data) ? data : []
}

export async function saveMemory(item: Partial<MemoryItem>): Promise<MemoryItem> {
  const res = await fetch(`${BASE_URL}/api/memories`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(item),
  })
  if (!res.ok) throw new Error('Failed to save memory')
  return res.json()
}

export async function promoteMemories(): Promise<{ promoted_count: number }> {
  const res = await fetch(`${BASE_URL}/api/memories/promote`, { method: 'POST' })
  if (!res.ok) throw new Error('Failed to promote memories')
  return res.json()
}
