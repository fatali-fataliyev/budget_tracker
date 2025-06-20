package contextutil

import "context"

type contextKey string

const TraceIDKey contextKey = "traceID"

func TraceIDFromContext(ctx context.Context) string {
	traceID, ok := ctx.Value(TraceIDKey).(string)
	if !ok {
		return "unknown-trace-id"
	}
	return traceID
}
