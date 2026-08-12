package esper

import (
	"fmt"
	"sort"
	"strings"
)

// IndexKind identifies the physical access contract of a declared
// infrastructure index. Hash indexes are intended for complete equality/IN
// keys; B-tree indexes additionally support ordered range access.
type IndexKind uint8

const (
	IndexHash IndexKind = iota
	IndexBTree
)

func (kind IndexKind) valid() bool { return kind == IndexHash || kind == IndexBTree }

func (kind IndexKind) String() string {
	switch kind {
	case IndexHash:
		return "hash"
	case IndexBTree:
		return "btree"
	default:
		return fmt.Sprintf("unknown(%d)", kind)
	}
}

// IndexAccessKind describes the logical predicate shape used to select an
// index. It is deliberately independent from a storage implementation so a
// plan can be inspected in tests without depending on a JVM query-plan hook.
type IndexAccessKind uint8

const (
	IndexAccessFullScan IndexAccessKind = iota
	IndexAccessEquality
	IndexAccessIn
	IndexAccessRange
)

func (kind IndexAccessKind) String() string {
	switch kind {
	case IndexAccessFullScan:
		return "full-scan"
	case IndexAccessEquality:
		return "equality"
	case IndexAccessIn:
		return "in"
	case IndexAccessRange:
		return "range"
	default:
		return fmt.Sprintf("unknown(%d)", kind)
	}
}

// IndexBackingKind is the stable, user-facing summary of an access path.
// The summary intentionally avoids exposing Java classes such as
// PropertyHashedEventTable or PropertySortedEventTable.
type IndexBackingKind uint8

const (
	IndexBackingScan IndexBackingKind = iota
	IndexBackingHash
	IndexBackingUniqueHash
	IndexBackingBTree
	IndexBackingUniqueBTree
)

func (kind IndexBackingKind) String() string {
	switch kind {
	case IndexBackingScan:
		return "scan"
	case IndexBackingHash:
		return "hash"
	case IndexBackingUniqueHash:
		return "unique-hash"
	case IndexBackingBTree:
		return "btree"
	case IndexBackingUniqueBTree:
		return "unique-btree"
	default:
		return fmt.Sprintf("unknown(%d)", kind)
	}
}

// IndexSelection is one source's selected access path. Source is the
// zero-based source position for joins and is always zero for a single-source
// query. A full-scan selection has an empty IndexName and Columns.
type IndexSelection struct {
	Source         int
	Module         string
	Object         string
	IndexName      string
	Columns        []string
	MatchedColumns []string
	Access         IndexAccessKind
	Backing        IndexBackingKind
	Hinted         bool
}

func (selection IndexSelection) clone() IndexSelection {
	selection.Columns = append([]string(nil), selection.Columns...)
	selection.MatchedColumns = append([]string(nil), selection.MatchedColumns...)
	return selection
}

// IndexPlan is the deterministic logical access summary attached to a Plan.
// It is useful for parity tests, observability and regression checks; query
// correctness never depends on a caller reading or mutating this value.
type IndexPlan struct {
	Selections []IndexSelection
}

func (plan IndexPlan) clone() IndexPlan {
	result := IndexPlan{Selections: make([]IndexSelection, len(plan.Selections))}
	for index, selection := range plan.Selections {
		result.Selections[index] = selection.clone()
	}
	return result
}

// ForSource returns the selected access path for one source position.
func (plan IndexPlan) ForSource(source int) (IndexSelection, bool) {
	for _, selection := range plan.Selections {
		if selection.Source == source {
			return selection.clone(), true
		}
	}
	return IndexSelection{}, false
}

type indexHint struct {
	source int
	name   string
}

// UseIndex selects a named index for the only source (or source zero) of a
// query. This is the fluent counterpart of an index hint and accepts only a
// structured name, never an EPL annotation string.
func UseIndex(name string) QueryOption {
	return UseIndexOn(0, name)
}

// UseIndexOn selects a named index for one join source.
func UseIndexOn(source int, name string) QueryOption {
	return func(spec *querySpec) {
		spec.indexHints = append(spec.indexHints, indexHint{source: source, name: strings.TrimSpace(name)})
	}
}

// WithIndexHints is the composable form for callers assembling a common
// query option list. Source indexes are zero-based and preserve declaration
// order in a join.
func WithIndexHints(hints ...IndexHint) QueryOption {
	return func(spec *querySpec) {
		for _, hint := range hints {
			spec.indexHints = append(spec.indexHints, indexHint{source: hint.Source, name: strings.TrimSpace(hint.Name)})
		}
	}
}

// IndexHint is a declarative source-to-index binding for WithIndexHints.
type IndexHint struct {
	Source int
	Name   string
}

type indexCandidate struct {
	name    string
	columns []string
	unique  bool
	kind    IndexKind
}

type indexPredicate struct {
	columns []string
	access  IndexAccessKind
}

func cloneTableIndexDefinitions(indexes []TableIndexDefinition) []TableIndexDefinition {
	result := make([]TableIndexDefinition, len(indexes))
	for index, definition := range indexes {
		result[index] = definition
		result[index].Columns = append([]string(nil), definition.Columns...)
	}
	return result
}

func cloneNamedWindowIndexDefinitions(indexes []NamedWindowIndexDefinition) []NamedWindowIndexDefinition {
	result := make([]NamedWindowIndexDefinition, len(indexes))
	for index, definition := range indexes {
		result[index] = definition
		result[index].Columns = append([]string(nil), definition.Columns...)
	}
	return result
}

func indexBacking(kind IndexKind, unique bool) IndexBackingKind {
	if kind == IndexBTree {
		if unique {
			return IndexBackingUniqueBTree
		}
		return IndexBackingBTree
	}
	if unique {
		return IndexBackingUniqueHash
	}
	return IndexBackingHash
}

func indexPredicatesFromExpression(expression Expr) []indexPredicate {
	if expression == nil || expression.node() == nil {
		return nil
	}
	return indexPredicatesFromNode(expression.node())
}

func indexPredicatesFromNode(node *exprNode) []indexPredicate {
	if node == nil {
		return nil
	}
	switch node.kind {
	case "and":
		result := make([]indexPredicate, 0)
		for _, child := range node.children {
			result = append(result, indexPredicatesFromNode(child)...)
		}
		return result
	case "eq", "equal-of", "is":
		if column, ok := equalityIndexColumn(node); ok {
			return []indexPredicate{{columns: []string{column}, access: IndexAccessEquality}}
		}
	case "in", "in-of":
		if column := fieldColumn(nodeChild(node, 0)); column != "" {
			return []indexPredicate{{columns: []string{column}, access: IndexAccessIn}}
		}
	case "between-of", "between":
		if column := fieldColumn(nodeChild(node, 0)); column != "" {
			return []indexPredicate{{columns: []string{column}, access: IndexAccessRange}}
		}
	case "gt", "gte", "lt", "lte", "exact-gt", "exact-gte", "exact-lt", "exact-lte", "greater-of", "greater-equal-of", "less-of", "less-equal-of":
		if column, ok := comparisonIndexColumn(node); ok {
			return []indexPredicate{{columns: []string{column}, access: IndexAccessRange}}
		}
	}
	return nil
}

func nodeChild(node *exprNode, index int) *exprNode {
	if node == nil || index < 0 || index >= len(node.children) {
		return nil
	}
	return node.children[index]
}

func fieldColumn(node *exprNode) string {
	if node == nil || node.kind != "field" {
		return ""
	}
	return strings.TrimSpace(node.fieldName)
}

func equalityIndexColumn(node *exprNode) (string, bool) {
	left := fieldColumn(nodeChild(node, 0))
	right := fieldColumn(nodeChild(node, 1))
	if left != "" && right == "" {
		return left, true
	}
	if right != "" && left == "" {
		return right, true
	}
	return "", false
}

func comparisonIndexColumn(node *exprNode) (string, bool) {
	return equalityIndexColumn(node)
}

func sourceIndexPredicates(node *streamNode) []indexPredicate {
	result := make([]indexPredicate, 0)
	for current := node; current != nil; current = current.input {
		if current.kind == streamFilter {
			result = append(result, indexPredicatesFromExpression(current.predicate)...)
		}
	}
	return result
}

func indexCandidatesForSource(env *Environment, source *streamNode) ([]indexCandidate, error) {
	base, err := sourceNode(source)
	if err != nil {
		return nil, err
	}
	if base.kind == streamTable {
		definition, ok := env.TableInModule(base.moduleName, base.sourceName)
		if !ok {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("table %q is not registered", base.sourceName))
		}
		candidates := make([]indexCandidate, 0, len(definition.indexes)+1)
		if len(definition.primaryKey) > 0 {
			candidates = append(candidates, indexCandidate{name: "<primary-key>", columns: append([]string(nil), definition.primaryKey...), unique: true, kind: IndexHash})
		}
		for _, definition := range definition.indexes {
			candidates = append(candidates, indexCandidate{name: definition.Name, columns: append([]string(nil), definition.Columns...), unique: definition.Unique, kind: definition.Kind})
		}
		return candidates, nil
	}
	if base.kind == streamNamedWindow {
		definition, ok := env.NamedWindowInModule(base.moduleName, base.sourceName)
		if !ok {
			return nil, NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", base.sourceName))
		}
		candidates := make([]indexCandidate, 0, len(definition.indexes)+1)
		for _, index := range definition.indexes {
			candidates = append(candidates, indexCandidate{name: index.Name, columns: append([]string(nil), index.Columns...), unique: index.Unique, kind: index.Kind})
		}
		if retention, ok := definition.retention.(UniqueWindowSpec); ok {
			if columns := simpleFieldColumns(retention.keyExpressions()); len(columns) > 0 {
				candidates = append(candidates, indexCandidate{name: "<retention-unique>", columns: columns, unique: true, kind: IndexHash})
			}
		}
		return candidates, nil
	}
	return nil, nil
}

func simpleFieldColumns(expressions []Expr) []string {
	result := make([]string, 0, len(expressions))
	for _, expression := range expressions {
		column := fieldColumn(expressionNode(expression))
		if column == "" {
			return nil
		}
		result = append(result, column)
	}
	return result
}

func expressionNode(expression Expr) *exprNode {
	if expression == nil {
		return nil
	}
	return expression.node()
}

func sourceIndexPredicatesForJoin(condition JoinCondition, result map[int][]indexPredicate) {
	for _, child := range condition.all {
		sourceIndexPredicatesForJoin(child, result)
	}
	for _, child := range condition.any {
		sourceIndexPredicatesForJoin(child, result)
	}
	if condition.Left == nil || condition.Right == nil {
		return
	}
	leftSource, rightSource := joinConditionSources(condition)
	if leftSource < 0 || rightSource < 0 {
		return
	}
	access := IndexAccessEquality
	if condition.Comparison != JoinEqual {
		access = IndexAccessRange
	}
	if leftColumn := joinConditionIndexColumn(condition.Left.node(), leftSource); leftColumn != "" {
		result[leftSource] = append(result[leftSource], indexPredicate{columns: []string{leftColumn}, access: access})
	}
	if rightColumn := joinConditionIndexColumn(condition.Right.node(), rightSource); rightColumn != "" {
		result[rightSource] = append(result[rightSource], indexPredicate{columns: []string{rightColumn}, access: access})
	}
}

func joinConditionIndexColumn(node *exprNode, source int) string {
	if column := fieldColumn(node); column != "" {
		return column
	}
	joinSource, column, ok := joinFieldColumn(node)
	if ok && joinSource == source {
		return column
	}
	return ""
}

func joinFieldColumn(node *exprNode) (int, string, bool) {
	if node == nil || node.kind != "join-field" || node.joinSource < 0 {
		return -1, "", false
	}
	column := strings.TrimSpace(node.fieldName)
	if column == "" {
		return -1, "", false
	}
	return node.joinSource, column, true
}

func sourceIndexPredicatesFromJoinExpression(expression Expr, result map[int][]indexPredicate) {
	if expression == nil || expression.node() == nil {
		return
	}
	sourceIndexPredicatesFromJoinNode(expression.node(), result)
}

func sourceIndexPredicatesFromJoinNode(node *exprNode, result map[int][]indexPredicate) {
	if node == nil {
		return
	}
	if node.kind == "and" {
		for _, child := range node.children {
			sourceIndexPredicatesFromJoinNode(child, result)
		}
		return
	}
	if node.kind != "eq" && node.kind != "equal-of" && node.kind != "is" &&
		node.kind != "in" && node.kind != "in-of" &&
		node.kind != "between-of" && node.kind != "between" &&
		node.kind != "gt" && node.kind != "gte" && node.kind != "lt" && node.kind != "lte" &&
		node.kind != "exact-gt" && node.kind != "exact-gte" && node.kind != "exact-lt" && node.kind != "exact-lte" &&
		node.kind != "greater-of" && node.kind != "greater-equal-of" && node.kind != "less-of" && node.kind != "less-equal-of" {
		return
	}
	access := IndexAccessEquality
	if node.kind == "in" || node.kind == "in-of" {
		access = IndexAccessIn
	} else if node.kind != "eq" && node.kind != "equal-of" && node.kind != "is" {
		access = IndexAccessRange
	}
	for _, child := range node.children {
		if source, column, ok := joinFieldColumn(child); ok {
			result[source] = append(result[source], indexPredicate{columns: []string{column}, access: access})
		}
	}
}

func appendUniqueIndexPredicates(destination []indexPredicate, predicates []indexPredicate) []indexPredicate {
	seen := make(map[string]struct{}, len(destination)+len(predicates))
	for _, predicate := range destination {
		if len(predicate.columns) == 1 {
			seen[fmt.Sprintf("%s:%d", predicate.columns[0], predicate.access)] = struct{}{}
		}
	}
	for _, predicate := range predicates {
		if len(predicate.columns) != 1 {
			continue
		}
		key := fmt.Sprintf("%s:%d", predicate.columns[0], predicate.access)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		destination = append(destination, predicate)
	}
	return destination
}

func matchIndex(candidate indexCandidate, predicates []indexPredicate) (IndexAccessKind, []string, bool) {
	if len(candidate.columns) == 0 || len(predicates) == 0 {
		return IndexAccessFullScan, nil, false
	}
	predicateByColumn := make(map[string]IndexAccessKind, len(predicates))
	for _, predicate := range predicates {
		if len(predicate.columns) != 1 || predicate.columns[0] == "" {
			continue
		}
		if previous, exists := predicateByColumn[predicate.columns[0]]; !exists || predicate.access == IndexAccessEquality || previous == IndexAccessRange {
			predicateByColumn[predicate.columns[0]] = predicate.access
		}
	}
	matched := make([]string, 0, len(candidate.columns))
	access := IndexAccessEquality
	for position, column := range candidate.columns {
		predicateAccess, exists := predicateByColumn[column]
		if !exists {
			break
		}
		if predicateAccess == IndexAccessRange || predicateAccess == IndexAccessIn {
			if predicateAccess == IndexAccessRange {
				access = IndexAccessRange
			} else if access != IndexAccessRange {
				access = IndexAccessIn
			}
			matched = append(matched, column)
			break
		}
		matched = append(matched, column)
		if position == len(candidate.columns)-1 {
			access = IndexAccessEquality
		}
	}
	if len(matched) == 0 {
		return IndexAccessFullScan, nil, false
	}
	// Hash lookup requires a complete key. B-tree access may use an equality
	// prefix and stop at a range predicate on the next column.
	if candidate.kind != IndexBTree && len(matched) != len(candidate.columns) {
		return IndexAccessFullScan, nil, false
	}
	if access == IndexAccessRange && candidate.kind != IndexBTree {
		return IndexAccessFullScan, nil, false
	}
	return access, matched, true
}

func chooseIndexSelection(env *Environment, source *streamNode, sourceIndex int, predicates []indexPredicate, hint *indexHint) (IndexSelection, error) {
	base, err := sourceNode(source)
	if err != nil {
		return IndexSelection{}, err
	}
	selection := IndexSelection{Source: sourceIndex, Module: base.moduleName, Object: base.sourceName, Access: IndexAccessFullScan, Backing: IndexBackingScan}
	candidates, err := indexCandidatesForSource(env, source)
	if err != nil {
		return IndexSelection{}, err
	}
	if hint != nil {
		if strings.TrimSpace(hint.name) == "" {
			return IndexSelection{}, NewError(ErrorInvalidRule, fmt.Sprintf("index hint for source %d requires an index name", sourceIndex))
		}
		found := false
		for _, candidate := range candidates {
			if candidate.name != hint.name {
				continue
			}
			found = true
			access, matched, usable := matchIndex(candidate, predicates)
			if !usable {
				return IndexSelection{}, NewError(ErrorInvalidRule, fmt.Sprintf("index hint %q for source %d cannot satisfy the query predicate", hint.name, sourceIndex))
			}
			selection.IndexName = candidate.name
			selection.Columns = append([]string(nil), candidate.columns...)
			selection.MatchedColumns = append([]string(nil), matched...)
			selection.Access = access
			selection.Backing = indexBacking(candidate.kind, candidate.unique)
			selection.Hinted = true
			return selection, nil
		}
		if !found {
			return IndexSelection{}, NewError(ErrorUnknownName, fmt.Sprintf("index hint %q for source %d does not exist", hint.name, sourceIndex))
		}
	}
	bestScore := -1
	var best indexCandidate
	var bestAccess IndexAccessKind
	var bestMatched []string
	for _, candidate := range candidates {
		access, matched, usable := matchIndex(candidate, predicates)
		if !usable {
			continue
		}
		score := len(matched) * 10
		if access == IndexAccessRange {
			score += 2
		}
		if candidate.unique {
			score++
		}
		if score > bestScore || (score == bestScore && candidate.name < best.name) {
			bestScore = score
			best = candidate
			bestAccess = access
			bestMatched = append([]string(nil), matched...)
		}
	}
	if bestScore >= 0 {
		selection.IndexName = best.name
		selection.Columns = append([]string(nil), best.columns...)
		selection.MatchedColumns = bestMatched
		selection.Access = bestAccess
		selection.Backing = indexBacking(best.kind, best.unique)
	}
	return selection, nil
}

func collectIndexHints(hints []indexHint) (map[int]indexHint, error) {
	result := make(map[int]indexHint, len(hints))
	for _, hint := range hints {
		if hint.source < 0 {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("index hint source %d is negative", hint.source))
		}
		if _, exists := result[hint.source]; exists {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("duplicate index hint for source %d", hint.source))
		}
		result[hint.source] = hint
	}
	return result, nil
}

func validateIndexHintSources(hints map[int]indexHint, sourceCount int) error {
	for source := range hints {
		if source >= sourceCount {
			return NewError(ErrorInvalidRule, fmt.Sprintf("index hint references source %d outside the query's %d sources", source, sourceCount))
		}
	}
	return nil
}

func (e *Environment) buildIndexPlan(query Query) (IndexPlan, error) {
	hints, err := collectIndexHints(query.indexHints)
	if err != nil {
		return IndexPlan{}, err
	}
	if query.onDemand != nil {
		if query.input == nil {
			return IndexPlan{}, nil
		}
		if err := validateIndexHintSources(hints, 1); err != nil {
			return IndexPlan{}, err
		}
		selection, selectionErr := chooseIndexSelection(e, query.input, 0, indexPredicatesFromExpression(query.onDemand.predicate), hintForSource(hints, 0))
		if selectionErr != nil {
			return IndexPlan{}, selectionErr
		}
		return IndexPlan{Selections: []IndexSelection{selection}}, nil
	}
	if query.join != nil {
		sources := joinDefinitionSources(query.join)
		predicates := make(map[int][]indexPredicate, len(sources))
		if err := validateIndexHintSources(hints, len(sources)); err != nil {
			return IndexPlan{}, err
		}
		for sourceIndex, source := range sources {
			predicates[sourceIndex] = append(predicates[sourceIndex], sourceIndexPredicates(source)...)
		}
		for _, condition := range joinDefinitionConditions(query.join) {
			sourceIndexPredicatesForJoin(condition, predicates)
		}
		sourceIndexPredicatesFromJoinExpression(query.joinWhere, predicates)
		selections := make([]IndexSelection, 0, len(sources))
		for sourceIndex, source := range sources {
			selection, selectionErr := chooseIndexSelection(e, source, sourceIndex, predicates[sourceIndex], hintForSource(hints, sourceIndex))
			if selectionErr != nil {
				return IndexPlan{}, selectionErr
			}
			selections = append(selections, selection)
		}
		return IndexPlan{Selections: selections}, nil
	}
	if query.input == nil {
		if len(hints) > 0 {
			return IndexPlan{}, NewError(ErrorInvalidRule, "index hints require a source")
		}
		return IndexPlan{}, nil
	}
	if err := validateIndexHintSources(hints, 1); err != nil {
		return IndexPlan{}, err
	}
	if len(hints) > 1 || (len(hints) == 1 && hintForSource(hints, 0) == nil) {
		return IndexPlan{}, NewError(ErrorInvalidRule, "single-source query accepts only source-zero index hints")
	}
	predicates := sourceIndexPredicates(query.input)
	if query.aggregate != nil {
		predicates = appendUniqueIndexPredicates(predicates, indexPredicatesFromExpression(query.aggregate.where))
	}
	selection, selectionErr := chooseIndexSelection(e, query.input, 0, predicates, hintForSource(hints, 0))
	if selectionErr != nil {
		return IndexPlan{}, selectionErr
	}
	return IndexPlan{Selections: []IndexSelection{selection}}, nil
}

func hintForSource(hints map[int]indexHint, source int) *indexHint {
	hint, ok := hints[source]
	if !ok {
		return nil
	}
	return &hint
}

func sortIndexSelections(selections []IndexSelection) {
	sort.SliceStable(selections, func(left, right int) bool { return selections[left].Source < selections[right].Source })
}
