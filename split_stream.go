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
		if branch.Target == "" {
			return NewError(ErrorInvalidRule, fmt.Sprintf("split-stream branch %d requires an insert target", index))
		}
		if _, ok := e.Schema(branch.Target); !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("split-stream branch %d references unknown event type %q", index, branch.Target))
		}
		if branch.Condition != nil {
			if branch.Condition.Type() != typeOf[bool]() {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("split-stream branch %d condition must return bool", index))
			}
			if isAggregateExpression(branch.Condition) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("split-stream branch %d condition cannot contain aggregation", index))
			}
			if err := e.validateExprFields(definition.input, branch.Condition); err != nil {
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
		if err := validateMergeInsertSelections(e, definition.input, branch.Target, branch.Selections); err != nil {
			return fmt.Errorf("split-stream branch %d: %w", index, err)
		}
	}
	return nil
}

func (s *Statement) processSplitStreamRuntime(ctx context.Context, runtime *statementRuntime, definition *triggerDefinition, now time.Time, event Event, variables map[string]Value) (ResultBatch, error) {
	result := ResultBatch{Time: now}
	processCandidate := func(candidate Event) error {
		matched := false
		for _, branch := range definition.splitBranches {
			if err := contextErr(ctx); err != nil {
				return err
			}
			evaluation := EvalContext{
				Engine:     s.engine,
				Event:      candidate,
				OuterEvent: candidate,
				Now:        now,
				Variables:  variables,
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
			s.engine.pendingRoutedEvents = append(s.engine.pendingRoutedEvents, routed)
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

func (s *Statement) splitStreamEvent(branch SplitStreamBranch, source Event, evaluation EvalContext, now time.Time) (Event, error) {
	if s == nil || s.engine == nil || s.engine.env == nil {
		return Event{}, NewError(ErrorDependency, "split-stream requires an engine")
	}
	target, ok := s.engine.env.Schema(branch.Target)
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
	underlying, err := projectMapToSchema(target, values)
	if err != nil {
		return Event{}, err
	}
	return newEvent(target, underlying, now)
}
