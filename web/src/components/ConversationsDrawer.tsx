import React, { useState, useEffect } from 'react'
import { Conversation } from '../types'
import { fetchConversations, deleteConversation, getExportCSVUrl } from '../lib/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { MessageSquare, Download, Trash2, Plus, Calendar, Search, X } from 'lucide-react'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentConversationId: string
  onSelectConversation: (id: string) => void
  onNewChat: () => void
}

export const ConversationsDrawer: React.FC<Props> = ({
  open,
  onOpenChange,
  currentConversationId,
  onSelectConversation,
  onNewChat,
}) => {
  const [conversations, setConversations] = useState<Conversation[]>([])
  const [loading, setLoading] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  const loadList = async () => {
    setLoading(true)
    try {
      const list = await fetchConversations()
      setConversations(Array.isArray(list) ? list : [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open) {
      loadList()
      setSearchQuery('')
    }
  }, [open])

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    if (confirm('Delete this conversation history?')) {
      await deleteConversation(id)
      if (id === currentConversationId) {
        onNewChat()
      }
      await loadList()
    }
  }

  const handleExport = (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    window.open(getExportCSVUrl(id), '_blank')
  }

  const filteredConversations = (conversations || []).filter((c) => {
    if (!searchQuery.trim()) return true
    const q = searchQuery.toLowerCase().trim()
    const title = (c.title || 'Untitled Chat').toLowerCase()
    const id = (c.id || '').toLowerCase()
    const formattedDate = new Date(c.updated_at).toLocaleString().toLowerCase()
    return title.includes(q) || id.includes(q) || formattedDate.includes(q)
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg max-h-[80vh] overflow-y-auto">
        <DialogHeader className="flex flex-row items-center justify-between">
          <DialogTitle className="text-xl font-bold flex items-center gap-2">
            <MessageSquare className="w-5 h-5 text-primary" /> Conversation Sessions
          </DialogTitle>
          <Button
            size="sm"
            onClick={() => {
              onNewChat()
              onOpenChange(false)
            }}
          >
            <Plus className="w-4 h-4 mr-1" /> New Chat
          </Button>
        </DialogHeader>

        {/* Search Bar */}
        <div className="relative mt-2">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search conversations by title, ID, or date..."
            className="pl-8 pr-8 text-xs h-9"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
          {searchQuery && (
            <button
              type="button"
              onClick={() => setSearchQuery('')}
              className="absolute right-2.5 top-2.5 text-muted-foreground hover:text-foreground cursor-pointer"
              title="Clear search"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        <div className="space-y-2 mt-3">
          {(conversations || []).length === 0 ? (
            <div className="text-center py-10 border border-dashed rounded-lg text-sm text-muted-foreground">
              {loading ? 'Loading conversations...' : 'No saved conversations found.'}
            </div>
          ) : filteredConversations.length === 0 ? (
            <div className="text-center py-8 border border-dashed rounded-lg text-sm text-muted-foreground space-y-2">
              <p>No conversations found matching "{searchQuery}"</p>
              <Button
                variant="ghost"
                size="sm"
                className="text-xs text-primary hover:underline h-7"
                onClick={() => setSearchQuery('')}
              >
                Clear search
              </Button>
            </div>
          ) : (
            <div className="space-y-2 max-h-[450px] overflow-y-auto pr-1">
              {filteredConversations.map((c) => {
                const isSelected = c.id === currentConversationId
                return (
                  <div
                    key={c.id}
                    onClick={() => {
                      onSelectConversation(c.id)
                      onOpenChange(false)
                    }}
                    className={`flex items-center justify-between p-3 rounded-lg border cursor-pointer transition-all ${
                      isSelected
                        ? 'border-primary bg-primary/10'
                        : 'border-border bg-card/60 hover:bg-card'
                    }`}
                  >
                    <div className="space-y-1 max-w-[70%]">
                      <div className="font-semibold text-sm truncate">{c.title || 'Untitled Chat'}</div>
                      <div className="flex items-center gap-1 text-xs text-muted-foreground">
                        <Calendar className="w-3 h-3" />
                        {new Date(c.updated_at).toLocaleString()}
                      </div>
                    </div>

                    <div className="flex items-center gap-1.5">
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Export Conversation to CSV"
                        onClick={(e) => handleExport(c.id, e)}
                      >
                        <Download className="w-4 h-4 text-primary" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        title="Delete Conversation"
                        className="text-destructive hover:bg-destructive/10"
                        onClick={(e) => handleDelete(c.id, e)}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
