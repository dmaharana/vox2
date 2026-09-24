import React, { useState, useEffect } from 'react'
import { CronJob } from '../types'
import {
  fetchCronJobs,
  createCronJob,
  toggleCronJob,
  runCronJob,
  deleteCronJob,
} from '../lib/api'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from './ui/dialog'
import { Switch } from './ui/switch'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Input } from './ui/input'
import {
  Clock,
  Play,
  Trash2,
  Plus,
  RefreshCw,
  MessageSquare,
  AlertCircle,
  CalendarClock,
  Sparkles,
  Terminal,
} from 'lucide-react'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  prefillIntent?: string
  onSelectConversation?: (convId: string) => void
}

const PRESETS = [
  { label: 'Every 5m', value: '*/5 * * * *' },
  { label: 'Every 10m', value: '*/10 * * * *' },
  { label: 'Every 15m', value: '*/15 * * * *' },
  { label: 'Hourly', value: '@hourly' },
  { label: 'Daily (9 AM)', value: '0 9 * * *' },
]

export const CronModal: React.FC<Props> = ({
  open,
  onOpenChange,
  prefillIntent = '',
  onSelectConversation,
}) => {
  const [jobs, setJobs] = useState<CronJob[]>([])
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState<Record<string, boolean>>({})

  // Form state
  const [name, setName] = useState('')
  const [schedule, setSchedule] = useState('*/10 * * * *')
  const [intent, setIntent] = useState('')

  const loadData = async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await fetchCronJobs()
      setJobs(Array.isArray(data) ? data : [])
    } catch (e: any) {
      setError(e.message || 'Failed to load cron jobs')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open) {
      loadData()
      if (prefillIntent) {
        setIntent(prefillIntent)
        if (!name) {
          setName(prefillIntent.startsWith('/') ? prefillIntent.slice(1) : prefillIntent)
        }
      }
    }
  }, [open, prefillIntent])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!schedule.trim() || !intent.trim()) {
      setError('Please provide both a schedule expression and target intent')
      return
    }

    setCreating(true)
    setError(null)
    try {
      const created = await createCronJob({
        name: name.trim() || intent.trim(),
        schedule: schedule.trim(),
        intent: intent.trim(),
      })
      setJobs((prev) => [...prev, created])
      setName('')
      setIntent('')
      setSchedule('*/10 * * * *')
    } catch (e: any) {
      setError(e.message || 'Failed to schedule job')
    } finally {
      setCreating(false)
    }
  }

  const handleToggle = async (job: CronJob) => {
    const nextState = !job.enabled
    try {
      const updated = await toggleCronJob(job.id, nextState)
      setJobs((prev) => prev.map((j) => (j.id === job.id ? updated : j)))
    } catch (e: any) {
      setError(e.message || 'Failed to toggle job')
    }
  }

  const handleRunNow = async (id: string) => {
    setActionLoading((prev) => ({ ...prev, [id]: true }))
    try {
      await runCronJob(id)
      setTimeout(loadData, 1000)
    } catch (e: any) {
      setError(e.message || 'Failed to run job')
    } finally {
      setActionLoading((prev) => ({ ...prev, [id]: false }))
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this scheduled cron job?')) {
      return
    }
    try {
      await deleteCronJob(id)
      setJobs((prev) => prev.filter((j) => j.id !== id))
    } catch (e: any) {
      setError(e.message || 'Failed to delete job')
    }
  }

  const handleViewChat = (convId: string) => {
    if (onSelectConversation && convId) {
      onSelectConversation(convId)
      onOpenChange(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl max-h-[85vh] overflow-y-auto">
        <DialogHeader className="flex flex-row items-center justify-between pb-2 border-b border-border/50">
          <div className="flex items-center gap-2.5">
            <div className="p-2 bg-primary/10 rounded-lg text-primary">
              <CalendarClock className="w-5 h-5" />
            </div>
            <div>
              <DialogTitle className="text-base font-semibold">
                Scheduled Cron Jobs
              </DialogTitle>
              <p className="text-xs text-muted-foreground">
                Automate periodic execution of skills, tools, and custom agent intents
              </p>
            </div>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={loadData}
            disabled={loading}
            className="h-8 gap-1.5 text-xs text-muted-foreground hover:text-foreground"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>
        </DialogHeader>

        {error && (
          <div className="flex items-center gap-2 p-3 text-xs rounded-lg bg-destructive/10 text-destructive border border-destructive/20 my-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span className="truncate">{error}</span>
          </div>
        )}

        {/* Schedule Creation Section */}
        <form
          onSubmit={handleCreate}
          className="p-3.5 rounded-xl border border-border/60 bg-muted/30 space-y-3 mt-1"
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold flex items-center gap-1.5 text-foreground">
              <Plus className="w-3.5 h-3.5 text-primary" />
              Schedule New Intent or Skill
            </span>
            <span className="text-[11px] text-muted-foreground">
              Single-Flight overlap safe
            </span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="text-[11px] font-medium text-muted-foreground block mb-1">
                Target Intent or Skill Command
              </label>
              <div className="relative">
                <Terminal className="w-3.5 h-3.5 absolute left-2.5 top-2.5 text-muted-foreground" />
                <Input
                  value={intent}
                  onChange={(e) => setIntent(e.target.value)}
                  placeholder="/check-stock-price AMD or prompt"
                  className="h-8 pl-8 text-xs font-mono"
                  required
                />
              </div>
            </div>

            <div>
              <label className="text-[11px] font-medium text-muted-foreground block mb-1">
                Friendly Job Name (Optional)
              </label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. AMD Stock Monitor"
                className="h-8 text-xs"
              />
            </div>
          </div>

          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="text-[11px] font-medium text-muted-foreground">
                Cron Schedule Expression
              </label>
              <div className="flex items-center gap-1">
                {PRESETS.map((p) => (
                  <button
                    key={p.label}
                    type="button"
                    onClick={() => setSchedule(p.value)}
                    className={`text-[10px] px-1.5 py-0.5 rounded border transition-colors ${
                      schedule === p.value
                        ? 'bg-primary text-primary-foreground border-primary'
                        : 'bg-background hover:bg-muted text-muted-foreground border-border/60'
                    }`}
                  >
                    {p.label}
                  </button>
                ))}
              </div>
            </div>
            <div className="relative">
              <Clock className="w-3.5 h-3.5 absolute left-2.5 top-2.5 text-muted-foreground" />
              <Input
                value={schedule}
                onChange={(e) => setSchedule(e.target.value)}
                placeholder="*/10 * * * * or @every 10m"
                className="h-8 pl-8 text-xs font-mono"
                required
              />
            </div>
          </div>

          <div className="flex justify-end pt-1">
            <Button
              type="submit"
              size="sm"
              disabled={creating || !intent.trim()}
              className="h-8 text-xs gap-1.5"
            >
              <Plus className="w-3.5 h-3.5" />
              {creating ? 'Scheduling...' : 'Create Cron Job'}
            </Button>
          </div>
        </form>

        {/* Existing Cron Jobs List */}
        <div className="space-y-2.5 mt-2">
          <div className="flex items-center justify-between text-xs font-medium text-muted-foreground px-0.5">
            <span>Active & Stored Schedules</span>
            <span>{jobs.length} jobs</span>
          </div>

          {jobs.length === 0 ? (
            <div className="text-center py-8 border border-dashed rounded-xl border-border/60 text-muted-foreground space-y-2">
              <Clock className="w-8 h-8 mx-auto stroke-1 opacity-40" />
              <div className="text-xs">No scheduled cron jobs found.</div>
              <div className="text-[11px] text-muted-foreground/80 max-w-sm mx-auto">
                Schedule a job above or type{' '}
                <code className="bg-muted px-1.5 py-0.5 rounded font-mono text-[10px]">
                  /cron "*/10 * * * *" -- /check-stock-price AMD
                </code>{' '}
                in chat.
              </div>
            </div>
          ) : (
            <div className="space-y-2">
              {jobs.map((job, idx) => {
                const isRunning = job.last_status === 'running' || actionLoading[job.id]
                let statusBadge = (
                  <Badge variant="outline" className="text-[10px] text-emerald-500 border-emerald-500/30 bg-emerald-500/10">
                    Active
                  </Badge>
                )
                if (!job.enabled) {
                  statusBadge = (
                    <Badge variant="outline" className="text-[10px] text-amber-500 border-amber-500/30 bg-amber-500/10">
                      Paused
                    </Badge>
                  )
                } else if (isRunning) {
                  statusBadge = (
                    <Badge variant="outline" className="text-[10px] text-cyan-400 border-cyan-400/30 bg-cyan-400/10 animate-pulse">
                      Running
                    </Badge>
                  )
                } else if (job.last_status === 'error') {
                  statusBadge = (
                    <Badge variant="outline" className="text-[10px] text-rose-500 border-rose-500/30 bg-rose-500/10">
                      Failed
                    </Badge>
                  )
                }

                return (
                  <div
                    key={job.id}
                    className="p-3 rounded-xl border border-border/60 bg-card hover:bg-muted/20 transition-colors flex flex-col md:flex-row md:items-center justify-between gap-3"
                  >
                    <div className="min-w-0 space-y-1.5 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-[11px] font-mono font-medium text-muted-foreground">
                          #{idx + 1}
                        </span>
                        <span className="text-xs font-semibold text-foreground truncate">
                          {job.name}
                        </span>
                        {statusBadge}
                      </div>

                      <div className="flex flex-wrap items-center gap-2 text-[11px]">
                        <span className="inline-flex items-center gap-1 font-mono px-2 py-0.5 rounded bg-muted/80 text-foreground border border-border/40">
                          <Clock className="w-3 h-3 text-primary" />
                          {job.schedule}
                        </span>

                        <span className="inline-flex items-center gap-1 font-mono px-2 py-0.5 rounded bg-purple-500/10 text-purple-400 border border-purple-500/20 truncate max-w-xs">
                          <Sparkles className="w-3 h-3 shrink-0" />
                          {job.intent}
                        </span>
                      </div>

                      <div className="flex items-center gap-3 text-[10px] text-muted-foreground pt-0.5">
                        <span>
                          Next run:{' '}
                          <strong className="text-foreground/80 font-mono">
                            {job.next_run ? new Date(job.next_run).toLocaleTimeString() : 'Paused'}
                          </strong>
                        </span>
                        {job.last_run && (
                          <span>
                            Last run:{' '}
                            <strong className="text-foreground/80 font-mono">
                              {new Date(job.last_run).toLocaleTimeString()}
                            </strong>
                          </span>
                        )}
                        {job.last_error && (
                          <span className="text-destructive truncate max-w-xs" title={job.last_error}>
                            Error: {job.last_error}
                          </span>
                        )}
                      </div>
                    </div>

                    <div className="flex items-center gap-2 shrink-0 self-end md:self-center">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleRunNow(job.id)}
                        disabled={isRunning}
                        title="Run this job immediately"
                        className="h-8 px-2.5 text-xs gap-1 text-muted-foreground hover:text-foreground"
                      >
                        <Play className={`w-3.5 h-3.5 ${isRunning ? 'animate-spin' : ''}`} />
                        Run Now
                      </Button>

                      {job.conversation_id && onSelectConversation && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleViewChat(job.conversation_id)}
                          title="Open dedicated conversation thread"
                          className="h-8 px-2.5 text-xs gap-1 text-muted-foreground hover:text-foreground"
                        >
                          <MessageSquare className="w-3.5 h-3.5" />
                          Chat
                        </Button>
                      )}

                      <div className="flex items-center gap-1.5 pl-1 border-l border-border/50">
                        <Switch
                          checked={job.enabled}
                          onCheckedChange={() => handleToggle(job)}
                          title={job.enabled ? 'Pause schedule' : 'Enable schedule'}
                        />

                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => handleDelete(job.id)}
                          className="h-8 w-8 text-muted-foreground hover:text-destructive"
                          title="Delete schedule"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </Button>
                      </div>
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
