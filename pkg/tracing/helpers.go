package tracing

import (
	"context"
	"encoding/json"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan starts a new child span using the harness tracer.
func StartSpan(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return Tracer().Start(ctx, spanName, opts...)
}

// TraceLLMCall records an LLM chat completion invocation span.
func TraceLLMCall(ctx context.Context, model, prompt string, duration time.Duration, promptTokens, completionTokens int, err error) {
	_, span := Tracer().Start(ctx, "llm.completion",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("llm.model", model),
			attribute.Int("llm.prompt_tokens", promptTokens),
			attribute.Int("llm.completion_tokens", completionTokens),
			attribute.Int64("llm.duration_ms", duration.Milliseconds()),
		),
	)
	defer span.End()

	if len(prompt) > 200 {
		span.SetAttributes(attribute.String("llm.prompt_preview", prompt[:200]+"..."))
	} else {
		span.SetAttributes(attribute.String("llm.prompt_preview", prompt))
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "success")
	}
}

// TraceToolCall records a tool execution span.
func TraceToolCall(ctx context.Context, toolName string, args any, result any, err error) {
	_, span := Tracer().Start(ctx, "tool."+toolName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("tool.name", toolName),
		),
	)
	defer span.End()

	if argsBytes, e := json.Marshal(args); e == nil {
		span.SetAttributes(attribute.String("tool.args", string(argsBytes)))
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		if resBytes, e := json.Marshal(result); e == nil {
			if len(resBytes) > 500 {
				span.SetAttributes(attribute.String("tool.result_preview", string(resBytes[:500])+"..."))
			} else {
				span.SetAttributes(attribute.String("tool.result", string(resBytes)))
			}
		}
		span.SetStatus(codes.Ok, "success")
	}
}

// TraceSkillUse records dynamic skill invocation.
func TraceSkillUse(ctx context.Context, skillName string, action string, err error) {
	_, span := Tracer().Start(ctx, "skill."+skillName,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("skill.name", skillName),
			attribute.String("skill.action", action),
		),
	)
	defer span.End()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "success")
	}
}

// TraceFlow records a parallel subflow execution.
func TraceFlow(ctx context.Context, flowID string, taskCount int, err error) {
	_, span := Tracer().Start(ctx, "flow.parallel_execution",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("flow.id", flowID),
			attribute.Int("flow.task_count", taskCount),
		),
	)
	defer span.End()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "success")
	}
}
