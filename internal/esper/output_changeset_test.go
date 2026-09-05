package esper

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The DISABLE_OUTPUTLIMIT_OPT hint maps the statement to the buffering
// changeset view: each update reaching the output view counts one interim
// pair, the count reads 5 after five events, and the timer delivery clears
// it. Without the hint (or with ENABLE) the unordered last-all view keeps
// the counter at zero. Java: ResultSetOutputLimitChangeSetOpt rounds 1/5
// (PLAIN registration, default compiler configuration).
type outputChangesetBean struct {
	TheString    string `esper:"theString"`
	IntPrimitive int    `esper:"intPrimitive"`
}

func TestOutputLimitChangesetCounter(t *testing.T) {
	ctx := context.Background()

	build := func(t *testing.T, hint string) (*Engine, *Statement, func()) {
		env := NewEnvironment()
		if _, err := RegisterStruct[outputChangesetBean](env, "SupportBean"); err != nil {
			t.Fatal(err)
		}
		stream := From[outputChangesetBean](env, "SupportBean").Window(LengthWindow(2))
		var options []QueryOption
		options = append(options, StatementName("s0"), WithOutput(OutputLastEveryTime(time.Second)))
		switch hint {
		case "ENABLED":
			options = append(options, WithStatementHints(mustHint(t, HintEnableOutputLimitOptimization)))
		case "DISABLED":
			options = append(options, WithStatementHints(mustHint(t, HintDisableOutputLimitOptimization)))
		}
		query := stream.Query(options...)
		plan, err := env.Build(query)
		if err != nil {
			t.Fatal(err)
		}
		engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
		deployment, err := engine.Deploy(ctx, plan)
		if err != nil {
			t.Fatal(err)
		}
		statement := deployment.Statements()[0]
		return engine, statement, func() { _ = engine.Close(ctx) }
	}

	for _, testCase := range []struct {
		hint    string
		after5  int
		after1s int
	}{
		{hint: "DEFAULT", after5: 0, after1s: 0},
		{hint: "DISABLED", after5: 5, after1s: 0},
	} {
		t.Run(testCase.hint, func(t *testing.T) {
			engine, statement, cleanup := build(t, testCase.hint)
			defer cleanup()
			for i := 0; i < 5; i++ {
				if err := engine.Send(ctx, "SupportBean", outputChangesetBean{TheString: "E" + string(rune('0'+i)), IntPrimitive: i}); err != nil {
					t.Fatal(err)
				}
			}
			if got := statement.NumChangesetRows(); got != testCase.after5 {
				t.Fatalf("changeset rows after 5 events = %d, want %d", got, testCase.after5)
			}
			if err := engine.AdvanceTime(ctx, time.UnixMilli(1000).UTC()); err != nil {
				t.Fatal(err)
			}
			if got := statement.NumChangesetRows(); got != testCase.after1s {
				t.Fatalf("changeset rows after timer = %d, want %d", got, testCase.after1s)
			}
		})
	}
}

// The ENABLE_OUTPUTLIMIT_OPT hint cannot be combined with an order-by
// clause: the build must reject the statement with Java's message.
func TestOutputLimitOptimizationHintOrderByRejected(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[outputChangesetBean](env, "SupportBean"); err != nil {
		t.Fatal(err)
	}
	query := From[outputChangesetBean](env, "SupportBean").Query(
		StatementName("s0"),
		WithStatementHints(mustHint(t, HintEnableOutputLimitOptimization)),
		OrderBy(Ascending(Field[outputChangesetBean, string]("theString"))),
	)
	_, err := env.Build(query)
	if err == nil {
		t.Fatal("expected build rejection for ENABLE_OUTPUTLIMIT_OPT with order-by")
	}
	if !containsErrorText(err, "The ENABLE_OUTPUTLIMIT_OPT hint is not supported with order-by") {
		t.Fatalf("error = %v, want order-by rejection message", err)
	}
}

func mustHint(t *testing.T, kind StatementHintKind) StatementHint {
	t.Helper()
	hint, err := NewStatementHint(kind)
	if err != nil {
		t.Fatal(err)
	}
	return hint
}

func containsErrorText(err error, text string) bool {
	for current := err; current != nil; {
		if e, ok := current.(*Error); ok && e.Message == text {
			return true
		}
		current = errors.Unwrap(current)
	}
	return false
}
