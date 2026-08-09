package esper

import (
	"context"
	"fmt"
	"strings"
)

// RuntimeService returns one immutable dependency installed with
// WithRuntimeService. The service value itself is owned by the caller; Engine
// only preserves the named reference for listener and extension access.
func (e *Engine) RuntimeService(name string) (any, bool) {
	if e == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	service, ok := e.services[name]
	return service, ok
}

// StatementPredicate is a typed statement-management filter. Predicates run
// against a detached ordered snapshot, so they may inspect Plan metadata
// without holding the Engine transaction lock.
type StatementPredicate func(*Statement) bool

// StatementNameEquals selects statements by their resolved deployment name.
func StatementNameEquals(name string) StatementPredicate {
	name = strings.TrimSpace(name)
	return func(statement *Statement) bool {
		return statement != nil && statement.Name() == name
	}
}

// StatementDeploymentIDContains selects statements whose stable deployment
// id contains text. It is the typed counterpart to management expressions over
// deploymentId and intentionally does not parse an expression string.
func StatementDeploymentIDContains(text string) StatementPredicate {
	return func(statement *Statement) bool {
		return statement != nil && strings.Contains(statement.DeploymentID(), text)
	}
}

// StatementContains searches the statement name, deployment id and canonical
// fluent Plan. This replaces Esper's EPL-text contains traversal while keeping
// selection independent of a text compiler.
func StatementContains(text string) StatementPredicate {
	return func(statement *Statement) bool {
		if statement == nil {
			return false
		}
		return strings.Contains(statement.Name(), text) ||
			strings.Contains(statement.DeploymentID(), text) ||
			strings.Contains(string(statement.Plan().Canonical()), text)
	}
}

// Statements returns active statement handles in deployment order. Multiple
// predicates are combined with logical AND; no predicates returns all active
// statements.
func (e *Engine) Statements(ctx context.Context, predicates ...StatementPredicate) ([]*Statement, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil {
		return nil, NewError(ErrorDependency, "nil engine")
	}
	for index, predicate := range predicates {
		if predicate == nil {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("statement predicate %d is nil", index))
		}
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, NewError(ErrorState, "engine is closed")
	}
	statements := e.sortedStatementsLocked()
	e.mu.Unlock()
	selected := make([]*Statement, 0, len(statements))
	for _, statement := range statements {
		matches := true
		for _, predicate := range predicates {
			if !predicate(statement) {
				matches = false
				break
			}
		}
		if matches {
			selected = append(selected, statement)
		}
	}
	return selected, nil
}

// TraverseStatements visits the same ordered snapshot as Statements and
// supplies each owning deployment. Visitor callbacks run without the Engine
// transaction lock and may safely call other management methods.
func (e *Engine) TraverseStatements(ctx context.Context, visitor func(*Deployment, *Statement) error) error {
	if visitor == nil {
		return NewError(ErrorInvalidRule, "statement visitor is nil")
	}
	statements, err := e.Statements(ctx)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if err := contextErr(ctx); err != nil {
			return err
		}
		if err := visitor(statement.deployment, statement); err != nil {
			return err
		}
	}
	return nil
}
