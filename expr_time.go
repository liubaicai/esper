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

// WindowCurrentCount exposes expression-view current_count as a typed
// aggregate-independent expression. It is valid only while an
// ExpressionWindow or ExpressionBatch predicate is evaluating.
func WindowCurrentCount() Expression[int64] {
	return makeExpr[int64]("window-current-count", "window.current_count", nil, func(ctx EvalContext) Value {
		return Present(int64(len(windowContextEvents(ctx))))
	})
}

// WindowOldestTimestamp and WindowNewestTimestamp expose retained event
// timestamps in epoch milliseconds, matching Esper expression-view numeric
// built-ins.
func WindowOldestTimestamp() Expression[int64] {
	return makeExpr[int64]("window-oldest-timestamp", "window.oldest_timestamp", nil, func(ctx EvalContext) Value {
		events := windowContextEvents(ctx)
		if len(events) == 0 {
			return Null()
		}
		return Present(events[0].ReceivedAt().UnixNano() / int64(time.Millisecond))
	})
}

func WindowNewestTimestamp() Expression[int64] {
	return makeExpr[int64]("window-newest-timestamp", "window.newest_timestamp", nil, func(ctx EvalContext) Value {
		events := windowContextEvents(ctx)
		if len(events) == 0 {
			return Null()
		}
		return Present(events[len(events)-1].ReceivedAt().UnixNano() / int64(time.Millisecond))
	})
}

// WindowExpiredCount is the number of rows already removed during the current
// expression-window expiry pass. Expression-batch predicates always see zero.
func WindowExpiredCount() Expression[int64] {
	return makeExpr[int64]("window-expired-count", "window.expired_count", nil, func(ctx EvalContext) Value {
		return Present(ctx.WindowExpiredCount)
	})
}

// WindowReference supplies a detached snapshot of the current view contents
// to Go UDFs. Callers cannot mutate runtime state through the returned slice.
func WindowReference() Expression[[]Event] {
	return makeExpr[[]Event]("window-reference", "window.reference", nil, func(ctx EvalContext) Value {
		return Present(append([]Event(nil), windowContextEvents(ctx)...))
	})
}

func WindowOldestEvent() Expression[Event] {
	return windowBoundaryEvent("window-oldest-event", true)
}

func WindowNewestEvent() Expression[Event] {
	return windowBoundaryEvent("window-newest-event", false)
}

func windowBoundaryEvent(kind string, oldest bool) Expression[Event] {
	return makeExpr[Event](kind, kind, nil, func(ctx EvalContext) Value {
		events := windowContextEvents(ctx)
		if len(events) == 0 {
			return Null()
		}
		if oldest {
			return Present(events[0])
		}
		return Present(events[len(events)-1])
	})
}

func windowContextEvents(ctx EvalContext) []Event {
	if ctx.WindowReference != nil {
		return ctx.WindowReference
	}
	return ctx.Group
}
