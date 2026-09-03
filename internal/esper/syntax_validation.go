package esper

import (
	"fmt"
	"strings"
)

// ValidateSyntax checks the structural validity of typed fluent queries
// without resolving event types, variables, contexts or other catalog names.
// It is the Go counterpart of Esper's syntaxValidate module operation: callers
// can validate query shape before the semantic Environment.Build pass.
//
// An empty query list is valid. Each supplied query must belong to env.
func (e *Environment) ValidateSyntax(queries ...Query) error {
	if e == nil {
		return NewError(ErrorDependency, "syntax validation requires an environment")
	}
	for index, query := range queries {
		if err := e.validateTypedQuerySyntax(query); err != nil {
			return WrapError(ErrorInvalidRule, fmt.Sprintf("query[%d]", index), err)
		}
	}
	return nil
}

func (e *Environment) validateTypedQuerySyntax(query Query) error {
	if query.env == nil || query.env != e {
		return NewError(ErrorDependency, "query belongs to a different or nil environment")
	}
	if query.input == nil && query.join == nil && !query.sourceLess {
		return NewError(ErrorInvalidRule, "query has no source")
	}
	if query.sourceLess && len(query.selections) == 0 {
		return NewError(ErrorInvalidRule, "source-less query requires at least one select expression")
	}
	if query.name != "" && strings.TrimSpace(query.name) == "" {
		return NewError(ErrorInvalidRule, "statement name cannot be blank")
	}
	if query.contextName != "" && strings.TrimSpace(query.contextName) == "" {
		return NewError(ErrorInvalidRule, "context name cannot be blank")
	}
	if query.moduleName != "" && strings.TrimSpace(query.moduleName) == "" {
		return NewError(ErrorInvalidRule, "module name cannot be blank")
	}
	for index, name := range query.moduleUses {
		if strings.TrimSpace(name) == "" {
			return NewError(ErrorInvalidRule, fmt.Sprintf("module use %d cannot be blank", index))
		}
	}
	if query.offset < 0 {
		return NewError(ErrorInvalidRule, "offset cannot be negative")
	}
	if err := validateOutputPolicy(query.output); err != nil {
		return err
	}
	if err := validateStatementMetadata(query.statementMetadata); err != nil {
		return err
	}

	if err := validateSyntaxSelections("select", query.selections); err != nil {
		return err
	}
	if err := validateSyntaxSelections("pattern select", query.patternSelections); err != nil {
		return err
	}
	for index, selection := range query.joinSelections {
		if strings.TrimSpace(selection.Name) == "" {
			return NewError(ErrorInvalidRule, fmt.Sprintf("join select expression %d requires a non-blank alias", index))
		}
		if selection.Expr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("join select expression %d is nil", index))
		}
	}

	if query.join != nil {
		sources := joinDefinitionSources(query.join)
		if len(sources) < 2 {
			return NewError(ErrorInvalidRule, "join requires at least two sources")
		}
		for index, source := range sources {
			if err := validateSyntaxStreamNode(source); err != nil {
				return fmt.Errorf("join source %d: %w", index, err)
			}
		}
	} else if query.aggregate != nil && query.aggregate.join != nil {
		sources := joinDefinitionSources(query.aggregate.join)
		if len(sources) < 2 {
			return NewError(ErrorInvalidRule, "aggregate join requires at least two sources")
		}
		for index, source := range sources {
			if err := validateSyntaxStreamNode(source); err != nil {
				return fmt.Errorf("aggregate join source %d: %w", index, err)
			}
		}
	} else if !query.sourceLess {
		if err := validateSyntaxStreamNode(query.input); err != nil {
			return err
		}
	}

	if query.pattern != nil {
		if err := validatePattern(query.pattern); err != nil {
			return err
		}
	}
	if query.rowRecog != nil && query.rowRecog.input == nil {
		return NewError(ErrorInvalidRule, "match-recognize requires a source")
	}
	if query.trigger != nil && query.trigger.input == nil {
		return NewError(ErrorInvalidRule, "trigger requires a source")
	}
	if query.aggregate != nil {
		if query.aggregate.input == nil && query.aggregate.join == nil {
			return NewError(ErrorInvalidRule, "aggregate requires a source")
		}
		if err := validateSyntaxSelections("aggregate select", query.aggregate.selections); err != nil {
			return err
		}
		for index, expression := range query.aggregate.groupBy {
			if expression == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("group-by expression %d is nil", index))
			}
		}
	}

	if err := visitQueryExpressions(nil, query, func(expression Expr) error {
		if expression == nil {
			return nil
		}
		if expression.node() == nil {
			return NewError(ErrorInvalidRule, "expression has no typed AST node")
		}
		return validateExpressionConfiguration(expression.node())
	}); err != nil {
		return err
	}
	return nil
}

func validateSyntaxSelections(scope string, selections []Selection) error {
	for index, selection := range selections {
		if strings.TrimSpace(selection.Name) == "" {
			return NewError(ErrorInvalidRule, fmt.Sprintf("%s expression %d requires a non-blank alias", scope, index))
		}
		if selection.Expr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("%s expression %d is nil", scope, index))
		}
	}
	return nil
}

func validateSyntaxStreamNode(node *streamNode) error {
	if node == nil {
		return NewError(ErrorInvalidRule, "stream node is nil")
	}
	if node.configurationError != "" {
		return NewError(ErrorInvalidRule, node.configurationError)
	}
	switch node.kind {
	case streamSource, streamNamedWindow, streamTable, streamHistorical, streamMethod:
		if strings.TrimSpace(node.sourceName) == "" {
			return NewError(ErrorInvalidRule, "stream source name cannot be blank")
		}
		return nil
	case streamFilter:
		if err := validateSyntaxStreamNode(node.input); err != nil {
			return err
		}
		if node.predicate == nil {
			return NewError(ErrorInvalidRule, "filter requires a predicate")
		}
		if node.predicate.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "filter predicate must return bool")
		}
		return validateExpressionConfiguration(node.predicate.node())
	case streamWindow:
		if err := validateSyntaxStreamNode(node.input); err != nil {
			return err
		}
		if node.window == nil {
			return NewError(ErrorInvalidRule, "window specification is nil")
		}
		return node.window.validate()
	case streamPattern:
		if node.pattern == nil {
			return NewError(ErrorInvalidRule, "pattern stream requires a pattern definition")
		}
		if err := validatePattern(node.pattern); err != nil {
			return err
		}
		for index, input := range patternDefinitionInputs(node.pattern) {
			if err := validateSyntaxStreamNode(input); err != nil {
				return fmt.Errorf("pattern source %d: %w", index, err)
			}
		}
		if node.patternWindow != nil {
			return node.patternWindow.validate()
		}
		return nil
	case streamDerived:
		if node.derived == nil || node.derived.aggregate == nil {
			return NewError(ErrorInvalidRule, "derived stream requires an aggregate definition")
		}
		return validateSyntaxStreamNode(node.input)
	case streamContained:
		if err := validateSyntaxStreamNode(node.input); err != nil {
			return err
		}
		if node.contained == nil || node.contained.property == nil {
			return NewError(ErrorInvalidRule, "unnest stream requires a contained property expression")
		}
		return validateExpressionConfiguration(node.contained.property.node())
	default:
		return NewError(ErrorInvalidRule, fmt.Sprintf("unknown stream node kind %d", node.kind))
	}
}
