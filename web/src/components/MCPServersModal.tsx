import React, { useState, useEffect } from 'react'
import { MCPServer } from '../types'
import {
  fetchMCPServers,
  addMCPServer,
  updateMCPServer,
  connectMCPServer,
  disconnectMCPServer,
  deleteMCPServer,
} from '../lib/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Tabs, TabsList, TabsTrigger, TabsContent } from './ui/tabs'
import { Badge } from './ui/badge'
import { Server, Plus, Trash2, Plug, Unplug, CheckCircle, AlertCircle, RefreshCw, Pencil, X } from 'lucide-react'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export const MCPServersModal: React.FC<Props> = ({ open, onOpenChange }) => {
  const [servers, setServers] = useState<MCPServer[]>([])
  const [loading, setLoading] = useState(false)

  // Form & Edit State
  const [editingServerId, setEditingServerId] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [transport, setTransport] = useState<'stdio' | 'sse'>('stdio')
  const [command, setCommand] = useState('')
  const [argsStr, setArgsStr] = useState('')
  const [url, setUrl] = useState('')
  const [envStr, setEnvStr] = useState('')
  const [headersStr, setHeadersStr] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const loadServers = async () => {
    setLoading(true)
    try {
      const list = await fetchMCPServers()
      setServers(Array.isArray(list) ? list : [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open) {
      loadServers()
    }
  }, [open])

  const handleStartEdit = (s: MCPServer) => {
    setEditingServerId(s.id)
    setName(s.name)
    setTransport(s.transport === 'sse' || s.transport === 'http' ? 'sse' : 'stdio')
    setCommand(s.command || '')
    setArgsStr((s.args || []).join(' '))
    setUrl(s.url || '')
    setEnvStr(
      s.env ? Object.entries(s.env).map(([k, v]) => `${k}=${v}`).join(', ') : ''
    )
    setHeadersStr(
      s.headers ? Object.entries(s.headers).map(([k, v]) => `${k}=${v}`).join(', ') : ''
    )
  }

  const handleCancelEdit = () => {
    setEditingServerId(null)
    setName('')
    setCommand('')
    setArgsStr('')
    setUrl('')
    setEnvStr('')
    setHeadersStr('')
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsSubmitting(true)
    try {
      const args = argsStr.trim() ? argsStr.trim().split(' ') : []
      const env: Record<string, string> = {}
      if (envStr.trim()) {
        envStr.split(',').forEach((pair) => {
          const [k, v] = pair.split('=')
          if (k && v) env[k.trim()] = v.trim()
        })
      }

      const headers: Record<string, string> = {}
      if (headersStr.trim()) {
        headersStr.split(/[\n,]+/).forEach((pair) => {
          let sepIdx = pair.indexOf(':')
          if (sepIdx === -1) {
            sepIdx = pair.indexOf('=')
          }
          if (sepIdx !== -1) {
            const k = pair.substring(0, sepIdx).trim()
            const v = pair.substring(sepIdx + 1).trim()
            if (k && v) headers[k] = v
          }
        })
      }

      const payload = {
        name,
        transport,
        command: transport === 'stdio' ? command : undefined,
        args: transport === 'stdio' ? args : undefined,
        env: transport === 'stdio' ? env : undefined,
        url: transport === 'sse' ? url : undefined,
        headers: transport === 'sse' && Object.keys(headers).length > 0 ? headers : undefined,
        enabled: true,
      }

      if (editingServerId) {
        await updateMCPServer(editingServerId, payload)
      } else {
        await addMCPServer(payload)
      }

      handleCancelEdit()
      await loadServers()
    } catch (err: any) {
      alert(err.message)
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleToggleConnect = async (s: MCPServer) => {
    if (s.status === 'connected') {
      await disconnectMCPServer(s.id)
    } else {
      try {
        await connectMCPServer(s.id)
      } catch (e: any) {
        alert(e.message)
      }
    }
    await loadServers()
  }

  const handleDelete = async (id: string) => {
    if (confirm('Delete this MCP Server?')) {
      await deleteMCPServer(id)
      await loadServers()
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
        <DialogHeader className="flex flex-row items-center justify-between">
          <DialogTitle className="text-xl font-bold flex items-center gap-2">
            <Server className="w-5 h-5 text-primary" /> Model Context Protocol (MCP) Servers
          </DialogTitle>
          <Button variant="ghost" size="icon" onClick={loadServers} disabled={loading}>
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </Button>
        </DialogHeader>

        {/* Existing Servers List */}
        <div className="space-y-3 my-2">
          <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">
            Connected & Configured Servers ({(servers || []).length})
          </h3>

          {(servers || []).length === 0 ? (
            <div className="text-center py-6 border border-dashed rounded-lg text-muted-foreground text-sm">
              No MCP servers configured yet. Add a stdio or SSE server below!
            </div>
          ) : (
            <div className="space-y-2">
              {(servers || []).map((s) => (
                <div
                  key={s.id}
                  className="flex items-center justify-between p-3 rounded-lg border bg-card/60 hover:bg-card transition-colors"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-sm">{s.name}</span>
                      <Badge variant="outline" className="text-xs uppercase">
                        {s.transport}
                      </Badge>
                      {s.status === 'connected' ? (
                        <Badge variant="default" className="bg-emerald-600 gap-1 text-xs">
                          <CheckCircle className="w-3 h-3" /> Connected
                        </Badge>
                      ) : s.status === 'error' ? (
                        <Badge variant="destructive" className="gap-1 text-xs" title={s.last_error}>
                          <AlertCircle className="w-3 h-3" /> Error
                        </Badge>
                      ) : (
                        <Badge variant="secondary" className="text-xs">
                          Disconnected
                        </Badge>
                      )}
                    </div>
                    <div className="text-xs text-muted-foreground font-mono truncate max-w-md">
                      {s.transport === 'stdio'
                        ? `${s.command} ${(s.args || []).join(' ')}`
                        : s.url}
                    </div>
                    {s.headers && Object.keys(s.headers).length > 0 && (
                      <div className="text-[11px] text-muted-foreground flex items-center gap-1 mt-0.5 flex-wrap">
                        <span className="font-semibold text-foreground/80 text-[10px]">Headers:</span>
                        {Object.keys(s.headers).map((k) => (
                          <Badge key={k} variant="outline" className="text-[10px] py-0 px-1 font-mono">
                            {k}
                          </Badge>
                        ))}
                      </div>
                    )}
                  </div>

                  <div className="flex items-center gap-1.5">
                    <Button
                      size="sm"
                      variant={s.status === 'connected' ? 'secondary' : 'default'}
                      onClick={() => handleToggleConnect(s)}
                    >
                      {s.status === 'connected' ? (
                        <>
                          <Unplug className="w-3.5 h-3.5 mr-1" /> Disconnect
                        </>
                      ) : (
                        <>
                          <Plug className="w-3.5 h-3.5 mr-1" /> Connect
                        </>
                      )}
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      title="Edit Server Configuration"
                      className="hover:bg-primary/10 text-primary"
                      onClick={() => handleStartEdit(s)}
                    >
                      <Pencil className="w-4 h-4" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="text-destructive hover:bg-destructive/10"
                      onClick={() => handleDelete(s.id)}
                    >
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Add / Edit Server Form */}
        <div className="border-t pt-4 mt-4">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-sm font-semibold flex items-center gap-2">
              {editingServerId ? (
                <>
                  <Pencil className="w-4 h-4 text-primary" /> Edit MCP Server Configuration
                </>
              ) : (
                <>
                  <Plus className="w-4 h-4 text-primary" /> Add New MCP Server
                </>
              )}
            </h3>
            {editingServerId && (
              <Button size="sm" variant="ghost" onClick={handleCancelEdit} className="h-7 text-xs gap-1">
                <X className="w-3.5 h-3.5" /> Cancel Edit
              </Button>
            )}
          </div>

          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="srvName">Server Name</Label>
              <Input
                id="srvName"
                placeholder="e.g. Filesystem MCP, PostgreSQL MCP"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </div>

            <Tabs value={transport} onValueChange={(v) => setTransport(v as 'stdio' | 'sse')}>
              <TabsList className="grid grid-cols-2 w-full">
                <TabsTrigger value="stdio">Stdio Process</TabsTrigger>
                <TabsTrigger value="sse">HTTP / SSE Stream</TabsTrigger>
              </TabsList>

              <TabsContent value="stdio" className="space-y-3 pt-2">
                <div className="space-y-2">
                  <Label htmlFor="command">Command / Executable</Label>
                  <Input
                    id="command"
                    placeholder="npx, python, /usr/local/bin/mcp-server"
                    value={command}
                    onChange={(e) => setCommand(e.target.value)}
                    required={transport === 'stdio'}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="args">Arguments (space separated)</Label>
                  <Input
                    id="args"
                    placeholder="-y @modelcontextprotocol/server-everything"
                    value={argsStr}
                    onChange={(e) => setArgsStr(e.target.value)}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="env">Environment Variables (KEY=VAL, comma separated)</Label>
                  <Input
                    id="env"
                    placeholder="DEBUG=1, API_KEY=secret"
                    value={envStr}
                    onChange={(e) => setEnvStr(e.target.value)}
                  />
                </div>
              </TabsContent>

              <TabsContent value="sse" className="space-y-3 pt-2">
                <div className="space-y-2">
                  <Label htmlFor="url">SSE / Stream Endpoint URL</Label>
                  <Input
                    id="url"
                    placeholder="http://localhost:3000/sse"
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    required={transport === 'sse'}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="headers">Custom HTTP Headers (KEY=VAL or Key: Value, comma/newline separated)</Label>
                  <Input
                    id="headers"
                    placeholder="Authorization=Bearer secret_token, X-API-Key=custom123"
                    value={headersStr}
                    onChange={(e) => setHeadersStr(e.target.value)}
                  />
                  <p className="text-[11px] text-muted-foreground">
                    Sent with every SSE/HTTP request (e.g. Authorization, X-API-Key, session cookies, etc.).
                  </p>
                </div>
              </TabsContent>
            </Tabs>

            <Button type="submit" className="w-full" disabled={isSubmitting}>
              {isSubmitting
                ? editingServerId
                  ? 'Updating & Reconnecting...'
                  : 'Adding & Connecting...'
                : editingServerId
                ? 'Update MCP Server'
                : 'Add & Connect MCP Server'}
            </Button>
          </form>
        </div>
      </DialogContent>
    </Dialog>
  )
}
