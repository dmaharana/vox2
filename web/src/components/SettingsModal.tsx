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
import { Tabs, TabsList, TabsTrigger, TabsContent } from './ui/tabs'
import { Badge } from './ui/badge'
import { ShieldCheck, KeyRound, Globe, Sparkles, Terminal, CheckCircle2, AlertCircle, Bot } from 'lucide-react'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export const SettingsModal: React.FC<Props> = ({ open, onOpenChange }) => {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [provider, setProvider] = useState<'openai' | 'copilot'>('openai')
  const [authType, setAuthType] = useState<'api_key' | 'oauth2'>('api_key')
  const [apiKey, setApiKey] = useState('')
  const [oauthClientId, setOauthClientId] = useState('')
  const [oauthClientSecret, setOauthClientSecret] = useState('')
  const [oauthTokenUrl, setOauthTokenUrl] = useState('')
  const [oauthScopes, setOauthScopes] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [model, setModel] = useState('')
  const [copilotBinary, setCopilotBinary] = useState('copilot')
  const [copilotTimeout, setCopilotTimeout] = useState(120)
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(4096)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (open) {
      fetchSettings().then((s) => {
        setSettings(s)
        const prov = (s.llm_provider || '').toLowerCase()
        if (prov === 'copilot' || prov === 'github-copilot' || prov === 'copilot-cli') {
          setProvider('copilot')
        } else {
          setProvider('openai')
        }
        setAuthType((s.llm_auth_type === 'oauth2' ? 'oauth2' : 'api_key'))
        setOauthClientId(s.llm_oauth_client_id || '')
        setOauthTokenUrl(s.llm_oauth_token_url || '')
        setOauthScopes(s.llm_oauth_scopes || '')
        setBaseUrl(s.llm_base_url)
        setModel(s.llm_model)
        setCopilotBinary(s.copilot_binary || 'copilot')
        setCopilotTimeout(s.copilot_timeout || 120)
        setTemperature(s.llm_temperature)
        setMaxTokens(s.llm_max_tokens)
      })
    }
  }, [open])

  const handleSave = async () => {
    setSaving(true)
    try {
      const updates: any = {
        llm_provider: provider,
        llm_base_url: baseUrl,
        llm_model: model,
        llm_auth_type: authType,
        llm_oauth_client_id: oauthClientId,
        llm_oauth_token_url: oauthTokenUrl,
        llm_oauth_scopes: oauthScopes,
        copilot_binary: copilotBinary,
        copilot_timeout: copilotTimeout,
        llm_temperature: temperature,
        llm_max_tokens: maxTokens,
      }
      if (apiKey) {
        updates.llm_api_key = apiKey
      }
      if (oauthClientSecret) {
        updates.llm_oauth_client_secret = oauthClientSecret
      }
      const updated = await updateSettings(updates)
      setSettings(updated)
      setApiKey('')
      setOauthClientSecret('')
      onOpenChange(false)
    } catch (e) {
      console.error(e)
    } finally {
      setSaving(false)
    }
  }

  const applyGooglePreset = () => {
    setAuthType('oauth2')
    setOauthTokenUrl('https://oauth2.googleapis.com/token')
    setOauthScopes('https://www.googleapis.com/auth/cloud-platform')
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="text-xl font-bold flex items-center gap-2">
            <KeyRound className="w-5 h-5 text-primary" /> LLM & Runtime Configuration
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-3">
          {/* Provider Selection */}
          <div className="space-y-2">
            <Label className="font-semibold text-sm">LLM Provider</Label>
            <Tabs value={provider} onValueChange={(v) => setProvider(v as 'openai' | 'copilot')}>
              <TabsList className="grid grid-cols-2 w-full">
                <TabsTrigger value="openai" className="gap-1.5 text-xs">
                  <Bot className="w-3.5 h-3.5" /> OpenAI-Compatible
                </TabsTrigger>
                <TabsTrigger value="copilot" className="gap-1.5 text-xs">
                  <Terminal className="w-3.5 h-3.5" /> GitHub Copilot (CLI)
                </TabsTrigger>
              </TabsList>
            </Tabs>
          </div>

          {provider === 'copilot' ? (
            /* GitHub Copilot CLI Settings */
            <div className="space-y-4 rounded-lg border border-primary/20 bg-primary/5 p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Terminal className="w-4 h-4 text-primary" />
                  <span className="text-sm font-semibold">GitHub Copilot CLI</span>
                </div>
                {settings?.copilot_installed ? (
                  <Badge variant="outline" className="text-emerald-500 border-emerald-500/30 bg-emerald-500/10 gap-1 text-[11px]">
                    <CheckCircle2 className="w-3 h-3" /> CLI Detected
                  </Badge>
                ) : (
                  <Badge variant="outline" className="text-amber-500 border-amber-500/30 bg-amber-500/10 gap-1 text-[11px]">
                    <AlertCircle className="w-3 h-3" /> CLI Not Found
                  </Badge>
                )}
              </div>

              {settings?.copilot_path && (
                <p className="text-xs text-muted-foreground font-mono bg-muted/60 px-2 py-1 rounded">
                  {settings.copilot_path}
                </p>
              )}

              <div className="space-y-2">
                <Label htmlFor="model">Model Name</Label>
                <Input
                  id="model"
                  placeholder="gpt-4o, claude-3.5-sonnet, gpt-5-mini, or leave empty"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                />
                <p className="text-[11px] text-muted-foreground">
                  Specifies the underlying Copilot model (e.g. <code>gpt-4o</code>, <code>claude-3.5-sonnet</code>, <code>gpt-5-mini</code>).
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="copilotBinary">Copilot Binary</Label>
                <Input
                  id="copilotBinary"
                  placeholder="copilot"
                  value={copilotBinary}
                  onChange={(e) => setCopilotBinary(e.target.value)}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="copilotTimeout">CLI Request Timeout (seconds)</Label>
                <Input
                  id="copilotTimeout"
                  type="number"
                  value={copilotTimeout}
                  onChange={(e) => setCopilotTimeout(parseInt(e.target.value) || 120)}
                />
              </div>

              <div className="text-xs space-y-1.5 text-muted-foreground bg-muted/40 p-2.5 rounded border">
                <p className="font-medium text-foreground">💡 Authentication & Latency Notes:</p>
                <p>• Authentication is handled directly by the CLI. Run <code className="text-primary font-semibold">copilot login</code> or set <code className="text-primary font-semibold">COPILOT_GITHUB_TOKEN</code> / <code className="text-primary font-semibold">GH_TOKEN</code>.</p>
                <p>• Copilot CLI acts as an autonomous agent; expect ~10–40s latency per turn.</p>
              </div>
            </div>
          ) : (
            /* OpenAI-Compatible Settings */
            <>
              <div className="space-y-2">
                <Label htmlFor="baseUrl">LLM Base URL</Label>
                <Input
                  id="baseUrl"
                  placeholder="https://api.openai.com/v1"
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                />
                <p className="text-xs text-muted-foreground">
                  Works with OpenAI, Google Vertex AI, Ollama, vLLM, Azure OpenAI, OpenRouter, etc.
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="model">Model Name</Label>
                <Input
                  id="model"
                  placeholder="gpt-4o, claude-3-5-sonnet, gemini-1.5-pro, llama3.2"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                />
              </div>

              {/* Authentication Mode Selection */}
              <div className="space-y-2 pt-1">
                <div className="flex items-center justify-between">
                  <Label className="font-semibold text-sm">Authentication Method</Label>
                  <span className="text-[11px] text-emerald-500 font-medium flex items-center gap-1">
                    <ShieldCheck className="w-3.5 h-3.5" /> AES-256 Encrypted in Database
                  </span>
                </div>

                <Tabs value={authType} onValueChange={(v) => setAuthType(v as 'api_key' | 'oauth2')}>
                  <TabsList className="grid grid-cols-2 w-full">
                    <TabsTrigger value="api_key" className="gap-1.5 text-xs">
                      <KeyRound className="w-3.5 h-3.5" /> API Key
                    </TabsTrigger>
                    <TabsTrigger value="oauth2" className="gap-1.5 text-xs">
                      <Globe className="w-3.5 h-3.5" /> OAuth 2.0 (Client Credentials)
                    </TabsTrigger>
                  </TabsList>

                  <TabsContent value="api_key" className="space-y-3 pt-2">
                    <div className="space-y-2">
                      <Label htmlFor="apiKey">
                        API Key {settings?.has_api_key && <span className="text-xs text-green-500 font-semibold">(Configured & Encrypted)</span>}
                      </Label>
                      <Input
                        id="apiKey"
                        type="password"
                        placeholder={settings?.has_api_key ? '••••••••••••••••' : 'Enter API Key if required'}
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                      />
                      <p className="text-[11px] text-muted-foreground">
                        Stored securely using AES-256-GCM encryption in the database.
                      </p>
                    </div>
                  </TabsContent>

                  <TabsContent value="oauth2" className="space-y-3 pt-2">
                    <div className="flex items-center justify-between pb-1">
                      <span className="text-xs text-muted-foreground">Quick Presets:</span>
                      <Button type="button" variant="outline" size="sm" onClick={applyGooglePreset} className="h-6 text-[11px] gap-1">
                        <Sparkles className="w-3 h-3 text-amber-500" /> Google Cloud / Vertex AI
                      </Button>
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="oauthClientId">Client ID</Label>
                      <Input
                        id="oauthClientId"
                        placeholder="e.g. 123456789.apps.googleusercontent.com"
                        value={oauthClientId}
                        onChange={(e) => setOauthClientId(e.target.value)}
                      />
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="oauthClientSecret">
                        Client Secret {settings?.has_oauth_client_secret && <span className="text-xs text-green-500 font-semibold">(Configured & Encrypted)</span>}
                      </Label>
                      <Input
                        id="oauthClientSecret"
                        type="password"
                        placeholder={settings?.has_oauth_client_secret ? '••••••••••••••••' : 'Enter OAuth Client Secret'}
                        value={oauthClientSecret}
                        onChange={(e) => setOauthClientSecret(e.target.value)}
                      />
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="oauthTokenUrl">OAuth2 Token Endpoint URL</Label>
                      <Input
                        id="oauthTokenUrl"
                        placeholder="https://oauth2.googleapis.com/token"
                        value={oauthTokenUrl}
                        onChange={(e) => setOauthTokenUrl(e.target.value)}
                      />
                    </div>

                    <div className="space-y-2">
                      <Label htmlFor="oauthScopes">Scopes (space or comma separated)</Label>
                      <Input
                        id="oauthScopes"
                        placeholder="https://www.googleapis.com/auth/cloud-platform"
                        value={oauthScopes}
                        onChange={(e) => setOauthScopes(e.target.value)}
                      />
                    </div>
                  </TabsContent>
                </Tabs>
              </div>

              <div className="space-y-2 pt-2 border-t">
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
            </>
          )}
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

