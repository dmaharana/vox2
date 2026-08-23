import React from 'react'
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
import { User, Bot, Wrench, CheckCircle, AlertCircle, Loader2, GitFork, Brain } from 'lucide-react'

interface Props {
  messages: ChatMessage[]
  streaming: boolean
}

export const ChatMessageList: React.FC<Props> = ({ messages, streaming }) => {
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
                      <CardContent className="p-3 pt-0 text-xs text-muted-foreground">
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
                    ? 'bg-primary text-primary-foreground rounded-tr-sm ml-auto'
                    : 'bg-card border border-border/80 rounded-tl-sm shadow-xs'
                }`}
              >
                {m.role === 'user' ? (
                  <div className="whitespace-pre-wrap">{m.content}</div>
                ) : (
                  <div className="prose prose-sm dark:prose-invert max-w-none break-words">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>
                      {m.content}
                    </ReactMarkdown>
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
