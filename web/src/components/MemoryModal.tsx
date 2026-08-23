import React, { useState, useEffect } from 'react'
import { MemoryItem } from '../types'
import { searchMemories, saveMemory, promoteMemories } from '../lib/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Badge } from './ui/badge'
import { Brain, Search, Plus, Sparkles, Clock, Layers } from 'lucide-react'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export const MemoryModal: React.FC<Props> = ({ open, onOpenChange }) => {
  const [memories, setMemories] = useState<MemoryItem[]>([])
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(false)
  const [isAdding, setIsAdding] = useState(false)

  // New Memory Form
  const [key, setKey] = useState('')
  const [content, setContent] = useState('')
  const [memoryType, setMemoryType] = useState<any>('semantic')
  const [tags, setTags] = useState('')

  const loadMemories = async (searchQuery = '') => {
    setLoading(true)
    try {
      const results = await searchMemories(searchQuery)
      setMemories(Array.isArray(results) ? results : [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open) {
      loadMemories(query)
    }
  }, [open])

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    loadMemories(query)
  }

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await saveMemory({
        key,
        content,
        memory_type: memoryType,
        tags,
      })
      setKey('')
      setContent('')
      setTags('')
      setIsAdding(false)
      await loadMemories(query)
    } catch (err: any) {
      alert(err.message)
    }
  }

  const handlePromote = async () => {
    try {
      const res = await promoteMemories()
      alert(`Promoted ${res.promoted_count} aging memories to Long-Term storage!`)
      await loadMemories(query)
    } catch (err: any) {
      alert(err.message)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader className="flex flex-row items-center justify-between">
          <DialogTitle className="text-xl font-bold flex items-center gap-2">
            <Brain className="w-5 h-5 text-primary" /> Searchable Cognitive Memory Bank
          </DialogTitle>
        </DialogHeader>

        {/* Cognitive Explanation */}
        <div className="bg-muted/40 p-3 rounded-lg border text-xs space-y-1 text-muted-foreground">
          <div className="flex items-center gap-1 font-semibold text-foreground">
            <Layers className="w-3.5 h-3.5 text-primary" /> Memory Architecture:
          </div>
          <div className="grid grid-cols-2 gap-2 pt-1">
            <div>
              <span className="font-semibold text-foreground">Short-Term:</span> Working context, Semantic prompt cache.
            </div>
            <div>
              <span className="font-semibold text-foreground">Long-Term:</span> Semantic facts, Episodic history, Procedural workflows.
            </div>
          </div>
        </div>

        {/* Search Bar & Actions */}
        <div className="flex items-center gap-2 pt-2">
          <form onSubmit={handleSearch} className="flex-1 flex gap-2">
            <div className="relative flex-1">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search memories with FTS5 keyword matching..."
                className="pl-8"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            <Button type="submit" variant="secondary">
              Search
            </Button>
          </form>
          <Button variant="outline" size="sm" onClick={handlePromote} title="Promote frequently accessed short-term memories">
            <Sparkles className="w-4 h-4 mr-1 text-amber-500" /> Promote Aging
          </Button>
          <Button size="sm" onClick={() => setIsAdding(!isAdding)}>
            <Plus className="w-4 h-4 mr-1" /> Add
          </Button>
        </div>

        {/* Add Memory Form */}
        {isAdding && (
          <form onSubmit={handleAdd} className="border p-4 rounded-lg bg-card/80 space-y-3 mt-2">
            <div className="font-semibold text-sm">Add Memory Entry</div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1">
                <Label htmlFor="memKey">Identifier / Key</Label>
                <Input
                  id="memKey"
                  placeholder="e.g. user_theme_preference"
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="memType">Memory Type</Label>
                <select
                  id="memType"
                  className="w-full h-9 rounded-md border bg-background px-3 text-sm"
                  value={memoryType}
                  onChange={(e) => setMemoryType(e.target.value)}
                >
                  <option value="semantic">Semantic (Facts, Profile)</option>
                  <option value="episodic">Episodic (Events, Past Actions)</option>
                  <option value="procedural">Procedural (How-To Workflows)</option>
                  <option value="working">Working Memory (Short-Term)</option>
                  <option value="semantic_cache">Semantic Cache (Short-Term)</option>
                </select>
              </div>
            </div>

            <div className="space-y-1">
              <Label htmlFor="memContent">Content</Label>
              <Input
                id="memContent"
                placeholder="The detailed fact, instruction, or event..."
                value={content}
                onChange={(e) => setContent(e.target.value)}
                required
              />
            </div>

            <div className="space-y-1">
              <Label htmlFor="memTags">Tags (comma separated)</Label>
              <Input
                id="memTags"
                placeholder="ui, preferences, dev"
                value={tags}
                onChange={(e) => setTags(e.target.value)}
              />
            </div>

            <div className="flex justify-end gap-2">
              <Button type="button" variant="ghost" size="sm" onClick={() => setIsAdding(false)}>
                Cancel
              </Button>
              <Button type="submit" size="sm">
                Save Memory
              </Button>
            </div>
          </form>
        )}

        {/* Memories List */}
        <div className="space-y-2 mt-3">
          <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
            Memory Bank Entries ({(memories || []).length})
          </div>

          {(memories || []).length === 0 ? (
            <div className="text-center py-8 border border-dashed rounded-lg text-sm text-muted-foreground">
              {loading ? 'Searching...' : 'No memory items matched the search.'}
            </div>
          ) : (
            <div className="space-y-2 max-h-[400px] overflow-y-auto pr-1">
              {(memories || []).map((m) => (
                <div key={m.id} className="p-3 border rounded-lg bg-card/60 space-y-1.5">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-sm font-semibold">{m.key}</span>
                      <Badge variant="outline" className="text-xs capitalize font-medium">
                        {m.memory_type}
                      </Badge>
                      <Badge
                        variant={m.tier === 'long_term' ? 'default' : 'secondary'}
                        className={`text-xs ${m.tier === 'long_term' ? 'bg-primary/90' : ''}`}
                      >
                        {m.tier === 'long_term' ? 'Long-Term' : 'Short-Term'}
                      </Badge>
                    </div>
                    <div className="flex items-center gap-1 text-xs text-muted-foreground">
                      <Clock className="w-3 h-3" /> Hits: {m.access_count}
                    </div>
                  </div>
                  <p className="text-sm text-foreground/90">{m.content}</p>
                  {m.tags && (
                    <div className="flex items-center gap-1 text-xs text-muted-foreground">
                      <span className="font-medium">Tags:</span> {m.tags}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
