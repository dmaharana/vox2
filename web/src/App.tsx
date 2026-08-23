import React, { useState, useEffect, useRef } from 'react'
import { ChatMessage, ToolCallState, SubflowState, MemoryItem } from './types'
import { fetchConversation } from './lib/api'
import { ChatMessageList } from './components/ChatMessageList'
import { SettingsModal } from './components/SettingsModal'
import { MCPServersModal } from './components/MCPServersModal'
import { ToolsSkillsModal } from './components/ToolsSkillsModal'
import { MemoryModal } from './components/MemoryModal'
import { ConversationsDrawer } from './components/ConversationsDrawer'
import { Button } from './components/ui/button'
import { Textarea } from './components/ui/textarea'
import { Badge } from './components/ui/badge'
import {
  Sliders,
  Server,
  Wrench,
  Brain,
  MessageSquare,
  Send,
  Square,
  Plus,
  Terminal,
  Wifi,
  WifiOff,
} from 'lucide-react'

export function App() {
  const [conversationId, setConversationId] = useState<string>(() => {
    return 'conv-' + Math.random().toString(36).substring(2, 9)
  })
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [streaming, setStreaming] = useState(false)
  const [wsConnected, setWsConnected] = useState(false)

  // Modals state
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [mcpOpen, setMcpOpen] = useState(false)
  const [toolsOpen, setToolsOpen] = useState(false)
  const [memoryOpen, setMemoryOpen] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(false)

  const wsRef = useRef<WebSocket | null>(null)
  const messagesEndRef = useRef<HTMLDivElement | null>(null)

  // Connect WebSocket
  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host || 'localhost:8080'
    const wsUrl = `${protocol}//${host}/ws`

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setWsConnected(true)
    }

    ws.onclose = () => {
      setWsConnected(false)
    }

    ws.onerror = (e) => {
      console.error('WebSocket error:', e)
      setWsConnected(false)
    }

    ws.onmessage = (evt) => {
      try {
        const msg = JSON.parse(evt.data)
        handleWebSocketMessage(msg)
      } catch (err) {
        console.error('Failed to parse WebSocket message:', err)
      }
    }

    return () => {
      ws.close()
    }
  }, [])

  const handleWebSocketMessage = (msg: any) => {
    switch (msg.type) {
      case 'token': {
        const payload = msg.payload
        setMessages((prev) => {
          const last = prev[prev.length - 1]
          if (last && last.role === 'assistant') {
            return [
              ...prev.slice(0, -1),
              {
                ...last,
                content: payload.full_text || last.content + payload.delta,
              },
            ]
          } else {
            return [
              ...prev,
              {
                id: 'msg-' + Date.now(),
                role: 'assistant',
                content: payload.delta,
                timestamp: new Date().toISOString(),
              },
            ]
          }
        })
        break
      }

      case 'tool_call': {
        const tc: ToolCallState = msg.payload
        setMessages((prev) => {
          const last = prev[prev.length - 1]
          if (last && last.role === 'assistant') {
            const existingCalls = last.toolCalls || []
            const idx = existingCalls.findIndex((c) => c.id === tc.id)
            let updatedCalls = [...existingCalls]
            if (idx >= 0) {
              updatedCalls[idx] = { ...updatedCalls[idx], ...tc }
            } else {
              updatedCalls.push(tc)
            }
            return [
              ...prev.slice(0, -1),
              { ...last, toolCalls: updatedCalls },
            ]
          } else {
            return [
              ...prev,
              {
                id: 'msg-' + Date.now(),
                role: 'assistant',
                content: '',
                toolCalls: [tc],
                timestamp: new Date().toISOString(),
              },
            ]
          }
        })
        break
      }

      case 'subflow_event': {
        const sf: SubflowState = msg.payload
        setMessages((prev) => {
          const last = prev[prev.length - 1]
          if (last && last.role === 'assistant') {
            const existingSubflows = last.subflows || []
            const idx = existingSubflows.findIndex(
              (s) => s.task_name === sf.task_name && s.flow_id === sf.flow_id
            )
            let updated = [...existingSubflows]
            if (idx >= 0) {
              updated[idx] = { ...updated[idx], ...sf }
            } else {
              updated.push(sf)
            }
            return [...prev.slice(0, -1), { ...last, subflows: updated }]
          }
          return prev
        })
        break
      }

      case 'memory_event': {
        const memPayload = msg.payload
        if (memPayload.action === 'retrieved' && Array.isArray(memPayload.items)) {
          setMessages((prev) => {
            const last = prev[prev.length - 1]
            if (last && last.role === 'assistant') {
              return [
                ...prev.slice(0, -1),
                { ...last, memoriesRetrieved: memPayload.items as MemoryItem[] },
              ]
            }
            return prev
          })
        }
        break
      }

      case 'done': {
        setStreaming(false)
        break
      }

      case 'error': {
        setStreaming(false)
        setMessages((prev) => [
          ...prev,
          {
            id: 'msg-' + Date.now(),
            role: 'assistant',
            content: `**Error:** ${msg.error || 'Unknown error occurred'}`,
            timestamp: new Date().toISOString(),
          },
        ])
        break
      }
    }
  }

  // Scroll on message change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streaming])

  const handleSend = () => {
    if (!input.trim() || streaming) return

    const userMessageText = input.trim()
    setInput('')

    // Append user message
    const userMsg: ChatMessage = {
      id: 'msg-' + Date.now(),
      role: 'user',
      content: userMessageText,
      timestamp: new Date().toISOString(),
    }

    setMessages((prev) => [...prev, userMsg])
    setStreaming(true)

    // Send over WebSocket
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          type: 'chat_message',
          conversation_id: conversationId,
          content: userMessageText,
        })
      )
    } else {
      setStreaming(false)
      alert('WebSocket is disconnected. Reconnecting...')
    }
  }

  const handleCancel = () => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          type: 'cancel',
          conversation_id: conversationId,
        })
      )
    }
    setStreaming(false)
  }

  const handleNewChat = () => {
    setConversationId('conv-' + Math.random().toString(36).substring(2, 9))
    setMessages([])
    setStreaming(false)
  }

  const handleSelectConversation = async (id: string) => {
    try {
      const data = await fetchConversation(id)
      setConversationId(data.conversation.id)
      const mapped: ChatMessage[] = (data.messages || []).map((m: any) => ({
        id: m.id,
        role: m.role as any,
        content: m.content,
        timestamp: m.created_at,
      }))
      setMessages(mapped)
    } catch (e) {
      console.error(e)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div className="flex flex-col h-screen bg-background text-foreground">
      {/* Top Navigation Bar */}
      <header className="h-14 border-b px-4 flex items-center justify-between shrink-0 bg-card/60 backdrop-blur-md sticky top-0 z-10">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 font-bold tracking-tight text-lg">
            <div className="w-8 h-8 rounded-lg bg-primary/10 border border-primary/20 flex items-center justify-center text-primary">
              <Terminal className="w-4 h-4" />
            </div>
            <span>Go Agent Harness</span>
          </div>

          <Badge
            variant={wsConnected ? 'outline' : 'destructive'}
            className="gap-1 text-xs py-0.5"
          >
            {wsConnected ? (
              <>
                <Wifi className="w-3 h-3 text-emerald-500" />
                <span className="text-muted-foreground">Connected</span>
              </>
            ) : (
              <>
                <WifiOff className="w-3 h-3" />
                <span>Offline</span>
              </>
            )}
          </Badge>
        </div>

        {/* Menu Buttons */}
        <div className="flex items-center gap-1.5">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setSettingsOpen(true)}
            className="gap-1.5 text-xs font-medium"
          >
            <Sliders className="w-3.5 h-3.5 text-primary" /> LLM Settings
          </Button>

          <Button
            variant="ghost"
            size="sm"
            onClick={() => setMcpOpen(true)}
            className="gap-1.5 text-xs font-medium"
          >
            <Server className="w-3.5 h-3.5 text-primary" /> MCP Servers
          </Button>

          <Button
            variant="ghost"
            size="sm"
            onClick={() => setToolsOpen(true)}
            className="gap-1.5 text-xs font-medium"
          >
            <Wrench className="w-3.5 h-3.5 text-primary" /> Tools & Skills
          </Button>

          <Button
            variant="ghost"
            size="sm"
            onClick={() => setMemoryOpen(true)}
            className="gap-1.5 text-xs font-medium"
          >
            <Brain className="w-3.5 h-3.5 text-primary" /> Memory
          </Button>

          <Button
            variant="ghost"
            size="sm"
            onClick={() => setHistoryOpen(true)}
            className="gap-1.5 text-xs font-medium"
          >
            <MessageSquare className="w-3.5 h-3.5 text-primary" /> History
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={handleNewChat}
            className="gap-1.5 text-xs font-medium ml-1"
          >
            <Plus className="w-3.5 h-3.5" /> New Chat
          </Button>
        </div>
      </header>

      {/* Main Chat Stream Area */}
      <main className="flex-1 overflow-y-auto px-4 md:px-8 py-6 max-w-4xl w-full mx-auto">
        {messages.length === 0 ? (
          <div className="h-full flex flex-col items-center justify-center text-center space-y-4 py-16 text-muted-foreground">
            <div className="w-14 h-14 rounded-2xl bg-primary/10 border border-primary/20 flex items-center justify-center text-primary shadow-xs">
              <Terminal className="w-7 h-7" />
            </div>
            <div className="space-y-1">
              <h2 className="text-lg font-bold text-foreground">Welcome to Go Agent Harness</h2>
              <p className="text-xs max-w-md">
                Powered by OpenAI-compatible LLMs, dynamic filesystem tools, official MCP servers, cognitive FTS5 memories, and parallel subflows.
              </p>
            </div>
            <div className="flex flex-wrap gap-2 justify-center max-w-lg pt-2">
              <button
                type="button"
                onClick={() => setInput('Read the files in this directory and summarize the codebase structure')}
                className="text-xs p-2.5 rounded-lg border bg-card/60 hover:bg-card hover:border-primary/50 transition-all text-left"
              >
                📁 Read the files in this directory and summarize structure
              </button>
              <button
                type="button"
                onClick={() => setInput('Run a parallel flow to investigate both code review and weather')}
                className="text-xs p-2.5 rounded-lg border bg-card/60 hover:bg-card hover:border-primary/50 transition-all text-left"
              >
                ⚡ Run a parallel flow to investigate independent tasks
              </button>
            </div>
          </div>
        ) : (
          <ChatMessageList messages={messages} streaming={streaming} />
        )}
        <div ref={messagesEndRef} />
      </main>

      {/* Bottom Message Input & Disclaimer Area */}
      <footer className="border-t bg-card/50 px-4 py-3 shrink-0">
        <div className="max-w-4xl mx-auto space-y-2">
          <div className="flex items-end gap-2 relative">
            <Textarea
              placeholder="Ask anything, execute tools, or trigger parallel subflows..."
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              rows={2}
              className="resize-none pr-12 min-h-[56px] max-h-[160px] text-sm"
            />
            {streaming ? (
              <Button
                variant="destructive"
                size="icon"
                className="h-10 w-10 shrink-0"
                onClick={handleCancel}
                title="Stop generation"
              >
                <Square className="w-4 h-4 fill-current" />
              </Button>
            ) : (
              <Button
                size="icon"
                className="h-10 w-10 shrink-0"
                onClick={handleSend}
                disabled={!input.trim() || !wsConnected}
                title="Send message (Enter)"
              >
                <Send className="w-4 h-4" />
              </Button>
            )}
          </div>

          {/* Required Disclaimer */}
          <div className="text-center text-[11px] text-muted-foreground font-medium">
            AI can make mistakes, so double-check responses
          </div>
        </div>
      </footer>

      {/* Modals & Drawers */}
      <SettingsModal open={settingsOpen} onOpenChange={setSettingsOpen} />
      <MCPServersModal open={mcpOpen} onOpenChange={setMcpOpen} />
      <ToolsSkillsModal open={toolsOpen} onOpenChange={setToolsOpen} />
      <MemoryModal open={memoryOpen} onOpenChange={setMemoryOpen} />
      <ConversationsDrawer
        open={historyOpen}
        onOpenChange={setHistoryOpen}
        currentConversationId={conversationId}
        onSelectConversation={handleSelectConversation}
        onNewChat={handleNewChat}
      />
    </div>
  )
}

export default App
