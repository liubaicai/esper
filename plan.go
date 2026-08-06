package esper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
)

const planSchemaVersion = "esper-go-plan/v2"

// Environment is the compile-time catalog for schemas and future extension
// registrations. It is safe to share for concurrent Plan construction.
type Environment struct {
	mu sync.RWMutex
	// buildMu serializes plan construction for one environment. Expression
	// references are expanded lazily during validation and may add the
	// definition node to the reference AST; keeping that one-time expansion
	// under a build lock preserves the concurrent-plan safety promised by the
	// Environment API without exposing mutable AST state to callers.
	buildMu          sync.Mutex
	schemas          map[string]Schema
	typeToName       map[reflect.Type]string
	variables        map[string]VariableDefinition
	aggregatePlugins map[string]aggregatePluginDefinition
	scripts          map[string]scriptDefinition
	tables           map[string]TableDefinition
	namedWindows     map[string]NamedWindowDefinition
	contexts         map[string]ContextDefinition
	dataflows        map[string]DataflowDefinition
	savedDataflows   map[string]DataflowDefinition
	expressions      map[string]ExpressionDefinition
}

func NewEnvironment() *Environment {
	return &Environment{
		schemas:          make(map[string]Schema),
		typeToName:       make(map[reflect.Type]string),
		variables:        make(map[string]VariableDefinition),
		aggregatePlugins: make(map[string]aggregatePluginDefinition),
		scripts:          make(map[string]scriptDefinition),
		tables:           make(map[string]TableDefinition),
		namedWindows:     make(map[string]NamedWindowDefinition),
		contexts:         make(map[string]ContextDefinition),
		dataflows:        make(map[string]DataflowDefinition),
		savedDataflows:   make(map[string]DataflowDefinition),
		expressions:      make(map[string]ExpressionDefinition),
	}
}

func (e *Environment) RegisterSchema(schema Schema) error {
	if e == nil {
		return fmt.Errorf("esper: nil environment")
	}
	if !schema.valid() {
		return fmt.Errorf("esper: cannot register an empty schema")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.schemas[schema.Name()]; exists {
		return fmt.Errorf("esper: schema %q is already registered", schema.Name())
	}
	if schema.kind == SchemaVariant && schema.variantMode == VariantPredefined {
		for _, member := range schema.variantMembers {
			if _, exists := e.schemas[member]; !exists {
				return fmt.Errorf("esper: variant schema %q references unregistered member schema %q", schema.Name(), member)
			}
		}
	}
	e.schemas[schema.Name()] = schema
	if schema.GoType() != nil {
		e.typeToName[schema.GoType()] = schema.Name()
	}
	return nil
}

func (e *Environment) Schema(name string) (Schema, bool) {
	if e == nil {
		return Schema{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	schema, ok := e.schemas[name]
	return schema, ok
}

// acceptsEventType reports whether an event declared as eventType can enter a
// source declared as targetType. Event inheritance is structural and may be
// multi-level or branched; a parent source therefore receives events of every
// registered descendant while sibling branches remain isolated.
func (e *Environment) acceptsEventType(targetType, eventType string) bool {
	if e == nil || strings.TrimSpace(targetType) == "" || strings.TrimSpace(eventType) == "" {
		return false
	}
	if targetType == eventType {
		return true
	}
	if target, ok := e.Schema(targetType); ok && target.kind == SchemaVariant {
		return e.variantAcceptsEventType(target, eventType)
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	visited := make(map[string]struct{})
	var visit func(string) bool
	visit = func(current string) bool {
		if current == targetType {
			return true
		}
		if _, seen := visited[current]; seen {
			return false
		}
		visited[current] = struct{}{}
		schema, ok := e.schemas[current]
		if !ok {
			return false
		}
		for _, parent := range schema.parentNames {
			if visit(parent) {
				return true
			}
		}
		return false
	}
	return visit(eventType)
}

func (e *Environment) variantAcceptsEventType(variant Schema, eventType string) bool {
	if e == nil || variant.kind != SchemaVariant || strings.TrimSpace(eventType) == "" {
		return false
	}
	if variant.variantMode == VariantAny {
		return true
	}
	for _, member := range variant.variantMembers {
		if e.acceptsEventType(member, eventType) {
			return true
		}
	}
	return false
}

func (e *Environment) Schemas() []Schema {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Schema, 0, len(e.schemas))
	for _, schema := range e.schemas {
		result = append(result, schema)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name() < result[j].Name() })
	return result
}

func (e *Environment) RegisterVariable(name string, initial any, options ...VariableOption) error {
	if e == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	definition, err := newVariableDefinition(name, initial, options...)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.variables[name]; exists {
		return fmt.Errorf("esper: variable %q is already registered", name)
	}
	e.variables[name] = definition
	return nil
}

// RegisterContextVariable declares a variable whose value is maintained
// independently for every materialized partition of contextName. The
// variable is visible only from statements using that context.
func (e *Environment) RegisterContextVariable(contextName, name string, initial any, options ...VariableOption) error {
	if e == nil {
		return NewError(ErrorDependency, "nil environment")
	}
	contextName = strings.TrimSpace(contextName)
	if contextName == "" {
		return NewError(ErrorInvalidRule, "context variable requires a context name")
	}
	if _, ok := e.Context(contextName); !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", contextName))
	}
	definition, err := newVariableDefinitionForContext(contextName, name, initial, options...)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.variables[name]; exists {
		return fmt.Errorf("esper: variable %q is already registered", name)
	}
	e.variables[name] = definition
	return nil
}

// RegisterVariableInContext is an expressive alias for RegisterContextVariable.
func (e *Environment) RegisterVariableInContext(contextName, name string, initial any, options ...VariableOption) error {
	return e.RegisterContextVariable(contextName, name, initial, options...)
}

func (e *Environment) Variable(name string) (VariableDefinition, bool) {
	if e == nil {
		return VariableDefinition{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.variables[name]
	return definition, ok
}

func (e *Environment) Variables() []VariableDefinition {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]VariableDefinition, 0, len(e.variables))
	for _, definition := range e.variables {
		result = append(result, definition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result
}

func (e *Environment) Tables() []TableDefinition {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]TableDefinition, 0, len(e.tables))
	for _, definition := range e.tables {
		result = append(result, definition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result
}

func (e *Environment) NamedWindows() []NamedWindowDefinition {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]NamedWindowDefinition, 0, len(e.namedWindows))
	for _, definition := range e.namedWindows {
		result = append(result, definition)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result
}

func RegisterStruct[T any](env *Environment, name string, opts ...SchemaOption) (Schema, error) {
	schema, err := StructSchema[T](name, opts...)
	if err != nil {
		return Schema{}, err
	}
	if err := env.RegisterSchema(schema); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

func RegisterMap(env *Environment, name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	schema, err := NewMapSchema(name, fields, opts...)
	if err != nil {
		return Schema{}, err
	}
	if err := env.RegisterSchema(schema); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

func RegisterJSON(env *Environment, name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	schema, err := NewJSONSchema(name, fields, opts...)
	if err != nil {
		return Schema{}, err
	}
	if err := env.RegisterSchema(schema); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

// RegisterJSONFor registers a JSON schema backed by a typed Go struct.
func RegisterJSONFor[T any](env *Environment, name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	schema, err := NewJSONSchemaFor[T](name, fields, opts...)
	if err != nil {
		return Schema{}, err
	}
	if err := env.RegisterSchema(schema); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

func RegisterXML(env *Environment, name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	schema, err := NewXMLSchema(name, fields, opts...)
	if err != nil {
		return Schema{}, err
	}
	if err := env.RegisterSchema(schema); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

func RegisterAvro(env *Environment, name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	schema, err := NewAvroSchema(name, fields, opts...)
	if err != nil {
		return Schema{}, err
	}
	if err := env.RegisterSchema(schema); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

// RegisterObjectArray registers a positional object-array event schema.
func RegisterObjectArray(env *Environment, name string, fields []FieldSpec, opts ...SchemaOption) (Schema, error) {
	schema, err := NewObjectArraySchema(name, fields, opts...)
	if err != nil {
		return Schema{}, err
	}
	if err := env.RegisterSchema(schema); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

// RegisterVariant registers a PREDEFINED variant whose members must already
// be present in the caller's schema catalog.
func RegisterVariant(env *Environment, name string, members ...Schema) (Schema, error) {
	schema, err := NewVariantSchema(name, members...)
	if err != nil {
		return Schema{}, err
	}
	if err := env.RegisterSchema(schema); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

// RegisterVariantAny registers an ANY variant. Runtime routing accepts every
// event type known to the environment at the time it is sent.
func RegisterVariantAny(env *Environment, name string, opts ...SchemaOption) (Schema, error) {
	schema, err := NewAnyVariantSchema(name, opts...)
	if err != nil {
		return Schema{}, err
	}
	if err := env.RegisterSchema(schema); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

// Plan is an immutable, canonicalized logical statement. It contains no raw
// function pointer and can later be extended with versioned serde metadata.
type Plan struct {
	schemaVersion string
	canonical     []byte
	hash          string
	query         Query
	resultSchema  Schema
}

func (p Plan) SchemaVersion() string { return p.schemaVersion }
func (p Plan) Hash() string          { return p.hash }
func (p Plan) Canonical() []byte     { return append([]byte(nil), p.canonical...) }
func (p Plan) Query() Query          { return p.query }
func (p Plan) ResultSchema() (Schema, bool) {
	return p.resultSchema, p.resultSchema.valid()
}

func (e *Environment) Build(query Query) (Plan, error) {
	if e == nil {
		return Plan{}, NewError(ErrorInvalidRule, "nil environment")
	}
	e.buildMu.Lock()
	defer e.buildMu.Unlock()
	if query.env == nil || query.env != e {
		return Plan{}, NewError(ErrorDependency, "query belongs to a different or nil environment")
	}
	if query.input == nil && query.join == nil && !query.sourceLess {
		return Plan{}, NewError(ErrorInvalidRule, "query has no source")
	}
	if query.discardPartialsOnMatch || query.suppressOverlappingMatches {
		if query.pattern == nil {
			return Plan{}, NewError(ErrorInvalidRule, "pattern consumption policies require a pattern query")
		}
		if query.contextName != "" || query.join != nil || query.trigger != nil {
			return Plan{}, NewError(ErrorInvalidRule, "pattern consumption policies are not supported with context, joins or actions")
		}
	}
	if query.sourceLess {
		if err := e.validateSourceLess(query.selections); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "select-once", err)
		}
	} else if query.trigger != nil {
		if err := e.validateTrigger(query.trigger); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "trigger", err)
		}
		if query.trigger.target == triggerTargetNamedWindow {
			if err := e.validateNamedWindowTriggerContext(query.trigger, query.contextName); err != nil {
				return Plan{}, WrapError(ErrorInvalidRule, "trigger context", err)
			}
		}
	} else if query.rowRecog != nil {
		if err := e.validateRowRecog(query.rowRecog, query.patternSelections); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "match-recognize", err)
		}
	} else if query.pattern != nil {
		if err := e.validatePattern(query.pattern, query.patternSelections); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "pattern", err)
		}
	} else if query.aggregate != nil {
		if err := e.validateAggregate(query.aggregate); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "aggregate", err)
		}
	} else if query.join != nil {
		if err := e.validateJoin(query.join, query.joinSelections); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "join", err)
		}
		if query.joinWhere != nil {
			if query.joinWhere.Type() != typeOf[bool]() {
				return Plan{}, WrapError(ErrorInvalidRule, "join where", NewError(ErrorTypeMismatch, "join where expression must return bool"))
			}
			if err := e.validateJoinScopedExpression(query.join, query.joinWhere, "join where"); err != nil {
				return Plan{}, WrapError(ErrorInvalidRule, "join where", err)
			}
		}
	} else if err := e.validateNode(query.input); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "stream", err)
	}
	if query.join == nil && (query.aggregate == nil || query.aggregate.join == nil) && streamHasMethodDependencies(query.input) {
		return Plan{}, WrapError(ErrorInvalidRule, "stream", NewError(ErrorInvalidRule, "method dependencies require a join"))
	}
	if query.contextName != "" {
		definition, ok := e.Context(query.contextName)
		if !ok {
			return Plan{}, NewError(ErrorUnknownName, fmt.Sprintf("context %q is not registered", query.contextName))
		}
		if query.output.Termination != OutputNoTermination && definition.kind != ContextInitiatedTerminated && !definition.isTemporal() {
			return Plan{}, NewError(ErrorInvalidRule, "context-termination output requires an initiated or temporal context")
		}
		if query.join != nil {
			for index, source := range joinDefinitionSources(query.join) {
				if err := e.validateContext(definition, source); err != nil {
					return Plan{}, WrapError(ErrorInvalidRule, fmt.Sprintf("context source %d", index), err)
				}
			}
		} else if err := e.validateContext(definition, query.input); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "context", err)
		}
	} else if err := validateContextFieldScope(query); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "context", err)
	}
	if err := e.validateContextVariableScope(query); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "context-variable", err)
	}
	if err := e.validateRoute(query); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "route", err)
	}
	if _, err := queryParameterTypes(e, query); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "parameters", err)
	}
	if err := validateOutputPolicy(query.output); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "output", err)
	}
	if err := e.validateOutputExpressions(query.output); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "output", err)
	}
	if err := e.validateQueryModifiers(query); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "result-set", err)
	}
	resultSchema, err := e.resultSchema(query)
	if err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "projection", err)
	}
	if err := e.validateIntoTable(query); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "into-table", err)
	}
	if query.name != "" && strings.TrimSpace(query.name) == "" {
		return Plan{}, fmt.Errorf("esper: statement name cannot be blank")
	}

	description := query.description()
	if query.routeTarget != "" {
		description += " -> route(" + query.routeTarget + ")"
	}
	canonicalParts := []string{planSchemaVersion, description}
	for _, schema := range e.Schemas() {
		fields := make([]string, 0, len(schema.fields))
		for _, field := range schema.fields {
			fields = append(fields, fmt.Sprintf("%s:%s:%t:%t:%t", field.Name, field.Type, field.Optional, field.StartTimestamp, field.EndTimestamp))
		}
		getterNames := make([]string, 0, len(schema.getters))
		for name := range schema.getters {
			getterNames = append(getterNames, name)
		}
		sort.Strings(getterNames)
		getters := make([]string, 0, len(getterNames))
		for _, name := range getterNames {
			getter := schema.getters[name]
			source := "callback"
			if getter.method != "" {
				source = "method:" + getter.method
			} else if getter.path != "" {
				source = "path:" + getter.path
			}
			getters = append(getters, fmt.Sprintf("%s:%s:%t:%s", getter.name, getter.typ, getter.optional, source))
		}
		setterNames := make([]string, 0, len(schema.setters))
		for name := range schema.setters {
			setterNames = append(setterNames, name)
		}
		sort.Strings(setterNames)
		setters := make([]string, 0, len(setterNames))
		for _, name := range setterNames {
			setter := schema.setters[name]
			source := "callback"
			if setter.method != "" {
				source = "method:" + setter.method
			}
			setters = append(setters, fmt.Sprintf("%s:%s:%s", setter.name, setter.typ, source))
		}
		nestedNames := make([]string, 0, len(schema.nested))
		for name, nested := range schema.nested {
			nestedNames = append(nestedNames, name+"="+nested.Name())
		}
		sort.Strings(nestedNames)
		members := strings.Join(schema.variantMembers, ",")
		parents := strings.Join(schema.parentNames, ",")
		canonicalParts = append(canonicalParts, fmt.Sprintf("schema(%s:%d:%d:%d:%t:variant=%d:parents=%s:%s:fields=%s:getters=%s:setters=%s:nested=%s)", schema.Name(), schema.kind, schema.resolution, schema.accessor, schema.allowDynamic, schema.variantMode, parents, members, strings.Join(fields, ","), strings.Join(getters, ","), strings.Join(setters, ","), strings.Join(nestedNames, ",")))
	}
	for _, variable := range e.Variables() {
		canonicalParts = append(canonicalParts, fmt.Sprintf("variable(%s:%s:%s:%s:%t)", variable.name, variable.context, variable.typ, variable.initial.String(), variable.constant))
	}
	e.mu.RLock()
	pluginNames := make([]string, 0, len(e.aggregatePlugins))
	pluginTypes := make(map[string]reflect.Type, len(e.aggregatePlugins))
	pluginFactoryFlags := make(map[string]bool, len(e.aggregatePlugins))
	for name, plugin := range e.aggregatePlugins {
		pluginNames = append(pluginNames, name)
		pluginTypes[name] = plugin.resultType
		pluginFactoryFlags[name] = plugin.factory != nil
	}
	e.mu.RUnlock()
	sort.Strings(pluginNames)
	for _, name := range pluginNames {
		canonicalParts = append(canonicalParts, fmt.Sprintf("aggregate-plugin(%s:%s:factory=%t)", name, pluginTypes[name], pluginFactoryFlags[name]))
	}
	e.mu.RLock()
	scriptNames := make([]string, 0, len(e.scripts))
	scriptDefinitions := make(map[string]scriptDefinition, len(e.scripts))
	for name, definition := range e.scripts {
		scriptNames = append(scriptNames, name)
		scriptDefinitions[name] = definition
	}
	e.mu.RUnlock()
	sort.Strings(scriptNames)
	for _, name := range scriptNames {
		definition := scriptDefinitions[name]
		argumentTypes := make([]string, 0, len(definition.argumentTypes))
		for _, argumentType := range definition.argumentTypes {
			if argumentType == nil {
				argumentTypes = append(argumentTypes, "any")
				continue
			}
			argumentTypes = append(argumentTypes, argumentType.String())
		}
		canonicalParts = append(canonicalParts, fmt.Sprintf("script(%s:%s:%s:%t:%s)", name, definition.dialect, definition.resultType, definition.argumentTypesSet, strings.Join(argumentTypes, ",")))
	}
	e.mu.RLock()
	expressionNames := make([]string, 0, len(e.expressions))
	expressionDefinitions := make(map[string]ExpressionDefinition, len(e.expressions))
	for name, definition := range e.expressions {
		expressionNames = append(expressionNames, name)
		expressionDefinitions[name] = definition
	}
	e.mu.RUnlock()
	sort.Strings(expressionNames)
	for _, name := range expressionNames {
		definition := expressionDefinitions[name]
		description := "<nil>"
		resultType := "<nil>"
		parameterTypes := make([]string, 0, len(definition.Parameters))
		for _, parameter := range definition.Parameters {
			parameterTypes = append(parameterTypes, fmt.Sprintf("%s:%s", parameter.Name, parameter.Type))
		}
		if definition.Expr != nil {
			description = definition.Expr.Description()
			if definition.Expr.Type() != nil {
				resultType = definition.Expr.Type().String()
			}
		}
		canonicalParts = append(canonicalParts, fmt.Sprintf("expression(%s:%s:%s:%s)", name, resultType, strings.Join(parameterTypes, ","), description))
	}
	for _, table := range e.Tables() {
		columns := make([]string, 0, len(table.columns))
		for _, column := range table.columns {
			columns = append(columns, fmt.Sprintf("%s:%s:%t:%t", column.Name, column.Type, column.Optional, column.PrimaryKey))
		}
		indexes := make([]string, 0, len(table.indexes))
		for _, index := range table.indexes {
			indexes = append(indexes, fmt.Sprintf("%s:%s:%t", index.Name, strings.Join(index.Columns, ","), index.Unique))
		}
		canonicalParts = append(canonicalParts, "table("+table.name+":"+strings.Join(columns, ",")+":"+strings.Join(indexes, ",")+")")
	}
	for _, window := range e.NamedWindows() {
		fields := make([]string, 0, len(window.schema.fields))
		for _, field := range window.schema.fields {
			fields = append(fields, fmt.Sprintf("%s:%s:%t:%t:%t", field.Name, field.Type, field.Optional, field.StartTimestamp, field.EndTimestamp))
		}
		canonicalParts = append(canonicalParts, "named-window("+window.name+":"+window.schema.Name()+":"+window.retention.description()+":context="+window.contextName+":"+strings.Join(fields, ",")+")")
	}
	for _, context := range e.Contexts() {
		canonicalParts = append(canonicalParts, "context("+context.name+":"+context.description()+")")
	}
	for _, dataflow := range e.Dataflows() {
		operators := make([]string, 0, len(dataflow.operators))
		for _, operator := range dataflow.operators {
			predicate := ""
			if operator.Predicate != nil {
				predicate = operator.Predicate.Description()
			}
			sourceFilter := ""
			if operator.SourceFilter != nil {
				sourceFilter = operator.SourceFilter.Description()
			}
			selections := make([]string, 0, len(operator.Selections))
			for _, selection := range operator.Selections {
				selections = append(selections, selection.description())
			}
			statement := ""
			if operator.Statement != nil {
				statement = operator.Statement.Name()
			}
			beacon := ""
			if operator.Kind == BeaconSourceKind && operator.BeaconConfigured {
				iterationsExpression := ""
				if operator.BeaconOptions.IterationsExpression != nil {
					iterationsExpression = operator.BeaconOptions.IterationsExpression.Description()
				}
				initialDelayExpression := ""
				if operator.BeaconOptions.InitialDelayExpression != nil {
					initialDelayExpression = operator.BeaconOptions.InitialDelayExpression.Description()
				}
				intervalExpression := ""
				if operator.BeaconOptions.IntervalExpression != nil {
					intervalExpression = operator.BeaconOptions.IntervalExpression.Description()
				}
				fieldParameters := append([]string(nil), operator.BeaconOptions.FieldParameters...)
				sort.Strings(fieldParameters)
				beacon = fmt.Sprintf("iterations=%d;iterationsExpr=%s;initial=%s;initialExpr=%s;interval=%s;intervalExpr=%s;factory=%t;event=%t;underlying=%t;fieldParameters=%s", operator.BeaconOptions.Iterations, iterationsExpression, operator.BeaconOptions.InitialDelay, initialDelayExpression, operator.BeaconOptions.Interval, intervalExpression, operator.BeaconOptions.Factory != nil, operator.BeaconEventConfigured, operator.BeaconUnderlying, strings.Join(fieldParameters, ","))
			}
			parameterNames := append([]string(nil), operator.ParameterNames...)
			sort.Strings(parameterNames)
			operators = append(operators, fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s:select=%s:join=%s:inputs=%s:outputs=%s:properties=%s:parameters=%s", operator.Name, operator.Kind, operator.EventType, predicate, sourceFilter, strings.Join(selections, ","), statement, beacon, canonicalDataflowSelect(operator), canonicalDataflowJoin(operator), canonicalDataflowPorts(operator, false), canonicalDataflowPorts(operator, true), canonicalDataflowProperties(operator.Properties), strings.Join(parameterNames, ",")))
		}
		edges := make([]string, 0, len(dataflow.edges))
		for _, edge := range dataflow.edges {
			edges = append(edges, fmt.Sprintf("%s:%s>%s:%s:feedback=%t", edge.From, edge.FromPort, edge.To, edge.ToPort, edge.Feedback))
		}
		canonicalParts = append(canonicalParts, "dataflow("+dataflow.name+":"+strings.Join(operators, ",")+":edges("+strings.Join(edges, ",")+"))")
	}
	canonical := []byte(strings.Join(canonicalParts, "\n"))
	digest := sha256.Sum256(canonical)
	return Plan{
		schemaVersion: planSchemaVersion,
		canonical:     canonical,
		hash:          hex.EncodeToString(digest[:]),
		query:         query,
		resultSchema:  resultSchema,
	}, nil
}

func canonicalDataflowSelect(operator DataflowOperator) string {
	if operator.Kind != SelectKind {
		return ""
	}
	options := operator.SelectOptions
	groups := make([]string, 0, len(options.GroupBy))
	for _, expression := range options.GroupBy {
		if expression == nil {
			groups = append(groups, "<nil>")
			continue
		}
		groups = append(groups, expression.Description())
	}
	ordering := make([]string, 0, len(options.OrderBy))
	for _, key := range options.OrderBy {
		description := "<nil>"
		if key.Expr != nil {
			description = key.Expr.Description()
		}
		ordering = append(ordering, fmt.Sprintf("%s:%t", description, key.Descending))
	}
	return fmt.Sprintf("output=%s;preserve=%t;time=%s;snapshot=%s;iterate=%t;group=%s;order=%s",
		options.OutputEventType,
		options.PreserveInput,
		options.TimeWindow,
		options.OutputSnapshotEvery,
		options.IterateOnFinalMarker,
		strings.Join(groups, ","),
		strings.Join(ordering, ","),
	)
}

func canonicalDataflowJoin(operator DataflowOperator) string {
	if !operator.JoinConfigured {
		return ""
	}
	conditions := make([]string, 0, len(operator.JoinOptions.Conditions))
	for _, condition := range operator.JoinOptions.Conditions {
		conditions = append(conditions, joinConditionDescription(condition))
	}
	windows := append([]DataflowJoinWindow(nil), operator.JoinOptions.Windows...)
	sort.SliceStable(windows, func(left, right int) bool {
		return windows[left].Input < windows[right].Input
	})
	windowParts := make([]string, 0, len(windows))
	for _, window := range windows {
		windowParts = append(windowParts, fmt.Sprintf("%d:length=%d:duration=%s", window.Input, window.Length, window.Duration))
	}
	return fmt.Sprintf("inputs=%d;kind=%d;retention=%d;conditions=%s;windows=%s", operator.JoinOptions.Inputs, operator.JoinOptions.Kind, operator.JoinOptions.Retention, strings.Join(conditions, ","), strings.Join(windowParts, ","))
}

func canonicalDataflowPorts(operator DataflowOperator, output bool) string {
	ports := operator.InputPorts
	defaultPort := "in"
	if output {
		ports = operator.OutputPorts
		defaultPort = "out"
	}
	if len(ports) == 0 && dataflowPortAllowed(operator, output, defaultPort) {
		ports = []string{defaultPort}
	}
	definitions := make([]string, 0, len(ports))
	for _, port := range ports {
		typ := "*"
		if portType := dataflowPortType(operator, output, port); portType != nil {
			typ = portType.String()
		}
		definitions = append(definitions, port+"="+typ)
	}
	return "[" + strings.Join(definitions, ",") + "]"
}

type canonicalDataflowReference struct {
	typ  reflect.Type
	kind reflect.Kind
	ptr  uintptr
}

func canonicalDataflowProperties(properties map[string]any) string {
	if properties == nil {
		return "nil"
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%q=%s", name, canonicalDataflowPropertyValue(reflect.ValueOf(properties[name]), make(map[canonicalDataflowReference]bool))))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func canonicalDataflowPropertyValue(value reflect.Value, visiting map[canonicalDataflowReference]bool) string {
	if !value.IsValid() {
		return "nil"
	}
	if value.CanInterface() {
		if expression, ok := value.Interface().(Expr); ok && expression != nil {
			return "expr(" + expression.Description() + ")"
		}
	}
	typ := value.Type().String()
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return typ + "(nil)"
		}
		return typ + "(" + canonicalDataflowPropertyValue(value.Elem(), visiting) + ")"
	case reflect.Bool:
		return fmt.Sprintf("%s(%t)", typ, value.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%s(%d)", typ, value.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return fmt.Sprintf("%s(%d)", typ, value.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%s(%g)", typ, value.Float())
	case reflect.Complex64, reflect.Complex128:
		return fmt.Sprintf("%s(%g)", typ, value.Complex())
	case reflect.String:
		return fmt.Sprintf("%s(%q)", typ, value.String())
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if value.IsNil() {
			return typ + "(nil)"
		}
		reference := canonicalDataflowReference{typ: value.Type(), kind: value.Kind(), ptr: value.Pointer()}
		if visiting[reference] {
			return typ + "(<cycle>)"
		}
		visiting[reference] = true
		defer delete(visiting, reference)
		if value.Kind() == reflect.Pointer {
			return typ + "(&" + canonicalDataflowPropertyValue(value.Elem(), visiting) + ")"
		}
		if value.Kind() == reflect.Slice {
			parts := make([]string, value.Len())
			for index := 0; index < value.Len(); index++ {
				parts[index] = canonicalDataflowPropertyValue(value.Index(index), visiting)
			}
			return typ + "([" + strings.Join(parts, ",") + "])"
		}
		parts := make([]string, 0, value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			parts = append(parts, canonicalDataflowPropertyValue(iterator.Key(), visiting)+"="+canonicalDataflowPropertyValue(iterator.Value(), visiting))
		}
		sort.Strings(parts)
		return typ + "({" + strings.Join(parts, ",") + "})"
	case reflect.Array:
		parts := make([]string, value.Len())
		for index := 0; index < value.Len(); index++ {
			parts[index] = canonicalDataflowPropertyValue(value.Index(index), visiting)
		}
		return typ + "([" + strings.Join(parts, ",") + "])"
	case reflect.Struct:
		parts := make([]string, value.NumField())
		for index := 0; index < value.NumField(); index++ {
			parts[index] = value.Type().Field(index).Name + "=" + canonicalDataflowPropertyValue(value.Field(index), visiting)
		}
		return typ + "({" + strings.Join(parts, ",") + "})"
	default:
		return typ
	}
}

func (e *Environment) validateNamedWindowTriggerContext(definition *triggerDefinition, contextName string) error {
	if e == nil || definition == nil || definition.target != triggerTargetNamedWindow {
		return nil
	}
	window, ok := e.NamedWindow(definition.table)
	if !ok {
		return nil
	}
	declared := strings.TrimSpace(window.contextName)
	contextName = strings.TrimSpace(contextName)
	if declared == contextName {
		return nil
	}
	if declared == "" {
		if contextName != "" {
			return NewError(ErrorInvalidRule, fmt.Sprintf("named window %q was declared without a context, but trigger uses context %q", definition.table, contextName))
		}
		return nil
	}
	if contextName == "" {
		return NewError(ErrorInvalidRule, fmt.Sprintf("named window %q was declared with context %q; trigger must declare the same context", definition.table, declared))
	}
	return NewError(ErrorInvalidRule, fmt.Sprintf("named window %q was declared with context %q; trigger uses context %q", definition.table, declared, contextName))
}

func validateContextFieldScope(query Query) error {
	return visitQueryExpressions(nil, query, func(expression Expr) error {
		if expression != nil && (expressionContainsKind(expression.node(), "context-field") ||
			expressionContainsKind(expression.node(), "context-pattern-event") ||
			expressionContainsKind(expression.node(), "context-pattern-field")) {
			return NewError(ErrorInvalidRule, "context fields require a statement context")
		}
		return nil
	})
}

func (e *Environment) validateContextVariableScope(query Query) error {
	return visitQueryExpressions(e, query, func(expression Expr) error {
		if expression == nil || expression.node() == nil {
			return nil
		}
		var names []string
		expression.node().referencedVariables(&names)
		for _, name := range names {
			definition, ok := e.Variable(name)
			if !ok || definition.context == "" {
				continue
			}
			if query.contextName == "" {
				return NewError(ErrorInvalidRule, fmt.Sprintf("context variable %q can only be accessed within context %q", name, definition.context))
			}
			if query.contextName != definition.context {
				return NewError(ErrorInvalidRule, fmt.Sprintf("context variable %q belongs to context %q, not %q", name, definition.context, query.contextName))
			}
		}
		return nil
	})
}

func expressionContainsKind(node *exprNode, kind string) bool {
	if node == nil {
		return false
	}
	if node.kind == kind {
		return true
	}
	for _, child := range node.children {
		if expressionContainsKind(child, kind) {
			return true
		}
	}
	if node.subquery != nil {
		if node.subquery.predicate != nil && expressionContainsKind(node.subquery.predicate.node(), kind) {
			return true
		}
		if node.subquery.projection != nil && expressionContainsKind(node.subquery.projection.node(), kind) {
			return true
		}
		for _, selection := range node.subquery.columns {
			if selection.Expr != nil && expressionContainsKind(selection.Expr.node(), kind) {
				return true
			}
		}
		if node.subquery.groupBy != nil && expressionContainsKind(node.subquery.groupBy.node(), kind) {
			return true
		}
		if node.subquery.having != nil && expressionContainsKind(node.subquery.having.node(), kind) {
			return true
		}
		for _, order := range node.subquery.orderBy {
			if order.Expression != nil && expressionContainsKind(order.Expression.node(), kind) {
				return true
			}
		}
	}
	return false
}

func (e *Environment) validateRoute(query Query) error {
	if query.routeTarget == "" {
		return nil
	}
	target, ok := e.Schema(query.routeTarget)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("route target %q is not registered", query.routeTarget))
	}
	if query.sourceLess || query.trigger != nil {
		return fmt.Errorf("route target %q requires a stream or projection source", query.routeTarget)
	}
	if query.rowRecog != nil {
		return fmt.Errorf("route target %q is not supported for match-recognize queries", query.routeTarget)
	}
	selections := append([]Selection(nil), query.selections...)
	switch {
	case query.aggregate != nil:
		selections = append(selections, query.aggregate.selections...)
	case query.join != nil:
		for _, selection := range query.joinSelections {
			selections = append(selections, Selection{Name: selection.Name, Expr: selection.Expr})
		}
	case query.pattern != nil:
		selections = append(selections, query.patternSelections...)
	}
	if len(selections) == 0 {
		if query.input == nil {
			return fmt.Errorf("route target %q has no input source", query.routeTarget)
		}
		source, err := sourceNode(query.input)
		if err != nil {
			return err
		}
		sourceSchema, err := e.sourceSchema(source)
		if err != nil {
			return err
		}
		if target.kind == SchemaVariant && target.variantMode == VariantPredefined && sourceSchema.kind != SchemaVariant {
			if !e.variantAcceptsEventType(target, sourceSchema.Name()) {
				return fmt.Errorf("source event type %q is not a member of predefined variant %q", sourceSchema.Name(), target.Name())
			}
		}
		return nil
	}
	if target.kind == SchemaVariant && target.variantMode == VariantPredefined {
		return fmt.Errorf("projected rows cannot route to predefined variant %q without a member identity", target.Name())
	}
	for index, selection := range selections {
		if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
			return fmt.Errorf("route projection %d requires a name and expression", index)
		}
		field, exists := target.Field(selection.Name)
		if !exists {
			if target.kind == SchemaMap || target.kind == SchemaJSON || target.kind == SchemaXML || target.IsVariantAny() {
				continue
			}
			return fmt.Errorf("route projection %q is not a field of target schema %q", selection.Name, target.Name())
		}
		if field.Type != nil && field.Type != typeOf[any]() && selection.Expr.Type() != nil &&
			!field.Type.AssignableTo(selection.Expr.Type()) && !selection.Expr.Type().AssignableTo(field.Type) && !numericTypes(field.Type, selection.Expr.Type()) {
			return fmt.Errorf("route projection %q has type %s, target expects %s", selection.Name, selection.Expr.Type(), field.Type)
		}
	}
	return nil
}

func (e *Environment) validateContext(definition ContextDefinition, node *streamNode) error {
	if node == nil {
		return fmt.Errorf("context requires a source stream")
	}
	if definition.parent != nil {
		if definition.parent.kind == ContextInitiatedTerminated {
			return fmt.Errorf("initiated-terminated parent contexts cannot be nested")
		}
		if definition.kind == ContextInitiatedTerminated && definition.startPattern != nil {
			return fmt.Errorf("pattern initiated children are not supported in nested contexts")
		}
		if definition.isTemporal() || definition.parent.isTemporal() {
			return fmt.Errorf("temporal contexts cannot be nested")
		}
		if err := e.validateContext(*definition.parent, node); err != nil {
			return fmt.Errorf("parent context: %w", err)
		}
	}
	switch definition.kind {
	case ContextKeySegmented, ContextHashSegmented, ContextInitiatedTerminated:
		if definition.kind == ContextInitiatedTerminated && definition.startPattern != nil {
			if definition.patternEnvironment != nil && definition.patternEnvironment != e {
				return NewError(ErrorDependency, "pattern context belongs to a different environment")
			}
			if err := e.validateContextPattern(definition.startPattern, "start"); err != nil {
				return err
			}
			if definition.endPattern != nil {
				if err := e.validateContextPattern(definition.endPattern, "end"); err != nil {
					return err
				}
			}
			return nil
		}
		keys := definition.contextKeys()
		if len(keys) == 0 {
			return fmt.Errorf("context key expression is required")
		}
		for index, key := range keys {
			var err error
			if definition.kind == ContextInitiatedTerminated {
				err = e.validateContextLifecycleExpression(key)
			} else {
				err = e.validateExprFields(node, key)
			}
			if err != nil {
				return fmt.Errorf("context key %d: %w", index+1, err)
			}
		}
		if definition.kind == ContextInitiatedTerminated {
			if definition.start == nil || definition.start.Type() != typeOf[bool]() {
				return fmt.Errorf("initiated-terminated context requires a bool start expression")
			}
			if err := e.validateContextLifecycleExpression(definition.start); err != nil {
				return fmt.Errorf("context start: %w", err)
			}
			if definition.end != nil {
				if definition.end.Type() != typeOf[bool]() {
					return fmt.Errorf("initiated-terminated context requires a bool end expression")
				}
				if err := e.validateContextLifecycleExpression(definition.end); err != nil {
					return fmt.Errorf("context end: %w", err)
				}
			}
		}
		if definition.kind == ContextHashSegmented && definition.partitions <= 0 {
			return fmt.Errorf("hash context partitions must be positive")
		}
	case ContextCategorySegmented:
		if len(definition.categories) == 0 {
			return fmt.Errorf("category context requires categories")
		}
		for _, category := range definition.categories {
			if category.predicate == nil || category.predicate.Type() != typeOf[bool]() {
				return fmt.Errorf("category %q requires a bool predicate", category.name)
			}
			if err := e.validateExprFields(node, category.predicate); err != nil {
				return fmt.Errorf("category %q: %w", category.name, err)
			}
		}
	case ContextTimePeriod:
		if definition.temporalStartAfter < 0 || definition.temporalActiveFor <= 0 {
			return fmt.Errorf("temporal context requires non-negative start delay and positive active duration")
		}
	case ContextDailyTime:
		if !definition.dailyStart.valid() || !definition.dailyEnd.valid() || definition.dailyStart.duration() == definition.dailyEnd.duration() {
			return fmt.Errorf("daily temporal context requires distinct valid start and end times")
		}
	case ContextCronTime:
		if definition.cronStart == nil || definition.cronEnd == nil || definition.cronStartResolved == nil || definition.cronEndResolved == nil {
			return fmt.Errorf("cron temporal context requires valid start and end schedules")
		}
	default:
		return fmt.Errorf("unknown context kind %d", definition.kind)
	}
	return nil
}

func (e *Environment) validateContextPattern(definition *patternDefinition, label string) error {
	if definition == nil {
		return fmt.Errorf("context %s pattern is required", label)
	}
	if err := validatePattern(definition); err != nil {
		return fmt.Errorf("context %s pattern: %w", label, err)
	}
	if err := e.validateNode(definition.input); err != nil {
		return fmt.Errorf("context %s pattern source: %w", label, err)
	}
	if definition.guard != nil {
		if err := e.validateExprFields(definition.input, definition.guard); err != nil {
			return fmt.Errorf("context %s pattern guard: %w", label, err)
		}
	}
	if err := e.validatePatternNodeFields(definition.input, definition.root, patternTagSources(definition), false); err != nil {
		return fmt.Errorf("context %s pattern: %w", label, err)
	}
	return nil
}

// validateContextLifecycleExpression validates the expression tree without
// binding its event fields to the statement source. Initiated-terminated
// contexts may be started and terminated by event types different from the
// stream consumed by a context statement; runtime evaluation supplies the
// incoming lifecycle event and the partition's captured context properties.
func (e *Environment) validateContextLifecycleExpression(expression Expr) error {
	if expression == nil {
		return fmt.Errorf("esper: nil expression")
	}
	if err := validateBitwiseExpressionNodes(expression.node()); err != nil {
		return err
	}
	if err := validateCoalesceExpressionNodes(expression.node()); err != nil {
		return err
	}
	if err := validateMethodNodes(expression.node()); err != nil {
		return err
	}
	if err := e.validateExprVariables(expression); err != nil {
		return err
	}
	return e.validateExpressionSubqueries(expression.node())
}

func (e *Environment) validateQueryModifiers(query Query) error {
	if query.limit < 0 || query.offset < 0 {
		return fmt.Errorf("limit and offset cannot be negative")
	}
	if query.rowRecog != nil && query.output.Kind == OutputSnapshotPolicy {
		return fmt.Errorf("snapshot output is not yet supported for match-recognize queries")
	}
	if query.rowRecog != nil && query.selector != SelectIStream {
		return fmt.Errorf("remove-stream selection is not yet supported for match-recognize queries")
	}
	for index, key := range query.orderBy {
		if key.Expr == nil {
			return fmt.Errorf("order-by key %d is nil", index)
		}
		if query.rowRecog != nil {
			node := key.Expr.node()
			if node == nil || node.kind != "result-field" {
				return fmt.Errorf("order-by key %d for match-recognize must use ResultField", index)
			}
			known := false
			for _, selection := range query.patternSelections {
				if selection.Name == node.fieldName {
					known = true
					break
				}
			}
			if !known {
				return NewError(ErrorUnknownName, fmt.Sprintf("order-by key %d references unknown match-recognize measure %q", index, node.fieldName))
			}
			continue
		}
		if query.sourceLess || query.pattern != nil {
			return fmt.Errorf("order-by is not yet supported for source-less, join, or pattern queries")
		}
		if query.join != nil && query.aggregate == nil {
			node := key.Expr.node()
			if node != nil && node.kind == "result-field" {
				known := false
				for _, selection := range query.joinSelections {
					if selection.Name == node.fieldName {
						known = true
						break
					}
				}
				if !known {
					return NewError(ErrorUnknownName, fmt.Sprintf("order-by key %d references unknown join result %q", index, node.fieldName))
				}
				continue
			}
			if err := e.validateJoinScopedExpression(query.join, key.Expr, "join order-by"); err != nil {
				return fmt.Errorf("order-by key %d: %w", index, err)
			}
			continue
		}
		if query.aggregate != nil && key.Expr.node() != nil && key.Expr.node().kind == "result-field" {
			known := false
			for _, selection := range query.aggregate.selections {
				if selection.Name == key.Expr.node().fieldName {
					known = true
					break
				}
			}
			if !known {
				return NewError(ErrorUnknownName, fmt.Sprintf("order-by key %d references unknown aggregate result %q", index, key.Expr.node().fieldName))
			}
			continue
		}
		input := query.input
		if query.aggregate != nil {
			input = query.aggregate.input
		}
		var err error
		if query.aggregate != nil && query.aggregate.join != nil {
			err = e.validateJoinAggregateFields(query.aggregate.join, key.Expr)
		} else {
			err = e.validateExprFields(input, key.Expr)
		}
		if err != nil {
			return fmt.Errorf("order-by key %d: %w", index, err)
		}
	}
	return nil
}

func (e *Environment) validateNode(node *streamNode) error {
	if node == nil {
		return fmt.Errorf("esper: nil stream node")
	}
	if node.configurationError != "" {
		return NewError(ErrorInvalidRule, node.configurationError)
	}
	switch node.kind {
	case streamSource:
		schema, ok := e.Schema(node.sourceName)
		if !ok {
			return fmt.Errorf("esper: source %q has no registered schema", node.sourceName)
		}
		if node.sourceType != nil && node.sourceType != typeOf[any]() && schema.GoType() != nil {
			if !node.sourceType.AssignableTo(schema.GoType()) && !schema.GoType().AssignableTo(node.sourceType) {
				return fmt.Errorf("esper: source %q expects Go type %s, schema exposes %s", node.sourceName, node.sourceType, schema.GoType())
			}
		}
		return nil
	case streamNamedWindow:
		definition, ok := e.NamedWindow(node.sourceName)
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", node.sourceName))
		}
		if node.sourceType != nil && node.sourceType != typeOf[any]() && definition.schema.GoType() != nil {
			if !node.sourceType.AssignableTo(definition.schema.GoType()) && !definition.schema.GoType().AssignableTo(node.sourceType) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("named window %q expects Go type %s, schema exposes %s", node.sourceName, node.sourceType, definition.schema.GoType()))
			}
		}
		return nil
	case streamTable:
		definition, ok := e.Table(node.sourceName)
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("table %q is not registered", node.sourceName))
		}
		if node.sourceType != nil && node.sourceType != typeOf[any]() && definition.schema.GoType() != nil {
			if !node.sourceType.AssignableTo(definition.schema.GoType()) && !definition.schema.GoType().AssignableTo(node.sourceType) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("table %q expects Go type %s, schema exposes %s", node.sourceName, node.sourceType, definition.schema.GoType()))
			}
		}
		return nil
	case streamHistorical:
		if node.historical == nil || node.historical.provider == nil {
			return NewError(ErrorDependency, fmt.Sprintf("historical source %q has no provider", node.sourceName))
		}
		if !node.historical.schema.valid() {
			return NewError(ErrorInvalidRule, fmt.Sprintf("historical source %q has no schema", node.sourceName))
		}
		if node.historical.trigger != "" {
			if _, ok := e.Schema(node.historical.trigger); !ok {
				return NewError(ErrorUnknownName, fmt.Sprintf("historical source %q references unknown trigger type %q", node.sourceName, node.historical.trigger))
			}
		}
		if node.sourceType != nil && node.sourceType != typeOf[any]() && node.historical.schema.GoType() != nil {
			if !node.sourceType.AssignableTo(node.historical.schema.GoType()) && !node.historical.schema.GoType().AssignableTo(node.sourceType) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("historical source %q expects Go type %s, schema exposes %s", node.sourceName, node.sourceType, node.historical.schema.GoType()))
			}
		}
		return nil
	case streamMethod:
		if node.method == nil || node.method.provider == nil {
			return NewError(ErrorDependency, fmt.Sprintf("method source %q has no provider", node.sourceName))
		}
		if !node.method.schema.valid() {
			return NewError(ErrorInvalidRule, fmt.Sprintf("method source %q has no schema", node.sourceName))
		}
		if node.method.trigger != "" {
			if _, ok := e.Schema(node.method.trigger); !ok {
				return NewError(ErrorUnknownName, fmt.Sprintf("method source %q references unknown trigger type %q", node.sourceName, node.method.trigger))
			}
		}
		if node.sourceType != nil && node.sourceType != typeOf[any]() && node.method.schema.GoType() != nil {
			if !node.sourceType.AssignableTo(node.method.schema.GoType()) && !node.method.schema.GoType().AssignableTo(node.sourceType) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("method source %q expects Go type %s, schema exposes %s", node.sourceName, node.sourceType, node.method.schema.GoType()))
			}
		}
		return nil
	case streamPattern:
		if node.pattern == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("pattern source %q has no pattern definition", node.sourceName))
		}
		if err := validatePattern(node.pattern); err != nil {
			return err
		}
		if _, err := patternJoinSchema(node.pattern); err != nil {
			return err
		}
		for index, input := range patternDefinitionInputs(node.pattern) {
			if err := e.validateNode(input); err != nil {
				return fmt.Errorf("pattern source %d: %w", index, err)
			}
		}
		if node.patternWindow != nil {
			if err := node.patternWindow.validate(); err != nil {
				return err
			}
		}
		if node.pattern.root != nil {
			return e.validatePatternNodeFields(node.pattern.input, node.pattern.root, patternTagSources(node.pattern), true)
		}
		for _, step := range node.pattern.steps {
			if err := e.validateExprFields(node.pattern.input, step.predicate); err != nil {
				return err
			}
		}
		return nil
	case streamDerived:
		if node.derived == nil || node.derived.aggregate == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("derived source %q has no aggregate definition", node.sourceName))
		}
		if !node.derived.schema.valid() {
			return NewError(ErrorInvalidRule, fmt.Sprintf("derived source %q has no result schema", node.sourceName))
		}
		if node.derived.aggregate.join != nil {
			return NewError(ErrorInvalidRule, "derived aggregate source cannot wrap a join aggregate")
		}
		return e.validateAggregate(node.derived.aggregate)
	case streamContained:
		if node.contained == nil || node.contained.property == nil {
			return NewError(ErrorInvalidRule, "unnest stream requires a contained property expression")
		}
		if node.input == nil {
			return NewError(ErrorDependency, "unnest stream has no parent input")
		}
		if err := e.validateNode(node.input); err != nil {
			return err
		}
		propertyType := node.contained.property.Type()
		if propertyType == nil {
			return fmt.Errorf("unnest property must return a slice, array or iter.Seq")
		}
		if node.contained.sequence {
			if !isContainedSequenceType(propertyType, node.contained.elementType) {
				return fmt.Errorf("unnest sequence element type %s does not match child element type %s", propertyType, node.contained.elementType)
			}
		} else {
			if propertyType.Kind() != reflect.Slice && propertyType.Kind() != reflect.Array {
				return fmt.Errorf("unnest property must return a slice or array, got %s", propertyType)
			}
			if node.contained.elementType == nil || propertyType.Elem() != node.contained.elementType {
				return fmt.Errorf("unnest property element type %s does not match child element type %s", propertyType.Elem(), node.contained.elementType)
			}
		}
		if target := strings.TrimSpace(node.contained.targetSchemaName); target != "" {
			if target == "<invalid>" {
				return NewError(ErrorInvalidRule, "unnest target event type is required")
			}
			if _, ok := e.Schema(target); !ok {
				return NewError(ErrorUnknownName, fmt.Sprintf("unnest target event type %q is not registered", target))
			}
		}
		if _, err := e.sourceSchema(node); err != nil {
			return err
		}
		if err := validateContainedPropertyExpression(node.contained.property); err != nil {
			return err
		}
		return e.validateExprFields(node.input, node.contained.property)
	case streamFilter:
		if node.predicate == nil {
			return fmt.Errorf("esper: filter predicate is required")
		}
		if node.predicate.Type() != typeOf[bool]() {
			return fmt.Errorf("esper: filter predicate must return bool, got %s", node.predicate.Type())
		}
		if err := e.validateNode(node.input); err != nil {
			return err
		}
		return e.validateExprFields(node.input, node.predicate)
	case streamWindow:
		if node.window == nil {
			return fmt.Errorf("esper: window specification is required")
		}
		if err := node.window.validate(); err != nil {
			return err
		}
		if unique, ok := node.window.(UniqueWindowSpec); ok {
			if err := e.validateNode(node.input); err != nil {
				return err
			}
			for _, key := range unique.keyExpressions() {
				if err := e.validateExprFields(node.input, key); err != nil {
					return err
				}
			}
			return nil
		}
		if external, ok := node.window.(ExternallyTimedWindowSpec); ok {
			if err := e.validateNode(node.input); err != nil {
				return err
			}
			return e.validateExprFields(node.input, external.Timestamp)
		}
		if ordered, ok := node.window.(TimeOrderWindowSpec); ok {
			if err := e.validateNode(node.input); err != nil {
				return err
			}
			return e.validateExprFields(node.input, ordered.Timestamp)
		}
		if timeToLive, ok := node.window.(TimeToLiveAtWindowSpec); ok {
			if err := e.validateNode(node.input); err != nil {
				return err
			}
			return e.validateExprFields(node.input, timeToLive.Timestamp)
		}
		if sortedWindow, ok := node.window.(SortedWindowSpec); ok {
			if err := e.validateNode(node.input); err != nil {
				return err
			}
			for _, key := range sortedWindow.UniqueKeys {
				if err := e.validateExprFields(node.input, key); err != nil {
					return err
				}
			}
			for _, key := range sortedWindow.Keys {
				if err := e.validateExprFields(node.input, key.Expr); err != nil {
					return err
				}
			}
			return nil
		}
		return e.validateNode(node.input)
	default:
		return fmt.Errorf("esper: unknown stream node kind %d", node.kind)
	}
}

func (e *Environment) validateExprFields(input *streamNode, expression Expr) error {
	if expression == nil {
		return fmt.Errorf("esper: nil expression")
	}
	node := expression.node()
	if node == nil {
		return fmt.Errorf("esper: expression has no analyzable node")
	}
	if node.configurationError != "" {
		return NewError(ErrorInvalidRule, node.configurationError)
	}
	if err := e.validateExpressionReferences(node, make(map[string]bool)); err != nil {
		return err
	}
	if err := validateBitwiseExpressionNodes(node); err != nil {
		return err
	}
	if err := validateCoalesceExpressionNodes(node); err != nil {
		return err
	}
	if err := validateMethodNodes(node); err != nil {
		return err
	}
	if err := e.validateScriptNodes(node); err != nil {
		return err
	}
	if err := e.validateExprVariables(expression); err != nil {
		return err
	}
	var fields []string
	node.referencedLocalFields(&fields)
	source, err := sourceNode(input)
	if err != nil {
		return err
	}
	schema, err := e.sourceSchema(source)
	if err != nil {
		return err
	}
	for _, name := range fields {
		field, exists := schema.Field(name)
		if !exists {
			return fmt.Errorf("esper: expression references unknown field %q on schema %q", name, schema.Name())
		}
		expressionType := expressionFieldType(expression.node(), name)
		if expressionType != nil && field.Type != nil && field.Type != typeOf[any]() {
			if !field.Type.AssignableTo(expressionType) && !expressionType.AssignableTo(field.Type) && !numericTypes(field.Type, expressionType) {
				return fmt.Errorf("esper: field %q has type %s, expression expects %s", name, field.Type, expressionType)
			}
		}
	}
	var parentFields []containedParentFieldReference
	node.referencedContainedParentFields(&parentFields)
	for _, reference := range parentFields {
		if reference.level <= 0 {
			return NewError(ErrorInvalidRule, "contained ancestor level must be positive")
		}
		parentSchema, err := e.containedAncestorSchema(input, reference.level)
		if err != nil {
			return err
		}
		field, exists := parentSchema.Field(reference.name)
		if !exists {
			return fmt.Errorf("esper: contained ancestor references unknown field %q on schema %q", reference.name, parentSchema.Name())
		}
		expressionType := expressionFieldType(node, reference.name)
		if expressionType != nil && field.Type != nil && field.Type != typeOf[any]() {
			if !field.Type.AssignableTo(expressionType) && !expressionType.AssignableTo(field.Type) && !numericTypes(field.Type, expressionType) {
				return fmt.Errorf("esper: contained ancestor field %q has type %s, expression expects %s", reference.name, field.Type, expressionType)
			}
		}
	}
	return e.validateExpressionSubqueries(node)
}

func (e *Environment) validateExpressionReferences(node *exprNode, visiting map[string]bool) error {
	if node == nil {
		return nil
	}
	if node.kind == "expression-ref" {
		name := strings.TrimSpace(node.expressionName)
		if name == "" {
			return NewError(ErrorInvalidRule, "expression reference name is required")
		}
		if node.expressionEnvironment != nil && node.expressionEnvironment != e {
			return NewError(ErrorDependency, fmt.Sprintf("expression definition %q belongs to a different environment", name))
		}
		definition, ok := e.Expression(name)
		if !ok || definition.Expr == nil || definition.Expr.node() == nil {
			return NewError(ErrorUnknownName, fmt.Sprintf("expression definition %q is not registered", name))
		}
		if !expressionTypesCompatible(node.typ, definition.Expr.Type()) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("expression definition %q returns %s, reference expects %s", name, definition.Expr.Type(), node.typ))
		}
		arguments := node.expressionArguments
		if len(arguments) != len(definition.Parameters) {
			return NewError(ErrorInvalidRule, fmt.Sprintf("expression definition %q expects %d arguments, received %d", name, len(definition.Parameters), len(arguments)))
		}
		for index, parameter := range definition.Parameters {
			argument := arguments[index]
			if argument == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("expression definition %q argument %d is required", name, index))
			}
			if !expressionTypesCompatible(parameter.Type, argument.typ) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("expression definition %q argument %d (%s) expects %s, received %s", name, index, parameter.Name, parameter.Type, argument.typ))
			}
		}
		if visiting[name] {
			return NewError(ErrorInvalidRule, fmt.Sprintf("expression definition %q has a cyclic dependency", name))
		}
		visiting[name] = true
		node.expressionBody = definition.Expr.node()
		bodyPresent := false
		for _, child := range node.children {
			if child == node.expressionBody {
				bodyPresent = true
				break
			}
		}
		if !bodyPresent {
			node.children = append(node.children, node.expressionBody)
		}
		if err := e.validateExpressionReferences(definition.Expr.node(), visiting); err != nil {
			delete(visiting, name)
			return err
		}
		delete(visiting, name)
	}
	for _, child := range node.children {
		if err := e.validateExpressionReferences(child, visiting); err != nil {
			return err
		}
	}
	return nil
}

// validateContainedPropertyExpression enforces the same evaluation boundary
// as Esper's contained-event property selection: the collection expression is
// evaluated once for the current parent event and cannot depend on aggregate
// state, a subquery snapshot, or previous/prior window navigation. Ordinary
// fields, deterministic Go UDFs and string/collection combinators remain
// analyzable and are intentionally allowed.
func validateContainedPropertyExpression(expression Expr) error {
	if expression == nil || expression.node() == nil {
		return NewError(ErrorInvalidRule, "contained event expression is required")
	}
	var visit func(*exprNode) error
	visit = func(node *exprNode) error {
		if node == nil {
			return nil
		}
		if strings.HasPrefix(node.kind, "subquery-") || node.subquery != nil {
			return NewError(ErrorInvalidRule, "contained event expression does not support subqueries")
		}
		if expressionNodeContainsAggregate(node) {
			return NewError(ErrorInvalidRule, "contained event expression does not support aggregation")
		}
		if strings.HasPrefix(node.kind, "prev") || strings.HasPrefix(node.kind, "prior") {
			return NewError(ErrorInvalidRule, "contained event expression does not support previous or prior access")
		}
		for _, child := range node.children {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(expression.node())
}

func isContainedSequenceType(sequenceType, elementType reflect.Type) bool {
	if sequenceType == nil || elementType == nil || sequenceType.Kind() != reflect.Func || sequenceType.NumIn() != 1 || sequenceType.NumOut() != 0 {
		return false
	}
	yieldType := sequenceType.In(0)
	if yieldType.Kind() != reflect.Func || yieldType.NumIn() != 1 || yieldType.NumOut() != 1 || yieldType.Out(0).Kind() != reflect.Bool {
		return false
	}
	itemType := yieldType.In(0)
	return itemType == elementType || itemType.AssignableTo(elementType) || elementType.AssignableTo(itemType)
}

func (e *Environment) containedAncestorSchema(input *streamNode, level int) (Schema, error) {
	node, err := sourceNode(input)
	if err != nil {
		return Schema{}, err
	}
	for index := 0; index < level; index++ {
		if node == nil || node.kind != streamContained || node.input == nil {
			return Schema{}, NewError(ErrorDependency, fmt.Sprintf("contained ancestor level %d requires a nested contained stream", level))
		}
		parentNode, sourceErr := sourceNode(node.input)
		if sourceErr != nil {
			return Schema{}, sourceErr
		}
		node = parentNode
	}
	return e.sourceSchema(node)
}

func validateMethodNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if (node.kind == "method" || node.kind == "duck-method") && strings.TrimSpace(node.methodName) == "" {
		return NewError(ErrorInvalidRule, "method name is required")
	}
	for _, child := range node.children {
		if err := validateMethodNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func validateBitwiseExpressionNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.kind == "bitwise-and" || node.kind == "bitwise-or" || node.kind == "bitwise-xor" {
		if len(node.children) != 2 || node.children[0] == nil || node.children[1] == nil {
			return NewError(ErrorInvalidRule, "bitwise operator requires two operands")
		}
		for index, child := range node.children {
			if !bitwiseTypesCompatible(node.typ, child.typ) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("bitwise operator %q operand %d has type %s, expected %s", node.kind, index, bitwiseTypeDescription(child.typ), bitwiseTypeDescription(node.typ)))
			}
		}
	}
	for _, child := range node.children {
		if err := validateBitwiseExpressionNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func bitwiseTypesCompatible(expected, actual reflect.Type) bool {
	if expected == nil || actual == nil || expected == typeOf[any]() || actual == typeOf[any]() {
		return true
	}
	for actual.Kind() == reflect.Pointer {
		actual = actual.Elem()
	}
	if actual == nil {
		return false
	}
	if actual.Kind() == reflect.Interface {
		// A non-empty interface does not reveal its concrete value at Build
		// time. Leave the final compatible-value check to evaluation.
		return true
	}
	return actual == expected || actual.AssignableTo(expected) || expected.AssignableTo(actual)
}

func bitwiseTypeDescription(typ reflect.Type) string {
	if typ == nil {
		return "any"
	}
	return typ.String()
}

func validateCoalesceExpressionNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.kind == "coalesce" {
		if len(node.children) < 2 {
			return NewError(ErrorInvalidRule, "coalesce requires at least two operands")
		}
		for index, child := range node.children {
			if child == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("coalesce operand %d is required", index))
			}
			if !coalesceTypesCompatible(node.typ, child.typ) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("coalesce operand %d has type %s, expected %s", index, bitwiseTypeDescription(child.typ), bitwiseTypeDescription(node.typ)))
			}
		}
	}
	for _, child := range node.children {
		if err := validateCoalesceExpressionNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func coalesceTypesCompatible(expected, actual reflect.Type) bool {
	if expected == nil || actual == nil || expected == typeOf[any]() || actual == typeOf[any]() {
		return true
	}
	if actual == expected || actual.AssignableTo(expected) || expected.AssignableTo(actual) {
		return true
	}
	if actual.Kind() == reflect.Interface {
		return true
	}
	for actual.Kind() == reflect.Pointer {
		actual = actual.Elem()
	}
	if actual == nil {
		return false
	}
	if actual.Kind() == reflect.Interface {
		return true
	}
	if isNumericType(expected) && isNumericType(actual) {
		return coalesceNumericRank(actual) <= coalesceNumericRank(expected)
	}
	return actual == expected || actual.AssignableTo(expected) || expected.AssignableTo(actual)
}

func coalesceNumericRank(typ reflect.Type) int {
	if typ == nil {
		return -1
	}
	switch typ.Kind() {
	case reflect.Int8, reflect.Uint8:
		return 1
	case reflect.Int16, reflect.Uint16:
		return 2
	case reflect.Int, reflect.Int32, reflect.Uint, reflect.Uint32:
		return 3
	case reflect.Int64, reflect.Uint64:
		return 4
	case reflect.Float32:
		return 5
	case reflect.Float64:
		return 6
	default:
		return -1
	}
}

func (e *Environment) validateScriptNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.kind == "script" {
		if node.scriptEnvironment == nil || node.scriptEnvironment != e {
			return NewError(ErrorDependency, fmt.Sprintf("script %q belongs to a different or nil environment", node.scriptName))
		}
		e.mu.RLock()
		definition, ok := e.scripts[node.scriptName]
		e.mu.RUnlock()
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("script %q is not registered", node.scriptName))
		}
		if definition.provider == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("script %q has no provider", node.scriptName))
		}
		if definition.resultType != nil && node.typ != nil && definition.resultType != node.typ &&
			!definition.resultType.AssignableTo(node.typ) && !node.typ.AssignableTo(definition.resultType) && !numericTypes(definition.resultType, node.typ) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("script %q returns %s, expression expects %s", node.scriptName, definition.resultType, node.typ))
		}
		if definition.argumentTypesSet {
			if len(definition.argumentTypes) != len(node.children) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("script %q expects %d arguments, received %d", node.scriptName, len(definition.argumentTypes), len(node.children)))
			}
			for index, expected := range definition.argumentTypes {
				child := node.children[index]
				if child == nil {
					return NewError(ErrorInvalidRule, fmt.Sprintf("script %q argument %d is required", node.scriptName, index))
				}
				actual := child.typ
				if expected != nil && actual != nil && expected != typeOf[any]() && actual != typeOf[any]() &&
					!expected.AssignableTo(actual) && !actual.AssignableTo(expected) && !numericTypes(expected, actual) {
					return NewError(ErrorTypeMismatch, fmt.Sprintf("script %q argument %d expects %s, received %s", node.scriptName, index, expected, actual))
				}
			}
		}
	}
	for _, child := range node.children {
		if err := e.validateScriptNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func (e *Environment) validateExpressionSubqueries(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.subquery != nil {
		if err := e.validateSubquery(node.subquery); err != nil {
			return err
		}
		if err := validateSubqueryComparison(node); err != nil {
			return err
		}
	}
	for _, child := range node.children {
		if err := e.validateExpressionSubqueries(child); err != nil {
			return err
		}
	}
	return nil
}

func (e *Environment) validateSubquery(definition *subqueryDefinition) error {
	if definition == nil || definition.source == nil {
		return NewError(ErrorInvalidRule, "subquery source is required")
	}
	if definition.quantified && definition.comparison > SubqueryLessOrEqual {
		return NewError(ErrorInvalidRule, "subquery comparison operator is invalid")
	}
	if definition.cardinality > SubqueryRequireSingle {
		return NewError(ErrorInvalidRule, "subquery cardinality mode is invalid")
	}
	if definition.offset < 0 {
		return NewError(ErrorInvalidRule, "subquery offset cannot be negative")
	}
	if definition.limitSet && definition.limit < 0 {
		return NewError(ErrorInvalidRule, "subquery limit cannot be negative")
	}
	base, err := subqueryRootSource(definition.source)
	if err != nil {
		return err
	}
	if base.kind != streamSource && base.kind != streamNamedWindow && base.kind != streamTable && base.kind != streamHistorical && base.kind != streamMethod {
		return NewError(ErrorInvalidRule, "subquery source must be an event stream, named window, table, historical source, or method source")
	}
	if base.kind == streamSource && !subquerySourceContainsWindow(definition.source) && !definition.aggregateProjection && !definition.grouped {
		return NewError(ErrorInvalidRule, "non-aggregated event-stream subquery requires a window")
	}
	if base.kind == streamNamedWindow && subquerySourceContainsWindow(definition.source) {
		return NewError(ErrorInvalidRule, "named-window subquery cannot declare a data window")
	}
	if err := e.validateNode(definition.source); err != nil {
		return err
	}
	if definition.predicate != nil {
		if definition.predicate.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "subquery predicate must return bool")
		}
		if err := e.validateExprFields(definition.source, definition.predicate); err != nil {
			return WrapError(ErrorInvalidRule, "subquery predicate", err)
		}
	}
	if definition.projection != nil {
		if err := e.validateExprFields(definition.source, definition.projection); err != nil {
			return WrapError(ErrorInvalidRule, "subquery projection", err)
		}
	}
	if definition.multiColumn && len(definition.columns) == 0 {
		return NewError(ErrorInvalidRule, "multi-column subquery requires at least one column")
	}
	if len(definition.columns) > 0 {
		seen := make(map[string]struct{}, len(definition.columns))
		hasAggregate := false
		allAggregate := true
		for index, selection := range definition.columns {
			if strings.TrimSpace(selection.Name) == "" {
				return NewError(ErrorInvalidRule, fmt.Sprintf("subquery column %d requires a name", index))
			}
			if selection.Expr == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("subquery column %q has a nil expression", selection.Name))
			}
			if _, exists := seen[selection.Name]; exists {
				return NewError(ErrorInvalidRule, fmt.Sprintf("subquery column duplicates alias %q", selection.Name))
			}
			seen[selection.Name] = struct{}{}
			if err := e.validateExprFields(definition.source, selection.Expr); err != nil {
				return WrapError(ErrorInvalidRule, fmt.Sprintf("subquery column %q", selection.Name), err)
			}
			aggregate := isAggregateExpression(selection.Expr)
			hasAggregate = hasAggregate || aggregate
			allAggregate = allAggregate && aggregate
		}
		if !definition.grouped && hasAggregate && !allAggregate {
			return NewError(ErrorInvalidRule, "multi-column subquery requires all columns to be aggregated unless a group-by clause is specified")
		}
	}
	if definition.grouped {
		if definition.groupBy == nil {
			return NewError(ErrorInvalidRule, "grouped subquery key is required")
		}
		if definition.projection == nil && len(definition.columns) == 0 {
			return NewError(ErrorInvalidRule, "grouped subquery projection is required")
		}
		if err := e.validateExprFields(definition.source, definition.groupBy); err != nil {
			return WrapError(ErrorInvalidRule, "subquery group-by key", err)
		}
		if isAggregateExpression(definition.groupBy) {
			return NewError(ErrorInvalidRule, "subquery group-by key cannot be an aggregate")
		}
		if expressionContainsKind(definition.groupBy.node(), "outer-field") {
			return NewError(ErrorInvalidRule, "subquery group-by key cannot reference the outer event")
		}
		if definition.groupedRowProjection && len(definition.columns) > 0 {
			groupKeyDescription := definition.groupBy.Description()
			for _, selection := range definition.columns {
				if !isAggregateExpression(selection.Expr) && selection.Expr.Description() != groupKeyDescription {
					return NewError(ErrorInvalidRule, fmt.Sprintf("subquery column %q must be an aggregate or match the group-by key", selection.Name))
				}
			}
		}
		if definition.having != nil {
			if definition.having.Type() != typeOf[bool]() {
				return NewError(ErrorTypeMismatch, "subquery having predicate must return bool")
			}
			if err := e.validateExprFields(definition.source, definition.having); err != nil {
				return WrapError(ErrorInvalidRule, "subquery having", err)
			}
		}
	}
	for index, order := range definition.orderBy {
		if order.Expression == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("subquery order key %d is nil", index))
		}
		if err := e.validateExprFields(definition.source, order.Expression); err != nil {
			return WrapError(ErrorInvalidRule, fmt.Sprintf("subquery order key %d", index), err)
		}
	}
	return nil
}

func validateSubqueryComparison(node *exprNode) error {
	if node == nil || node.subquery == nil || node.subquery.projection == nil || len(node.children) == 0 {
		return nil
	}
	switch node.kind {
	case "subquery-in", "subquery-any", "subquery-all":
	default:
		return nil
	}
	left := node.children[0].typ
	right := node.subquery.projection.Type()
	if left == nil || right == nil || left == typeOf[any]() || right == typeOf[any]() {
		return nil
	}
	if left.AssignableTo(right) || right.AssignableTo(left) || numericTypes(left, right) {
		return nil
	}
	return NewError(ErrorTypeMismatch, fmt.Sprintf("subquery comparison types %s and %s are incompatible", left, right))
}

// validateTriggerTargetExpression validates an expression that may read both
// the incoming trigger event and one candidate state row. Ordinary Field
// nodes resolve against input; targetKind nodes resolve against targetSchema.
func (e *Environment) validateTriggerTargetExpression(input *streamNode, targetSchema Schema, expression Expr, targetKind string) error {
	if expression == nil {
		return NewError(ErrorInvalidRule, "trigger expression is nil")
	}
	if err := e.validateExprVariables(expression); err != nil {
		return err
	}
	if err := e.validateExprFields(input, expression); err != nil {
		return err
	}
	var fields []string
	expression.node().referencedTargetFields(targetKind, &fields)
	for _, name := range fields {
		field, exists := targetSchema.Field(name)
		if !exists {
			return fmt.Errorf("expression references unknown target field %q on schema %q", name, targetSchema.Name())
		}
		expressionType := expressionTargetFieldType(expression.node(), targetKind, name)
		if expressionType != nil && field.Type != nil && field.Type != typeOf[any]() {
			if !field.Type.AssignableTo(expressionType) && !expressionType.AssignableTo(field.Type) && !numericTypes(field.Type, expressionType) {
				return fmt.Errorf("target field %q has type %s, expression expects %s", name, field.Type, expressionType)
			}
		}
	}
	return nil
}

func (e *Environment) sourceSchema(source *streamNode) (Schema, error) {
	if source == nil {
		return Schema{}, NewError(ErrorDependency, "nil source")
	}
	if source.kind == streamContained {
		if source.contained != nil && strings.TrimSpace(source.contained.targetSchemaName) != "" {
			target := strings.TrimSpace(source.contained.targetSchemaName)
			if target == "<invalid>" {
				return Schema{}, NewError(ErrorInvalidRule, "unnest target event type is required")
			}
			if schema, ok := e.Schema(target); ok {
				return schema, nil
			}
			return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("unnest target event type %q is not registered", target))
		}
		if source.contained == nil || source.contained.childType == nil {
			return Schema{}, NewError(ErrorDependency, fmt.Sprintf("unnest source %q has no child type", source.sourceName))
		}
		if schema, ok := e.schemaForGoType(source.contained.childType); ok {
			return schema, nil
		}
		return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("unnest child type %s has no registered schema", source.contained.childType))
	}
	if source.kind == streamNamedWindow {
		definition, ok := e.NamedWindow(source.sourceName)
		if !ok {
			return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", source.sourceName))
		}
		return definition.schema, nil
	}
	if source.kind == streamTable {
		definition, ok := e.Table(source.sourceName)
		if !ok {
			return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("table %q is not registered", source.sourceName))
		}
		return definition.schema, nil
	}
	if source.kind == streamHistorical {
		if source.historical == nil || !source.historical.schema.valid() {
			return Schema{}, NewError(ErrorDependency, fmt.Sprintf("historical source %q has no schema", source.sourceName))
		}
		return source.historical.schema, nil
	}
	if source.kind == streamMethod {
		if source.method == nil || !source.method.schema.valid() {
			return Schema{}, NewError(ErrorDependency, fmt.Sprintf("method source %q has no schema", source.sourceName))
		}
		return source.method.schema, nil
	}
	if source.kind == streamPattern {
		return patternJoinSchema(source.pattern)
	}
	if source.kind == streamDerived {
		if source.derived == nil || !source.derived.schema.valid() {
			return Schema{}, NewError(ErrorDependency, fmt.Sprintf("derived source %q has no result schema", source.sourceName))
		}
		return source.derived.schema, nil
	}
	schema, ok := e.Schema(source.sourceName)
	if !ok {
		return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("source %q has no registered schema", source.sourceName))
	}
	return schema, nil
}

func (e *Environment) schemaForGoType(typ reflect.Type) (Schema, bool) {
	if e == nil || typ == nil {
		return Schema{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, schema := range e.schemas {
		if schema.GoType() == typ {
			return schema, true
		}
	}
	for _, schema := range e.schemas {
		goType := schema.GoType()
		if goType != nil && (typ.AssignableTo(goType) || goType.AssignableTo(typ)) {
			return schema, true
		}
	}
	return Schema{}, false
}

func (e *Environment) validateExprVariables(expression Expr) error {
	if expression == nil || expression.node() == nil {
		return fmt.Errorf("esper: nil expression")
	}
	if err := validateExpressionConfiguration(expression.node()); err != nil {
		return err
	}
	if err := validateEnumExpressionNodes(expression.node()); err != nil {
		return err
	}
	var variables []string
	expression.node().referencedVariables(&variables)
	for _, name := range variables {
		definition, exists := e.Variable(name)
		if !exists {
			return NewError(ErrorUnknownName, fmt.Sprintf("expression references unknown variable %q", name))
		}
		expressionType := expressionVariableType(expression.node(), name)
		if expressionType == nil || definition.typ == nil || definition.typ == typeOf[any]() || expressionType == typeOf[any]() {
			continue
		}
		if !definition.typ.AssignableTo(expressionType) && !expressionType.AssignableTo(definition.typ) && !numericTypes(definition.typ, expressionType) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("variable %q has type %s, expression expects %s", name, definition.typ, expressionType))
		}
	}
	return nil
}

// validateExpressionConfiguration walks all expression nodes for constructor
// diagnostics. Specialized validators historically checked only selected node
// families; keeping this generic pass here ensures an invalid nested fluent
// operator cannot be hidden inside an otherwise valid parent expression.
func validateExpressionConfiguration(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.configurationError != "" {
		return NewError(ErrorInvalidRule, node.configurationError)
	}
	for _, child := range node.children {
		if err := validateExpressionConfiguration(child); err != nil {
			return err
		}
	}
	return nil
}

func requiredQueryParameters(environment *Environment, query Query) []string {
	parameterTypes, err := queryParameterTypes(environment, query)
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(parameterTypes))
	for name := range parameterTypes {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func queryParameterTypes(environment *Environment, query Query) (map[string]reflect.Type, error) {
	parameterTypes := make(map[string]reflect.Type)
	visit := func(expression Expr) error {
		return collectExpressionParameterTypes(expression, parameterTypes)
	}
	if err := visitQueryExpressions(environment, query, visit); err != nil {
		return nil, err
	}
	if strings.TrimSpace(query.contextName) != "" && environment != nil {
		if definition, ok := environment.Context(query.contextName); ok {
			definitionCopy := definition
			if err := visitContextDefinitionExpressions(&definitionCopy, visit, make(map[*ContextDefinition]struct{})); err != nil {
				return nil, err
			}
		}
	}
	return parameterTypes, nil
}

func visitContextDefinitionExpressions(definition *ContextDefinition, visit func(Expr) error, visited map[*ContextDefinition]struct{}) error {
	if definition == nil {
		return nil
	}
	if _, exists := visited[definition]; exists {
		return nil
	}
	visited[definition] = struct{}{}
	for _, key := range definition.keys {
		if err := visit(key); err != nil {
			return err
		}
	}
	if len(definition.keys) == 0 {
		if err := visit(definition.key); err != nil {
			return err
		}
	}
	for _, category := range definition.categories {
		if err := visit(category.predicate); err != nil {
			return err
		}
	}
	if err := visit(definition.start); err != nil {
		return err
	}
	if err := visit(definition.end); err != nil {
		return err
	}
	if err := visitCronScheduleExpressions(definition.cronStart, visit); err != nil {
		return err
	}
	if err := visitCronScheduleExpressions(definition.cronEnd, visit); err != nil {
		return err
	}
	for _, pattern := range []*patternDefinition{definition.startPattern, definition.endPattern} {
		if err := visitPatternDefinitionExpressions(pattern, visit); err != nil {
			return err
		}
	}
	return visitContextDefinitionExpressions(definition.parent, visit, visited)
}

func visitPatternDefinitionExpressions(definition *patternDefinition, visit func(Expr) error) error {
	if definition == nil {
		return nil
	}
	if err := visitStreamNodeExpressions(definition.input, visit); err != nil {
		return err
	}
	for _, step := range definition.steps {
		if err := visit(step.predicate); err != nil {
			return err
		}
	}
	if err := visit(definition.everyDistinct); err != nil {
		return err
	}
	if err := visit(definition.guard); err != nil {
		return err
	}
	return visitPatternNodeExpressions(definition.root, visit)
}

func visitQueryExpressions(environment *Environment, query Query, visit func(Expr) error) error {
	if err := visitCronScheduleExpressions(query.output.Cron, visit); err != nil {
		return err
	}
	if err := visit(query.output.When); err != nil {
		return err
	}
	for _, assignment := range query.output.Then {
		if err := visit(assignment.Expr); err != nil {
			return err
		}
	}
	if err := visitStreamNodeExpressions(query.input, visit); err != nil {
		return err
	}
	if err := visitSelectionsExpressions(query.selections, visit); err != nil {
		return err
	}
	if err := visitSortExpressions(query.orderBy, visit); err != nil {
		return err
	}
	if query.aggregate != nil {
		if err := visitStreamNodeExpressions(query.aggregate.input, visit); err != nil {
			return err
		}
		for _, expression := range query.aggregate.groupBy {
			if err := visit(expression); err != nil {
				return err
			}
		}
		if err := visitSelectionsExpressions(query.aggregate.selections, visit); err != nil {
			return err
		}
		if err := visit(query.aggregate.where); err != nil {
			return err
		}
		if err := visit(query.aggregate.having); err != nil {
			return err
		}
	}
	if query.join != nil {
		for _, source := range joinDefinitionSources(query.join) {
			if err := visitStreamNodeExpressions(source, visit); err != nil {
				return err
			}
		}
		for _, condition := range joinDefinitionConditions(query.join) {
			if err := visitJoinConditionExpressions(condition, visit); err != nil {
				return err
			}
		}
		for _, selection := range query.joinSelections {
			if err := visit(selection.Expr); err != nil {
				return err
			}
		}
		if err := visit(query.joinWhere); err != nil {
			return err
		}
	}
	if query.pattern != nil {
		if err := visitStreamNodeExpressions(query.pattern.input, visit); err != nil {
			return err
		}
		for _, step := range query.pattern.steps {
			if err := visit(step.predicate); err != nil {
				return err
			}
		}
		if err := visit(query.pattern.everyDistinct); err != nil {
			return err
		}
		if err := visit(query.pattern.guard); err != nil {
			return err
		}
		if err := visitPatternNodeExpressions(query.pattern.root, visit); err != nil {
			return err
		}
		if err := visitSelectionsExpressions(query.patternSelections, visit); err != nil {
			return err
		}
	}
	if query.rowRecog != nil {
		if err := visitStreamNodeExpressions(query.rowRecog.input, visit); err != nil {
			return err
		}
		names := make([]string, 0, len(query.rowRecog.defines))
		for name := range query.rowRecog.defines {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := visit(query.rowRecog.defines[name]); err != nil {
				return err
			}
		}
		for _, expression := range query.rowRecog.partition {
			if err := visit(expression); err != nil {
				return err
			}
		}
		if err := visitSelectionsExpressions(query.patternSelections, visit); err != nil {
			return err
		}
	}
	if query.trigger != nil {
		if err := visitStreamNodeExpressions(query.trigger.input, visit); err != nil {
			return err
		}
		if err := visit(query.trigger.where); err != nil {
			return err
		}
		for _, expression := range query.trigger.keys {
			if err := visit(expression); err != nil {
				return err
			}
		}
		for _, assignment := range query.trigger.assignments {
			if err := visit(assignment.Expr); err != nil {
				return err
			}
		}
		for _, assignment := range query.trigger.variableAssignments {
			if err := visit(assignment.Expr); err != nil {
				return err
			}
		}
		for _, clause := range query.trigger.merge {
			if err := visit(clause.Condition); err != nil {
				return err
			}
			for _, assignment := range clause.Assignments {
				if err := visit(assignment.Expr); err != nil {
					return err
				}
			}
		}
		if err := visitSelectionsExpressions(query.trigger.selections, visit); err != nil {
			return err
		}
	}
	if environment != nil && query.contextName != "" {
		if definition, ok := environment.Context(query.contextName); ok {
			for current := &definition; current != nil; current = current.parent {
				for _, key := range current.contextKeys() {
					if err := visit(key); err != nil {
						return err
					}
				}
				if err := visit(current.start); err != nil {
					return err
				}
				if err := visit(current.end); err != nil {
					return err
				}
				for _, category := range current.categories {
					if err := visit(category.predicate); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func visitSelectionsExpressions(selections []Selection, visit func(Expr) error) error {
	for _, selection := range selections {
		if err := visit(selection.Expr); err != nil {
			return err
		}
	}
	return nil
}

func visitSortExpressions(keys []SortKey, visit func(Expr) error) error {
	for _, key := range keys {
		if err := visit(key.Expr); err != nil {
			return err
		}
	}
	return nil
}

func visitJoinConditionExpressions(condition JoinCondition, visit func(Expr) error) error {
	for _, child := range condition.all {
		if err := visitJoinConditionExpressions(child, visit); err != nil {
			return err
		}
	}
	for _, child := range condition.any {
		if err := visitJoinConditionExpressions(child, visit); err != nil {
			return err
		}
	}
	if err := visit(condition.Left); err != nil {
		return err
	}
	return visit(condition.Right)
}

func visitPatternNodeExpressions(node *patternNode, visit func(Expr) error) error {
	if node == nil {
		return nil
	}
	if err := visit(node.predicate); err != nil {
		return err
	}
	if err := visit(node.sequenceMaxExpr); err != nil {
		return err
	}
	if err := visit(node.everyExpr); err != nil {
		return err
	}
	if err := visit(node.minimumExpr); err != nil {
		return err
	}
	if err := visit(node.maximumExpr); err != nil {
		return err
	}
	if err := visit(node.durationExpr); err != nil {
		return err
	}
	if err := visitCronScheduleExpressions(node.cron, visit); err != nil {
		return err
	}
	if err := visitPatternNodeExpressions(node.left, visit); err != nil {
		return err
	}
	if err := visitPatternNodeExpressions(node.right, visit); err != nil {
		return err
	}
	return visitPatternNodeExpressions(node.child, visit)
}

func visitStreamNodeExpressions(node *streamNode, visit func(Expr) error) error {
	if node == nil {
		return nil
	}
	if err := visit(node.predicate); err != nil {
		return err
	}
	if node.contained != nil {
		if err := visit(node.contained.property); err != nil {
			return err
		}
	}
	if err := visitWindowExpressions(node.window, visit); err != nil {
		return err
	}
	if node.derived != nil && node.derived.aggregate != nil {
		for _, key := range node.derived.aggregate.groupBy {
			if err := visit(key); err != nil {
				return err
			}
		}
		if err := visit(node.derived.aggregate.where); err != nil {
			return err
		}
		if err := visit(node.derived.aggregate.having); err != nil {
			return err
		}
		for _, selection := range node.derived.aggregate.selections {
			if err := visit(selection.Expr); err != nil {
				return err
			}
		}
	}
	return visitStreamNodeExpressions(node.input, visit)
}

func visitWindowExpressions(window WindowSpec, visit func(Expr) error) error {
	switch value := window.(type) {
	case ExpressionWindowSpec:
		return visit(value.Keep)
	case ExpressionBatchWindowSpec:
		return visit(value.Trigger)
	case GroupWindowSpec:
		if err := visit(value.Key); err != nil {
			return err
		}
		return visitWindowExpressions(value.Inner, visit)
	case CompositeWindowSpec:
		for _, child := range value.Windows {
			if err := visitWindowExpressions(child, visit); err != nil {
				return err
			}
		}
	case ExternallyTimedWindowSpec:
		return visit(value.Timestamp)
	case TimeOrderWindowSpec:
		return visit(value.Timestamp)
	case TimeToLiveAtWindowSpec:
		return visit(value.Timestamp)
	case SortedWindowSpec:
		for _, key := range value.UniqueKeys {
			if err := visit(key); err != nil {
				return err
			}
		}
		return visitSortExpressions(value.Keys, visit)
	case UniqueWindowSpec:
		for _, key := range value.keyExpressions() {
			if err := visit(key); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectExpressionParameterTypes(expression Expr, parameterTypes map[string]reflect.Type) error {
	if expression == nil || expression.node() == nil {
		return nil
	}
	var visit func(*exprNode) error
	visit = func(node *exprNode) error {
		if node == nil {
			return nil
		}
		if node.kind == "parameter" {
			name := strings.TrimSpace(node.parameterName)
			if name == "" {
				return fmt.Errorf("substitution parameter name cannot be blank")
			}
			if previous, exists := parameterTypes[name]; exists {
				if !parameterTypesCompatible(previous, node.typ) {
					return fmt.Errorf("parameter %q has incompatible type assignment between %s and %s", name, parameterTypeDescription(previous), parameterTypeDescription(node.typ))
				}
				if previous == typeOf[any]() && node.typ != typeOf[any]() {
					parameterTypes[name] = node.typ
				}
			} else {
				parameterTypes[name] = node.typ
			}
		}
		for _, child := range node.children {
			if err := visit(child); err != nil {
				return err
			}
		}
		if node.subquery != nil {
			if err := collectExpressionParameterTypes(node.subquery.predicate, parameterTypes); err != nil {
				return err
			}
			if err := collectExpressionParameterTypes(node.subquery.projection, parameterTypes); err != nil {
				return err
			}
			for _, selection := range node.subquery.columns {
				if err := collectExpressionParameterTypes(selection.Expr, parameterTypes); err != nil {
					return err
				}
			}
			if err := collectExpressionParameterTypes(node.subquery.groupBy, parameterTypes); err != nil {
				return err
			}
			if err := collectExpressionParameterTypes(node.subquery.having, parameterTypes); err != nil {
				return err
			}
			for _, order := range node.subquery.orderBy {
				if err := collectExpressionParameterTypes(order.Expression, parameterTypes); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(expression.node())
}

func parameterTypesCompatible(left, right reflect.Type) bool {
	if left == nil || right == nil || left == typeOf[any]() || right == typeOf[any]() {
		return true
	}
	return left == right
}

func parameterTypeDescription(typ reflect.Type) string {
	if typ == nil || typ == typeOf[any]() {
		return "any"
	}
	return typ.String()
}

func expressionFieldType(node *exprNode, fieldName string) reflect.Type {
	if node == nil {
		return nil
	}
	if (node.kind == "field" || node.kind == "tag-field" || node.kind == "tag-field-at" || node.kind == "contained-parent-field" || node.kind == "contained-ancestor-field") && node.fieldName == fieldName {
		return node.typ
	}
	for _, child := range node.children {
		if typ := expressionFieldType(child, fieldName); typ != nil {
			return typ
		}
	}
	return nil
}

func expressionTargetFieldType(node *exprNode, kind, fieldName string) reflect.Type {
	if node == nil {
		return nil
	}
	if node.kind == kind && node.fieldName == fieldName {
		return node.typ
	}
	for _, child := range node.children {
		if typ := expressionTargetFieldType(child, kind, fieldName); typ != nil {
			return typ
		}
	}
	return nil
}

func expressionVariableType(node *exprNode, variableName string) reflect.Type {
	if node == nil {
		return nil
	}
	if node.kind == "variable" && node.variableName == variableName {
		return node.typ
	}
	for _, child := range node.children {
		if typ := expressionVariableType(child, variableName); typ != nil {
			return typ
		}
	}
	return nil
}

func numericTypes(left, right reflect.Type) bool {
	return isNumericType(left) && isNumericType(right)
}

func isNumericType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func isIntegralType(typ reflect.Type) bool {
	if typ == nil {
		return false
	}
	switch typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	default:
		return false
	}
}

func sourceNode(node *streamNode) (*streamNode, error) {
	for node != nil {
		if node.kind == streamSource || node.kind == streamNamedWindow || node.kind == streamTable || node.kind == streamHistorical || node.kind == streamMethod || node.kind == streamPattern || node.kind == streamDerived || node.kind == streamContained {
			return node, nil
		}
		node = node.input
	}
	return nil, fmt.Errorf("esper: stream has no source")
}

func (e *Environment) resultSchema(query Query) (Schema, error) {
	if query.trigger != nil && query.trigger.target == triggerTargetNamedWindow && query.trigger.action == triggerSelectTable {
		window, ok := e.NamedWindow(query.trigger.table)
		if !ok {
			return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("trigger references unknown named window %q", query.trigger.table))
		}
		if len(query.selections) == 0 {
			return window.schema, nil
		}
		fields := make([]FieldSpec, 0, len(query.selections))
		seen := make(map[string]struct{}, len(query.selections))
		for _, selection := range query.selections {
			if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
				return Schema{}, NewError(ErrorInvalidRule, "named-window select requires a name and expression")
			}
			if _, exists := seen[selection.Name]; exists {
				return Schema{}, NewError(ErrorInvalidRule, fmt.Sprintf("named-window select duplicates alias %q", selection.Name))
			}
			seen[selection.Name] = struct{}{}
			fields = append(fields, FieldSpec{Name: selection.Name, Type: selection.Expr.Type()})
		}
		return NewSchema("result:"+query.name, fields...)
	}
	if query.trigger != nil && query.trigger.target == triggerTargetNamedWindow && query.trigger.action != triggerSetVariables {
		window, ok := e.NamedWindow(query.trigger.table)
		if !ok {
			return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("trigger references unknown named window %q", query.trigger.table))
		}
		return window.schema, nil
	}
	if query.trigger != nil && query.trigger.action == triggerSelectTable {
		if len(query.selections) == 0 {
			return Schema{}, NewError(ErrorInvalidRule, "table select requires at least one projection")
		}
		fields := make([]FieldSpec, 0, len(query.selections))
		seen := make(map[string]struct{}, len(query.selections))
		for _, selection := range query.selections {
			if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
				return Schema{}, NewError(ErrorInvalidRule, "table select projection requires a name and expression")
			}
			if _, exists := seen[selection.Name]; exists {
				return Schema{}, NewError(ErrorInvalidRule, fmt.Sprintf("table select duplicates alias %q", selection.Name))
			}
			seen[selection.Name] = struct{}{}
			fields = append(fields, FieldSpec{Name: selection.Name, Type: selection.Expr.Type()})
		}
		return NewSchema("result:"+query.name, fields...)
	}
	if query.trigger != nil && query.trigger.action != triggerSetVariables {
		table, ok := e.Table(query.trigger.table)
		if !ok {
			return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("trigger references unknown table %q", query.trigger.table))
		}
		return table.schema, nil
	}
	if query.sourceLess {
		fields := make([]FieldSpec, 0, len(query.selections))
		for _, selection := range query.selections {
			if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
				return Schema{}, NewError(ErrorInvalidRule, "source-less projection requires a name and expression")
			}
			fields = append(fields, FieldSpec{Name: selection.Name, Type: selection.Expr.Type()})
		}
		return NewSchema("result:"+query.name, fields...)
	}
	if query.rowRecog != nil {
		if len(query.patternSelections) == 0 {
			return Schema{}, NewError(ErrorInvalidRule, "match-recognize requires at least one measure")
		}
		fields := make([]FieldSpec, 0, len(query.patternSelections))
		seen := make(map[string]struct{}, len(query.patternSelections))
		for _, selection := range query.patternSelections {
			if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
				return Schema{}, NewError(ErrorInvalidRule, "match-recognize measure requires a name and expression")
			}
			if _, exists := seen[selection.Name]; exists {
				return Schema{}, NewError(ErrorInvalidRule, fmt.Sprintf("match-recognize measure duplicates alias %q", selection.Name))
			}
			seen[selection.Name] = struct{}{}
			fields = append(fields, FieldSpec{Name: selection.Name, Type: selection.Expr.Type()})
		}
		return NewSchema("result:"+query.name, fields...)
	}
	if query.pattern != nil {
		if len(query.patternSelections) == 0 {
			return Schema{}, NewError(ErrorInvalidRule, "pattern requires at least one projection")
		}
		fields := make([]FieldSpec, 0, len(query.patternSelections))
		seen := make(map[string]struct{}, len(query.patternSelections))
		for _, selection := range query.patternSelections {
			if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
				return Schema{}, NewError(ErrorInvalidRule, "pattern projection requires a name and expression")
			}
			if _, exists := seen[selection.Name]; exists {
				return Schema{}, NewError(ErrorInvalidRule, fmt.Sprintf("pattern projection duplicates alias %q", selection.Name))
			}
			seen[selection.Name] = struct{}{}
			fields = append(fields, FieldSpec{Name: selection.Name, Type: selection.Expr.Type()})
		}
		return NewSchema("result:"+query.name, fields...)
	}
	if query.aggregate != nil {
		if len(query.aggregate.selections) == 0 {
			return Schema{}, NewError(ErrorInvalidRule, "aggregate requires at least one projection")
		}
		fields := make([]FieldSpec, 0, len(query.aggregate.selections))
		seen := make(map[string]struct{}, len(query.aggregate.selections))
		for _, selection := range query.aggregate.selections {
			if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
				return Schema{}, NewError(ErrorInvalidRule, "aggregate projection requires a name and expression")
			}
			if _, exists := seen[selection.Name]; exists {
				return Schema{}, NewError(ErrorInvalidRule, fmt.Sprintf("aggregate projection duplicates alias %q", selection.Name))
			}
			seen[selection.Name] = struct{}{}
			fields = append(fields, FieldSpec{Name: selection.Name, Type: selection.Expr.Type()})
		}
		return NewSchema("result:"+query.name, fields...)
	}
	if query.join != nil {
		if len(query.joinSelections) == 0 {
			return Schema{}, fmt.Errorf("esper: join requires at least one projection")
		}
		fields := make([]FieldSpec, 0, len(query.joinSelections))
		seen := make(map[string]struct{}, len(query.joinSelections))
		for _, selection := range query.joinSelections {
			if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
				return Schema{}, fmt.Errorf("esper: join projection requires a name and expression")
			}
			if _, exists := seen[selection.Name]; exists {
				return Schema{}, fmt.Errorf("esper: join projection duplicates alias %q", selection.Name)
			}
			seen[selection.Name] = struct{}{}
			fields = append(fields, FieldSpec{Name: selection.Name, Type: selection.Expr.Type()})
		}
		return NewSchema("result:"+query.name, fields...)
	}
	if len(query.selections) == 0 {
		return Schema{}, nil
	}
	fields := make([]FieldSpec, 0, len(query.selections))
	seen := make(map[string]struct{}, len(query.selections))
	for _, selection := range query.selections {
		if strings.TrimSpace(selection.Name) == "" {
			return Schema{}, fmt.Errorf("esper: projection alias cannot be blank")
		}
		if selection.Expr == nil {
			return Schema{}, fmt.Errorf("esper: projection %q has nil expression", selection.Name)
		}
		if _, exists := seen[selection.Name]; exists {
			return Schema{}, fmt.Errorf("esper: projection duplicates alias %q", selection.Name)
		}
		seen[selection.Name] = struct{}{}
		if err := e.validateExprFields(query.input, selection.Expr); err != nil {
			return Schema{}, err
		}
		fields = append(fields, FieldSpec{Name: selection.Name, Type: selection.Expr.Type()})
	}
	return NewSchema("result:"+query.name, fields...)
}

func (e *Environment) validateJoin(definition *joinDefinition, selections []JoinSelection) error {
	if definition == nil {
		return fmt.Errorf("join definition is required")
	}
	sources := joinDefinitionSources(definition)
	if len(sources) < 2 {
		return fmt.Errorf("join requires at least two streams")
	}
	for index, source := range sources {
		if source == nil {
			return fmt.Errorf("join source %d is nil", index)
		}
		if err := e.validateNode(source); err != nil {
			return fmt.Errorf("join source %d: %w", index, err)
		}
	}
	if err := validateUnidirectionalJoin(definition, sources); err != nil {
		return err
	}
	if err := e.validateJoinEdges(definition, sources); err != nil {
		return err
	}
	if _, err := methodJoinEvaluationOrder(definition); err != nil {
		return err
	}
	conditions := joinDefinitionConditions(definition)
	for index, condition := range conditions {
		if err := e.validateJoinCondition(condition, sources); err != nil {
			return fmt.Errorf("join condition %d: %w", index, err)
		}
	}
	for _, selection := range selections {
		if selection.Expr == nil {
			return fmt.Errorf("join projection %q has nil expression", selection.Name)
		}
		source := selection.sourceIndex()
		if source < 0 || source >= len(sources) {
			return fmt.Errorf("join projection %q references source %d, have %d sources", selection.Name, source, len(sources))
		}
		if err := e.validateExprFields(sources[source], selection.Expr); err != nil {
			return fmt.Errorf("join source %d projection %q: %w", source, selection.Name, err)
		}
	}
	return nil
}

func validateUnidirectionalJoin(definition *joinDefinition, sources []*streamNode) error {
	if definition == nil || !joinDefinitionHasUnidirectional(definition) {
		return nil
	}
	if len(definition.unidirectional) != len(sources) {
		return fmt.Errorf("unidirectional join has %d source flags for %d sources", len(definition.unidirectional), len(sources))
	}
	count := joinDefinitionUnidirectionalCount(definition)
	for index, flagged := range definition.unidirectional {
		if !flagged {
			continue
		}
		for node := sources[index]; node != nil; node = node.input {
			if node.kind == streamWindow {
				return fmt.Errorf("unidirectional join source %d cannot declare a window view", index)
			}
		}
		if source := sources[index]; source != nil && source.kind == streamPattern && source.patternWindow != nil {
			return fmt.Errorf("unidirectional join source %d cannot declare a pattern result window", index)
		}
	}
	if count == 1 {
		return nil
	}
	if count != len(sources) {
		return fmt.Errorf("unidirectional must apply to exactly one source or every source of a full outer join")
	}
	if len(definition.edges) > 0 {
		for index, edge := range definition.edges {
			if edge.kind != JoinFullOuter {
				return fmt.Errorf("all-unidirectional join edge %d must be full outer", index)
			}
		}
		return nil
	}
	if definition.kind != JoinFullOuter {
		return fmt.Errorf("all-unidirectional join must be full outer")
	}
	return nil
}

func (e *Environment) validateJoinEdges(definition *joinDefinition, sources []*streamNode) error {
	if len(definition.edges) == 0 {
		if definition.kind > JoinFullOuter {
			return fmt.Errorf("unknown join kind %d", definition.kind)
		}
		return nil
	}
	if len(definition.edges) != len(sources)-1 {
		return fmt.Errorf("join chain has %d edges for %d sources", len(definition.edges), len(sources))
	}
	for edgeIndex, edge := range definition.edges {
		if edge.kind > JoinFullOuter {
			return fmt.Errorf("join edge %d has unknown kind %d", edgeIndex, edge.kind)
		}
		if len(edge.conditions) == 0 && !joinDefinitionHasUnidirectional(definition) {
			return fmt.Errorf("join edge %d requires at least one condition", edgeIndex)
		}
		introducedSource := edgeIndex + 1
		for conditionIndex, condition := range edge.conditions {
			if !joinConditionReferencesSource(condition, introducedSource) {
				return fmt.Errorf("join edge %d condition %d does not reference introduced source %d", edgeIndex, conditionIndex, introducedSource)
			}
			if err := e.validateJoinCondition(condition, sources[:introducedSource+1]); err != nil {
				return fmt.Errorf("join edge %d condition %d: %w", edgeIndex, conditionIndex, err)
			}
		}
	}
	return nil
}

func joinConditionReferencesSource(condition JoinCondition, source int) bool {
	for _, child := range condition.all {
		if joinConditionReferencesSource(child, source) {
			return true
		}
	}
	for _, child := range condition.any {
		if joinConditionReferencesSource(child, source) {
			return true
		}
	}
	left, right := joinConditionSources(condition)
	return condition.Left != nil && condition.Right != nil && (left == source || right == source)
}

func (e *Environment) validateJoinCondition(condition JoinCondition, sources []*streamNode) error {
	if len(condition.all) > 0 && len(condition.any) > 0 {
		return fmt.Errorf("condition cannot combine all and any")
	}
	if len(condition.all) > 0 || len(condition.any) > 0 {
		children := condition.all
		logic := "all"
		if len(condition.any) > 0 {
			children = condition.any
			logic = "any"
		}
		if len(children) == 0 {
			return fmt.Errorf("%s condition requires at least one child", logic)
		}
		for index, child := range children {
			if err := e.validateJoinCondition(child, sources); err != nil {
				return fmt.Errorf("%s child %d: %w", logic, index, err)
			}
		}
		return nil
	}
	if condition.Left == nil || condition.Right == nil {
		return fmt.Errorf("condition requires left and right expressions")
	}
	leftSource, rightSource := joinConditionSources(condition)
	if leftSource < 0 || leftSource >= len(sources) || rightSource < 0 || rightSource >= len(sources) {
		return fmt.Errorf("condition source indexes (%d,%d) are outside %d sources", leftSource, rightSource, len(sources))
	}
	if condition.Comparison > JoinGreaterOrEqual {
		return fmt.Errorf("unknown join comparison %d", condition.Comparison)
	}
	if err := e.validateExprFields(sources[leftSource], condition.Left); err != nil {
		return fmt.Errorf("left source %d: %w", leftSource, err)
	}
	if err := e.validateExprFields(sources[rightSource], condition.Right); err != nil {
		return fmt.Errorf("right source %d: %w", rightSource, err)
	}
	return nil
}

func (e *Environment) validateAggregate(definition *aggregateDefinition) error {
	if definition == nil || (definition.input == nil && definition.join == nil) {
		return NewError(ErrorInvalidRule, "aggregate requires a source")
	}
	if definition.join != nil {
		if err := e.validateJoin(definition.join, nil); err != nil {
			return err
		}
	} else if err := e.validateNode(definition.input); err != nil {
		return err
	}
	validateFields := e.validateExprFields
	if definition.join != nil {
		validateFields = func(input *streamNode, expression Expr) error {
			return e.validateJoinAggregateFields(definition.join, expression)
		}
	}
	if err := validateAggregateGrouping(definition); err != nil {
		return err
	}
	if definition.grouping != aggregateGroupingPlain {
		for _, selection := range definition.selections {
			if expressionTreeContainsLocalGroup(selection.Expr) {
				return NewError(ErrorInvalidRule, "dimensional grouping cannot be combined with local group-by aggregate parameters")
			}
		}
		if expressionTreeContainsLocalGroup(definition.having) {
			return NewError(ErrorInvalidRule, "dimensional grouping cannot be combined with local group-by aggregate parameters")
		}
	}
	for _, key := range definition.groupBy {
		if key == nil {
			return NewError(ErrorInvalidRule, "group-by expression is required")
		}
		if err := validateFields(definition.input, key); err != nil {
			return err
		}
	}
	if len(definition.selections) == 0 {
		return NewError(ErrorInvalidRule, "aggregate requires at least one projection")
	}
	if definition.where != nil {
		if definition.where.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "aggregate where expression must return bool")
		}
		if definition.join != nil {
			if err := e.validateJoinScopedExpression(definition.join, definition.where, "join aggregate where"); err != nil {
				return err
			}
		} else if err := e.validateExprFields(definition.input, definition.where); err != nil {
			return err
		}
	}
	seen := make(map[string]struct{}, len(definition.selections))
	for _, selection := range definition.selections {
		if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
			return NewError(ErrorInvalidRule, "aggregate projection requires a name and expression")
		}
		if _, exists := seen[selection.Name]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("aggregate projection duplicates alias %q", selection.Name))
		}
		seen[selection.Name] = struct{}{}
		if err := validateAggregateExpressionNodes(selection.Expr.node()); err != nil {
			return err
		}
		if err := e.validateAggregatePluginNodes(selection.Expr.node()); err != nil {
			return err
		}
		if err := validateFields(definition.input, selection.Expr); err != nil {
			return err
		}
	}
	if definition.having != nil {
		if definition.having.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "having expression must return bool")
		}
		if err := validateAggregateExpressionNodes(definition.having.node()); err != nil {
			return err
		}
		if err := e.validateAggregatePluginNodes(definition.having.node()); err != nil {
			return err
		}
		if err := validateFields(definition.input, definition.having); err != nil {
			return err
		}
	}
	return nil
}

func (e *Environment) validateJoinAggregateFields(definition *joinDefinition, expression Expr) error {
	return e.validateJoinScopedExpression(definition, expression, "join aggregate")
}

func (e *Environment) validateJoinScopedExpression(definition *joinDefinition, expression Expr, scope string) error {
	if definition == nil || expression == nil {
		return NewError(ErrorInvalidRule, scope+" expression is required")
	}
	if err := validateBitwiseExpressionNodes(expression.node()); err != nil {
		return err
	}
	if err := validateCoalesceExpressionNodes(expression.node()); err != nil {
		return err
	}
	if err := validateMethodNodes(expression.node()); err != nil {
		return err
	}
	if err := e.validateExprVariables(expression); err != nil {
		return err
	}
	sources := joinDefinitionSources(definition)
	var visit func(*exprNode) error
	visit = func(node *exprNode) error {
		if node == nil {
			return NewError(ErrorInvalidRule, "join aggregate expression contains a nil node")
		}
		switch node.kind {
		case "join-field":
			if node.joinSource < 0 || node.joinSource >= len(sources) {
				return fmt.Errorf("join aggregate field %q references source %d, have %d sources", node.fieldName, node.joinSource, len(sources))
			}
			source, err := sourceNode(sources[node.joinSource])
			if err != nil {
				return err
			}
			schema, err := e.sourceSchema(source)
			if err != nil {
				return err
			}
			field, exists := schema.Field(node.fieldName)
			if !exists {
				return fmt.Errorf("join aggregate references unknown field %q on schema %q", node.fieldName, schema.Name())
			}
			if node.typ != nil && field.Type != nil && field.Type != typeOf[any]() &&
				!field.Type.AssignableTo(node.typ) && !node.typ.AssignableTo(field.Type) && !numericTypes(field.Type, node.typ) {
				return fmt.Errorf("join field %q has type %s, expression expects %s", node.fieldName, field.Type, node.typ)
			}
		case "join-event":
			if node.joinSource < 0 || node.joinSource >= len(sources) {
				return fmt.Errorf("join aggregate event references source %d, have %d sources", node.joinSource, len(sources))
			}
		case "field":
			return NewError(ErrorInvalidRule, scope+" fields must use JoinField(source, name)")
		}
		for _, child := range node.children {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(expression.node()); err != nil {
		return err
	}
	return e.validateExpressionSubqueries(expression.node())
}

func (e *Environment) validateIntoTable(query Query) error {
	if strings.TrimSpace(query.tableTarget) == "" {
		return nil
	}
	if query.aggregate == nil {
		return NewError(ErrorInvalidRule, "into-table requires an aggregate query")
	}
	if query.contextName != "" {
		return NewError(ErrorInvalidRule, "into-table does not yet support context-partitioned aggregate state")
	}
	definition, ok := e.Table(query.tableTarget)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("into-table target %q is not registered", query.tableTarget))
	}
	columns := definition.Columns()
	columnByName := make(map[string]TableColumn, len(columns))
	for _, column := range columns {
		columnByName[column.Name] = column
	}
	selected := make(map[string]struct{}, len(query.aggregate.selections))
	for _, selection := range query.aggregate.selections {
		selected[selection.Name] = struct{}{}
		column, exists := columnByName[selection.Name]
		if !exists {
			return NewError(ErrorUnknownName, fmt.Sprintf("aggregate projection %q is not a column of table %q", selection.Name, query.tableTarget))
		}
		if column.Type != nil && column.Type != typeOf[any]() && selection.Expr.Type() != nil &&
			!column.Type.AssignableTo(selection.Expr.Type()) && !selection.Expr.Type().AssignableTo(column.Type) && !numericTypes(column.Type, selection.Expr.Type()) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("into-table column %q expects %s, aggregate returns %s", selection.Name, column.Type, selection.Expr.Type()))
		}
	}
	for _, column := range columns {
		if column.PrimaryKey || !column.Optional {
			if _, exists := selected[column.Name]; !exists {
				return NewError(ErrorInvalidRule, fmt.Sprintf("into-table projection must provide required column %q", column.Name))
			}
		}
	}
	if len(query.aggregate.groupBy) > 0 && len(definition.PrimaryKey()) == 0 {
		return NewError(ErrorInvalidRule, "grouped into-table aggregation requires a primary-key column")
	}
	return nil
}

func (e *Environment) validateAggregatePluginNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.kind == "aggregate-plugin-ref" || node.kind == "aggregate-plugin-factory-ref" {
		if node.pluginEnvironment == nil || node.pluginEnvironment != e {
			return NewError(ErrorDependency, fmt.Sprintf("aggregate plugin %q belongs to a different environment", node.pluginName))
		}
		e.mu.RLock()
		definition, ok := e.aggregatePlugins[node.pluginName]
		e.mu.RUnlock()
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("aggregate plugin %q is not registered", node.pluginName))
		}
		if definition.resultType != nil && node.typ != nil && definition.resultType != node.typ &&
			!definition.resultType.AssignableTo(node.typ) && !node.typ.AssignableTo(definition.resultType) && !numericTypes(definition.resultType, node.typ) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("aggregate plugin %q returns %s, expression expects %s", node.pluginName, definition.resultType, node.typ))
		}
		if node.kind == "aggregate-plugin-factory-ref" && definition.factory == nil {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("aggregate plugin %q is not a stateful factory", node.pluginName))
		}
		if node.kind == "aggregate-plugin-ref" && definition.evaluate == nil {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("aggregate plugin %q is not a callback evaluator", node.pluginName))
		}
	}
	for _, child := range node.children {
		if err := e.validateAggregatePluginNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func validateAggregateExpressionNodes(node *exprNode) error {
	if node == nil {
		return NewError(ErrorInvalidRule, "aggregate expression node is required")
	}
	if node.kind == "count-ever-invalid" {
		return NewError(ErrorInvalidRule, "count-ever accepts at most one expression")
	}
	switch node.kind {
	case "tag-sum", "tag-avg", "tag-min", "tag-max", "tag-first", "tag-last":
		if strings.TrimSpace(node.tagName) == "" {
			return NewError(ErrorInvalidRule, "tag aggregate requires a tag name")
		}
		if len(node.children) != 1 || node.children[0] == nil {
			return NewError(ErrorInvalidRule, "tag aggregate requires an element expression")
		}
	}
	if node.kind == "sorted-access" && (len(node.children) != 2 || node.children[0] == nil || node.children[1] == nil) {
		return NewError(ErrorInvalidRule, "sorted access aggregate requires value and key expressions")
	}
	if strings.HasPrefix(node.kind, "sorted-access-") && (len(node.children) == 0 || node.children[0] == nil) {
		return NewError(ErrorInvalidRule, "sorted access method requires a sorted access aggregate")
	}
	if node.kind == "window-access" && (len(node.children) != 1 || node.children[0] == nil) {
		return NewError(ErrorInvalidRule, "window access aggregate requires a value expression")
	}
	if strings.HasPrefix(node.kind, "window-access-") && (len(node.children) == 0 || node.children[0] == nil) {
		return NewError(ErrorInvalidRule, "window access method requires a window access aggregate")
	}
	if node.kind == "count-min-sketch" && (len(node.children) < 1 || len(node.children) > 2 || node.children[0] == nil || (len(node.children) == 2 && node.children[1] == nil)) {
		return NewError(ErrorInvalidRule, "count-min-sketch requires a value and optional predicate")
	}
	if node.kind == "count-min-frequency" && (len(node.children) != 2 || node.children[0] == nil || node.children[1] == nil) {
		return NewError(ErrorInvalidRule, "count-min-sketch frequency requires a sketch and key")
	}
	if node.kind == "count-min-total" && (len(node.children) != 1 || node.children[0] == nil) {
		return NewError(ErrorInvalidRule, "count-min-sketch total requires a sketch")
	}
	if node.kind == "rate-timestamp" {
		if len(node.children) < 1 || len(node.children) > 2 || node.children[0] == nil {
			return NewError(ErrorInvalidRule, "timestamp rate requires a timestamp and optional predicate")
		}
		if node.children[0].typ != typeOf[any]() && !isIntegralType(node.children[0].typ) {
			return NewError(ErrorTypeMismatch, "timestamp rate requires an integral timestamp expression")
		}
		if len(node.children) == 2 && (node.children[1] == nil || node.children[1].typ != typeOf[bool]()) {
			return NewError(ErrorTypeMismatch, "timestamp rate filter predicate must return bool")
		}
	}
	if node.kind == "rate" {
		if len(node.children) > 1 || (len(node.children) == 1 && node.children[0] == nil) {
			return NewError(ErrorInvalidRule, "rate accepts at most one boolean filter predicate")
		}
		if len(node.children) == 1 && node.children[0].typ != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "rate filter predicate must return bool")
		}
	}
	if node.kind == "rate-quantity-timestamp" {
		if len(node.children) < 2 || len(node.children) > 3 || node.children[0] == nil || node.children[1] == nil {
			return NewError(ErrorInvalidRule, "quantity rate requires timestamp and quantity expressions")
		}
		if node.children[0].typ != typeOf[any]() && !isIntegralType(node.children[0].typ) {
			return NewError(ErrorTypeMismatch, "quantity rate requires an integral timestamp expression")
		}
		if node.children[1].typ != typeOf[any]() && !isNumericType(node.children[1].typ) {
			return NewError(ErrorTypeMismatch, "quantity rate requires a numeric quantity expression")
		}
		if len(node.children) == 3 && (node.children[2] == nil || node.children[2].typ != typeOf[bool]()) {
			return NewError(ErrorTypeMismatch, "quantity rate filter predicate must return bool")
		}
	}
	if node.kind == "leaving" {
		if len(node.children) > 1 || (len(node.children) == 1 && node.children[0] == nil) {
			return NewError(ErrorInvalidRule, "leaving accepts at most one boolean filter predicate")
		}
		if len(node.children) == 1 && node.children[0].typ != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "leaving filter predicate must return bool")
		}
	}
	if node.kind == "aggregate-plugin" || node.kind == "aggregate-plugin-ref" || node.kind == "aggregate-plugin-factory" || node.kind == "aggregate-plugin-factory-ref" {
		if strings.TrimSpace(node.pluginName) == "" {
			return NewError(ErrorInvalidRule, "plugin aggregate name is required")
		}
		if (node.kind == "aggregate-plugin" || node.kind == "aggregate-plugin-factory") && !node.pluginReady {
			return NewError(ErrorInvalidRule, fmt.Sprintf("plugin aggregate %q has no evaluator", node.pluginName))
		}
		if (node.kind == "aggregate-plugin-factory" || node.kind == "aggregate-plugin-factory-ref") && len(node.children) > 1 {
			return NewError(ErrorInvalidRule, "plugin aggregate factory accepts at most one input expression")
		}
	}
	if node.kind == "aggregate-filter" {
		if len(node.children) != 2 || node.children[0] == nil || node.children[1] == nil {
			return NewError(ErrorInvalidRule, "filtered aggregate requires an aggregate and predicate")
		}
		if node.children[1].typ != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "filtered aggregate predicate must return bool")
		}
	}
	if node.kind == "aggregate-local-group" {
		if len(node.children) == 0 || node.children[0] == nil {
			return NewError(ErrorInvalidRule, "local group aggregate requires an aggregate expression")
		}
		for index, child := range node.children[1:] {
			if child == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("local group key %d is nil", index))
			}
			if expressionNodeContainsAggregate(child) {
				return NewError(ErrorInvalidRule, "local group key cannot contain an aggregate expression")
			}
		}
	}
	for _, child := range node.children {
		if err := validateAggregateExpressionNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func expressionNodeContainsAggregate(node *exprNode) bool {
	if node == nil {
		return false
	}
	if node.kind == "sorted-access" || strings.HasPrefix(node.kind, "sorted-access-") || node.kind == "window-access" || strings.HasPrefix(node.kind, "window-access-") {
		return true
	}
	switch node.kind {
	case "tag-sum", "tag-avg", "tag-min", "tag-max", "tag-first", "tag-last":
		return true
	}
	switch node.kind {
	case "aggregate-filter", "aggregate-local-group", "aggregate-plugin", "aggregate-plugin-ref", "aggregate-plugin-factory", "aggregate-plugin-factory-ref", "count-min-sketch", "count-min-frequency", "count-min-total", "rate-timestamp", "rate-quantity-timestamp", "leaving", "count", "sum", "sum-exact", "avg", "avg-exact", "min", "min-exact", "max", "max-exact", "first", "last", "nth", "count-distinct", "median", "stddev", "stddev-pop", "variance", "avedev", "weighted-avg", "rate", "min-by", "max-by", "min-by-ever", "max-by-ever", "window", "set", "sorted", "count-ever", "first-ever", "last-ever":
		return true
	}
	for _, child := range node.children {
		if expressionNodeContainsAggregate(child) {
			return true
		}
	}
	return false
}

func expressionTreeContainsLocalGroup(expression Expr) bool {
	if expression == nil || expression.node() == nil {
		return false
	}
	var visit func(*exprNode) bool
	visit = func(node *exprNode) bool {
		if node == nil {
			return false
		}
		if node.kind == "aggregate-local-group" {
			return true
		}
		for _, child := range node.children {
			if visit(child) {
				return true
			}
		}
		return false
	}
	return visit(expression.node())
}

func validateAggregateGrouping(definition *aggregateDefinition) error {
	if definition == nil {
		return NewError(ErrorInvalidRule, "aggregate grouping is required")
	}
	if definition.grouping < aggregateGroupingPlain || definition.grouping > aggregateGroupingSets {
		return NewError(ErrorInvalidRule, fmt.Sprintf("unknown aggregate grouping mode %d", definition.grouping))
	}
	seenDimensions := make(map[string]struct{}, len(definition.groupBy))
	for _, expression := range definition.groupBy {
		if expression == nil {
			continue
		}
		key := groupingExpressionKey(expression)
		if _, exists := seenDimensions[key]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("aggregate grouping duplicates expression %q", expression.Description()))
		}
		seenDimensions[key] = struct{}{}
	}
	if definition.grouping == aggregateGroupingPlain {
		if len(definition.groupingSets) != 0 {
			return NewError(ErrorInvalidRule, "plain aggregate grouping cannot carry grouping sets")
		}
		return nil
	}
	if len(definition.groupBy) > 20 {
		return NewError(ErrorInvalidRule, "aggregate grouping has too many dimensions")
	}
	if definition.grouping == aggregateGroupingSets && len(definition.groupingSets) == 0 {
		return NewError(ErrorInvalidRule, "grouping sets requires at least one set")
	}
	if definition.grouping != aggregateGroupingSets && len(definition.groupingSets) != 0 {
		return NewError(ErrorInvalidRule, "rollup/cube cannot carry explicit grouping sets")
	}
	if len(definition.groupBy) == 0 {
		if definition.grouping == aggregateGroupingSets && len(definition.groupingSets) == 1 && len(definition.groupingSets[0]) == 0 {
			return NewError(ErrorInvalidRule, "overall grouping cannot be the only grouping set")
		}
		return NewError(ErrorInvalidRule, "dimensional grouping requires at least one expression")
	}
	if definition.grouping != aggregateGroupingSets {
		return nil
	}
	seenSets := make(map[string]struct{}, len(definition.groupingSets))
	for setIndex, set := range definition.groupingSets {
		canonical := append([]int(nil), set...)
		sort.Ints(canonical)
		parts := make([]string, 0, len(canonical))
		seenInSet := make(map[int]struct{}, len(canonical))
		for _, dimension := range canonical {
			if dimension < 0 || dimension >= len(definition.groupBy) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("grouping set %d references dimension %d outside %d dimensions", setIndex, dimension, len(definition.groupBy)))
			}
			if _, exists := seenInSet[dimension]; exists {
				return NewError(ErrorInvalidRule, fmt.Sprintf("grouping set %d duplicates dimension %d", setIndex, dimension))
			}
			seenInSet[dimension] = struct{}{}
			parts = append(parts, fmt.Sprintf("%d", dimension))
		}
		key := strings.Join(parts, ",")
		if _, exists := seenSets[key]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("grouping sets duplicates set %q", key))
		}
		seenSets[key] = struct{}{}
	}
	return nil
}

func (e *Environment) validateRowRecog(definition *rowRecogDefinition, selections []Selection) error {
	if definition == nil || definition.input == nil {
		return NewError(ErrorInvalidRule, "match-recognize requires a source")
	}
	if err := e.validateNode(definition.input); err != nil {
		return err
	}
	if err := definition.pattern.validate("pattern"); err != nil {
		return NewError(ErrorInvalidRule, err.Error())
	}
	variables := make(map[string]struct{})
	rowPatternVariables(definition.pattern, variables)
	if len(variables) == 0 {
		return NewError(ErrorInvalidRule, "match-recognize pattern requires at least one variable")
	}
	if len(definition.duplicateDefines) > 0 {
		duplicates := make([]string, 0, len(definition.duplicateDefines))
		for name := range definition.duplicateDefines {
			duplicates = append(duplicates, name)
		}
		sort.Strings(duplicates)
		return NewError(ErrorInvalidRule, fmt.Sprintf("match-recognize DEFINE variable %q has already been defined", duplicates[0]))
	}
	for name, predicate := range definition.defines {
		name = strings.TrimSpace(name)
		if name == "" {
			return NewError(ErrorInvalidRule, "match-recognize DEFINE name cannot be blank")
		}
		if _, exists := variables[name]; !exists {
			return NewError(ErrorUnknownName, fmt.Sprintf("DEFINE references variable %q that is not present in the pattern", name))
		}
		if predicate == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("DEFINE %q has a nil predicate", name))
		}
		if predicate.Type() != nil && predicate.Type() != typeOf[bool]() && predicate.Type() != typeOf[any]() {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("DEFINE %q predicate must return bool", name))
		}
		if err := e.validateExprFields(definition.input, predicate); err != nil {
			return fmt.Errorf("DEFINE %q: %w", name, err)
		}
		if err := validateRowRecogDefineTags(predicate, name, definition.pattern, variables); err != nil {
			return fmt.Errorf("DEFINE %q: %w", name, err)
		}
		if rowRecogContainsRegularAggregate(predicate.node()) {
			return NewError(ErrorInvalidRule, fmt.Sprintf("DEFINE %q cannot contain an aggregate expression", name))
		}
	}
	for index, key := range definition.partition {
		if key == nil {
			return fmt.Errorf("partition-by expression %d is nil", index)
		}
		if err := e.validateExprFields(definition.input, key); err != nil {
			return fmt.Errorf("partition-by expression %d: %w", index, err)
		}
	}
	if definition.maxStates < 0 {
		return NewError(ErrorInvalidRule, "match-recognize max states cannot be negative")
	}
	if definition.interval < 0 {
		return NewError(ErrorInvalidRule, "match-recognize interval cannot be negative")
	}
	if definition.intervalCalendar != nil {
		period := definition.intervalCalendar
		if period.Years < 0 || period.Months < 0 || period.Days < 0 {
			return NewError(ErrorInvalidRule, "match-recognize calendar interval cannot be negative")
		}
		if period.Years == 0 && period.Months == 0 && period.Days == 0 {
			return NewError(ErrorInvalidRule, "match-recognize calendar interval must be positive")
		}
	}
	if definition.skip > RowRecogSkipToCurrentRow {
		return NewError(ErrorInvalidRule, fmt.Sprintf("unknown match-recognize skip strategy %d", definition.skip))
	}
	if len(selections) == 0 {
		return NewError(ErrorInvalidRule, "match-recognize requires at least one measure")
	}
	seen := make(map[string]struct{}, len(selections))
	for _, selection := range selections {
		name := strings.TrimSpace(selection.Name)
		if name == "" || selection.Expr == nil {
			return NewError(ErrorInvalidRule, "match-recognize measure requires a name and expression")
		}
		if _, exists := seen[name]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("match-recognize measure duplicates alias %q", name))
		}
		seen[name] = struct{}{}
		if err := e.validateExprFields(definition.input, selection.Expr); err != nil {
			return fmt.Errorf("measure %q: %w", name, err)
		}
		if err := validateRowRecogTags(selection.Expr, variables); err != nil {
			return fmt.Errorf("measure %q: %w", name, err)
		}
		if err := validateRowRecogMeasureAggregates(selection.Expr.node(), definition.pattern); err != nil {
			return fmt.Errorf("measure %q: %w", name, err)
		}
	}
	return nil
}

func validateRowRecogDefineTags(expression Expr, variable string, pattern RowPattern, variables map[string]struct{}) error {
	if expression == nil || expression.node() == nil {
		return NewError(ErrorInvalidRule, "match-recognize expression is nil")
	}
	var tags []string
	expression.node().referencedTags(&tags)
	allowed := map[string]struct{}{variable: {}}
	order, linear := rowPatternLinearVariables(pattern)
	if linear {
		position := -1
		for index, name := range order {
			if name == variable {
				position = index
				break
			}
		}
		if position >= 0 {
			for _, name := range order[:position] {
				allowed[name] = struct{}{}
			}
		}
	}
	for _, tag := range tags {
		if _, ok := variables[tag]; !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("match-recognize expression references unknown variable tag %q", tag))
		}
		if linear {
			if _, ok := allowed[tag]; !ok {
				return NewError(ErrorInvalidRule, fmt.Sprintf("DEFINE %q references future variable tag %q", variable, tag))
			}
		}
	}
	return nil
}

func rowPatternLinearVariables(pattern RowPattern) ([]string, bool) {
	result := make([]string, 0)
	var walk func(RowPattern) bool
	walk = func(current RowPattern) bool {
		switch current.kind {
		case rowPatternVariable:
			if current.name != "" {
				result = append(result, current.name)
			}
			return true
		case rowPatternSequence:
			for _, part := range current.parts {
				if !walk(part) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	if !walk(pattern) {
		return nil, false
	}
	counts := make(map[string]int, len(result))
	for _, name := range result {
		counts[name]++
		if counts[name] > 1 {
			return nil, false
		}
	}
	return result, true
}

func rowRecogContainsRegularAggregate(node *exprNode) bool {
	if node == nil {
		return false
	}
	if rowRecogIsRegularAggregateKind(node.kind) {
		return true
	}
	for _, child := range node.children {
		if rowRecogContainsRegularAggregate(child) {
			return true
		}
	}
	return false
}

func rowRecogIsRegularAggregateKind(kind string) bool {
	if strings.HasPrefix(kind, "tag-") {
		return false
	}
	if strings.HasPrefix(kind, "aggregate-") || strings.HasPrefix(kind, "linear-regression-") || strings.HasPrefix(kind, "univariate-statistics-") {
		return true
	}
	switch kind {
	case "count", "sum", "sum-exact", "avg", "avg-exact", "min", "min-exact", "max", "max-exact", "first", "last", "first-ever", "last-ever", "count-ever", "count-distinct", "median", "stddev", "stddev-pop", "variance", "avedev", "weighted-avg", "correlation", "rate", "min-by", "max-by", "min-by-ever", "max-by-ever", "window", "sorted", "set", "count-min-frequency", "count-min-total":
		return true
	default:
		return false
	}
}

func validateRowRecogMeasureAggregates(node *exprNode, pattern RowPattern) error {
	if node == nil {
		return NewError(ErrorInvalidRule, "match-recognize expression is nil")
	}
	if rowRecogIsRegularAggregateKind(node.kind) {
		var tags []string
		node.referencedTags(&tags)
		unique := make(map[string]struct{}, len(tags))
		for _, tag := range tags {
			unique[tag] = struct{}{}
		}
		if len(unique) == 0 {
			// Go's ordinary aggregate expressions intentionally evaluate over
			// the current match group. CountAll and Sum(Field(...)) therefore
			// remain valid even though the Java EPL spelling would require an
			// explicit group-variable property.
			return nil
		}
		if len(unique) > 1 {
			return NewError(ErrorInvalidRule, "aggregate measure must refer to properties of exactly one group variable returning multiple events")
		}
		for tag := range unique {
			if !rowPatternVariableMayRepeat(pattern, tag) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("aggregate measure group variable %q must return multiple events", tag))
			}
		}
	}
	for _, child := range node.children {
		if err := validateRowRecogMeasureAggregates(child, pattern); err != nil {
			return err
		}
	}
	return nil
}

func rowPatternVariableMayRepeat(pattern RowPattern, name string) bool {
	if !rowPatternContainsVariableNamed(pattern, name) {
		return false
	}
	_, maximum := rowPatternBounds(pattern)
	if maximum == 0 || maximum > 1 {
		return true
	}
	if pattern.kind == rowPatternVariable {
		return false
	}
	if rowPatternVariableCount(pattern, name) > 1 {
		return true
	}
	for _, part := range pattern.parts {
		if rowPatternVariableMayRepeat(part, name) {
			return true
		}
	}
	return false
}

func rowPatternContainsVariableNamed(pattern RowPattern, name string) bool {
	if pattern.kind == rowPatternVariable {
		return pattern.name == name
	}
	for _, part := range pattern.parts {
		if rowPatternContainsVariableNamed(part, name) {
			return true
		}
	}
	return false
}

func rowPatternVariableCount(pattern RowPattern, name string) int {
	if pattern.kind == rowPatternVariable {
		if pattern.name == name {
			return 1
		}
		return 0
	}
	count := 0
	for _, part := range pattern.parts {
		count += rowPatternVariableCount(part, name)
	}
	return count
}

func validateRowRecogTags(expression Expr, variables map[string]struct{}) error {
	if expression == nil || expression.node() == nil {
		return NewError(ErrorInvalidRule, "match-recognize expression is nil")
	}
	var tags []string
	expression.node().referencedTags(&tags)
	for _, tag := range tags {
		if _, ok := variables[tag]; !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("match-recognize expression references unknown variable tag %q", tag))
		}
	}
	return nil
}

func (e *Environment) validatePattern(definition *patternDefinition, selections []Selection) error {
	if err := validatePattern(definition); err != nil {
		return err
	}
	inputs := patternDefinitionInputs(definition)
	if len(inputs) == 0 {
		return NewError(ErrorInvalidRule, "pattern requires at least one source")
	}
	for index, input := range inputs {
		if err := e.validateNode(input); err != nil {
			return fmt.Errorf("pattern source %d: %w", index, err)
		}
	}
	if definition.guard != nil {
		if err := e.validateExprFields(definition.input, definition.guard); err != nil {
			return fmt.Errorf("pattern while guard: %w", err)
		}
	}
	tagSources := patternTagSources(definition)
	if definition.root != nil {
		if err := e.validatePatternNodeFields(definition.input, definition.root, tagSources, true); err != nil {
			return err
		}
	} else {
		for _, step := range definition.steps {
			if err := e.validatePatternExpressionFields(definition.input, step.predicate, tagSources, true); err != nil {
				return err
			}
		}
	}
	if len(selections) == 0 {
		return NewError(ErrorInvalidRule, "pattern requires at least one projection")
	}
	seen := make(map[string]struct{}, len(selections))
	for _, selection := range selections {
		if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
			return NewError(ErrorInvalidRule, "pattern projection requires a name and expression")
		}
		if _, exists := seen[selection.Name]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("pattern projection duplicates alias %q", selection.Name))
		}
		seen[selection.Name] = struct{}{}
		if err := e.validatePatternExpressionFields(definition.input, selection.Expr, tagSources, false); err != nil {
			return err
		}
	}
	return nil
}

func patternTagSources(definition *patternDefinition) map[string][]*streamNode {
	result := make(map[string][]*streamNode)
	if definition == nil {
		return result
	}
	var visit func(*patternNode)
	visit = func(node *patternNode) {
		if node == nil {
			return
		}
		if node.kind == patternEventNode && strings.TrimSpace(node.tag) != "" {
			source := node.source
			if source == nil {
				source = definition.input
			}
			if source != nil {
				known := false
				for _, existing := range result[node.tag] {
					if existing == source {
						known = true
						break
					}
				}
				if !known {
					result[node.tag] = append(result[node.tag], source)
				}
			}
		}
		visit(node.left)
		visit(node.right)
		visit(node.child)
	}
	visit(definition.root)
	return result
}

func (e *Environment) validatePatternExpressionFields(input *streamNode, expression Expr, tagSources map[string][]*streamNode, requireTags bool) error {
	if err := e.validateExprFields(input, expression); err != nil {
		return err
	}
	var references []tagFieldReference
	expression.node().referencedTagFields(&references)
	for _, reference := range references {
		sources := tagSources[reference.tag]
		if len(sources) == 0 && !requireTags {
			continue
		}
		if len(sources) == 0 {
			return NewError(ErrorUnknownName, fmt.Sprintf("pattern expression references unknown tag %q", reference.tag))
		}
		matched := false
		var lastSchema Schema
		for _, source := range sources {
			schema, err := e.sourceSchema(source)
			if err != nil {
				return err
			}
			lastSchema = schema
			field, exists := schema.Field(reference.name)
			if !exists {
				continue
			}
			if reference.typ == nil || field.Type == nil || field.Type == typeOf[any]() || reference.typ == typeOf[any]() || field.Type.AssignableTo(reference.typ) || reference.typ.AssignableTo(field.Type) || numericTypes(field.Type, reference.typ) {
				matched = true
				break
			}
		}
		if !matched {
			if lastSchema.valid() {
				return fmt.Errorf("esper: pattern tag %q field %q is not valid on schema %q", reference.tag, reference.name, lastSchema.Name())
			}
			return fmt.Errorf("esper: pattern tag %q field %q is not valid", reference.tag, reference.name)
		}
	}
	return nil
}

func (e *Environment) validatePatternNodeFields(input *streamNode, node *patternNode, tagSources map[string][]*streamNode, requireTags bool) error {
	if node == nil {
		return NewError(ErrorInvalidRule, "pattern expression cannot be nil")
	}
	switch node.kind {
	case patternEventNode:
		eventInput := node.source
		if eventInput == nil {
			eventInput = input
		}
		return e.validatePatternExpressionFields(eventInput, node.predicate, tagSources, requireTags)
	case patternSequenceNode, patternAndNode, patternOrNode:
		if node.kind == patternSequenceNode && node.sequenceMaxExpr != nil {
			if err := e.validateExprFields(input, node.sequenceMaxExpr); err != nil {
				return fmt.Errorf("followed-by maximum: %w", err)
			}
			var fields []string
			node.sequenceMaxExpr.node().referencedFields(&fields)
			var tags []string
			node.sequenceMaxExpr.node().referencedTags(&tags)
			if len(fields) > 0 || len(tags) > 0 {
				return NewError(ErrorInvalidRule, "followed-by maximum expression cannot reference event fields or pattern tags")
			}
		}
		if err := e.validatePatternNodeFields(input, node.left, tagSources, requireTags); err != nil {
			return err
		}
		return e.validatePatternNodeFields(input, node.right, tagSources, requireTags)
	case patternNotNode:
		return e.validatePatternNodeFields(input, node.child, tagSources, requireTags)
	case patternMatchUntilNode:
		if node.minimumExpr != nil {
			if err := e.validateExprFields(input, node.minimumExpr); err != nil {
				return fmt.Errorf("match-until minimum: %w", err)
			}
		}
		if node.maximumExpr != nil {
			if err := e.validateExprFields(input, node.maximumExpr); err != nil {
				return fmt.Errorf("match-until maximum: %w", err)
			}
		}
		if err := e.validatePatternNodeFields(input, node.child, tagSources, requireTags); err != nil {
			return err
		}
		if node.right != nil {
			return e.validatePatternNodeFields(input, node.right, tagSources, requireTags)
		}
		return nil
	case patternUntilNode:
		if err := e.validatePatternNodeFields(input, node.child, tagSources, requireTags); err != nil {
			return err
		}
		return e.validatePatternNodeFields(input, node.right, tagSources, requireTags)
	case patternEveryNode:
		if node.everyExpr != nil {
			if err := e.validateExprFields(input, node.everyExpr); err != nil {
				return err
			}
		}
		return e.validatePatternNodeFields(input, node.child, tagSources, requireTags)
	case patternWithinNode:
		if node.durationExpr != nil {
			if err := e.validateExprFields(input, node.durationExpr); err != nil {
				return err
			}
		}
		return e.validatePatternNodeFields(input, node.child, tagSources, requireTags)
	case patternTimerIntervalNode:
		if node.durationExpr != nil {
			return e.validateExprFields(input, node.durationExpr)
		}
		return nil
	case patternTimerAtNode:
		return nil
	case patternTimerScheduleNode:
		if node.scheduleExpr != nil {
			return e.validateExprFields(input, node.scheduleExpr)
		}
		return nil
	case patternTimerCronNode:
		return e.validateCronScheduleForPattern(node.cron)
	default:
		return NewError(ErrorInvalidRule, "unknown pattern expression kind")
	}
}

func (e *Environment) validateSourceLess(selections []Selection) error {
	if len(selections) == 0 {
		return NewError(ErrorInvalidRule, "source-less query requires at least one projection")
	}
	seen := make(map[string]struct{}, len(selections))
	for _, selection := range selections {
		if strings.TrimSpace(selection.Name) == "" || selection.Expr == nil {
			return NewError(ErrorInvalidRule, "source-less projection requires a name and expression")
		}
		if _, exists := seen[selection.Name]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("source-less projection duplicates alias %q", selection.Name))
		}
		seen[selection.Name] = struct{}{}
		if err := e.validateExprVariables(selection.Expr); err != nil {
			return err
		}
		if err := validateBitwiseExpressionNodes(selection.Expr.node()); err != nil {
			return err
		}
		if err := validateCoalesceExpressionNodes(selection.Expr.node()); err != nil {
			return err
		}
		var fields []string
		selection.Expr.node().referencedFields(&fields)
		if len(fields) > 0 {
			return NewError(ErrorDependency, "source-less query cannot reference event fields")
		}
	}
	return nil
}

func validateOutputPolicy(policy OutputPolicy) error {
	if policy.Snapshot && policy.Kind != OutputEveryPolicy && policy.Kind != OutputEveryTimePolicy {
		return NewError(ErrorInvalidRule, "snapshot flag is only valid for every-count or every-time output")
	}
	if policy.Snapshot && policy.When != nil {
		return NewError(ErrorInvalidRule, "snapshot-every output cannot be combined with a when condition")
	}
	if policy.Snapshot && policy.Cron != nil {
		return NewError(ErrorInvalidRule, "snapshot-every output cannot be combined with a calendar schedule")
	}
	if (policy.Kind == OutputFirstPolicy || policy.Kind == OutputEveryPolicy || policy.Kind == OutputFirstEveryEventsPolicy || policy.Kind == OutputLastEveryEventsPolicy) && policy.Count <= 0 {
		return NewError(ErrorInvalidRule, "output count must be positive")
	}
	if (policy.Kind == OutputEveryTimePolicy || policy.Kind == OutputFirstEveryTimePolicy || policy.Kind == OutputLastEveryTimePolicy) && policy.Interval <= 0 {
		return NewError(ErrorInvalidRule, "time-based output interval must be positive")
	}
	if policy.Kind > OutputLastEveryTimePolicy {
		return NewError(ErrorInvalidRule, "unknown output policy")
	}
	switch policy.Termination {
	case OutputNoTermination, OutputAndOnTermination, OutputOnlyOnTermination:
	default:
		return NewError(ErrorInvalidRule, "unknown context-termination output mode")
	}
	if policy.Termination == OutputNoTermination && (policy.TerminationWhen != nil || len(policy.TerminationThen) > 0) {
		return NewError(ErrorInvalidRule, "termination condition and assignments require context-termination output")
	}
	if len(policy.TerminationThen) > 0 && policy.TerminationWhen == nil {
		return NewError(ErrorInvalidRule, "termination assignments require a termination condition")
	}
	switch policy.After {
	case OutputAfterNone:
	case OutputAfterEventCount:
		if policy.AfterCount < 0 {
			return NewError(ErrorInvalidRule, "output-after event count cannot be negative")
		}
	case OutputAfterDuration:
		if policy.AfterDuration < 0 {
			return NewError(ErrorInvalidRule, "output-after duration cannot be negative")
		}
	case OutputAfterCalendarKind:
		period := policy.AfterCalendar
		if period.Years < 0 || period.Months < 0 || period.Days < 0 {
			return NewError(ErrorInvalidRule, "output-after calendar period cannot be negative")
		}
		if period.Years == 0 && period.Months == 0 && period.Days == 0 {
			return NewError(ErrorInvalidRule, "output-after calendar period must be positive")
		}
	default:
		return NewError(ErrorInvalidRule, "unknown output-after condition")
	}
	return nil
}

func (e *Environment) validateOutputExpressions(policy OutputPolicy) error {
	if err := e.validateCronSchedule(policy.Cron); err != nil {
		return err
	}
	if policy.When == nil && len(policy.Then) > 0 {
		return NewError(ErrorInvalidRule, "output assignments require a when condition")
	}
	if policy.When != nil {
		if err := e.validateExprVariables(policy.When); err != nil {
			return err
		}
		if err := validateCoalesceExpressionNodes(policy.When.node()); err != nil {
			return err
		}
		if typ := policy.When.Type(); typ != nil && typ != typeOf[any]() && typ != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("output when condition must be boolean, got %s", typ))
		}
		var fields []string
		policy.When.node().referencedFields(&fields)
		if len(fields) > 0 {
			return NewError(ErrorInvalidRule, "output when condition can reference variables only")
		}
	}
	seen := make(map[string]struct{}, len(policy.Then))
	for index, assignment := range policy.Then {
		name := strings.TrimSpace(assignment.Name)
		if name == "" || assignment.Expr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("output assignment %d is invalid", index))
		}
		if _, exists := seen[name]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("output variable %q is assigned more than once", name))
		}
		seen[name] = struct{}{}
		definition, ok := e.Variable(name)
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("output assignment references unknown variable %q", name))
		}
		if definition.constant {
			return NewError(ErrorState, fmt.Sprintf("output assignment cannot update constant variable %q", name))
		}
		if err := e.validateExprVariables(assignment.Expr); err != nil {
			return err
		}
		if err := validateCoalesceExpressionNodes(assignment.Expr.node()); err != nil {
			return err
		}
		var fields []string
		assignment.Expr.node().referencedFields(&fields)
		if len(fields) > 0 {
			return NewError(ErrorInvalidRule, "output assignments can reference variables only")
		}
		if expressionType := assignment.Expr.Type(); definition.typ != nil && definition.typ != typeOf[any]() && expressionType != nil && expressionType != typeOf[any]() {
			if !definition.typ.AssignableTo(expressionType) && !expressionType.AssignableTo(definition.typ) && !numericTypes(definition.typ, expressionType) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("output variable %q has type %s, assignment expression has type %s", name, definition.typ, expressionType))
			}
		}
	}
	if policy.TerminationWhen != nil {
		if err := e.validateExprVariables(policy.TerminationWhen); err != nil {
			return err
		}
		if err := validateCoalesceExpressionNodes(policy.TerminationWhen.node()); err != nil {
			return err
		}
		if typ := policy.TerminationWhen.Type(); typ != nil && typ != typeOf[any]() && typ != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("termination output condition must be boolean, got %s", typ))
		}
		var fields []string
		policy.TerminationWhen.node().referencedFields(&fields)
		if len(fields) > 0 {
			return NewError(ErrorInvalidRule, "termination output condition can reference variables only")
		}
	}
	seen = make(map[string]struct{}, len(policy.TerminationThen))
	for index, assignment := range policy.TerminationThen {
		name := strings.TrimSpace(assignment.Name)
		if name == "" || assignment.Expr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("termination output assignment %d is invalid", index))
		}
		if _, exists := seen[name]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("termination output variable %q is assigned more than once", name))
		}
		seen[name] = struct{}{}
		definition, ok := e.Variable(name)
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("termination output assignment references unknown variable %q", name))
		}
		if definition.constant {
			return NewError(ErrorState, fmt.Sprintf("termination output assignment cannot update constant variable %q", name))
		}
		if err := e.validateExprVariables(assignment.Expr); err != nil {
			return err
		}
		if err := validateCoalesceExpressionNodes(assignment.Expr.node()); err != nil {
			return err
		}
		var fields []string
		assignment.Expr.node().referencedFields(&fields)
		if len(fields) > 0 {
			return NewError(ErrorInvalidRule, "termination output assignments can reference variables only")
		}
		if expressionType := assignment.Expr.Type(); definition.typ != nil && definition.typ != typeOf[any]() && expressionType != nil && expressionType != typeOf[any]() {
			if !definition.typ.AssignableTo(expressionType) && !expressionType.AssignableTo(definition.typ) && !numericTypes(definition.typ, expressionType) {
				return NewError(ErrorTypeMismatch, fmt.Sprintf("termination output variable %q has type %s, assignment expression has type %s", name, definition.typ, expressionType))
			}
		}
	}
	return nil
}
