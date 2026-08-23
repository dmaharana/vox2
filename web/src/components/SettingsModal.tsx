import React, { useState, useEffect } from 'react'
import { Settings } from '../types'
import { fetchSettings, updateSettings } from '../lib/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from './ui/dialog'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Slider } from './ui/slider'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export const SettingsModal: React.FC<Props> = ({ open, onOpenChange }) => {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [model, setModel] = useState('')
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(4096)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (open) {
      fetchSettings().then((s) => {
        setSettings(s)
        setBaseUrl(s.llm_base_url)
        setModel(s.llm_model)
        setTemperature(s.llm_temperature)
        setMaxTokens(s.llm_max_tokens)
      })
    }
  }, [open])

  const handleSave = async () => {
    setSaving(true)
    try {
      const updates: any = {
        llm_base_url: baseUrl,
        llm_model: model,
        llm_temperature: temperature,
        llm_max_tokens: maxTokens,
      }
      if (apiKey) {
        updates.llm_api_key = apiKey
      }
      const updated = await updateSettings(updates)
      setSettings(updated)
      setApiKey('')
      onOpenChange(false)
    } catch (e) {
      console.error(e)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-xl font-bold">LLM & Runtime Configuration</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-3">
          <div className="space-y-2">
            <Label htmlFor="baseUrl">LLM Base URL</Label>
            <Input
              id="baseUrl"
              placeholder="https://api.openai.com/v1"
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">
              Works with OpenAI, Ollama (http://localhost:11434/v1), vLLM, LMStudio, OpenRouter, etc.
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="model">Model Name</Label>
            <Input
              id="model"
              placeholder="gpt-4o, claude-3-5-sonnet, llama3.2"
              value={model}
              onChange={(e) => setModel(e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="apiKey">
              API Key {settings?.has_api_key && <span className="text-xs text-green-500 font-semibold">(Configured)</span>}
            </Label>
            <Input
              id="apiKey"
              type="password"
              placeholder={settings?.has_api_key ? '••••••••••••••••' : 'Enter API Key if required'}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
            />
          </div>

          <div className="space-y-2">
            <div className="flex justify-between">
              <Label>Temperature: {temperature.toFixed(2)}</Label>
            </div>
            <Slider
              value={[temperature]}
              min={0}
              max={1.5}
              step={0.05}
              onValueChange={(val) => setTemperature(val[0])}
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="maxTokens">Max Completion Tokens</Label>
            <Input
              id="maxTokens"
              type="number"
              value={maxTokens}
              onChange={(e) => setMaxTokens(parseInt(e.target.value) || 2048)}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? 'Saving...' : 'Save Settings'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
