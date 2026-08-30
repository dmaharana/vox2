import React, { useState, useEffect } from 'react'
import { ToolDefinition, Skill } from '../types'
import { fetchTools, toggleTool, fetchSkills, toggleSkill, refreshSkills } from '../lib/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import { Tabs, TabsList, TabsTrigger, TabsContent } from './ui/tabs'
import { Switch } from './ui/switch'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Wrench, Sparkles, FolderCode, RefreshCw, FileCode, BookOpen, Search, X } from 'lucide-react'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export const ToolsSkillsModal: React.FC<Props> = ({ open, onOpenChange }) => {
  const [tools, setTools] = useState<ToolDefinition[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [skillSearch, setSkillSearch] = useState('')
  const [toolSearch, setToolSearch] = useState('')
  const [loading, setLoading] = useState(false)
  const [refreshing, setRefreshing] = useState(false)

  const loadData = async () => {
    setLoading(true)
    try {
      const [t, s] = await Promise.all([fetchTools(), fetchSkills()])
      setTools(Array.isArray(t) ? t : [])
      setSkills(Array.isArray(s) ? s : [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open) {
      loadData()
      setSkillSearch('')
      setToolSearch('')
    }
  }, [open])

  const handleRefreshSkills = async () => {
    setRefreshing(true)
    try {
      const refreshed = await refreshSkills()
      setSkills(Array.isArray(refreshed) ? refreshed : [])
    } catch (e: any) {
      alert(e.message || 'Failed to refresh skills from disk')
    } finally {
      setRefreshing(false)
    }
  }

  const handleToggleTool = async (name: string, current: boolean) => {
    await toggleTool(name, !current)
    setTools((prev) =>
      prev.map((t) => (t.name === name ? { ...t, enabled: !current } : t))
    )
  }

  const handleToggleSkill = async (name: string, current: boolean) => {
    await toggleSkill(name, !current)
    setSkills((prev) =>
      prev.map((s) => (s.name === name ? { ...s, enabled: !current } : s))
    )
  }

  const filteredSkills = (skills || []).filter((s) => {
    if (!skillSearch.trim()) return true
    const q = skillSearch.toLowerCase().trim()
    const name = (s.name || '').toLowerCase()
    const desc = (s.description || '').toLowerCase()
    const status = s.enabled ? 'active' : 'disabled'
    const matchesFiles = (s.files || []).some(
      (f) => f.path.toLowerCase().includes(q) || f.category.toLowerCase().includes(q)
    )
    return name.includes(q) || desc.includes(q) || status.includes(q) || matchesFiles
  })

  const filteredTools = (tools || []).filter((t) => {
    if (!toolSearch.trim()) return true
    const q = toolSearch.toLowerCase().trim()
    const name = (t.name || '').toLowerCase()
    const desc = (t.description || '').toLowerCase()
    const category = (t.category || '').toLowerCase()
    const status = t.enabled ? 'enabled' : 'disabled'
    return name.includes(q) || desc.includes(q) || category.includes(q) || status.includes(q)
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader className="flex flex-row items-center justify-between">
          <DialogTitle className="text-xl font-bold flex items-center gap-2">
            <Wrench className="w-5 h-5 text-primary" /> Tools & Skills Configuration
          </DialogTitle>
          <Button
            variant="ghost"
            size="icon"
            onClick={loadData}
            disabled={loading || refreshing}
            title="Reload Tools & Skills"
          >
            <RefreshCw className={`w-4 h-4 ${(loading || refreshing) ? 'animate-spin' : ''}`} />
          </Button>
        </DialogHeader>

        <Tabs defaultValue="skills" className="mt-2">
          <TabsList className="grid grid-cols-2 w-full">
            <TabsTrigger value="skills" className="flex items-center gap-1.5">
              <Sparkles className="w-4 h-4" /> Loaded Skills ({(skills || []).length})
            </TabsTrigger>
            <TabsTrigger value="tools" className="flex items-center gap-1.5">
              <FolderCode className="w-4 h-4" /> Agent Tools ({(tools || []).length})
            </TabsTrigger>
          </TabsList>

          <TabsContent value="skills" className="space-y-3 pt-3">
            <div className="flex items-center justify-between bg-muted/40 p-2.5 rounded-lg border">
              <p className="text-xs text-muted-foreground max-w-[75%]">
                Skills are discovered from <code>SKILL.md</code> folders with companion scripts & references. Click refresh to scan for newly added or edited skills on disk without restarting the server.
              </p>
              <Button
                variant="outline"
                size="sm"
                onClick={handleRefreshSkills}
                disabled={refreshing}
                className="gap-1.5 text-xs shrink-0"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin' : ''}`} />
                {refreshing ? 'Scanning...' : 'Refresh from Disk'}
              </Button>
            </div>

            {/* Search Loaded Skills */}
            <div className="relative">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search loaded skills by name, description, script..."
                className="pl-8 pr-8 text-xs h-9"
                value={skillSearch}
                onChange={(e) => setSkillSearch(e.target.value)}
              />
              {skillSearch && (
                <button
                  type="button"
                  onClick={() => setSkillSearch('')}
                  className="absolute right-2.5 top-2.5 text-muted-foreground hover:text-foreground cursor-pointer"
                  title="Clear search"
                >
                  <X className="h-4 w-4" />
                </button>
              )}
            </div>

            {(skills || []).length === 0 ? (
              <div className="text-center py-8 border border-dashed rounded-lg text-sm text-muted-foreground">
                No skills discovered in the skills folder. Add a folder with <code>SKILL.md</code> and click <strong>Refresh from Disk</strong>!
              </div>
            ) : filteredSkills.length === 0 ? (
              <div className="text-center py-8 border border-dashed rounded-lg text-sm text-muted-foreground space-y-2">
                <p>No skills found matching "{skillSearch}"</p>
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-xs text-primary hover:underline h-7"
                  onClick={() => setSkillSearch('')}
                >
                  Clear search
                </Button>
              </div>
            ) : (
              <div className="space-y-2">
                {filteredSkills.map((s) => {
                  const scriptsCount = (s.files || []).filter((f) => f.category === 'script').length
                  const refCount = (s.files || []).filter((f) => f.category === 'reference' || f.category === 'template').length

                  return (
                    <div
                      key={s.name}
                      className="flex items-center justify-between p-3 border rounded-lg bg-card/60 hover:bg-card transition-colors"
                    >
                      <div className="space-y-1.5 max-w-[80%]">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="font-semibold text-sm font-mono">{s.name}</span>
                          {s.enabled ? (
                            <Badge variant="default" className="bg-emerald-600/90 text-[10px]">
                              Active
                            </Badge>
                          ) : (
                            <Badge variant="secondary" className="text-[10px]">
                              Disabled
                            </Badge>
                          )}
                          {scriptsCount > 0 && (
                            <Badge variant="outline" className="text-[10px] gap-1 font-normal bg-muted/40">
                              <FileCode className="w-3 h-3 text-blue-500" /> {scriptsCount} {scriptsCount === 1 ? 'script' : 'scripts'}
                            </Badge>
                          )}
                          {refCount > 0 && (
                            <Badge variant="outline" className="text-[10px] gap-1 font-normal bg-muted/40">
                              <BookOpen className="w-3 h-3 text-amber-500" /> {refCount} {refCount === 1 ? 'reference' : 'references'}
                            </Badge>
                          )}
                        </div>
                        <p className="text-xs text-muted-foreground">{s.description}</p>
                      </div>
                      <Switch
                        checked={s.enabled}
                        onCheckedChange={() => handleToggleSkill(s.name, s.enabled)}
                      />
                    </div>
                  )
                })}
              </div>
            )}
          </TabsContent>

          <TabsContent value="tools" className="space-y-3 pt-3">
            <p className="text-xs text-muted-foreground">
              Toggle built-in filesystem tools, cognitive memory tools, subflow orchestrators, or MCP tools.
            </p>

            {/* Search Agent Tools */}
            <div className="relative">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                placeholder="Search agent tools by name, category, description..."
                className="pl-8 pr-8 text-xs h-9"
                value={toolSearch}
                onChange={(e) => setToolSearch(e.target.value)}
              />
              {toolSearch && (
                <button
                  type="button"
                  onClick={() => setToolSearch('')}
                  className="absolute right-2.5 top-2.5 text-muted-foreground hover:text-foreground cursor-pointer"
                  title="Clear search"
                >
                  <X className="h-4 w-4" />
                </button>
              )}
            </div>

            {(tools || []).length === 0 ? (
              <div className="text-center py-8 border border-dashed rounded-lg text-sm text-muted-foreground">
                No agent tools found.
              </div>
            ) : filteredTools.length === 0 ? (
              <div className="text-center py-8 border border-dashed rounded-lg text-sm text-muted-foreground space-y-2">
                <p>No tools found matching "{toolSearch}"</p>
                <Button
                  variant="ghost"
                  size="sm"
                  className="text-xs text-primary hover:underline h-7"
                  onClick={() => setToolSearch('')}
                >
                  Clear search
                </Button>
              </div>
            ) : (
              <div className="space-y-2">
                {filteredTools.map((t) => (
                  <div
                    key={t.name}
                    className="flex items-center justify-between p-3 border rounded-lg bg-card/60 hover:bg-card transition-colors"
                  >
                    <div className="space-y-1 max-w-[80%]">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-sm font-mono">{t.name}</span>
                        <Badge variant="outline" className="text-xs capitalize">
                          {t.category}
                        </Badge>
                      </div>
                      <p className="text-xs text-muted-foreground">{t.description}</p>
                    </div>
                    <Switch
                      checked={t.enabled}
                      onCheckedChange={() => handleToggleTool(t.name, t.enabled)}
                    />
                  </div>
                ))}
              </div>
            )}
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
