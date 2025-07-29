package contextutil

import "context"

type contextKey string

const TraceIDKey contextKey = "traceID"
const Token contextKey = "token"

func TraceIDFromContext(ctx context.Context) string {
	traceID, ok := ctx.Value(TraceIDKey).(string)
	if !ok {
		return "unknown-trace-id"
	}
	return traceID
}
