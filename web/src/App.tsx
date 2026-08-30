import React, { useState, useEffect, useRef } from 'react'
import { ChatMessage, ToolCallState, SubflowState, MemoryItem, ToolDefinition, Skill } from './types'
import { fetchConversation, fetchTools, fetchSkills } from './lib/api'
import { ThemeProvider } from './lib/theme'
import { ChatMessageList } from './components/ChatMessageList'
import { SettingsModal } from './components/SettingsModal'
import { MCPServersModal } from './components/MCPServersModal'
import { ToolsSkillsModal } from './components/ToolsSkillsModal'
import { MemoryModal } from './components/MemoryModal'
import { ConversationsDrawer } from './components/ConversationsDrawer'
import { AppSidebar } from './components/AppSidebar'
import { VoxLogo } from './components/VoxLogo'
import { SlashAutocomplete, getMatchingSlashItems, SlashItem } from './components/SlashAutocomplete'
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
  PanelLeft,
  ArrowDown,
} from 'lucide-react'

export function AppContent() {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true)
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

  // Tools & Skills for Slash Command Autocomplete
  const [availableTools, setAvailableTools] = useState<ToolDefinition[]>([])
  const [availableSkills, setAvailableSkills] = useState<Skill[]>([])
  const [slashIndex, setSlashIndex] = useState(0)
  const [slashOpen, setSlashOpen] = useState(true)

  // Load tools & skills for autocomplete & registry
  useEffect(() => {
    fetchTools().then((t) => setAvailableTools(Array.isArray(t) ? t : [])).catch(console.error)
    fetchSkills().then((s) => setAvailableSkills(Array.isArray(s) ? s : [])).catch(console.error)
  }, [toolsOpen])

  const wsRef = useRef<WebSocket | null>(null)
  const messagesEndRef = useRef<HTMLDivElement | null>(null)
  const chatContainerRef = useRef<HTMLDivElement | null>(null)
  const [showScrollBottom, setShowScrollBottom] = useState(false)
  const userScrolledUpRef = useRef(false)

  // Connect & Auto-Reconnect WebSocket
  useEffect(() => {
    let reconnectTimeout: ReturnType<typeof setTimeout>
    let isUnmounted = false
    let retryDelay = 1000

    const connect = () => {
      if (isUnmounted) return

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const host = window.location.host || 'localhost:8080'
      const wsUrl = `${protocol}//${host}/ws`

      try {
        const ws = new WebSocket(wsUrl)
        wsRef.current = ws

        ws.onopen = () => {
          if (isUnmounted) return
          setWsConnected(true)
          retryDelay = 1000 // Reset backoff on successful connection
        }

        ws.onclose = () => {
          if (isUnmounted) return
          setWsConnected(false)
          // Schedule auto-reconnect with exponential backoff (capped at 8s)
          clearTimeout(reconnectTimeout)
          reconnectTimeout = setTimeout(() => {
            retryDelay = Math.min(retryDelay * 1.5, 8000)
            connect()
          }, retryDelay)
        }

        ws.onerror = () => {
          if (isUnmounted) return
          setWsConnected(false)
          ws.close()
        }

        ws.onmessage = (evt) => {
          try {
            const msg = JSON.parse(evt.data)
            handleWebSocketMessage(msg)
          } catch (err) {
            console.error('Failed to parse WebSocket message:', err)
          }
        }
      } catch (err) {
        if (isUnmounted) return
        setWsConnected(false)
        clearTimeout(reconnectTimeout)
        reconnectTimeout = setTimeout(() => {
          retryDelay = Math.min(retryDelay * 1.5, 8000)
          connect()
        }, retryDelay)
      }
    }

    connect()

    return () => {
      isUnmounted = true
      clearTimeout(reconnectTimeout)
      if (wsRef.current) {
        wsRef.current.close()
      }
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

  const scrollToBottom = (smooth = false) => {
    userScrolledUpRef.current = false
    setShowScrollBottom(false)
    if (chatContainerRef.current) {
      if (smooth) {
        chatContainerRef.current.scrollTo({
          top: chatContainerRef.current.scrollHeight,
          behavior: 'smooth',
        })
      } else {
        chatContainerRef.current.scrollTop = chatContainerRef.current.scrollHeight
      }
    }
  }

  const handleScroll = () => {
    if (!chatContainerRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = chatContainerRef.current
    const distanceFromBottom = scrollHeight - scrollTop - clientHeight
    if (distanceFromBottom > 40) {
      userScrolledUpRef.current = true
      setShowScrollBottom(true)
    } else {
      userScrolledUpRef.current = false
      setShowScrollBottom(false)
    }
  }

  const handleWheel = (e: React.WheelEvent<HTMLDivElement>) => {
    if (e.deltaY < 0) {
      // User is scrolling UP - immediately stop auto-scrolling
      userScrolledUpRef.current = true
      setShowScrollBottom(true)
    }
  }

  const handleTouchMove = () => {
    if (!chatContainerRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = chatContainerRef.current
    const distanceFromBottom = scrollHeight - scrollTop - clientHeight
    if (distanceFromBottom > 40) {
      userScrolledUpRef.current = true
      setShowScrollBottom(true)
    }
  }

  // Scroll on message change unless user has manually scrolled up
  useEffect(() => {
    if (userScrolledUpRef.current) return
    if (chatContainerRef.current) {
      chatContainerRef.current.scrollTop = chatContainerRef.current.scrollHeight
    }
  }, [messages, streaming])

  const handleSend = () => {
    if (!input.trim() || streaming) return

    const userMessageText = input.trim()
    setInput('')

    // Reset user scroll state to enable auto-scrolling
    userScrolledUpRef.current = false
    setShowScrollBottom(false)

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
    userScrolledUpRef.current = false
    setShowScrollBottom(false)
  }

  const handleSelectConversation = async (id: string) => {
    try {
      const data = await fetchConversation(id)
      setConversationId(data.conversation.id)
      userScrolledUpRef.current = false
      setShowScrollBottom(false)
      const mapped: ChatMessage[] = (data.messages || []).map((m: any) => {
        let toolCalls: ToolCallState[] | undefined
        let subflows: SubflowState[] | undefined
        let memoriesRetrieved: MemoryItem[] | undefined

        if (m.tool_calls) {
          try {
            toolCalls = typeof m.tool_calls === 'string' ? JSON.parse(m.tool_calls) : m.tool_calls
          } catch {}
        }
        if (m.subflows) {
          try {
            subflows = typeof m.subflows === 'string' ? JSON.parse(m.subflows) : m.subflows
          } catch {}
        }
        if (m.memories_retrieved) {
          try {
            memoriesRetrieved = typeof m.memories_retrieved === 'string' ? JSON.parse(m.memories_retrieved) : m.memories_retrieved
          } catch {}
        }

        return {
          id: m.id,
          role: m.role as any,
          content: m.content,
          toolCalls: toolCalls && toolCalls.length > 0 ? toolCalls : undefined,
          subflows: subflows && subflows.length > 0 ? subflows : undefined,
          memoriesRetrieved: memoriesRetrieved && memoriesRetrieved.length > 0 ? memoriesRetrieved : undefined,
          traceId: m.trace_id || undefined,
          timestamp: m.created_at,
        }
      })
      setMessages(mapped)
    } catch (e) {
      console.error(e)
    }
  }

  const matchingSlashItems = getMatchingSlashItems(input, availableTools, availableSkills)
  const isSlashActive = slashOpen && matchingSlashItems.length > 0

  const handleSelectSlashItem = (item: SlashItem) => {
    setInput(`${item.label} `)
    setSlashOpen(false)
  }

  const handleInputChange = (val: string) => {
    setInput(val)
    if (val.startsWith('/') && !val.includes(' ')) {
      setSlashOpen(true)
      setSlashIndex(0)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (isSlashActive) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSlashIndex((prev) => (prev + 1) % matchingSlashItems.length)
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSlashIndex((prev) => (prev - 1 + matchingSlashItems.length) % matchingSlashItems.length)
        return
      }
      if (e.key === 'Tab' || (e.key === 'Enter' && !e.shiftKey)) {
        e.preventDefault()
        if (matchingSlashItems[slashIndex]) {
          handleSelectSlashItem(matchingSlashItems[slashIndex])
          return
        }
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        setSlashOpen(false)
        return
      }
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background text-foreground">
      {/* Left Collapsible Sidebar Navigation */}
      <AppSidebar
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed(!sidebarCollapsed)}
        onOpenSettings={() => setSettingsOpen(true)}
        onOpenMcp={() => setMcpOpen(true)}
        onOpenTools={() => setToolsOpen(true)}
        onOpenMemory={() => setMemoryOpen(true)}
        onOpenHistory={() => setHistoryOpen(true)}
        onNewChat={handleNewChat}
        wsConnected={wsConnected}
        activeModal={
          settingsOpen
            ? 'settings'
            : mcpOpen
            ? 'mcp'
            : toolsOpen
            ? 'tools'
            : memoryOpen
            ? 'memory'
            : historyOpen
            ? 'history'
            : null
        }
      />

      {/* Main Chat Workspace */}
      <div className="flex flex-col flex-1 min-w-0 h-full overflow-hidden bg-background">
        {/* Top Header Bar */}
        <header className="h-14 border-b px-4 md:px-6 flex items-center justify-between shrink-0 bg-card/40 backdrop-blur-md sticky top-0 z-10">
          <div className="flex items-center gap-2.5">
            {sidebarCollapsed && (
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setSidebarCollapsed(false)}
                className="h-8 w-8 text-muted-foreground hover:text-foreground"
                title="Expand sidebar"
              >
                <PanelLeft className="h-4 w-4" />
              </Button>
            )}
            <div className="flex items-center gap-2 text-xs">
              <span className="text-foreground font-semibold">Active Session:</span>
              <span className="text-muted-foreground font-mono bg-muted/60 px-2 py-0.5 rounded-md border text-[11px]">
                {conversationId}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleNewChat}
              className="gap-1.5 text-xs font-medium h-8"
            >
              <Plus className="w-3.5 h-3.5" /> New Chat
            </Button>
          </div>
        </header>

        {/* Main Chat Stream Area */}
        <div className="flex-1 relative min-h-0 overflow-hidden flex flex-col">
          <main
            ref={chatContainerRef}
            onScroll={handleScroll}
            onWheel={handleWheel}
            onTouchMove={handleTouchMove}
            className="flex-1 overflow-y-auto px-4 md:px-8 py-6 max-w-4xl w-full mx-auto"
          >
            {messages.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-center space-y-4 py-16 text-muted-foreground">
                <div className="relative group">
                  <div className="absolute -inset-1 rounded-2xl bg-gradient-to-r from-cyan-500/30 to-purple-500/30 blur-lg opacity-75 group-hover:opacity-100 transition duration-500"></div>
                  <VoxLogo size={64} className="relative" />
                </div>
                <div className="space-y-1.5">
                  <h2 className="text-xl font-extrabold tracking-tight text-foreground flex items-center justify-center gap-1.5">
                    Welcome to <span className="bg-gradient-to-r from-cyan-400 via-indigo-400 to-purple-500 bg-clip-text text-transparent">Vox2 Studio</span>
                  </h2>
                  <p className="text-xs max-w-md mx-auto text-muted-foreground">
                    Next-generation harness for autonomous AI agents, dynamic MCP tools, cognitive memory, and parallel subflows.
                  </p>
                </div>
                <div className="flex flex-wrap gap-2 justify-center max-w-lg pt-3">
                  <button
                    type="button"
                    onClick={() => setInput('Read the files in this directory and summarize the codebase structure')}
                    className="text-xs p-2.5 rounded-lg border bg-card/60 hover:bg-card hover:border-primary/50 transition-all text-left cursor-pointer"
                  >
                    📁 Read the files in this directory and summarize structure
                  </button>
                  <button
                    type="button"
                    onClick={() => setInput('Run a parallel flow to investigate both code review and weather')}
                    className="text-xs p-2.5 rounded-lg border bg-card/60 hover:bg-card hover:border-primary/50 transition-all text-left cursor-pointer"
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

          {/* Floating Scroll to Bottom Button */}
          {showScrollBottom && (
            <div className="absolute bottom-3 left-1/2 -translate-x-1/2 z-20">
              <Button
                variant="outline"
                size="sm"
                onClick={() => scrollToBottom(true)}
                className="rounded-full shadow-lg bg-background/95 backdrop-blur-md border border-border hover:bg-accent text-xs gap-1.5 h-8 px-3.5 transition-all cursor-pointer animate-in fade-in slide-in-from-bottom-2"
              >
                <ArrowDown className="w-3.5 h-3.5 text-primary" />
                {streaming ? (
                  <span className="flex items-center gap-1.5">
                    <span className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" />
                    Resume auto-scroll
                  </span>
                ) : (
                  <span>Scroll to bottom</span>
                )}
              </Button>
            </div>
          )}
        </div>

        {/* Bottom Message Input & Disclaimer Area */}
        <footer className="border-t bg-card/50 px-4 py-3 shrink-0">
          <div className="max-w-4xl mx-auto space-y-2">
            <div className="flex items-end gap-2 relative">
              <SlashAutocomplete
                input={input}
                tools={availableTools}
                skills={availableSkills}
                selectedIndex={slashIndex}
                onSelect={handleSelectSlashItem}
                visible={isSlashActive}
              />
              <Textarea
                placeholder="Ask anything, type / for skills & tools, or trigger parallel subflows..."
                value={input}
                onChange={(e) => handleInputChange(e.target.value)}
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
    </div>
  )
}

export function App() {
  return (
    <ThemeProvider>
      <AppContent />
    </ThemeProvider>
  )
}

export default App
