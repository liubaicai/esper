package esper

import (
	"fmt"
	"strings"
)

// CompileItem describes one typed Query in a CompileBatch operation. Its line
// and diagnostic text are application metadata only; they are never parsed as
// EPL and do not enter Plan identity.
type CompileItem struct {
	query          Query
	options        []CompileOption
	line           int
	lineSet        bool
	diagnosticText string
	textSet        bool
}

// NewCompileItem creates a batch item and detaches the supplied compile-option
// slice. Functional option values themselves remain application-owned.
func NewCompileItem(query Query, options ...CompileOption) CompileItem {
	return CompileItem{query: query, options: append([]CompileOption(nil), options...)}
}

// AtLine assigns the one-based source or model line reported for this item.
// When omitted, CompileBatch uses the one-based item position.
func (item CompileItem) AtLine(line int) CompileItem {
	item.line = line
	item.lineSet = true
	return item
}

// WithDiagnosticText assigns stable application-facing item text. Whitespace,
// including newlines, is collapsed for deterministic single-line reporting.
func (item CompileItem) WithDiagnosticText(text string) CompileItem {
	item.diagnosticText = normalizeCompileDiagnosticText(text)
	item.textSet = true
	return item
}

func normalizeCompileDiagnosticText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// CompileErrorItem reports one failed CompileItem.
type CompileErrorItem struct {
	Index      int
	Line       int
	Expression string
	Cause      error
}

// Error returns the item cause while retaining item location metadata through
// the exported fields.
func (item CompileErrorItem) Error() string {
	if item.Cause == nil {
		return fmt.Sprintf("compile item %d at line %d failed", item.Index, item.Line)
	}
	return fmt.Sprintf("compile item %d at line %d (%s): %v", item.Index, item.Line, item.Expression, item.Cause)
}

// CompileBatchError aggregates every item failure in input order.
type CompileBatchError struct {
	items []CompileErrorItem
}

func (err *CompileBatchError) Error() string {
	if err == nil {
		return "esper: compile batch: <nil error>"
	}
	if len(err.items) == 0 {
		return "esper: compile batch failed"
	}
	return fmt.Sprintf("esper: compile batch failed with %d item(s): %v", len(err.items), err.items[0].Cause)
}

// Unwrap exposes the first cause for errors.Is/errors.As compatibility, while
// Items retains every failure.
func (err *CompileBatchError) Unwrap() error {
	if err == nil || len(err.items) == 0 {
		return nil
	}
	return err.items[0].Cause
}

// Items returns a detached copy of all compile failures in input order.
func (err *CompileBatchError) Items() []CompileErrorItem {
	if err == nil {
		return nil
	}
	return append([]CompileErrorItem(nil), err.items...)
}

// CompileBatch compiles every typed item and collects all failures. It returns
// Plans only when the entire batch succeeds; partial successful Plans are not
// exposed when any item fails.
func CompileBatch(env *Environment, items ...CompileItem) ([]Plan, error) {
	if env == nil {
		return nil, NewError(ErrorDependency, "compile batch requires an environment")
	}
	if len(items) == 0 {
		return []Plan{}, nil
	}
	plans := make([]Plan, len(items))
	failures := make([]CompileErrorItem, 0)
	for index, item := range items {
		line := index + 1
		if item.lineSet {
			line = item.line
		}
		expression := fmt.Sprintf("query[%d]", index)
		if item.textSet {
			expression = item.diagnosticText
		}
		var err error
		switch {
		case line <= 0:
			err = NewError(ErrorInvalidRule, "compile item line must be positive")
		case item.textSet && expression == "":
			err = NewError(ErrorInvalidRule, "compile item diagnostic text cannot be blank")
		default:
			plans[index], err = env.Build(item.query, append([]CompileOption(nil), item.options...)...)
		}
		if err != nil {
			failures = append(failures, CompileErrorItem{
				Index: index, Line: line, Expression: expression, Cause: err,
			})
		}
	}
	if len(failures) > 0 {
		return nil, &CompileBatchError{items: failures}
	}
	return plans, nil
}
