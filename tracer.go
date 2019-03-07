package main

import (
	"context"
	"fmt"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	opentracing "github.com/opentracing/opentracing-go"
)

type Tracer struct {
	result    TracingResult
	resolvers map[string]*TracingResolverResult
	rootSpan  opentracing.Span
}

func (t *Tracer) Init(ctx context.Context, p *graphql.Params) context.Context {
	return ctx
}

func (t *Tracer) Name() string {
	return "tracing"
}

func (t *Tracer) HasResult() bool {
	return false
}

func (t *Tracer) GetResult(ctx context.Context) interface{} {
	return nil
}

func (t *Tracer) ParseDidStart(ctx context.Context) (context.Context, graphql.ParseFinishFunc) {
	label := fmt.Sprint("parse")
	span, _ := opentracing.StartSpanFromContext(ctx, label)
	return ctx, func(err error) {
		if err != nil {
			span.SetTag("error", err)
		}
		span.Finish()
	}
}

func (t *Tracer) ValidationDidStart(ctx context.Context) (context.Context, graphql.ValidationFinishFunc) {
	label := fmt.Sprint("validation")
	span, _ := opentracing.StartSpanFromContext(ctx, label)
	return ctx, func(errs []gqlerrors.FormattedError) {
		span.SetTag("errors", errs)
		span.Finish()
	}
}

func (t *Tracer) ExecutionDidStart(ctx context.Context) (context.Context, graphql.ExecutionFinishFunc) {
	label := fmt.Sprint("execution")
	span, spanCtx := opentracing.StartSpanFromContext(ctx, label)
	return spanCtx, func(*graphql.Result) {
		span.Finish()
	}
}

func (t *Tracer) ResolveFieldDidStart(ctx context.Context, i *graphql.ResolveInfo) (context.Context, graphql.ResolveFieldFinishFunc) {
	label := fmt.Sprint(i.Path.AsArray())
	span := opentracing.StartSpan(label, opentracing.ChildOf(opentracing.SpanFromContext(ctx).Context()))
	return ctx, func(v interface{}, err error) {
		span.SetTag("error", err)
		span.SetTag("value", v)
		span.Finish()
	}
}

type TracingResult struct {
	Version    int                     `json:"version"`
	StartTime  time.Time               `json:"startTime"`
	EndTime    time.Time               `json:"endTime"`
	Duration   time.Duration           `json:"duration"`
	Parsing    TracingParsingResult    `json:"parsing"`
	Validation TracingValidationResult `json:"validation"`
	Execution  TracingExecutionResult  `json:"execution"`
}

type TracingParsingResult struct {
	StartOffset time.Duration `json:"startOffset"`
	Duration    time.Duration `json:"duration"`
}

type TracingValidationResult struct {
	StartOffset time.Duration `json:"startOffset"`
	Duration    time.Duration `json:"duration"`
}

type TracingExecutionResult struct {
	Resolvers []TracingResolverResult `json:"resolvers"`
}

type TracingResolverResult struct {
	Path        []interface{} `json:"path"`
	ParentType  string        `json:"parentType"`
	FieldName   string        `json:"fieldName"`
	ReturnType  string        `json:"returnType"`
	StartOffset time.Duration `json:"startOffset"`
	Duration    time.Duration `json:"duration"`
}
