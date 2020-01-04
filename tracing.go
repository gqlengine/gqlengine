// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"context"
	"time"

	"github.com/karfield/graphql/gqlerrors"

	"github.com/karfield/graphql"
)

type TracingRecord struct {
	StartOffset int64 `json:"startOffset"`
	Duration    int64 `json:"duration"`
}

type TracingResolveRecord struct {
	TracingRecord
	Path       []interface{} `json:"path"`
	ParentType string        `json:"parentType"`
	FieldName  string        `json:"fieldName"`
	ReturnType string        `json:"returnType"`
}

type TracingExecution struct {
	TracingRecord
	Resolvers []*TracingResolveRecord `json:"resolvers"`
}

type Tracing struct {
	Version    int              `json:"version"`
	StartTime  time.Time        `json:"startTime"`
	EndTime    time.Time        `json:"endTime"`
	Duration   int64            `json:"duration"`
	Parsing    TracingRecord    `json:"parsing"`
	Validation TracingRecord    `json:"validation"`
	Execution  TracingExecution `json:"execution"`
}

type tracingContextKey struct{}

type tracingExtension struct{}

func (t *tracingExtension) Init(ctx context.Context, params *graphql.Params) context.Context {
	return context.WithValue(ctx, tracingContextKey{}, &Tracing{
		Version:   1,
		StartTime: time.Now(),
	})
}

func (t *tracingExtension) Name() string {
	return "tracing"
}

func (t *tracingExtension) ParseDidStart(ctx context.Context) (context.Context, graphql.ParseFinishFunc) {
	tracing := ctx.Value(tracingContextKey{}).(*Tracing)
	startTs := time.Now()
	tracing.Parsing.StartOffset = startTs.Sub(tracing.StartTime).Milliseconds()
	return ctx, func(err error) {
		tracing.Parsing.Duration = time.Now().Sub(startTs).Milliseconds()
	}
}

func (t *tracingExtension) ValidationDidStart(ctx context.Context) (context.Context, graphql.ValidationFinishFunc) {
	tracing := ctx.Value(tracingContextKey{}).(*Tracing)
	startTs := time.Now()
	tracing.Validation.StartOffset = startTs.Sub(tracing.StartTime).Milliseconds()
	return ctx, func(errors []gqlerrors.FormattedError) {
		tracing.Validation.Duration = time.Now().Sub(startTs).Milliseconds()
	}
}

func (t *tracingExtension) ExecutionDidStart(ctx context.Context) (context.Context, graphql.ExecutionFinishFunc) {
	tracing := ctx.Value(tracingContextKey{}).(*Tracing)
	startTs := time.Now()
	tracing.Execution.StartOffset = startTs.Sub(tracing.StartTime).Milliseconds()
	return ctx, func(result *graphql.Result) {
		tracing.EndTime = time.Now()
		tracing.Execution.Duration = tracing.EndTime.Sub(startTs).Milliseconds()
	}
}

func (t *tracingExtension) ResolveFieldDidStart(ctx context.Context, info *graphql.ResolveInfo) (context.Context, graphql.ResolveFieldFinishFunc) {
	tracing := ctx.Value(tracingContextKey{}).(*Tracing)
	startTs := time.Now()
	record := &TracingResolveRecord{
		TracingRecord: TracingRecord{
			StartOffset: startTs.Sub(tracing.StartTime).Milliseconds(),
		},
		Path:       info.Path.AsArray(),
		ParentType: info.ParentType.String(),
		FieldName:  info.FieldName,
		ReturnType: info.ReturnType.String(),
	}
	tracing.Execution.Resolvers = append(tracing.Execution.Resolvers, record)
	return ctx, func(i interface{}, err error) {
		record.TracingRecord.Duration = time.Now().Sub(startTs).Milliseconds()
	}
}

func (t *tracingExtension) HasResult() bool {
	return true
}

func (t *tracingExtension) GetResult(ctx context.Context) interface{} {
	return ctx.Value(tracingContextKey{})
}
