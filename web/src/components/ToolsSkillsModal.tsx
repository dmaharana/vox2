import React, { useState, useEffect } from 'react'
import { ToolDefinition, Skill } from '../types'
import { fetchTools, toggleTool, fetchSkills, toggleSkill } from '../lib/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import { Tabs, TabsList, TabsTrigger, TabsContent } from './ui/tabs'
import { Switch } from './ui/switch'
import { Badge } from './ui/badge'
import { Wrench, Sparkles, FolderCode } from 'lucide-react'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export const ToolsSkillsModal: React.FC<Props> = ({ open, onOpenChange }) => {
  const [tools, setTools] = useState<ToolDefinition[]>([])
  const [skills, setSkills] = useState<Skill[]>([])

  const loadData = async () => {
    const [t, s] = await Promise.all([fetchTools(), fetchSkills()])
    setTools(Array.isArray(t) ? t : [])
    setSkills(Array.isArray(s) ? s : [])
  }

  useEffect(() => {
    if (open) {
      loadData()
    }
  }, [open])

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

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-xl font-bold flex items-center gap-2">
            <Wrench className="w-5 h-5 text-primary" /> Tools & Skills Configuration
          </DialogTitle>
        </DialogHeader>

        <Tabs defaultValue="tools" className="mt-2">
          <TabsList className="grid grid-cols-2 w-full">
            <TabsTrigger value="tools" className="flex items-center gap-1.5">
              <FolderCode className="w-4 h-4" /> Agent Tools ({(tools || []).length})
            </TabsTrigger>
            <TabsTrigger value="skills" className="flex items-center gap-1.5">
              <Sparkles className="w-4 h-4" /> Loaded Skills ({(skills || []).length})
            </TabsTrigger>
          </TabsList>

          <TabsContent value="tools" className="space-y-3 pt-4">
            <p className="text-xs text-muted-foreground">
              Toggle built-in filesystem tools, cognitive memory tools, subflow orchestrators, or MCP tools.
            </p>

            <div className="space-y-2">
              {(tools || []).map((t) => (
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
          </TabsContent>

          <TabsContent value="skills" className="space-y-3 pt-4">
            <p className="text-xs text-muted-foreground">
              Skills are dynamically loaded from your configured skills folder. Active skills are automatically injected into the agent system instructions.
            </p>

            {(skills || []).length === 0 ? (
              <div className="text-center py-8 border border-dashed rounded-lg text-sm text-muted-foreground">
                No skills discovered in the skills folder. Add a folder with <code>SKILL.md</code> to activate!
              </div>
            ) : (
              <div className="space-y-2">
                {(skills || []).map((s) => (
                  <div
                    key={s.name}
                    className="flex items-center justify-between p-3 border rounded-lg bg-card/60 hover:bg-card transition-colors"
                  >
                    <div className="space-y-1 max-w-[80%]">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-sm">{s.name}</span>
                      </div>
                      <p className="text-xs text-muted-foreground">{s.description}</p>
                    </div>
                    <Switch
                      checked={s.enabled}
                      onCheckedChange={() => handleToggleSkill(s.name, s.enabled)}
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
