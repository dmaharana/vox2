import React, { useEffect, useRef } from 'react'
import { ToolDefinition, Skill } from '../types'
import { Badge } from './ui/badge'
import { Sparkles, Wrench, Terminal, Slash } from 'lucide-react'

export interface SlashItem {
  name: string
  label: string
  category: 'skill' | 'tool' | 'command'
  description: string
  badge?: string
}

interface SlashAutocompleteProps {
  input: string
  tools: ToolDefinition[]
  skills: Skill[]
  selectedIndex: number
  onSelect: (item: SlashItem) => void
  visible: boolean
}

export function getMatchingSlashItems(
  input: string,
  tools: ToolDefinition[],
  skills: Skill[]
): SlashItem[] {
  if (!input.startsWith('/') || input.includes(' ')) {
    return []
  }

  const query = input.slice(1).toLowerCase().trim()

  const systemCommands: SlashItem[] = [
    {
      name: 'skills',
      label: '/skills',
      category: 'command',
      description: 'List all discovered skills and their active status',
      badge: 'Command',
    },
    {
      name: 'tools',
      label: '/tools',
      category: 'command',
      description: 'List all registered agent tools and categories',
      badge: 'Command',
    },
    {
      name: 'help',
      label: '/help',
      category: 'command',
      description: 'Show slash commands and help reference',
      badge: 'Command',
    },
  ]

  const skillItems: SlashItem[] = skills.map((s) => ({
    name: s.name,
    label: `/${s.name}`,
    category: 'skill',
    description: s.description || `Specialized guidelines for ${s.name}`,
    badge: s.enabled ? 'Skill' : 'Skill (Disabled)',
  }))

  const toolItems: SlashItem[] = tools.map((t) => ({
    name: t.name,
    label: `/${t.name}`,
    category: 'tool',
    description: t.description || `Tool: ${t.name}`,
    badge: t.category || 'Tool',
  }))

  const allItems = [...systemCommands, ...skillItems, ...toolItems]

  if (!query) {
    return allItems
  }

  return allItems.filter(
    (item) =>
      item.name.toLowerCase().includes(query) ||
      item.description.toLowerCase().includes(query) ||
      item.category.toLowerCase().includes(query)
  )
}

export const SlashAutocomplete: React.FC<SlashAutocompleteProps> = ({
  input,
  tools,
  skills,
  selectedIndex,
  onSelect,
  visible,
}) => {
  const itemRefs = useRef<(HTMLDivElement | null)[]>([])
  const items = getMatchingSlashItems(input, tools, skills)

  useEffect(() => {
    if (visible && itemRefs.current[selectedIndex]) {
      itemRefs.current[selectedIndex]?.scrollIntoView({
        block: 'nearest',
        behavior: 'smooth',
      })
    }
  }, [selectedIndex, visible])

  if (!visible || items.length === 0) {
    return null
  }

  return (
    <div className="absolute bottom-full mb-2 left-0 right-0 z-50 max-h-72 overflow-y-auto rounded-xl border bg-popover/95 backdrop-blur-md p-1.5 text-popover-foreground shadow-2xl animate-in fade-in slide-in-from-bottom-2 duration-150">
      <div className="flex items-center justify-between px-2.5 py-1 text-[11px] font-medium text-muted-foreground border-b border-border/40 mb-1">
        <span className="flex items-center gap-1">
          <Slash className="w-3 h-3 text-primary" />
          <span>Slash Commands, Skills & Tools</span>
        </span>
        <span>{items.length} options</span>
      </div>

      <div className="space-y-0.5">
        {items.map((item, index) => {
          const isSelected = index === selectedIndex

          let Icon = Terminal
          let badgeVariant: 'default' | 'secondary' | 'outline' = 'secondary'
          let badgeClass = 'text-[10px] py-0 px-1.5 font-normal'

          if (item.category === 'skill') {
            Icon = Sparkles
            badgeClass += ' bg-purple-500/15 text-purple-400 border-purple-500/30'
          } else if (item.category === 'tool') {
            Icon = Wrench
            badgeClass += ' bg-cyan-500/15 text-cyan-400 border-cyan-500/30'
          } else {
            badgeClass += ' bg-amber-500/15 text-amber-400 border-amber-500/30'
          }

          return (
            <div
              key={`${item.category}-${item.name}`}
              ref={(el) => {
                itemRefs.current[index] = el
              }}
              onClick={() => onSelect(item)}
              className={`flex items-center justify-between gap-3 px-3 py-2 rounded-lg cursor-pointer text-xs transition-colors select-none ${
                isSelected
                  ? 'bg-primary text-primary-foreground font-medium shadow-sm'
                  : 'hover:bg-muted/70 text-foreground'
              }`}
            >
              <div className="flex items-center gap-2 min-w-0">
                <Icon
                  className={`w-3.5 h-3.5 shrink-0 ${
                    isSelected
                      ? 'text-primary-foreground'
                      : item.category === 'skill'
                      ? 'text-purple-400'
                      : item.category === 'tool'
                      ? 'text-cyan-400'
                      : 'text-amber-400'
                  }`}
                />
                <div className="flex flex-col min-w-0">
                  <span className="font-mono font-semibold tracking-tight truncate">
                    {item.label}
                  </span>
                  <span
                    className={`text-[11px] truncate max-w-sm md:max-w-md ${
                      isSelected ? 'text-primary-foreground/80' : 'text-muted-foreground'
                    }`}
                  >
                    {item.description}
                  </span>
                </div>
              </div>

              <div className="flex items-center gap-1.5 shrink-0">
                <Badge
                  variant={badgeVariant}
                  className={`${badgeClass} ${
                    isSelected ? '!bg-primary-foreground/20 !text-primary-foreground !border-primary-foreground/30' : ''
                  }`}
                >
                  {item.badge || item.category}
                </Badge>
              </div>
            </div>
          )
        })}
      </div>

      <div className="flex items-center justify-between px-2.5 pt-1.5 pb-0.5 text-[10px] text-muted-foreground border-t border-border/40 mt-1">
        <span>
          <kbd className="px-1 py-0.5 bg-muted rounded border text-[9px]">↑</kbd>{' '}
          <kbd className="px-1 py-0.5 bg-muted rounded border text-[9px]">↓</kbd> navigate
        </span>
        <span>
          <kbd className="px-1 py-0.5 bg-muted rounded border text-[9px]">Tab</kbd> or{' '}
          <kbd className="px-1 py-0.5 bg-muted rounded border text-[9px]">Enter</kbd> select
        </span>
        <span>
          <kbd className="px-1 py-0.5 bg-muted rounded border text-[9px]">Esc</kbd> close
        </span>
      </div>
    </div>
  )
}
