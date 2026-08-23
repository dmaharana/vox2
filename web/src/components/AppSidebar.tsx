import React from 'react'
import { Button } from './ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from './ui/tooltip'
import { useTheme } from '../lib/theme'
import { VoxLogo } from './VoxLogo'
import {
  Sliders,
  Server,
  Wrench,
  Brain,
  MessageSquare,
  Plus,
  PanelLeftClose,
  PanelLeft,
  Sun,
  Moon,
  Wifi,
  WifiOff,
  Bot,
  Activity,
} from 'lucide-react'

interface AppSidebarProps {
  collapsed: boolean
  onToggleCollapse: () => void
  onOpenSettings: () => void
  onOpenMcp: () => void
  onOpenTools: () => void
  onOpenMemory: () => void
  onOpenHistory: () => void
  onNewChat: () => void
  wsConnected: boolean
  activeModal?: string | null
}

export const AppSidebar: React.FC<AppSidebarProps> = ({
  collapsed,
  onToggleCollapse,
  onOpenSettings,
  onOpenMcp,
  onOpenTools,
  onOpenMemory,
  onOpenHistory,
  onNewChat,
  wsConnected,
  activeModal,
}) => {
  const { theme, setTheme, isDark } = useTheme()

  const navItems = [
    {
      id: 'settings',
      label: 'LLM Settings',
      description: 'Model, Base URL, Temperature',
      icon: Sliders,
      onClick: onOpenSettings,
      badge: null,
    },
    {
      id: 'mcp',
      label: 'MCP Servers',
      description: 'Stdio & Streamable HTTP / SSE',
      icon: Server,
      onClick: onOpenMcp,
      badge: null,
    },
    {
      id: 'tools',
      label: 'Tools & Skills',
      description: 'Built-in Tools & SKILL.md Loader',
      icon: Wrench,
      onClick: onOpenTools,
      badge: null,
    },
    {
      id: 'memory',
      label: 'Memory Bank',
      description: 'Cognitive FTS5 & Short/Long-Term',
      icon: Brain,
      onClick: onOpenMemory,
      badge: null,
    },
    {
      id: 'history',
      label: 'Chat History',
      description: 'Stored Conversations',
      icon: MessageSquare,
      onClick: onOpenHistory,
      badge: null,
    },
  ]

  const toggleTheme = () => {
    setTheme(isDark ? 'light' : 'dark')
  }

  return (
    <TooltipProvider delayDuration={150}>
      <aside
        className={`relative flex flex-col border-r bg-card/80 backdrop-blur-md transition-all duration-300 ease-in-out shrink-0 z-30 select-none ${
          collapsed ? 'w-16' : 'w-64'
        }`}
      >
        {/* Sidebar Header / Brand */}
        <div
          className={`flex h-14 items-center border-b ${
            collapsed ? 'justify-center px-2' : 'justify-between px-3'
          }`}
        >
          {collapsed ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={onToggleCollapse}
                  className="h-10 w-10 p-0 rounded-xl hover:bg-primary/10 transition-colors"
                >
                  <VoxLogo size={32} />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right" className="text-xs">
                Expand sidebar (Vox2 Studio)
              </TooltipContent>
            </Tooltip>
          ) : (
            <>
              <VoxLogo size={32} showText={true} subtext="v1.0.0 • AI Studio" />

              {/* Toggle Collapse */}
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={onToggleCollapse}
                    className="h-8 w-8 text-muted-foreground hover:text-foreground shrink-0"
                  >
                    <PanelLeftClose className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="right" className="text-xs">
                  Collapse sidebar
                </TooltipContent>
              </Tooltip>
            </>
          )}
        </div>

        {/* Action: New Chat */}
        <div className="p-2 border-b border-border/50">
          {collapsed ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  onClick={onNewChat}
                  variant="default"
                  size="icon"
                  className="h-10 w-10 mx-auto rounded-xl shadow-xs transition-transform active:scale-95 flex items-center justify-center"
                >
                  <Plus className="h-5 w-5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right" className="text-xs font-medium">
                New Conversation
              </TooltipContent>
            </Tooltip>
          ) : (
            <Button
              onClick={onNewChat}
              variant="default"
              className="w-full justify-start gap-2.5 h-9 rounded-lg font-medium text-xs shadow-xs transition-transform active:scale-[0.98]"
            >
              <Plus className="h-4 w-4" />
              <span>New Conversation</span>
            </Button>
          )}
        </div>

        {/* Navigation Items */}
        <div className="flex-1 overflow-y-auto px-2 py-3 space-y-1">
          {!collapsed && (
            <div className="px-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              Configuration & State
            </div>
          )}

          {navItems.map((item) => {
            const Icon = item.icon
            const isActive = activeModal === item.id

            if (collapsed) {
              return (
                <Tooltip key={item.id}>
                  <TooltipTrigger asChild>
                    <Button
                      variant={isActive ? 'secondary' : 'ghost'}
                      size="icon"
                      onClick={item.onClick}
                      className={`h-10 w-10 mx-auto rounded-xl transition-colors ${
                        isActive
                          ? 'bg-primary/10 text-primary hover:bg-primary/20 font-semibold'
                          : 'text-muted-foreground hover:text-foreground hover:bg-accent/60'
                      }`}
                    >
                      <Icon className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="right" className="text-xs font-medium">
                    <p className="font-semibold">{item.label}</p>
                    <p className="text-[10px] opacity-80">{item.description}</p>
                  </TooltipContent>
                </Tooltip>
              )
            }

            return (
              <Button
                key={item.id}
                variant={isActive ? 'secondary' : 'ghost'}
                onClick={item.onClick}
                className={`w-full justify-start gap-3 h-9 px-2.5 rounded-lg text-xs font-medium transition-colors ${
                  isActive
                    ? 'bg-primary/10 text-primary font-semibold hover:bg-primary/15'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent/60'
                }`}
              >
                <Icon className={`h-4 w-4 shrink-0 ${isActive ? 'text-primary' : ''}`} />
                <span className="truncate">{item.label}</span>
              </Button>
            )
          })}
        </div>

        {/* Sidebar Footer: Theme Toggle & Connection Status */}
        <div className="border-t p-2 space-y-1.5 bg-card/40">
          {/* Theme Switcher Button */}
          {collapsed ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={toggleTheme}
                  className="h-10 w-10 mx-auto rounded-xl text-muted-foreground hover:text-foreground hover:bg-accent/60"
                >
                  {isDark ? (
                    <Sun className="h-4 w-4 text-amber-400 animate-in spin-in-90 duration-200" />
                  ) : (
                    <Moon className="h-4 w-4 text-indigo-500 animate-in spin-in-90 duration-200" />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right" className="text-xs font-medium">
                Switch to {isDark ? 'Light' : 'Dark'} Mode
              </TooltipContent>
            </Tooltip>
          ) : (
            <Button
              variant="ghost"
              onClick={toggleTheme}
              className="w-full justify-start gap-3 h-9 px-2.5 rounded-lg text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-accent/60"
            >
              {isDark ? (
                <>
                  <Sun className="h-4 w-4 text-amber-400 shrink-0" />
                  <span>Light Mode</span>
                </>
              ) : (
                <>
                  <Moon className="h-4 w-4 text-indigo-500 shrink-0" />
                  <span>Dark Mode</span>
                </>
              )}
            </Button>
          )}

          {/* Connection Status */}
          {collapsed ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="flex h-10 w-10 mx-auto items-center justify-center rounded-xl cursor-default">
                  {wsConnected ? (
                    <span className="relative flex h-2.5 w-2.5">
                      <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                      <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500"></span>
                    </span>
                  ) : (
                    <span className="relative flex h-2.5 w-2.5">
                      <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
                      <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-amber-500"></span>
                    </span>
                  )}
                </div>
              </TooltipTrigger>
              <TooltipContent side="right" className="text-xs font-medium">
                {wsConnected ? 'WebSocket: Connected (Live)' : 'WebSocket: Disconnected (Auto-Reconnecting...)'}
              </TooltipContent>
            </Tooltip>
          ) : (
            <div className="flex items-center justify-between px-2.5 py-1.5 rounded-lg bg-background/50 border border-border/40 text-[11px]">
              <div className="flex items-center gap-2">
                {wsConnected ? (
                  <span className="relative flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                  </span>
                ) : (
                  <span className="relative flex h-2 w-2">
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
                  </span>
                )}
                <span className={`font-medium ${wsConnected ? 'text-muted-foreground' : 'text-amber-500'}`}>
                  {wsConnected ? 'Connected' : 'Reconnecting...'}
                </span>
              </div>
              <span className="text-[10px] font-mono text-muted-foreground/80">WS:8080</span>
            </div>
          )}
        </div>
      </aside>
    </TooltipProvider>
  )
}
