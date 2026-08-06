package esper

import "time"

// CurrentTimestamp returns the current statement time as epoch milliseconds.
// It follows Esper's current_timestamp() result type and uses EvalContext.Now
// so virtual-clock and fire-and-forget evaluation remain deterministic.
func CurrentTimestamp() Expression[int64] {
	return makeExpr[int64]("current-timestamp", "current_timestamp()", nil, func(ctx EvalContext) Value {
		return Present(ctx.Now.UnixNano() / int64(time.Millisecond))
	})
}
