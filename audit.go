package esper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// AuditCategory identifies one Esper audit path without accepting EPL text.
type AuditCategory string

const (
	AuditProperty             AuditCategory = "PROPERTY"
	AuditExpression           AuditCategory = "EXPRESSION"
	AuditExpressionNested     AuditCategory = "EXPRESSION-NESTED"
	AuditExpressionDefinition AuditCategory = "EXPRDEF"
	AuditView                 AuditCategory = "VIEW"
	AuditPattern              AuditCategory = "PATTERN"
	AuditPatternInstances     AuditCategory = "PATTERN-INSTANCES"
	AuditStream               AuditCategory = "STREAM"
	AuditSchedule             AuditCategory = "SCHEDULE"
	AuditInsert               AuditCategory = "INSERT"
	AuditDataflowSource       AuditCategory = "DATAFLOW-SOURCE"
	AuditDataflowOperator     AuditCategory = "DATAFLOW-OP"
	AuditDataflowTransition   AuditCategory = "DATAFLOW-TRANSITION"
	AuditContextPartition     AuditCategory = "CONTEXTPARTITION"
	AuditAll                  AuditCategory = "*"
)

var allAuditCategories = []AuditCategory{
	AuditProperty, AuditExpression, AuditExpressionNested, AuditExpressionDefinition,
	AuditView, AuditPattern, AuditPatternInstances, AuditStream, AuditSchedule,
	AuditInsert, AuditDataflowSource, AuditDataflowOperator,
	AuditDataflowTransition, AuditContextPartition,
}

func normalizeAuditCategories(categories []AuditCategory) []AuditCategory {
	if len(categories) == 0 {
		return append([]AuditCategory(nil), allAuditCategories...)
	}
	result := make([]AuditCategory, 0, len(categories))
	seen := make(map[AuditCategory]struct{}, len(categories))
	for _, original := range categories {
		category := AuditCategory(strings.ToUpper(strings.TrimSpace(string(original))))
		if category == AuditAll {
			return append([]AuditCategory(nil), allAuditCategories...)
		}
		if _, duplicate := seen[category]; duplicate {
			continue
		}
		seen[category] = struct{}{}
		result = append(result, category)
	}
	return result
}

func validateAuditCategories(categories []AuditCategory) error {
	valid := make(map[AuditCategory]struct{}, len(allAuditCategories))
	for _, category := range allAuditCategories {
		valid[category] = struct{}{}
	}
	for index, category := range categories {
		if _, ok := valid[category]; !ok {
			return fmt.Errorf("esper: audit category %d %q is invalid", index, category)
		}
	}
	return nil
}

func auditCategoriesContain(categories []AuditCategory, category AuditCategory) bool {
	for _, candidate := range categories {
		if candidate == category {
			return true
		}
	}
	return false
}

// AuditRecord is an immutable statement or dataflow audit callback record.
// RuntimeTime is the Engine virtual clock in Unix milliseconds, matching the
// Java AuditContext contract; VirtualTime retains the native time.Time value.
type AuditRecord struct {
	runtimeURI      string
	deploymentID    string
	statementName   string
	agentInstanceID int
	category        AuditCategory
	virtualTime     time.Time
	message         string
}

func (r AuditRecord) RuntimeURI() string      { return r.runtimeURI }
func (r AuditRecord) DeploymentID() string    { return r.deploymentID }
func (r AuditRecord) StatementName() string   { return r.statementName }
func (r AuditRecord) AgentInstanceID() int    { return r.agentInstanceID }
func (r AuditRecord) Category() AuditCategory { return r.category }
func (r AuditRecord) VirtualTime() time.Time  { return r.virtualTime }
func (r AuditRecord) RuntimeTime() int64      { return r.virtualTime.UnixMilli() }
func (r AuditRecord) Message() string         { return r.message }

// Format returns Esper's default audit text.
func (r AuditRecord) Format() string {
	return fmt.Sprintf("Statement %s partition %d %s %s", r.statementName, r.agentInstanceID, strings.ToLower(string(r.category)), r.message)
}

// AuditFormatter supports Esper's %u/%d/%s/%i/%c/%m/%tutc/%tzone pattern
// tokens while keeping timezone selection explicit and Engine-local.
type AuditFormatter struct {
	pattern  string
	location *time.Location
}

func NewAuditFormatter(pattern string, location *time.Location) AuditFormatter {
	if location == nil {
		location = time.Local
	}
	return AuditFormatter{pattern: pattern, location: location}
}

func (f AuditFormatter) Format(record AuditRecord) string {
	if f.pattern == "" {
		return record.Format()
	}
	location := f.location
	if location == nil {
		location = time.Local
	}
	replacements := []string{
		"%tutc", record.virtualTime.UTC().Format(time.RFC3339Nano),
		"%tzone", record.virtualTime.In(location).Format(time.RFC3339Nano),
		"%u", record.runtimeURI,
		"%d", record.deploymentID,
		"%s", record.statementName,
		"%i", fmt.Sprintf("%d", record.agentInstanceID),
		"%c", string(record.category),
		"%m", record.message,
	}
	return strings.NewReplacer(replacements...).Replace(f.pattern)
}

// AuditListener receives committed audit records outside the Engine lock.
type AuditListener func(context.Context, AuditRecord) error

// SubscribeAudit registers one Engine-local audit observer.
func (e *Engine) SubscribeAudit(listener AuditListener) (*AuditSubscription, error) {
	if e == nil {
		return nil, NewError(ErrorDependency, "nil engine")
	}
	if listener == nil {
		return nil, NewError(ErrorInvalidRule, "audit listener is nil")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil, NewError(ErrorState, "engine is closed")
	}
	e.nextAuditListenerID++
	id := e.nextAuditListenerID
	e.auditListeners[id] = listener
	return &AuditSubscription{engine: e, id: id}, nil
}

type AuditSubscription struct {
	engine *Engine
	id     uint64
	once   sync.Once
}

func (s *AuditSubscription) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		if s.engine == nil {
			return
		}
		s.engine.mu.Lock()
		delete(s.engine.auditListeners, s.id)
		s.engine.mu.Unlock()
	})
	return nil
}

func (e *Engine) queueStatementAuditLocked(statement *Statement, runtime *statementRuntime, category AuditCategory, at time.Time, message string) {
	if e == nil || statement == nil || !auditCategoriesContain(statement.plan.query.statementMetadata.auditCategories, category) {
		return
	}
	partitionID := -1
	if runtime != nil && runtime.partitionContextName != "" {
		partitionID = runtime.partitionID
	}
	deploymentID := ""
	if statement.deployment != nil {
		deploymentID = statement.deployment.id
	}
	e.pendingAuditRecords = append(e.pendingAuditRecords, AuditRecord{
		runtimeURI: e.runtimeURI, deploymentID: deploymentID,
		statementName: statement.name, agentInstanceID: partitionID,
		category: category, virtualTime: at, message: message,
	})
}

func (e *Engine) takeAuditDispatchLocked() ([]AuditRecord, []AuditListener) {
	if e == nil || len(e.pendingAuditRecords) == 0 {
		return nil, nil
	}
	records := append([]AuditRecord(nil), e.pendingAuditRecords...)
	e.pendingAuditRecords = nil
	listeners := make([]AuditListener, 0, len(e.auditListeners))
	for id := uint64(1); id <= e.nextAuditListenerID; id++ {
		if listener := e.auditListeners[id]; listener != nil {
			listeners = append(listeners, listener)
		}
	}
	return records, listeners
}

func dispatchAuditRecords(ctx context.Context, records []AuditRecord, listeners []AuditListener) error {
	for _, record := range records {
		for _, listener := range listeners {
			if listener == nil {
				continue
			}
			if err := listener(ctx, record); err != nil {
				return fmt.Errorf("esper: audit listener: %w", err)
			}
		}
	}
	return nil
}

func (e *Engine) emitAuditRecord(ctx context.Context, record AuditRecord) error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	listeners := make([]AuditListener, 0, len(e.auditListeners))
	for id := uint64(1); id <= e.nextAuditListenerID; id++ {
		if listener := e.auditListeners[id]; listener != nil {
			listeners = append(listeners, listener)
		}
	}
	e.mu.Unlock()
	return dispatchAuditRecords(ctx, []AuditRecord{record}, listeners)
}

func auditValue(value any) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprintf("%v", value)
}

func auditEventSummary(event Event) string {
	underlying := event.Underlying()
	name := event.TypeName()
	if name == "" && underlying != nil {
		typ := reflect.TypeOf(underlying)
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		name = typ.Name()
	}
	return fmt.Sprintf("%s[%v]", name, underlying)
}

func auditStreamFilterText(query Query) string {
	input := query.input
	if query.aggregate != nil {
		input = query.aggregate.input
	}
	if query.rowRecog != nil {
		input = query.rowRecog.input
	}
	if input == nil {
		return "<stream>"
	}
	base, _ := sourceNode(input)
	name := "<stream>"
	if base != nil && base.sourceName != "" {
		name = base.sourceName
	}
	for node := input; node != nil; node = node.input {
		if node.kind != streamFilter || node.predicate == nil || node.predicate.node() == nil {
			continue
		}
		expression := node.predicate.node()
		if expression.kind == "eq" && len(expression.children) == 2 {
			left, right := expression.children[0], expression.children[1]
			if left.kind == "literal" {
				left, right = right, left
			}
			if left.kind == "field" && right.kind == "literal" {
				return fmt.Sprintf("%s(%s=...)", name, left.fieldName)
			}
		}
		return fmt.Sprintf("%s(%s)", name, node.predicate.Description())
	}
	return name
}

func auditResultValue(result Result, name string) (any, bool) {
	if row, ok := result.Row(); ok {
		value := row.Get(name)
		return value.Any(), !value.IsMissing()
	}
	if event, ok := result.Event(); ok {
		value := event.Get(name)
		return value.Any(), !value.IsMissing()
	}
	return nil, false
}

func queryAuditSelections(query Query) []Selection {
	if query.aggregate != nil {
		return query.aggregate.selections
	}
	if query.pattern != nil || query.rowRecog != nil {
		return query.patternSelections
	}
	return query.selections
}

func queryAuditExpressionNodes(query Query) []*exprNode {
	nodes := make([]*exprNode, 0)
	for _, selection := range queryAuditSelections(query) {
		if selection.Expr != nil {
			nodes = append(nodes, selection.Expr.node())
		}
	}
	return nodes
}

func auditNodeContainsExpressionDefinition(node *exprNode) bool {
	if node == nil {
		return false
	}
	if node.expressionName != "" || node.kind == "expression-ref" {
		return true
	}
	for _, child := range node.children {
		if auditNodeContainsExpressionDefinition(child) {
			return true
		}
	}
	return false
}

func queryHasWindow(node *streamNode) bool {
	for current := node; current != nil; current = current.input {
		if current.kind == streamWindow || current.kind == streamDerived {
			return true
		}
	}
	return false
}

func queryHasAuditableView(query Query) bool {
	if query.aggregate != nil && queryHasWindow(query.aggregate.input) {
		return true
	}
	if query.rowRecog != nil && queryHasWindow(query.rowRecog.input) {
		return true
	}
	return queryHasWindow(query.input)
}

func queryHasSchedule(query Query) bool {
	if query.pattern != nil && patternContainsTimer(query.pattern.root) {
		return true
	}
	if query.output.Kind != OutputAllPolicy || query.output.After != OutputAfterNone || query.output.Cron != nil || query.output.When != nil || len(query.output.Then) > 0 || query.output.Termination != OutputNoTermination || query.output.TerminationWhen != nil || len(query.output.TerminationThen) > 0 {
		return true
	}
	return queryHasAuditableView(query)
}

func statementAuditRuntime(statement *Statement) *statementRuntime {
	if statement == nil || statement.plan.query.contextName == "" {
		return nil
	}
	if len(statement.runtime.partitions) == 1 {
		for _, runtime := range statement.runtime.partitions {
			return runtime
		}
	}
	return nil
}

func (e *Engine) auditStatementProcessLocked(statement *Statement, event Event, at time.Time, accepted bool, batch ResultBatch, changed bool) {
	if e == nil || statement == nil || len(statement.plan.query.statementMetadata.auditCategories) == 0 {
		return
	}
	runtime := statementAuditRuntime(statement)
	query := statement.plan.query
	if accepted {
		e.queueStatementAuditLocked(statement, runtime, AuditStream, at, auditStreamFilterText(query)+" inserted "+auditEventSummary(event))
		for _, node := range queryAuditExpressionNodes(query) {
			fields := make([]string, 0)
			node.referencedFields(&fields)
			for _, field := range fields {
				value := event.Get(field)
				if !value.IsMissing() {
					e.queueStatementAuditLocked(statement, runtime, AuditProperty, at, field+" value "+auditValue(value.Any()))
				}
			}
		}
		if queryHasAuditableView(query) {
			e.queueStatementAuditLocked(statement, runtime, AuditView, at, fmt.Sprintf("view insert {%s} remove {%d}", auditEventSummary(event), len(batch.Old)))
		}
		if queryHasSchedule(query) {
			e.queueStatementAuditLocked(statement, runtime, AuditSchedule, at, "add after "+fmt.Sprintf("%d", at.UnixMilli())+" VIEW")
		}
	}
	if query.pattern != nil && accepted {
		e.queueStatementAuditLocked(statement, runtime, AuditPattern, at, query.pattern.description()+" evaluate-true")
		e.queueStatementAuditLocked(statement, runtime, AuditPatternInstances, at, query.pattern.description()+" increased to 1")
	}
	if !changed {
		return
	}
	selections := queryAuditSelections(query)
	for _, selection := range selections {
		value := any(nil)
		found := false
		if len(batch.New) > 0 {
			value, found = auditResultValue(batch.New[0], selection.Name)
		}
		if !found {
			value = "<evaluated>"
		}
		e.queueStatementAuditLocked(statement, runtime, AuditExpression, at, selection.Expr.Description()+" value "+auditValue(value))
		if selection.Expr != nil && len(selection.Expr.node().children) > 0 {
			e.queueStatementAuditLocked(statement, runtime, AuditExpressionNested, at, selection.Expr.Description()+" value "+auditValue(value))
		}
		if selection.Expr != nil && auditNodeContainsExpressionDefinition(selection.Expr.node()) {
			e.queueStatementAuditLocked(statement, runtime, AuditExpressionDefinition, at, selection.Expr.Description()+" value "+auditValue(value))
		}
	}
	if query.routeTarget != "" || query.tableTarget != "" {
		for _, result := range batch.New {
			message := fmt.Sprintf("%v", result)
			if event, ok := result.Event(); ok {
				message = auditEventSummary(event)
			}
			e.queueStatementAuditLocked(statement, runtime, AuditInsert, at, message)
		}
	}
}

func (e *Engine) auditStatementScheduleFireLocked(statement *Statement, at time.Time, batch ResultBatch, changed bool) {
	if !changed || statement == nil || !queryHasSchedule(statement.plan.query) {
		return
	}
	runtime := statementAuditRuntime(statement)
	e.queueStatementAuditLocked(statement, runtime, AuditSchedule, at, "fire VIEW")
	if queryHasAuditableView(statement.plan.query) {
		e.queueStatementAuditLocked(statement, runtime, AuditView, at, fmt.Sprintf("view insert {%d} remove {%d}", len(batch.New), len(batch.Old)))
	}
}

func (e *Engine) auditNamedWindowProcessLocked(statement *Statement, window *NamedWindow, delta NamedWindowDelta, at time.Time, changed bool, batch ResultBatch) {
	if statement == nil || window == nil || !statementConsumesNamedWindow(statement.plan.query, window) {
		return
	}
	runtime := statementAuditRuntime(statement)
	message := fmt.Sprintf("%s insert {%d} remove {%d}", window.state.def.Name(), len(delta.New), len(delta.Old))
	e.queueStatementAuditLocked(statement, runtime, AuditStream, at, message)
	if changed {
		e.queueStatementAuditLocked(statement, runtime, AuditView, at, fmt.Sprintf("named-window insert {%d} remove {%d}", len(batch.New), len(batch.Old)))
	}
}

func (e *Engine) auditContextPartitionLocked(contextName string, partitionID int, allocate bool, at time.Time) {
	if e == nil {
		return
	}
	verb := "Destroy"
	if allocate {
		verb = "Allocate"
	}
	for _, statement := range e.statements {
		if statement == nil || statement.plan.query.contextName != contextName {
			continue
		}
		e.queueStatementAuditLocked(statement, &statementRuntime{partitionContextName: contextName, partitionID: partitionID}, AuditContextPartition, at, fmt.Sprintf("%s cpid %d", verb, partitionID))
	}
}

func dataflowStateText(state DataflowState) string {
	switch state {
	case DataflowInstantiated:
		return "INSTANTIATED"
	case DataflowRunning:
		return "RUNNING"
	case DataflowComplete:
		return "COMPLETE"
	case DataflowCanceled:
		return "CANCELLED"
	default:
		return "(none)"
	}
}

func (d *DataflowInstance) emitDataflowAudit(ctx context.Context, category AuditCategory, message string) error {
	if d == nil || d.engine == nil || !auditCategoriesContain(d.definition.auditCategories, category) {
		return nil
	}
	record := AuditRecord{
		runtimeURI: d.engine.RuntimeURI(), deploymentID: "environment",
		statementName: d.definition.name, agentInstanceID: -1,
		category: category, virtualTime: d.engine.Now(), message: message,
	}
	return d.engine.emitAuditRecord(ctx, record)
}

func (d *DataflowInstance) auditDataflowTransition(ctx context.Context, from, to DataflowState, hasFrom bool) error {
	fromText := "(none)"
	if hasFrom {
		fromText = dataflowStateText(from)
	}
	message := fmt.Sprintf("dataflow %s instance %s from state %s to state %s", d.definition.name, d.options.InstanceID, fromText, dataflowStateText(to))
	return d.emitDataflowAudit(ctx, AuditDataflowTransition, message)
}

func (d *DataflowInstance) auditDataflowSource(ctx context.Context, operator DataflowOperator) error {
	number, _ := d.dataflowOperatorDetails(operator.Name)
	message := fmt.Sprintf("dataflow %s instance %s operator %s(%d) invoking source.next()", d.definition.name, d.options.InstanceID, operator.Name, number)
	return d.emitDataflowAudit(ctx, AuditDataflowSource, message)
}

func (d *DataflowInstance) auditDataflowOperator(ctx context.Context, operator DataflowOperator, values ...any) error {
	number, _ := d.dataflowOperatorDetails(operator.Name)
	message := fmt.Sprintf("dataflow %s instance %s operator %s(%d) parameters %v", d.definition.name, d.options.InstanceID, operator.Name, number, values)
	return d.emitDataflowAudit(ctx, AuditDataflowOperator, message)
}
