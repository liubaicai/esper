package esper

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SplitStreamBranch is one ordered branch of an on-event split. A nil
// Condition is unconditional and an empty Selections slice forwards the
// complete source event. The target event type remains explicit because Go
// does not synthesize public schemas from EPL text.
type SplitStreamBranch struct {
	Target     string
	Condition  Expr
	Selections []Selection
	Precedence Expr
	source     *streamNode
	env        *Environment
}

// SplitStreamBranchBuilder binds one split branch to a fluent source derived
// from the trigger stream. It is the Go counterpart of an EPL split-stream
// branch that carries its own contained-event from-clause.
type SplitStreamBranchBuilder struct {
	source *streamNode
	env    *Environment
}

// SplitFrom starts a branch-local source traversal. The source must resolve
// to the same root event type as the OnEvent trigger and may add stateless
// Filter and Unnest operators. Branches are evaluated in declaration order.
func SplitFrom[T any](source Stream[T]) SplitStreamBranchBuilder {
	return SplitStreamBranchBuilder{source: source.node, env: source.env}
}

// Into completes an unconditional branch-local insert.
func (b SplitStreamBranchBuilder) Into(target string, selections ...Selection) SplitStreamBranch {
	branch := SplitInto(target, selections...)
	branch.source = b.source
	branch.env = b.env
	return branch
}

// IntoWhen completes a conditional branch-local insert.
func (b SplitStreamBranchBuilder) IntoWhen(condition Expr, target string, selections ...Selection) SplitStreamBranch {
	branch := b.Into(target, selections...)
	branch.Condition = condition
	return branch
}

// SplitInto creates an unconditional split branch.
func SplitInto(target string, selections ...Selection) SplitStreamBranch {
	return SplitStreamBranch{
		Target:     strings.TrimSpace(target),
		Selections: append([]Selection(nil), selections...),
	}
}

// SplitIntoWhen creates a conditional split branch.
func SplitIntoWhen(condition Expr, target string, selections ...Selection) SplitStreamBranch {
	branch := SplitInto(target, selections...)
	branch.Condition = condition
	return branch
}

// SplitIntoWithPrecedence creates an unconditional split branch with an
// event-precedence expression. Higher precedence values are processed first;
// branches without precedence are processed last.
func SplitIntoWithPrecedence(precedence Expr, target string, selections ...Selection) SplitStreamBranch {
	branch := SplitInto(target, selections...)
	branch.Precedence = precedence
	return branch
}

// SplitIntoWhenWithPrecedence creates a conditional split branch with an
// event-precedence expression.
func SplitIntoWhenWithPrecedence(condition Expr, precedence Expr, target string, selections ...Selection) SplitStreamBranch {
	branch := SplitIntoWithPrecedence(precedence, target, selections...)
	branch.Condition = condition
	return branch
}

// SplitFirst routes only the first matching branch. If no branch matches, the
// trigger statement emits the original event. This is Esper split-stream's
// default/output-first behavior expressed as a typed fluent plan.
func (s TriggerStream[T]) SplitFirst(branches ...SplitStreamBranch) TriggerQuery {
	return s.splitStream(false, branches)
}

// SplitAll routes every matching branch. If no branch matches, the trigger
// statement emits the original event.
func (s TriggerStream[T]) SplitAll(branches ...SplitStreamBranch) TriggerQuery {
	return s.splitStream(true, branches)
}

func (s TriggerStream[T]) splitStream(all bool, branches []SplitStreamBranch) TriggerQuery {
	copyBranches := make([]SplitStreamBranch, len(branches))
	for index, branch := range branches {
		copyBranches[index] = SplitStreamBranch{
			Target:     strings.TrimSpace(branch.Target),
			Condition:  branch.Condition,
			Selections: append([]Selection(nil), branch.Selections...),
			Precedence: branch.Precedence,
			source:     cloneStreamNode(branch.source),
			env:        branch.env,
		}
	}
	return TriggerQuery{
		env: s.env,
		definition: &triggerDefinition{
			input:         s.node,
			action:        triggerSplitStream,
			splitBranches: copyBranches,
			splitAll:      all,
		},
	}
}

func (e *Environment) validateSplitStream(definition *triggerDefinition) error {
	if definition == nil || definition.input == nil {
		return NewError(ErrorInvalidRule, "split-stream requires a source")
	}
	if len(definition.splitBranches) == 0 {
		return NewError(ErrorInvalidRule, "split-stream requires at least one insert branch")
	}
	for index, branch := range definition.splitBranches {
		input := definition.input
		if branch.source != nil {
			input = branch.source
			if branch.env != nil && branch.env != e {
				return NewError(ErrorDependency, fmt.Sprintf("split-stream branch %d belongs to a different environment", index))
			}
			if err := e.validateNode(input); err != nil {
				return fmt.Errorf("split-stream branch %d source: %w", index, err)
			}
			if !splitStreamSharesRoot(definition.input, input) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("split-stream branch %d source must derive from the trigger source", index))
			}
			if !triggerInputSupportsContainedTraversal(input) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("split-stream branch %d source supports only source, filter and contained-event operators", index))
			}
		}
		if branch.Target == "" {
			return NewError(ErrorInvalidRule, fmt.Sprintf("split-stream branch %d requires an insert target", index))
		}
		targetSchema, targetOK := splitStreamTargetSchema(e, branch.Target)
		if !targetOK {
			return NewError(ErrorUnknownName, fmt.Sprintf("split-stream branch %d references unknown event type %q", index, branch.Target))
		}
		if branch.Condition != nil {
			if branch.Condition.Type() != typeOf[bool]() {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("split-stream branch %d condition must return bool", index))
			}
			if isAggregateExpression(branch.Condition) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("split-stream branch %d condition cannot contain aggregation", index))
			}
			if err := e.validateExprFields(input, branch.Condition); err != nil {
				return fmt.Errorf("split-stream branch %d condition: %w", index, err)
			}
		}
		if len(branch.Selections) == 0 {
			continue
		}
		for selectionIndex, selection := range branch.Selections {
			if selection.Expr != nil && isAggregateExpression(selection.Expr) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("split-stream branch %d projection %d cannot contain aggregation", index, selectionIndex))
			}
		}
		if err := validateMergeInsertSelectionsAgainstSchema(e, input, branch.Target, targetSchema, branch.Selections, "", Schema{}); err != nil {
			return fmt.Errorf("split-stream branch %d: %w", index, err)
		}
	}
	return nil
}

func splitStreamTargetSchema(env *Environment, target string) (Schema, bool) {
	if env == nil {
		return Schema{}, false
	}
	if window, ok := env.NamedWindow(target); ok {
		return window.schema, window.schema.valid()
	}
	return env.Schema(target)
}

func splitStreamSharesRoot(trigger, branch *streamNode) bool {
	triggerRoot := splitStreamRoot(trigger)
	branchRoot := splitStreamRoot(branch)
	if triggerRoot == nil || branchRoot == nil {
		return false
	}
	return triggerRoot.kind == branchRoot.kind &&
		triggerRoot.sourceName == branchRoot.sourceName &&
		triggerRoot.moduleName == branchRoot.moduleName &&
		triggerRoot.sourceType == branchRoot.sourceType
}

func splitStreamRoot(node *streamNode) *streamNode {
	for node != nil && node.input != nil {
		node = node.input
	}
	return node
}

func (s *Statement) processSplitStreamRuntime(ctx context.Context, runtime *statementRuntime, definition *triggerDefinition, now time.Time, event Event, variables map[string]Value) (ResultBatch, error) {
	for _, branch := range definition.splitBranches {
		if branch.source != nil {
			return s.processSplitStreamBranchSources(ctx, runtime, definition, now, event, variables)
		}
	}
	result := ResultBatch{Time: now}
	processCandidate := func(candidate Event) error {
		matched := false
		for _, branch := range definition.splitBranches {
			if err := contextErr(ctx); err != nil {
				return err
			}
			evaluation := EvalContext{
				Engine:               s.engine,
				Event:                candidate,
				OuterEvent:           candidate,
				ContainedParentEvent: containedParentEvent(candidate),
				Now:                  now,
				Variables:            variables,
			}
			if branch.Condition != nil {
				value, ok := boolValue(branch.Condition.eval(evaluation))
				if !ok || !value {
					continue
				}
			}
			routed, err := s.splitStreamEvent(branch, candidate, evaluation, now)
			if err != nil {
				return err
			}
			if err := s.deliverSplitStreamEvent(ctx, branch, routed, now, variables); err != nil {
				return err
			}
			matched = true
			if !definition.splitAll {
				break
			}
		}
		if !matched {
			result.New = append(result.New, resultEvent(candidate))
		}
		return nil
	}

	var err error
	if triggerInputContainsContained(definition.input) && triggerInputSupportsContainedTraversal(definition.input) {
		err = runtime.forEachTriggerCandidate(definition.input, event, now, processCandidate)
	} else {
		var delta eventDelta
		delta, err = runtime.insert(definition.input, event, now)
		if err == nil {
			for _, candidate := range delta.newEvents {
				if err = processCandidate(candidate); err != nil {
					break
				}
			}
		}
	}
	if err != nil {
		return ResultBatch{}, err
	}
	if !result.empty() {
		result.Sequence = runtime.seq.Add(1)
	}
	return result, nil
}

// processSplitStreamBranchSources evaluates branch-local contained-event
// traversals branch-first. This preserves Esper's begin/body/end ordering:
// every routed event from one insert clause is queued before the next clause
// starts. In output-first mode the first branch producing at least one row
// wins, while all matching rows from that branch are retained.
func (s *Statement) processSplitStreamBranchSources(ctx context.Context, runtime *statementRuntime, definition *triggerDefinition, now time.Time, event Event, variables map[string]Value) (ResultBatch, error) {
	result := ResultBatch{Time: now}
	matched := false
	for _, branch := range definition.splitBranches {
		input := definition.input
		if branch.source != nil {
			input = branch.source
		}
		branchMatched := false
		visit := func(candidate Event) error {
			evaluation := EvalContext{
				Engine:               s.engine,
				Event:                candidate,
				OuterEvent:           candidate,
				ContainedParentEvent: containedParentEvent(candidate),
				Now:                  now,
				Variables:            variables,
			}
			if branch.Condition != nil {
				value, ok := boolValue(branch.Condition.eval(evaluation))
				if !ok || !value {
					return nil
				}
			}
			routed, err := s.splitStreamEvent(branch, candidate, evaluation, now)
			if err != nil {
				return err
			}
			if err := s.deliverSplitStreamEvent(ctx, branch, routed, now, variables); err != nil {
				return err
			}
			branchMatched = true
			return nil
		}

		var err error
		if triggerInputContainsContained(input) && triggerInputSupportsContainedTraversal(input) {
			err = runtime.forEachTriggerCandidate(input, event, now, visit)
		} else {
			var delta eventDelta
			delta, err = runtime.insert(input, event, now)
			if err == nil {
				for _, candidate := range delta.newEvents {
					if err = visit(candidate); err != nil {
						break
					}
				}
			}
		}
		if err != nil {
			return ResultBatch{}, err
		}
		if branchMatched {
			matched = true
			if !definition.splitAll {
				break
			}
		}
	}
	if !matched {
		result.New = append(result.New, resultEvent(event))
		result.Sequence = runtime.seq.Add(1)
	}
	return result, nil
}

// deliverSplitStreamEvent keeps ordinary stream routes queued until the
// trigger statement has completed, while applying Named Window targets
// immediately. Esper gives data-window inserts preemptive visibility: an
// earlier ordinary route may cascade only after every split branch has run,
// so it observes a later-declared window insert from the same trigger.
func (s *Statement) deliverSplitStreamEvent(ctx context.Context, branch SplitStreamBranch, routed Event, now time.Time, variables map[string]Value) error {
	if s == nil || s.engine == nil {
		return NewError(ErrorDependency, "split-stream requires an engine")
	}
	if _, ok := s.engine.env.NamedWindow(branch.Target); !ok {
		re := routedEvent{event: routed}
		if branch.Precedence != nil {
			re.precedence = evaluatePrecedenceExpr(branch.Precedence, resultEvent(routed), s.engine)
			re.hasPrec = true
		}
		s.engine.insertRoutedEventLocked(re)
		return nil
	}
	window, ok := s.engine.ensureNamedWindowLocked(branch.Target)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("split-stream named window %q is not available", branch.Target))
	}
	target, exists, err := window.scopedForVariables(variables, true)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	insertUnderlying := namedWindowInsertUnderlying(window, routed)
	delta, err := target.insertWithVariables(ctx, now, insertUnderlying, variables)
	if err != nil {
		return err
	}
	return s.engine.queueNamedWindowDeltaLocked(ctx, now, window, delta, &variables, s)
}

func (s *Statement) splitStreamEvent(branch SplitStreamBranch, source Event, evaluation EvalContext, now time.Time) (Event, error) {
	if s == nil || s.engine == nil || s.engine.env == nil {
		return Event{}, NewError(ErrorDependency, "split-stream requires an engine")
	}
	target, ok := splitStreamTargetSchema(s.engine.env, branch.Target)
	if !ok {
		return Event{}, NewError(ErrorUnknownName, fmt.Sprintf("split-stream target %q is not registered", branch.Target))
	}
	if len(branch.Selections) == 0 {
		underlying, err := projectEventUnderlying(target, source)
		if err != nil {
			return Event{}, err
		}
		return newEvent(target, underlying, now)
	}
	values := make(map[string]any, len(branch.Selections))
	for _, selection := range branch.Selections {
		value := selection.Expr.eval(evaluation)
		values[selection.Name] = value.Any()
	}
	underlying, err := projectMapToSchemaWithSource(target, values, source.Schema())
	if err != nil {
		return Event{}, err
	}
	return newEvent(target, underlying, now)
}
