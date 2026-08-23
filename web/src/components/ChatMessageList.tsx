import React, { useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { ChatMessage, ToolCallState, SubflowState } from '../types'
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from './ui/accordion'
import { Badge } from './ui/badge'
import { Card, CardHeader, CardTitle, CardContent } from './ui/card'
import { Button } from './ui/button'
import {
  User,
  Bot,
  Wrench,
  CheckCircle,
  AlertCircle,
  Loader2,
  GitFork,
  Brain,
  ExternalLink,
  FileCode,
  Copy,
  Check,
} from 'lucide-react'

interface Props {
  messages: ChatMessage[]
  streaming: boolean
}

const PreBlock: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [copied, setCopied] = useState(false)
  const preRef = React.useRef<HTMLPreElement>(null)

  const handleCopyCode = async () => {
    if (preRef.current) {
      const text = preRef.current.innerText || ''
      try {
        await navigator.clipboard.writeText(text)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      } catch (err) {
        console.error('Failed to copy code', err)
      }
    }
  }

  return (
    <div className="relative group/code my-3 not-prose">
      <div className="absolute right-2 top-2 z-10 opacity-0 group-hover/code:opacity-100 transition-opacity">
        <Button
          size="sm"
          variant="secondary"
          className="h-6 px-2 text-[10px] gap-1 bg-background/80 hover:bg-background border shadow-xs cursor-pointer"
          onClick={handleCopyCode}
          title="Copy code block"
        >
          {copied ? (
            <>
              <Check className="w-3 h-3 text-emerald-500" /> Copied
            </>
          ) : (
            <>
              <Copy className="w-3 h-3" /> Copy
            </>
          )}
        </Button>
      </div>
      <pre
        ref={preRef}
        className="bg-muted/80 p-3.5 pt-4 rounded-xl border border-border/60 text-[12px] font-mono overflow-x-auto shadow-xs"
      >
        {children}
      </pre>
    </div>
  )
}

export const ChatMessageList: React.FC<Props> = ({ messages, streaming }) => {
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const handleCopy = async (id: string, text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedId(id)
      setTimeout(() => {
        setCopiedId((curr) => (curr === id ? null : curr))
      }, 2000)
    } catch (err) {
      console.error('Failed to copy to clipboard', err)
    }
  }
  return (
    <div className="space-y-6 pb-6">
      {messages.map((m) => (
        <div
          key={m.id}
          className={`flex gap-3.5 ${m.role === 'user' ? 'justify-end' : 'justify-start'}`}
        >
          {m.role !== 'user' && (
            <div className="w-8 h-8 rounded-full bg-primary/10 border border-primary/20 flex items-center justify-center shrink-0 mt-0.5">
              <Bot className="w-4 h-4 text-primary" />
            </div>
          )}

          <div className={`space-y-2 max-w-[85%] ${m.role === 'user' ? 'items-end' : 'items-start'}`}>
            {/* Memory Retrieved Notification Badge */}
            {m.memoriesRetrieved && m.memoriesRetrieved.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mb-1">
                {m.memoriesRetrieved.map((mem) => (
                  <Badge
                    key={mem.id}
                    variant="outline"
                    className="bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20 text-[11px] gap-1 py-0.5"
                  >
                    <Brain className="w-3 h-3" /> Context: {mem.key} ({mem.memory_type})
                  </Badge>
                ))}
              </div>
            )}

            {/* Subflow Event Cards */}
            {m.subflows && m.subflows.length > 0 && (
              <div className="space-y-2 my-2 w-full">
                {m.subflows.map((sf, idx) => (
                  <Card key={idx} className="border-blue-500/30 bg-blue-500/5">
                    <CardHeader className="p-3 pb-1.5 flex flex-row items-center justify-between">
                      <CardTitle className="text-xs font-semibold flex items-center gap-1.5 text-blue-600 dark:text-blue-400">
                        <GitFork className="w-3.5 h-3.5" /> Parallel Flow: {sf.task_name} ({sf.task_index}/{sf.total_tasks})
                      </CardTitle>
                      <Badge
                        variant={sf.status === 'completed' ? 'default' : sf.status === 'failed' ? 'destructive' : 'secondary'}
                        className="text-[10px]"
                      >
                        {sf.status}
                      </Badge>
                    </CardHeader>
                    {sf.summary && (
                      <CardContent className="p-3 pt-0 text-xs font-mono text-muted-foreground">
                        {sf.summary}
                      </CardContent>
                    )}
                  </Card>
                ))}
              </div>
            )}

            {/* Tool Call Cards Accordion */}
            {m.toolCalls && m.toolCalls.length > 0 && (
              <div className="w-full my-2">
                <Accordion type="multiple" className="w-full space-y-1.5">
                  {m.toolCalls.map((tc: ToolCallState) => (
                    <AccordionItem
                      key={tc.id}
                      value={tc.id}
                      className="border rounded-md px-3 bg-muted/30"
                    >
                      <AccordionTrigger className="py-2 hover:no-underline text-xs">
                        <div className="flex items-center gap-2">
                          <Wrench className="w-3.5 h-3.5 text-primary" />
                          <span className="font-mono font-semibold">{tc.tool}</span>
                          {tc.status === 'started' ? (
                            <Badge variant="outline" className="gap-1 text-[10px] bg-amber-500/10 text-amber-600 border-amber-500/30">
                              <Loader2 className="w-2.5 h-2.5 animate-spin" /> Executing
                            </Badge>
                          ) : tc.status === 'completed' ? (
                            <Badge variant="outline" className="gap-1 text-[10px] bg-emerald-500/10 text-emerald-600 border-emerald-500/30">
                              <CheckCircle className="w-2.5 h-2.5" /> Completed
                            </Badge>
                          ) : (
                            <Badge variant="destructive" className="gap-1 text-[10px]">
                              <AlertCircle className="w-2.5 h-2.5" /> Failed
                            </Badge>
                          )}
                        </div>
                      </AccordionTrigger>
                      <AccordionContent className="pt-1 pb-3 space-y-2 text-xs">
                        {tc.arguments && (
                          <div>
                            <div className="text-[11px] font-semibold text-muted-foreground">Input:</div>
                            <pre className="bg-muted p-2 rounded text-[11px] font-mono overflow-x-auto">
                              {typeof tc.arguments === 'string'
                                ? tc.arguments
                                : JSON.stringify(tc.arguments, null, 2)}
                            </pre>
                          </div>
                        )}
                        {tc.result && (
                          <div>
                            <div className="text-[11px] font-semibold text-muted-foreground">Result:</div>
                            <pre className="bg-muted p-2 rounded text-[11px] font-mono overflow-x-auto max-h-48">
                              {typeof tc.result === 'string'
                                ? tc.result
                                : JSON.stringify(tc.result, null, 2)}
                            </pre>
                          </div>
                        )}
                        {tc.error && (
                          <div className="text-destructive font-mono text-[11px]">
                            Error: {tc.error}
                          </div>
                        )}
                      </AccordionContent>
                    </AccordionItem>
                  ))}
                </Accordion>
              </div>
            )}

            {/* Main Message Content */}
            {m.content && (
              <div
                className={`p-3.5 rounded-2xl text-sm leading-relaxed ${
                  m.role === 'user'
                    ? 'bg-primary text-primary-foreground rounded-tr-sm ml-auto shadow-xs'
                    : 'bg-card border border-border/80 rounded-tl-sm shadow-xs'
                }`}
              >
                {m.role === 'user' ? (
                  <div className="group relative">
                    <div className="whitespace-pre-wrap">{m.content}</div>
                    <div className="flex justify-end pt-1 mt-1 border-t border-primary-foreground/15">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleCopy(m.id, m.content)}
                        className="h-5 px-1.5 text-[10px] text-primary-foreground/75 hover:text-primary-foreground hover:bg-primary-foreground/10 gap-1 rounded cursor-pointer"
                        title="Copy question"
                      >
                        {copiedId === m.id ? (
                          <>
                            <Check className="w-3 h-3 text-emerald-300" /> Copied
                          </>
                        ) : (
                          <>
                            <Copy className="w-3 h-3" /> Copy
                          </>
                        )}
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div>
                    <div className="prose prose-sm dark:prose-invert max-w-none break-words text-foreground">
                      <ReactMarkdown
                        remarkPlugins={[remarkGfm]}
                        urlTransform={(url) => url}
                        components={{
                          a: ({ href, children, ...props }) => {
                            const isFileLink = href?.startsWith('file://') || (href?.includes('.') && !href?.startsWith('http'))
                            return (
                              <a
                                href={href}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-primary font-medium underline underline-offset-3 hover:text-primary/80 inline-flex items-center gap-1 transition-colors px-1 py-0.5 rounded bg-primary/10 hover:bg-primary/20 text-[13px] break-all not-prose"
                                {...props}
                              >
                                {isFileLink ? (
                                  <FileCode className="w-3.5 h-3.5 inline-block shrink-0 text-primary opacity-80" />
                                ) : (
                                  <ExternalLink className="w-3.5 h-3.5 inline-block shrink-0 text-primary opacity-80" />
                                )}
                                <span>{children}</span>
                              </a>
                            )
                          },
                          code: ({ className, children, ...props }) => {
                            const isInline = !className && typeof children === 'string' && !children.includes('\n')
                            if (isInline) {
                              return (
                                <code className="bg-muted px-1.5 py-0.5 rounded text-[12px] font-mono font-medium text-foreground border border-border/40" {...props}>
                                  {children}
                                </code>
                              )
                            }
                            return (
                              <code className={className} {...props}>
                                {children}
                              </code>
                            )
                          },
                          pre: ({ children }) => <PreBlock>{children}</PreBlock>,
                        }}
                      >
                        {m.content}
                      </ReactMarkdown>
                    </div>

                    {/* Assistant Response Actions Toolbar */}
                    <div className="flex items-center justify-between pt-2.5 mt-2.5 border-t border-border/40 text-[11px] text-muted-foreground select-none">
                      <span className="text-[10px] opacity-60 font-mono">
                        {m.timestamp ? new Date(m.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : ''}
                      </span>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => handleCopy(m.id, m.content)}
                        className="h-6 px-2 text-[11px] text-muted-foreground hover:text-foreground hover:bg-accent/60 gap-1.5 rounded-md cursor-pointer"
                        title="Copy entire response"
                      >
                        {copiedId === m.id ? (
                          <>
                            <Check className="w-3 h-3 text-emerald-500" /> Copied Response
                          </>
                        ) : (
                          <>
                            <Copy className="w-3 h-3" /> Copy Response
                          </>
                        )}
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>

          {m.role === 'user' && (
            <div className="w-8 h-8 rounded-full bg-primary flex items-center justify-center shrink-0 mt-0.5 text-primary-foreground">
              <User className="w-4 h-4" />
            </div>
          )}
        </div>
      ))}

      {streaming && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground pt-1 pl-12">
          <Loader2 className="w-3.5 h-3.5 animate-spin text-primary" /> Generating response...
        </div>
      )}
    </div>
  )
}
