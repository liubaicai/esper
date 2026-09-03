package esper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const planSchemaVersion = "esper-go-plan/v2"

// CompilerVersion identifies the public fluent-plan compiler contract. It is
// intentionally independent from the serialized Plan schema version.
const CompilerVersion = "esper-go-compiler/v1"

const compilerProvider = "github.com/liubaicai/esper.Environment.Build"

// Environment is the compile-time catalog for schemas and future extension
// registrations. It is safe to share for concurrent Plan construction.
type Environment struct {
	mu sync.RWMutex
	// buildMu serializes plan construction for one environment. Expression
	// references are expanded lazily during validation and may add the
	// definition node to the reference AST; keeping that one-time expansion
	// under a build lock preserves the concurrent-plan safety promised by the
	// Environment API without exposing mutable AST state to callers.
	buildMu               sync.Mutex
	schemas               map[string]Schema
	typeToName            map[reflect.Type][]string
	variables             map[string]VariableDefinition
	aggregatePlugins      map[string]aggregatePluginDefinition
	aggregateMultiPlugins map[string]aggregateMultiPluginDefinition
	enumPlugins           map[string]enumPluginDefinition
	dateTimePlugins       map[string]dateTimePluginDefinition
	scripts               map[string]scriptDefinition
	modules               map[string]moduleDefinition
	moduleObjects         map[string]string
	tables                map[string]TableDefinition
	namedWindows          map[string]NamedWindowDefinition
	contexts              map[string]ContextDefinition
	dataflows             map[string]DataflowDefinition
	savedDataflows        map[string]DataflowDefinition
	expressions           map[string]ExpressionDefinition
	extensions            *extensionRegistry
	// decimalMathContext is immutable after construction. The separate flag
	// distinguishes an explicitly configured unlimited context from the
	// unconfigured environment, whose plans intentionally retain the legacy
	// exact aggregate identity and behavior.
	decimalMathContext     DecimalMathContext
	decimalMathContextSet  bool
	environmentOptionError string
}

// EnvironmentOption configures one Environment at construction time.
// Options are applied before the Environment is returned, so the resulting
// catalog can be shared safely without exposing mutable configuration state.
type EnvironmentOption func(*environmentConfig)

type environmentConfig struct {
	decimalMathContext     DecimalMathContext
	decimalMathContextSet  bool
	environmentOptionError string
}

// WithDecimalMathContext configures significant-digit rounding for exact
// decimal AvgExact aggregates evaluated by Engines using the Environment.
// The value is copied when NewEnvironment applies this option. Invalid
// contexts are retained as construction diagnostics and rejected by Build.
func WithDecimalMathContext(context DecimalMathContext) EnvironmentOption {
	return func(config *environmentConfig) {
		config.decimalMathContext = context
		config.decimalMathContextSet = true
		config.environmentOptionError = ""
		if err := context.validate(); err != nil {
			config.environmentOptionError = err.Error()
		}
	}
}

func NewEnvironment(options ...EnvironmentOption) *Environment {
	config := environmentConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	return &Environment{
		schemas:                make(map[string]Schema),
		typeToName:             make(map[reflect.Type][]string),
		variables:              make(map[string]VariableDefinition),
		aggregatePlugins:       make(map[string]aggregatePluginDefinition),
		aggregateMultiPlugins:  make(map[string]aggregateMultiPluginDefinition),
		enumPlugins:            make(map[string]enumPluginDefinition),
		dateTimePlugins:        make(map[string]dateTimePluginDefinition),
		scripts:                make(map[string]scriptDefinition),
		modules:                make(map[string]moduleDefinition),
		moduleObjects:          make(map[string]string),
		tables:                 make(map[string]TableDefinition),
		namedWindows:           make(map[string]NamedWindowDefinition),
		contexts:               make(map[string]ContextDefinition),
		dataflows:              make(map[string]DataflowDefinition),
		savedDataflows:         make(map[string]DataflowDefinition),
		expressions:            make(map[string]ExpressionDefinition),
		extensions:             newExtensionRegistry(),
		decimalMathContext:     config.decimalMathContext,
		decimalMathContextSet:  config.decimalMathContextSet,
		environmentOptionError: config.environmentOptionError,
	}
}

func (e *Environment) decimalMathContextSnapshot() (DecimalMathContext, bool) {
	if e == nil {
		return DecimalMathContext{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.decimalMathContext, e.decimalMathContextSet
}

func (e *Environment) environmentOptionErrorMessage() string {
	if e == nil {
		return ""
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.environmentOptionError
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
		return duplicateModuleObjectError(DeploymentResourceEventType, schema.Name())
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
		typ := schema.GoType()
		names := e.typeToName[typ]
		for _, existing := range names {
			if existing == schema.Name() {
				return nil
			}
		}
		e.typeToName[typ] = append(names, schema.Name())
	}
	return nil
}

// typeNames returns the registered schema names for a Go struct type in
// registration order. A Go type may be registered under several names (for
// example the same bean shape used as both a source and an insert-into
// target); type-based SendEvent is ambiguous in that case and callers must
// use the name-based Engine.Send form.
func (e *Environment) typeNames(typ reflect.Type) []string {
	if e == nil || typ == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]string(nil), e.typeToName[typ]...)
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

// routeTargetSchema resolves the two Esper insert-into target namespaces:
// registered event schemas and named windows. Named windows intentionally do
// not appear in Environment.schemas because they are current-state stores,
// but insert-into statements may target either namespace.
func (e *Environment) routeTargetSchema(moduleName, targetName string) (Schema, bool, bool) {
	if e == nil {
		return Schema{}, false, false
	}
	if definition, ok := e.NamedWindowInModule(moduleName, targetName); ok {
		return definition.schema, true, true
	}
	if schema, ok := e.Schema(strings.TrimSpace(targetName)); ok {
		return schema, false, true
	}
	return Schema{}, false, false
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
		return duplicateModuleObjectError(DeploymentResourceVariable, name)
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
		return duplicateModuleObjectError(DeploymentResourceVariable, name)
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
	sort.Slice(result, func(i, j int) bool {
		return catalogKey(result[i].moduleName, result[i].name) < catalogKey(result[j].moduleName, result[j].name)
	})
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
	sort.Slice(result, func(i, j int) bool {
		return catalogKey(result[i].moduleName, result[i].name) < catalogKey(result[j].moduleName, result[j].name)
	})
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
	schemaVersion   string
	compilerVersion string
	canonical       []byte
	hash            string
	query           Query
	resultSchema    Schema
	indexPlan       IndexPlan
}

// PlanManifest identifies the compiler contract and Go provider that produced
// one immutable Plan. It is the Go counterpart of Esper's EPCompiledManifest;
// generated JVM module-provider class names are represented by the stable
// fluent builder provider instead.
type PlanManifest struct {
	CompilerVersion string
	Provider        string
}

func (p Plan) SchemaVersion() string { return p.schemaVersion }
func (p Plan) Hash() string          { return p.hash }
func (p Plan) Canonical() []byte     { return append([]byte(nil), p.canonical...) }

// Manifest returns compiler provenance for a valid Plan.
func (p Plan) Manifest() PlanManifest {
	if p.schemaVersion == "" || p.compilerVersion == "" || len(p.canonical) == 0 || p.hash == "" {
		return PlanManifest{}
	}
	return PlanManifest{CompilerVersion: p.compilerVersion, Provider: compilerProvider}
}

// IndexPlan returns a detached, deterministic access-path summary. It is a
// Go-native replacement for Java's query-plan hook assertions and does not
// expose implementation-specific JVM backing-table classes.
func (p Plan) IndexPlan() IndexPlan { return p.indexPlan.clone() }
func (p Plan) Query() Query         { return p.query }
func (p Plan) ResultSchema() (Schema, bool) {
	return p.resultSchema, p.resultSchema.valid()
}

// StatementCompileContext describes one typed statement before Build validates
// and canonicalizes it. TypedDescription replaces Java's raw EPL supplier;
// Query and Metadata expose the analyzable fluent source without parsing text.
type StatementCompileContext struct {
	StatementNumber  int
	Query            Query
	TypedDescription string
	StatementName    string
	ModuleName       string
	Metadata         StatementMetadata
}

// StatementNameResolver selects a compile-time statement name. Returning an
// error aborts Build before a Plan is produced.
type StatementNameResolver func(StatementCompileContext) (string, error)

// StatementUserObjectResolver attaches one opaque Go value to the compiled
// statement. The value is application metadata and does not enter Plan
// canonical identity.
type StatementUserObjectResolver func(StatementCompileContext) (any, error)

type compileConfig struct {
	nameResolver       StatementNameResolver
	userObjectResolver StatementUserObjectResolver
}

// CompileOption configures one Environment.Build operation.
type CompileOption func(*compileConfig)

// WithStatementNameResolver resolves the compile-time statement name from a
// typed fluent query context.
func WithStatementNameResolver(resolver StatementNameResolver) CompileOption {
	return func(config *compileConfig) { config.nameResolver = resolver }
}

// WithStatementUserObjectResolver resolves application-owned compile-time
// metadata after any statement-name resolver has run.
func WithStatementUserObjectResolver(resolver StatementUserObjectResolver) CompileOption {
	return func(config *compileConfig) { config.userObjectResolver = resolver }
}

func applyCompileOptions(query Query, options []CompileOption) (Query, error) {
	config := compileConfig{}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	context := StatementCompileContext{
		StatementNumber:  0,
		Query:            query,
		TypedDescription: query.description(),
		StatementName:    query.name,
		ModuleName:       query.moduleName,
		Metadata:         query.Metadata(),
	}
	if config.nameResolver != nil {
		resolved, err := config.nameResolver(context)
		if err != nil {
			return Query{}, WrapError(ErrorInvalidRule, "resolve statement name", err)
		}
		query.name = resolved
		context.Query = query
		context.StatementName = resolved
		context.Metadata = query.Metadata()
	}
	if config.userObjectResolver != nil {
		resolved, err := config.userObjectResolver(context)
		if err != nil {
			return Query{}, WrapError(ErrorInvalidRule, "resolve statement user object", err)
		}
		query.statementUserObject = resolved
	}
	return query, nil
}

func (e *Environment) Build(query Query, options ...CompileOption) (Plan, error) {
	if e == nil {
		return Plan{}, NewError(ErrorInvalidRule, "nil environment")
	}
	if optionError := e.environmentOptionErrorMessage(); optionError != "" {
		return Plan{}, NewError(ErrorInvalidRule, "environment decimal math context: "+optionError)
	}
	if query.env == nil || query.env != e {
		return Plan{}, NewError(ErrorDependency, "query belongs to a different or nil environment")
	}
	if query.input == nil && query.join == nil && !query.sourceLess {
		return Plan{}, NewError(ErrorInvalidRule, "query has no source")
	}
	var err error
	query, err = applyCompileOptions(query, options)
	if err != nil {
		return Plan{}, err
	}
	e.buildMu.Lock()
	defer e.buildMu.Unlock()
	if query.discardPartialsOnMatch || query.suppressOverlappingMatches {
		if query.pattern == nil {
			return Plan{}, NewError(ErrorInvalidRule, "pattern consumption policies require a pattern query")
		}
		if query.contextName != "" || query.join != nil || query.trigger != nil {
			return Plan{}, NewError(ErrorInvalidRule, "pattern consumption policies are not supported with context, joins or actions")
		}
	}
	if query.namedWindowDirect {
		if query.input == nil || query.aggregate != nil || query.join != nil || query.pattern != nil || query.rowRecog != nil || query.trigger != nil || query.contextName != "" || query.tableTarget != "" || query.routeTarget != "" {
			return Plan{}, NewError(ErrorInvalidRule, "direct named-window query requires a plain named-window source")
		}
		base, sourceErr := sourceNode(query.input)
		if sourceErr != nil || base == nil || base.kind != streamNamedWindow {
			return Plan{}, NewError(ErrorInvalidRule, "direct named-window query requires a named-window source")
		}
		if err := e.validateNode(query.input); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "direct named-window query", err)
		}
	} else if query.onDemand != nil {
		if err := e.validateOnDemand(query); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "on-demand", err)
		}
	} else if query.sourceLess {
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
	} else if query.updateStream != nil {
		if err := e.validateUpdateStream(query); err != nil {
			return Plan{}, WrapError(ErrorInvalidRule, "update", err)
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

		if query.joinHaving != nil {
			if query.joinHaving.Type() != typeOf[bool]() {
				return Plan{}, WrapError(ErrorInvalidRule, "join having", NewError(ErrorTypeMismatch, "join having expression must return bool"))
			}
			if err := e.validateJoinScopedExpression(query.join, query.joinHaving, "join having"); err != nil {
				return Plan{}, WrapError(ErrorInvalidRule, "join having", err)
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
		if definition.kind == ContextCategorySegmented && query.join == nil && (query.aggregate == nil || query.aggregate.join == nil) && query.input != nil {
			if err := e.validateCategoryContextEventType(definition, query.input); err != nil {
				return Plan{}, WrapError(ErrorInvalidRule, "context", err)
			}
		}
		if query.output.Termination != OutputNoTermination && definition.kind != ContextInitiatedTerminated && !definition.isTemporal() {
			return Plan{}, NewError(ErrorInvalidRule, "context-termination output requires an initiated or temporal context")
		}
		if query.join != nil {
			sources := joinDefinitionSources(query.join)
			for index, source := range sources {
				err := e.validateContext(definition, source)
				if err == nil {
					continue
				}
				if definition.kind == ContextKeySegmented || definition.kind == ContextHashSegmented {
					// A segmented/hash context partitions only the event
					// types declared in its create-context statement. Join
					// partners of other types legitimately lack the key
					// property: Esper requires only that at least one
					// statement source carries the context's key (the
					// partitioned type appears in the statement), and
					// dispatches events of the other types to every
					// existing partition.
					if contextKeyValidatesOnAnySource(e, definition, sources) {
						continue
					}
				}
				return Plan{}, WrapError(ErrorInvalidRule, fmt.Sprintf("context source %d", index), err)
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
	parameterTypes, parameterErr := queryParameterTypes(e, query)
	if parameterErr != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "parameters", parameterErr)
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
	if query.onDemand != nil && (query.statementPrioritySet || query.statementDrop) {
		return Plan{}, NewError(ErrorInvalidRule, "statement priority/drop applies only to continuous statements")
	}
	if query.updateStream != nil && (query.statementPrioritySet || query.statementDrop) {
		return Plan{}, NewError(ErrorInvalidRule, "update-istream priority/drop must use UpdatePriority and UpdateDrop")
	}
	resultSchema, err := e.resultSchema(query)
	if err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "projection", err)
	}
	if err := e.validateIntoTable(query); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "into-table", err)
	}
	indexPlan, err := e.buildIndexPlan(query)
	if err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "index-plan", err)
	}
	sortIndexSelections(indexPlan.Selections)
	if query.name != "" && strings.TrimSpace(query.name) == "" {
		return Plan{}, fmt.Errorf("esper: statement name cannot be blank")
	}
	query.name = strings.TrimSpace(query.name)
	if err := validateStatementMetadata(query.statementMetadata); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "statement metadata", err)
	}
	if err := e.validateReclaimHintParameters(query.statementMetadata); err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "statement metadata", err)
	}

	description := query.description()
	if query.routeTarget != "" {
		description += " -> route(" + query.routeTarget + ")"
	}
	canonicalParts := []string{planSchemaVersion, CompilerVersion, description}
	parameterNames := make([]string, 0, len(parameterTypes))
	for name := range parameterTypes {
		parameterNames = append(parameterNames, name)
	}
	sort.Strings(parameterNames)
	parameterDescriptions := make([]string, 0, len(parameterNames))
	for _, name := range parameterNames {
		typ := parameterTypes[name]
		typeDescription := "any"
		if typ != nil {
			typeDescription = typ.String()
		}
		parameterDescriptions = append(parameterDescriptions, name+":"+typeDescription)
	}
	canonicalParts = append(canonicalParts, "parameters("+strings.Join(parameterDescriptions, ",")+")")
	if context, configured := e.decimalMathContextSnapshot(); configured {
		canonicalParts = append(canonicalParts, "environment-math-context("+context.description()+")")
	}
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
		defaultNames := make([]string, 0, len(schema.defaults))
		for name := range schema.defaults {
			defaultNames = append(defaultNames, name)
		}
		sort.Strings(defaultNames)
		defaults := make([]string, 0, len(defaultNames))
		for _, name := range defaultNames {
			defaults = append(defaults, fmt.Sprintf("%s:%T:%v", name, schema.defaults[name], schema.defaults[name]))
		}
		canonicalParts = append(canonicalParts, fmt.Sprintf("schema(%s:%d:%d:%d:%t:bus=%t:variant=%d:parents=%s:%s:fields=%s:getters=%s:setters=%s:nested=%s:defaults=%s:annotations=%s)", schema.Name(), schema.kind, schema.resolution, schema.accessor, schema.allowDynamic, schema.busVisible, schema.variantMode, parents, members, strings.Join(fields, ","), strings.Join(getters, ","), strings.Join(setters, ","), strings.Join(nestedNames, ","), strings.Join(defaults, ","), statementAnnotationsCanonical(schema.annotations)))
		if len(schema.jsonAdapters) > 0 {
			adapterNames := make([]string, 0, len(schema.jsonAdapters))
			for name := range schema.jsonAdapters {
				adapterNames = append(adapterNames, name)
			}
			sort.Strings(adapterNames)
			adapters := make([]string, 0, len(adapterNames))
			for _, name := range adapterNames {
				adapters = append(adapters, name+":"+jsonFieldAdapterPlanIdentity(schema.jsonAdapters[name]))
			}
			canonicalParts = append(canonicalParts, fmt.Sprintf("schema-adapters(%s:%s)", schema.Name(), strings.Join(adapters, ",")))
		}
	}
	e.mu.RLock()
	moduleNames := make([]string, 0, len(e.modules))
	moduleDefinitions := make(map[string]moduleDefinition, len(e.modules))
	for name, definition := range e.modules {
		moduleNames = append(moduleNames, name)
		moduleDefinitions[name] = definition
	}
	e.mu.RUnlock()
	sort.Strings(moduleNames)
	for _, name := range moduleNames {
		canonicalParts = append(canonicalParts, fmt.Sprintf("module-definition(%s:%s)", name, moduleDefinitions[name].visibility))
	}
	for _, variable := range e.Variables() {
		canonicalParts = append(canonicalParts, fmt.Sprintf("variable(%s:%s:%s:%s:%t)", variable.name, variable.context, variable.typ, variable.initial.String(), variable.constant))
	}
	e.mu.RLock()
	pluginNames := make([]string, 0, len(e.aggregatePlugins))
	pluginTypes := make(map[string]reflect.Type, len(e.aggregatePlugins))
	pluginFactoryFlags := make(map[string]bool, len(e.aggregatePlugins))
	pluginAccessFlags := make(map[string]bool, len(e.aggregatePlugins))
	for name, plugin := range e.aggregatePlugins {
		pluginNames = append(pluginNames, name)
		pluginTypes[name] = plugin.resultType
		pluginFactoryFlags[name] = plugin.factory != nil
		pluginAccessFlags[name] = plugin.access
	}
	e.mu.RUnlock()
	sort.Strings(pluginNames)
	for _, name := range pluginNames {
		canonicalParts = append(canonicalParts, fmt.Sprintf("aggregate-plugin(%s:%s:factory=%t:access=%t)", name, pluginTypes[name], pluginFactoryFlags[name], pluginAccessFlags[name]))
	}
	e.mu.RLock()
	multiPluginNames := make([]string, 0, len(e.aggregateMultiPlugins))
	multiPluginDefinitions := make(map[string]aggregateMultiPluginDefinition, len(e.aggregateMultiPlugins))
	for name, definition := range e.aggregateMultiPlugins {
		multiPluginNames = append(multiPluginNames, name)
		multiPluginDefinitions[name] = definition
	}
	e.mu.RUnlock()
	sort.Strings(multiPluginNames)
	for _, name := range multiPluginNames {
		canonicalParts = append(canonicalParts, fmt.Sprintf("aggregate-multi-plugin(%s:%s)", name, aggregateMultiPluginMethodsCanonical(multiPluginDefinitions[name].methods)))
	}
	e.mu.RLock()
	enumPluginNames := make([]string, 0, len(e.enumPlugins))
	enumPluginDefinitions := make(map[string]enumPluginDefinition, len(e.enumPlugins))
	for name, definition := range e.enumPlugins {
		enumPluginNames = append(enumPluginNames, name)
		enumPluginDefinitions[name] = definition
	}
	e.mu.RUnlock()
	sort.Strings(enumPluginNames)
	for _, name := range enumPluginNames {
		definition := enumPluginDefinitions[name]
		canonicalParts = append(canonicalParts, fmt.Sprintf("enum-plugin(%s:%s:%s)", name, definition.resultType, enumPluginFootprintsCanonical(definition.footprints)))
	}
	e.mu.RLock()
	dateTimePluginNames := make([]string, 0, len(e.dateTimePlugins))
	dateTimePluginDefinitions := make(map[string]dateTimePluginDefinition, len(e.dateTimePlugins))
	for name, definition := range e.dateTimePlugins {
		dateTimePluginNames = append(dateTimePluginNames, name)
		dateTimePluginDefinitions[name] = definition
	}
	e.mu.RUnlock()
	sort.Strings(dateTimePluginNames)
	for _, name := range dateTimePluginNames {
		definition := dateTimePluginDefinitions[name]
		canonicalParts = append(canonicalParts, fmt.Sprintf("datetime-plugin(%s:%s)", name, dateTimePluginFootprintsCanonical(definition.footprints)))
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
			columnDescription := fmt.Sprintf("%s:%s:%t:%t", column.Name, column.Type, column.Optional, column.PrimaryKey)
			if column.Nested.valid() {
				columnDescription += ":nested=" + column.Nested.Name()
			}
			columns = append(columns, columnDescription)
		}
		indexes := make([]string, 0, len(table.indexes))
		for _, index := range table.indexes {
			indexes = append(indexes, fmt.Sprintf("%s:%s:%t:%s", index.Name, strings.Join(index.Columns, ","), index.Unique, index.Kind))
		}
		canonicalParts = append(canonicalParts, "table("+catalogKey(table.moduleName, table.name)+":"+strings.Join(columns, ",")+":"+strings.Join(indexes, ",")+")")
	}
	for _, window := range e.NamedWindows() {
		fields := make([]string, 0, len(window.schema.fields))
		for _, field := range window.schema.fields {
			fields = append(fields, fmt.Sprintf("%s:%s:%t:%t:%t", field.Name, field.Type, field.Optional, field.StartTimestamp, field.EndTimestamp))
		}
		indexes := make([]string, 0, len(window.indexes))
		for _, index := range window.indexes {
			indexes = append(indexes, fmt.Sprintf("%s:%s:%t:%s", index.Name, strings.Join(index.Columns, ","), index.Unique, index.Kind))
		}
		canonicalParts = append(canonicalParts, "named-window("+catalogKey(window.moduleName, window.name)+":"+window.schema.Name()+":"+window.retention.description()+":context="+window.contextName+":subquery-index-sharing="+fmt.Sprint(window.subqueryIndexSharing)+":"+strings.Join(fields, ",")+":indexes="+strings.Join(indexes, ",")+")")
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
		schemaVersion:   planSchemaVersion,
		compilerVersion: CompilerVersion,
		canonical:       canonical,
		hash:            hex.EncodeToString(digest[:]),
		query:           query,
		resultSchema:    resultSchema,
		indexPlan:       indexPlan.clone(),
	}, nil
}

func (e *Environment) validateReclaimHintParameters(metadata statementMetadata) error {
	if e == nil {
		return nil
	}
	for _, hint := range metadata.hints {
		switch hint.kind {
		case HintReclaimGroupAged, HintReclaimGroupFreq:
			for _, parameter := range hint.parameters {
				if value, err := strconv.ParseFloat(parameter, 64); err == nil {
					if value <= 0 {
						return NewError(ErrorInvalidRule, fmt.Sprintf("reclaim hint parameter %q must be a positive seconds value", parameter))
					}
					continue
				}
				if definition, ok := e.Variable(parameter); ok && isNumericType(definition.Type()) {
					continue
				}
				return NewError(ErrorInvalidRule, fmt.Sprintf("reclaim hint parameter %q must be a numeric seconds value or a numeric variable name", parameter))
			}
		}
	}
	return nil
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
	window, ok := e.NamedWindowInModule(definition.moduleName, definition.table)
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

// expressionFieldNames collects the inner-event field names an expression
// reads, recursing into subqueries. Used by grouped subselect validation to
// check that non-aggregate columns derive from the group-by key.
func expressionFieldNames(node *exprNode) map[string]struct{} {
	names := map[string]struct{}{}
	var walk func(node *exprNode)
	walk = func(node *exprNode) {
		if node == nil {
			return
		}
		if node.kind == "field" {
			names[node.fieldName] = struct{}{}
		}
		for _, child := range node.children {
			walk(child)
		}
		if node.subquery != nil {
			if node.subquery.predicate != nil {
				walk(node.subquery.predicate.node())
			}
			if node.subquery.projection != nil {
				walk(node.subquery.projection.node())
			}
			for _, selection := range node.subquery.columns {
				if selection.Expr != nil {
					walk(selection.Expr.node())
				}
			}
			if node.subquery.groupBy != nil {
				walk(node.subquery.groupBy.node())
			}
			if node.subquery.having != nil {
				walk(node.subquery.having.node())
			}
		}
	}
	walk(node)
	return names
}

func (e *Environment) validateRoute(query Query) error {
	if query.routeTarget == "" {
		return nil
	}
	target, _, ok := e.routeTargetSchema(query.moduleName, query.routeTarget)
	if !ok {
		return NewError(ErrorUnknownName, fmt.Sprintf("route target %q is not registered", query.routeTarget))
	}
	// On-trigger select statements (Esper's "on T insert into Target select ...
	// from Source" stream form) may route their projection into a stream;
	// mutation triggers and source-less queries cannot.
	if query.sourceLess || (query.trigger != nil && query.trigger.action != triggerSelectTable) {
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
	// Transpose routing applies to a plain record projection (no aggregate,
	// join, pattern, match-recognize or trigger select). The transpose
	// function must occur alone in the select clause unless the companion
	// columns coexist inside a Wrapper-type target, which the Go API models
	// as a pre-registered Map target accepting the payload, merged by
	// property name. These errors are validated before the per-selection
	// field loop so transpose selections (which carry no column name) are
	// not mistaken for named projections.
	if query.aggregate == nil && query.join == nil && query.pattern == nil && query.rowRecog == nil && query.trigger == nil && len(query.selections) > 0 {
		transposeCount := 0
		transposeIndex := -1
		for index, selection := range query.selections {
			if isTransposeExpression(selection.Expr) {
				transposeCount++
				transposeIndex = index
			}
		}
		switch {
		case transposeCount >= 2:
			return fmt.Errorf("a column name must be supplied for all but one stream if multiple streams are selected via the stream.* notation")
		case transposeCount == 1:
			transpose := query.selections[transposeIndex]
			if err := e.validateTransposeExpression(target, transpose.Expr, len(query.selections) > 1); err != nil {
				return err
			}
			if len(query.selections) > 1 {
				// Additional properties are routed only into a Map target
				// whose container properties are untyped, mirroring Esper's
				// auto-created Wrapper/Pair acceptance of named columns
				// alongside a transpose payload.
				if target.kind != SchemaMap || target.goType != nil {
					return fmt.Errorf("cannot transpose additional properties in the select-clause to target event type %q with underlying type %s, the transpose function must occur alone in the select clause", target.Name(), transposeUnderlyingTypeName(target))
				}
			}
		}
	}
	for index, selection := range selections {
		if isTransposeExpression(selection.Expr) {
			continue
		}
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
		// The runtime route assignment dereferences pointer fields and boxes
		// values into them, so the shared field/expression compatibility rule
		// applies here rather than bare assignability. A pointer-typed
		// expression (nullable boxed value) routes into a value field as the
		// dereferenced value with null producing the field zero value, so the
		// expression element type is also tested.
		exprType := selection.Expr.Type()
		if exprType != nil && exprType.Kind() == reflect.Pointer {
			elem := exprType.Elem()
			if fieldExpressionTypesCompatible(field.Type, elem) || field.Type == elem {
				continue
			}
		}
		// A single-valued expression routed into a slice-typed column wraps
		// into a length-1 array at runtime, mirroring Java's
		// SelectExprProcessorHelper insert-into wrap of an aggregate value
		// (for example maxby(...) as sbarr) into an array-typed target
		// column. An []Event column accepts any single value: the runtime
		// materializes the element as a fragment carrying the source
		// representation. Other slice elements require the expression type
		// to be compatible with the declared element type.
		// Anonymous-struct member check (Java's typable-new population): a
		// projected new{...} struct routed into a declared member column must
		// satisfy the column's nested schema field-for-field, otherwise the
		// mismatched member or unknown member is rejected at compile time
		// (SelectExprInsertEventBeanFactory / SelectExprProcessorHelper).
		if node := selection.Expr.node(); node != nil && node.kind == "struct" {
			if nested, ok := target.nested[field.Name]; ok && len(nested.fields) > 0 {
				members := structMembersFromDescription(node.description)
				for index, member := range members {
					name := member.name
					child := (*exprNode)(nil)
					if index < len(node.children) {
						child = node.children[index]
					}
					nestedField, memberExists := nested.Field(name)
					if !memberExists {
						return fmt.Errorf("Failed to find property %q among properties for target event type %q", name, nested.Name())
					}
					if child != nil && nestedField.Type != nil && child.typ != nil &&
						!fieldExpressionTypesCompatible(nestedField.Type, child.typ) {
						return fmt.Errorf("Invalid assignment of column '%s' of type '%s' to event property '%s' typed as '%s', column and parameter types mismatch",
							name, goTypeName(child.typ), name, goTypeName(nestedField.Type))
					}
				}
			}
		}
		// Member identity check: a column declared with a nested event schema
		// only accepts projections of that same representation (Java's
		// "Incompatible type detected attempting to insert into column"
		// diagnostic; SelectExprProcessorHelper). The subquery AST carries the
		// inner source stream node; its registered schema name is the selected
		// member type.
		if nested, ok := target.nested[field.Name]; ok && nested.Name() != "" {
			node := selection.Expr.node()
			if node == nil || node.subquery == nil {
				continue
			}
			// Named-column forms are validated per member at population time;
			// only the whole-event `(select * ...)` form pins the selected
			// representation to the declared member schema (Java compares the
			// SELECTED TYPE, which for select * — projected either as the
			// raw inner event or via an event() wrapper — is the inner event
			// type itself).
			projectionIsWholeEvent := node.subquery.projection != nil &&
				node.subquery.projection.node() != nil &&
				node.subquery.projection.node().kind == "event-value"
			isWholeEvent := node.subquery.wholeEvent ||
				(len(node.subquery.columns) == 0 && projectionIsWholeEvent)
			if !isWholeEvent {
				continue
			}
			sourceName := ""
			if node.subquery != nil && node.subquery.source != nil {
				sourceNode, err := sourceNode(node.subquery.source)
				if err == nil {
					if schema, schemaErr := e.sourceSchema(sourceNode); schemaErr == nil {
						sourceName = schema.Name()
					}
				}
			}
			if sourceName != "" && sourceName != nested.Name() {
				return fmt.Errorf("Incompatible type detected attempting to insert into column '%s' type '%s' compared to selected type '%s'",
					field.Name, nested.Name(), sourceName)
			}
		}
		if field.Type != nil && field.Type.Kind() == reflect.Slice &&
			exprType != nil && exprType.Kind() != reflect.Slice {
			elem := field.Type.Elem()
			exprElem := exprType
			if exprElem.Kind() == reflect.Pointer {
				exprElem = exprElem.Elem()
			}
			if elem == typeOf[Event]() || fieldExpressionTypesCompatible(elem, exprElem) {
				continue
			}
		}
		// A projected event fragment routes into any typed member column of
		// a Map target when the projection carries its own representation
		// (new{}-shaped case results; Java's typable-new insert population).
		if exprType == typeOf[Event]() && target.goType == nil && field.Type != typeOf[any]() {
			continue
		}
		if !fieldExpressionTypesCompatible(field.Type, exprType) {
			return fmt.Errorf("route projection %q has type %s, target expects %s", selection.Name, exprType, field.Type)
		}
	}
	return nil
}

// goTypeName renders a reflect.Type using the kind vocabulary Java's typable-
// new diagnostics use (String/Integer/Long/...), so pinned texts stay stable.
func goTypeName(typ reflect.Type) string {
	if typ == nil {
		return "null"
	}
	switch typ.Kind() {
	case reflect.String:
		return "String"
	case reflect.Int:
		return "Integer"
	case reflect.Int64:
		return "Long"
	case reflect.Float64, reflect.Float32:
		return "Double"
	case reflect.Bool:
		return "Boolean"
	default:
		return typ.String()
	}
}

// structMemberName is one new{...} member parsed from the struct expression
// description: the quoted name precedes '='.
type structMemberName struct {
	name string
}

// structMembersFromDescription extracts declared member names from a struct
// expression description of the form struct{"a"=..., "b"=...}.
func structMembersFromDescription(description string) []structMemberName {
	open := strings.Index(description, `struct{"`)
	if open < 0 {
		return nil
	}
	body := description[open+len(`struct{"`):]
	var members []structMemberName
	for len(body) > 0 {
		close := strings.IndexByte(body, '"')
		if close < 0 {
			break
		}
		members = append(members, structMemberName{name: body[:close]})
		next := strings.Index(body[close:], `,`)
		if next < 0 {
			break
		}
		body = body[close+next+1:]
	}
	return members
}

// isTransposeExpression reports whether a selection is an insert-into
// transpose projection. The runtime detects transpose purely from the
// expression kind, so Build and the projection path share this single
// predicate and a plan carries no additional transpose state.
func isTransposeExpression(expression Expr) bool {
	return expression != nil && expression.node() != nil && expression.node().kind == "transpose"
}

// transposeUnderlyingTypeName describes the routed target's underlying
// representation for transpose error messages. It mirrors the Java
// insertIntoTargetType.getUnderlyingEPType().getTypeName() spelling: an
// Avro-backed target reports *esper.AvroRecord, a struct-backed target
// reports its Go type, an object-array target reports []any, a JSON target
// reports string, and a Map target reports map[string]any.
func transposeUnderlyingTypeName(target Schema) string {
	if target.kind == SchemaAvro {
		return reflect.PointerTo(typeOf[AvroRecord]()).String()
	}
	if target.goType != nil {
		return target.goType.String()
	}
	switch target.kind {
	case SchemaObjectArray:
		return typeOf[[]any]().String()
	case SchemaJSON:
		return typeOf[string]().String()
	default:
		return typeOf[map[string]any]().String()
	}
}

// validateTransposeExpression freezes the transpose payload/target coercion
// matrix at Build time. The transpose child expression is evaluated per event
// and its dynamic value must be convertible to the target event's underlying
// representation, mirroring SelectExprProcessorHelper's native-expression
// coercion branches:
//
//   - struct/bean target: the payload must be assignable to the target Go
//     type, directly or through a pointer element (CoerceNative);
//   - Avro target: a map or *AvroRecord payload is field-coerced to the target
//     schema (CoerceAvro; Go models the generic GenericData.Record as a map);
//   - object-array target: an []any payload is coerced directly (CoerceOA);
//   - JSON target: a string payload is parsed against the JSON schema
//     (CoerceJson);
//   - Map target: an untyped map payload is retained (CoerceMap);
//   - anything else fails the expression-returned-value message, including a
//     struct payload routed to a Map or Avro target other than the Wrapper
//     props merge below.
//
// When the transpose coexists with named columns (withAdditionalProperties),
// the target must be the Go model of an auto-created Esper Wrapper/Pair: a
// Map target whose properties merge with the payload by name, so any payload
// representation is accepted.
//
// A null payload (`transpose(null)`) is rejected like Java's null-type
// message; a Transpose with no children is a single-parameter error which the
// Go single-argument signature can only express as Transpose(nil).
func (e *Environment) validateTransposeExpression(target Schema, expression Expr, withAdditionalProperties bool) error {
	node := expression.node()
	if node == nil {
		return fmt.Errorf("esper: transpose function requires a single parameter expression")
	}
	if len(node.children) != 1 {
		return fmt.Errorf("esper: transpose function requires a single parameter expression")
	}
	child := node.children[0]
	if child != nil && child.kind == "null" {
		return fmt.Errorf("cannot transpose a null-type value")
	}
	if withAdditionalProperties {
		return nil
	}
	payloadType := node.children[0].typ
	if payloadType != nil && payloadType.Kind() == reflect.Pointer {
		payloadType = payloadType.Elem()
	}
	targetType := target.goType
	if targetType != nil && targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	switch target.kind {
	case SchemaStruct:
		if payloadType != nil && targetType != nil && (payloadType.AssignableTo(targetType) || payloadType.AssignableTo(reflect.PointerTo(targetType))) {
			return nil
		}
	case SchemaAvro:
		if payloadType == typeOf[map[string]any]() {
			return nil
		}
		if payloadType != nil && (payloadType.AssignableTo(typeOf[AvroRecord]()) || payloadType.AssignableTo(reflect.PointerTo(typeOf[AvroRecord]()))) {
			return nil
		}
	case SchemaObjectArray:
		if payloadType == typeOf[[]any]() {
			return nil
		}
	case SchemaJSON:
		if payloadType == typeOf[string]() {
			return nil
		}
	default:
		if payloadType == nil || payloadType == typeOf[any]() || payloadType == typeOf[map[string]any]() {
			return nil
		}
	}
	return fmt.Errorf("expression-returned value of type %s cannot be converted to target event type %q with underlying type %s", expressionTypeName(payloadType), target.Name(), transposeUnderlyingTypeName(target))
}

func expressionTypeName(typ reflect.Type) string {
	if typ == nil {
		return "<null>"
	}
	return typ.String()
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
			if definition.end != nil {
				if definition.end.Type() != typeOf[bool]() {
					return NewError(ErrorInvalidRule, "initiated-terminated context requires a bool end expression")
				}
				if err := e.validateContextLifecycleExpression(definition.end); err != nil {
					return err
				}
			}
			return nil
		}
		keys := definition.contextKeysForNode(node)
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

// validateCategoryContextEventType mirrors Esper's category-context stream
// restriction: a flat statement must consume at least one of the event types
// referenced by the category declarations. Typed fluent predicates carry
// their source Go type through Field[T,V], allowing mismatched but
// field-compatible event types to be rejected before deployment.
func (e *Environment) validateCategoryContextEventType(definition ContextDefinition, node *streamNode) error {
	if e == nil || definition.kind != ContextCategorySegmented || node == nil {
		return nil
	}
	source, err := sourceNode(node)
	if err != nil {
		return nil
	}
	schema, err := e.sourceSchema(source)
	if err != nil {
		return nil
	}
	statementType := schema.GoType()
	if statementType == nil || statementType == typeOf[any]() {
		return nil
	}
	categoryTypes := make(map[reflect.Type]struct{})
	for _, category := range definition.categories {
		collectExpressionFieldSourceTypes(category.predicate.node(), categoryTypes)
	}
	if len(categoryTypes) == 0 {
		return nil
	}
	for categoryType := range categoryTypes {
		if categoryType == statementType || categoryType.AssignableTo(statementType) || statementType.AssignableTo(categoryType) {
			return nil
		}
	}
	return fmt.Errorf("category context %q requires that any of the event types that are listed in the category context also appear in any of the filter expressions of the statement", definition.name)
}

func collectExpressionFieldSourceTypes(node *exprNode, types map[reflect.Type]struct{}) {
	if node == nil {
		return
	}
	if node.kind == "field" && node.fieldSourceType != nil && node.fieldSourceType != typeOf[any]() {
		types[node.fieldSourceType] = struct{}{}
	}
	for _, child := range node.children {
		collectExpressionFieldSourceTypes(child, types)
	}
	if node.expressionBody != nil {
		collectExpressionFieldSourceTypes(node.expressionBody, types)
	}
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
	if query.offset < 0 {
		return fmt.Errorf("offset cannot be negative")
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
		if expressionNodeContainsSubquery(key.Expr.node()) {
			return NewError(ErrorInvalidRule, fmt.Sprintf("order-by key %d: subselects not allowed within order-by clause", index))
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

// namedWindowConsumerSource unwraps filter nodes and reports the named window
// name when the chain consumes a named window. Esper rejects data window views
// declared onto a named window by consuming statements.
func namedWindowConsumerSource(node *streamNode) (string, bool) {
	for node != nil && node.kind == streamFilter {
		node = node.input
	}
	if node != nil && node.kind == streamNamedWindow {
		return node.sourceName, true
	}
	return "", false
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
		if !ok && node.isAlias {
			if aliasSchema, aliasOk := e.schemaForGoType(node.sourceType); aliasOk {
				schema, ok = aliasSchema, true
			}
		}
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
		definition, ok := e.NamedWindowInModule(node.moduleName, node.sourceName)
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
		definition, ok := e.TableInModule(node.moduleName, node.sourceName)
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
		if node.method.evaluateOnce {
			if len(node.method.dependencies) > 0 {
				return NewError(ErrorInvalidRule, fmt.Sprintf("method source %q EvaluateOnce cannot be combined with dependencies", node.sourceName))
			}
			if node.method.trigger != "" {
				return NewError(ErrorInvalidRule, fmt.Sprintf("method source %q EvaluateOnce cannot be combined with a trigger type", node.sourceName))
			}
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
		if _, grouped := node.window.(GroupWindowSpec); !grouped {
			if source, ok := namedWindowConsumerSource(node.input); ok {
				// Esper rejects data window views on named-window consumers:
				// "Consuming statements to a named window cannot declare a data
				// window view onto the named window". A grouped (groupwin) view is
				// exempt because it is not a data window view.
				return NewError(ErrorInvalidRule, fmt.Sprintf("consuming statements to named window %q cannot declare a data window view onto the named window", source))
			}
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

// contextKeysForNode returns the partition key expressions for the source
// node's event type. A multi-stream segmented context validates each
// declared type's keys against statements that reference that type; the
// contextKeyValidatesOnAnySource wrapper tolerates other join sources.
func (d ContextDefinition) contextKeysForNode(node *streamNode) []Expr {
	if len(d.streamKeys) == 0 || node == nil {
		return d.contextKeys()
	}
	source, err := sourceNode(node)
	if err != nil {
		return nil
	}
	return d.contextKeysForType(source.sourceName)
}

func (d ContextDefinition) contextKeysForType(typeName string) []Expr {
	if len(d.streamKeys) == 0 {
		return d.contextKeys()
	}
	return d.streamKeys[typeName]
}

// contextKeyValidatesOnAnySource reports whether a segmented or hash
// context's key expressions resolve against at least one statement source.
// Esper requires the partitioned event types to appear among the statement's
// filters; other sources (join partners, pattern streams) may be of types
// not listed in the context and legitimately lack the key property.
func contextKeyValidatesOnAnySource(e *Environment, definition ContextDefinition, sources []*streamNode) bool {
	if e == nil || definition.kind != ContextKeySegmented && definition.kind != ContextHashSegmented {
		return false
	}
	for _, source := range sources {
		if err := e.validateContext(definition, source); err == nil {
			return true
		}
	}
	return false
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
			if !fieldExpressionTypesCompatible(field.Type, expressionType) {
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
			if !fieldExpressionTypesCompatible(field.Type, expressionType) {
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
	if err := e.validateSubqueryIndexOptions(definition, base); err != nil {
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
	if subqueryDefinitionContainsSubquery(definition) {
		return NewError(ErrorInvalidRule, "subquery-within-subquery is not supported")
	}
	if definition.predicate != nil {
		if definition.predicate.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "subquery predicate must return bool")
		}
		if expressionNodeContainsAggregate(definition.predicate.node()) {
			return NewError(ErrorInvalidRule, "aggregation functions are not supported within subquery filters, consider a having clause or insert-into instead")
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
	if definition.having != nil && !definition.grouped {
		// Esper also accepts a non-aggregated having without group-by: it
		// filters subquery rows one by one after the where clause.
		if definition.having.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "subquery having predicate must return bool")
		}
		if err := e.validateExprFields(definition.source, definition.having); err != nil {
			return WrapError(ErrorInvalidRule, "subquery having", err)
		}
	}
	if definition.multiColumn && len(definition.columns) == 0 && !definition.wholeEvent {
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
		if expressionContainsPreviousAccess(definition.groupBy.node()) {
			return NewError(ErrorInvalidRule, "subquery group-by key cannot use previous or prior access")
		}
		if definition.groupedRowProjection && len(definition.columns) > 0 {
			keyFields := expressionFieldNames(definition.groupBy.node())
			for _, selection := range definition.columns {
				if isAggregateExpression(selection.Expr) {
					continue
				}
				// Java allows non-aggregate columns that are functions of
				// the group key (e.g. theString||'x', intPrimitive*1000) in
				// grouped multi-column subselects: every inner field the
				// column reads must be a group-by key field.
				columnFields := expressionFieldNames(selection.Expr.node())
				for name := range columnFields {
					if _, ok := keyFields[name]; !ok {
						return NewError(ErrorInvalidRule, fmt.Sprintf("subquery column %q must be an aggregate or derive from the group-by key", selection.Name))
					}
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

func (e *Environment) validateSubqueryIndexOptions(definition *subqueryDefinition, base *streamNode) error {
	if definition == nil {
		return nil
	}
	if definition.noIndex && definition.indexName != "" {
		return NewError(ErrorInvalidRule, "subquery cannot combine no-index and an explicit index")
	}
	if definition.indexName == "" {
		return nil
	}
	if base == nil || (base.kind != streamNamedWindow && base.kind != streamTable) {
		return NewError(ErrorInvalidRule, "subquery index options require a named-window or table root source")
	}
	if definition.source != base {
		return NewError(ErrorInvalidRule, "subquery index options require an unwrapped root source")
	}
	if strings.HasPrefix(definition.indexName, "<") {
		return NewError(ErrorInvalidRule, fmt.Sprintf("subquery index %q is an internal access path", definition.indexName))
	}
	predicates := indexPredicatesFromExpression(definition.predicate)
	if len(predicates) == 0 {
		return NewError(ErrorInvalidRule, fmt.Sprintf("subquery index %q requires an analyzable equality, IN, or range predicate", definition.indexName))
	}
	selection, err := chooseIndexSelection(e, base, 0, predicates, &indexHint{name: definition.indexName})
	if err != nil {
		return err
	}
	if selection.IndexName != definition.indexName {
		return NewError(ErrorInvalidRule, fmt.Sprintf("subquery index %q cannot satisfy the predicate", definition.indexName))
	}
	return nil
}

// expressionContainsPreviousAccess identifies the view-resource navigation
// family (prev, prior and their window/count/tail/dynamic variants). Esper
// does not allow these expressions as grouped-subquery keys because the key
// would depend on per-row view state rather than the subquery source alone.
func expressionContainsPreviousAccess(node *exprNode) bool {
	if node == nil {
		return false
	}
	if strings.HasPrefix(node.kind, "prev") || strings.HasPrefix(node.kind, "prior") {
		return true
	}
	for _, child := range node.children {
		if expressionContainsPreviousAccess(child) {
			return true
		}
	}
	if node.subquery != nil {
		if node.subquery.predicate != nil && expressionContainsPreviousAccess(node.subquery.predicate.node()) {
			return true
		}
		if node.subquery.projection != nil && expressionContainsPreviousAccess(node.subquery.projection.node()) {
			return true
		}
		for _, selection := range node.subquery.columns {
			if selection.Expr != nil && expressionContainsPreviousAccess(selection.Expr.node()) {
				return true
			}
		}
		if node.subquery.groupBy != nil && expressionContainsPreviousAccess(node.subquery.groupBy.node()) {
			return true
		}
		if node.subquery.having != nil && expressionContainsPreviousAccess(node.subquery.having.node()) {
			return true
		}
		for _, order := range node.subquery.orderBy {
			if order.Expression != nil && expressionContainsPreviousAccess(order.Expression.node()) {
				return true
			}
		}
	}
	return false
}

// subqueryDefinitionContainsSubquery reports whether the subquery's scalar
// projection nests another subquery, which Esper rejects as
// subquery-within-subquery. Go intentionally keeps supporting subqueries in
// filter predicates (SubqueryExists scopes) and multi-column SubqueryRow
// fragments, so those shapes are not rejected here.
func subqueryDefinitionContainsSubquery(definition *subqueryDefinition) bool {
	if definition == nil {
		return false
	}
	if definition.projection != nil {
		return expressionNodeContainsSubquery(definition.projection.node())
	}
	return false
}

func expressionNodeContainsSubquery(node *exprNode) bool {
	if node == nil {
		return false
	}
	if node.subquery != nil {
		return true
	}
	for _, child := range node.children {
		if expressionNodeContainsSubquery(child) {
			return true
		}
	}
	return false
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
			if !fieldExpressionTypesCompatible(field.Type, expressionType) {
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
		definition, ok := e.NamedWindowInModule(source.moduleName, source.sourceName)
		if !ok {
			return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", source.sourceName))
		}
		return definition.schema, nil
	}
	if source.kind == streamTable {
		definition, ok := e.TableInModule(source.moduleName, source.sourceName)
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
		if source.kind == streamSource && source.isAlias {
			if aliasSchema, aliasOk := e.schemaForGoType(source.sourceType); aliasOk {
				return aliasSchema, nil
			}
		}
		return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("source %q has no registered schema", source.sourceName))
	}
	return schema, nil
}

func (e *Environment) schemaForGoType(typ reflect.Type) (Schema, bool) {
	if e == nil || typ == nil {
		return Schema{}, false
	}
	names := e.typeNames(typ)
	for _, name := range names {
		schema, ok := e.schemas[name]
		if ok && schema.GoType() == typ {
			return schema, true
		}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, schema := range e.schemas {
		goType := schema.GoType()
		if goType != nil && goType != typ && (typ.AssignableTo(goType) || goType.AssignableTo(typ)) {
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
	if err := e.validateEnumPluginNodes(expression.node()); err != nil {
		return err
	}
	if err := e.validateDateTimePluginNodes(expression.node()); err != nil {
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
	if err := collectSQLHistoricalParameterTypes(query.input, parameterTypes); err != nil {
		return nil, err
	}
	if query.aggregate != nil {
		if err := collectSQLHistoricalParameterTypes(query.aggregate.input, parameterTypes); err != nil {
			return nil, err
		}
	}
	if query.join != nil {
		for _, source := range joinDefinitionSources(query.join) {
			if err := collectSQLHistoricalParameterTypes(source, parameterTypes); err != nil {
				return nil, err
			}
		}
	}
	if strings.TrimSpace(query.contextName) != "" && environment != nil {
		if definition, ok := environment.Context(query.contextName); ok {
			definitionCopy := definition
			if err := visitContextDefinitionExpressions(&definitionCopy, visit, make(map[*ContextDefinition]struct{})); err != nil {
				return nil, err
			}
		}
	}
	if err := validateQueryParameterModes(parameterTypes); err != nil {
		return nil, err
	}
	return parameterTypes, nil
}

func collectSQLHistoricalParameterTypes(node *streamNode, parameterTypes map[string]reflect.Type) error {
	for current := node; current != nil; current = current.input {
		if current.kind != streamHistorical || current.historical == nil || current.historical.provider == nil {
			continue
		}
		provider, ok := current.historical.provider.(*SQLHistoricalProvider)
		if !ok {
			continue
		}
		for name, typ := range provider.parameterTypes {
			if previous, exists := parameterTypes[name]; exists {
				if !parameterTypesCompatible(previous, typ) {
					return fmt.Errorf("parameter %q has incompatible type assignment between %s and %s", name, parameterTypeDescription(previous), parameterTypeDescription(typ))
				}
				if previous == typeOf[any]() && typ != typeOf[any]() {
					parameterTypes[name] = typ
				}
				continue
			}
			parameterTypes[name] = typ
		}
	}
	return nil
}

// validateQueryParameterModes keeps the two Java substitution-parameter
// forms explicit in the Go API. A plan may use named parameters or positional
// parameters, but a single query cannot mix the two forms. Positional indexes
// are 1-based and contiguous, while repeated references to one index are
// allowed and must retain one compatible type.
func validateQueryParameterModes(parameterTypes map[string]reflect.Type) error {
	hasNamed := false
	positions := make(map[int]struct{})
	for name := range parameterTypes {
		if position, positional := positionalParameterPosition(name); positional {
			positions[position] = struct{}{}
			continue
		}
		hasNamed = true
	}
	if hasNamed && len(positions) > 0 {
		return fmt.Errorf("inconsistent use of substitution parameters, use either all named or all positional parameters")
	}
	if len(positions) == 0 {
		return nil
	}
	for position := 1; position <= len(positions); position++ {
		if _, exists := positions[position]; !exists {
			return fmt.Errorf("positional substitution parameters must be contiguous starting at index 1 (missing index %d)", position)
		}
	}
	return nil
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
	if query.updateStream != nil {
		for _, assignment := range query.updateStream.assignments {
			if err := visit(assignment.Expr); err != nil {
				return err
			}
			if err := visit(assignment.Index); err != nil {
				return err
			}
		}
		if err := visit(query.updateStream.where); err != nil {
			return err
		}
	}
	if query.onDemand != nil {
		if err := visit(query.onDemand.predicate); err != nil {
			return err
		}
		for _, assignment := range query.onDemand.assignments {
			if err := visit(assignment.Expr); err != nil {
				return err
			}
			if err := visit(assignment.Index); err != nil {
				return err
			}
		}
		for _, row := range query.onDemand.rows {
			for _, expression := range row.values {
				if err := visit(expression); err != nil {
					return err
				}
			}
		}
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
		if err := visit(query.joinHaving); err != nil {
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
		if err := visit(query.patternWhere); err != nil {
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
			if err := visit(assignment.Index); err != nil {
				return err
			}
		}
		for _, expression := range query.trigger.groupByKeys {
			if err := visit(expression); err != nil {
				return err
			}
		}
		for _, assignment := range query.trigger.variableAssignments {
			if err := visit(assignment.Expr); err != nil {
				return err
			}
			if assignment.Index != nil {
				if err := visit(assignment.Index); err != nil {
					return err
				}
			}
		}
		if err := visit(query.trigger.eventExpression); err != nil {
			return err
		}
		for _, clause := range query.trigger.merge {
			for _, action := range tableMergeClauseActions(clause) {
				if err := visit(action.Condition); err != nil {
					return err
				}
				for _, assignment := range action.Assignments {
					if err := visit(assignment.Expr); err != nil {
						return err
					}
					if err := visit(assignment.Index); err != nil {
						return err
					}
				}
				for _, selection := range action.InsertSelections {
					if err := visit(selection.Expr); err != nil {
						return err
					}
				}
			}
		}
		if err := visitSelectionsExpressions(query.trigger.selections, visit); err != nil {
			return err
		}
		for _, branch := range query.trigger.splitBranches {
			if err := visitStreamNodeExpressions(branch.source, visit); err != nil {
				return err
			}
			if err := visit(branch.Condition); err != nil {
				return err
			}
			if err := visitSelectionsExpressions(branch.Selections, visit); err != nil {
				return err
			}
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
	case TimeWindowSpec:
		return visit(value.Expr)
	case ExpressionWindowSpec:
		return visit(value.Keep)
	case ExpressionBatchWindowSpec:
		return visit(value.Trigger)
	case GroupWindowSpec:
		for _, key := range value.effectiveKeys() {
			if err := visit(key); err != nil {
				return err
			}
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
			if err := visitStreamNodeExpressions(node.subquery.source, func(expression Expr) error {
				return collectExpressionParameterTypes(expression, parameterTypes)
			}); err != nil {
				return err
			}
			if err := collectSQLHistoricalParameterTypes(node.subquery.source, parameterTypes); err != nil {
				return err
			}
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

// fieldExpressionTypesCompatible reports whether a schema field of fieldType
// can back an expression of expressionType. The schema dereferences pointer
// fields at runtime (a *int field yields int or Null), so a pointer element
// type is also tested against the expression type.
func fieldExpressionTypesCompatible(fieldType, expressionType reflect.Type) bool {
	if fieldType == nil || expressionType == nil || fieldType == typeOf[any]() {
		return true
	}
	if fieldType.AssignableTo(expressionType) || expressionType.AssignableTo(fieldType) {
		return true
	}
	if numericTypes(fieldType, expressionType) {
		return true
	}
	if fieldType.Kind() == reflect.Pointer {
		elem := fieldType.Elem()
		if elem.AssignableTo(expressionType) || expressionType.AssignableTo(elem) || numericTypes(elem, expressionType) {
			return true
		}
	}
	return false
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
	if query.trigger != nil && query.trigger.action == triggerSplitStream {
		if query.input == nil {
			return Schema{}, NewError(ErrorInvalidRule, "split-stream source is required")
		}
		source, err := sourceNode(query.input)
		if err != nil {
			return Schema{}, err
		}
		return e.sourceSchema(source)
	}
	if query.updateStream != nil {
		if query.input == nil {
			return Schema{}, NewError(ErrorInvalidRule, "update target stream is required")
		}
		source, err := sourceNode(query.input)
		if err != nil {
			return Schema{}, err
		}
		return e.sourceSchema(source)
	}
	if query.onDemand != nil {
		if query.input == nil {
			return Schema{}, NewError(ErrorInvalidRule, "on-demand target is required")
		}
		source, err := sourceNode(query.input)
		if err != nil {
			return Schema{}, err
		}
		return e.sourceSchema(source)
	}
	if query.trigger != nil && query.trigger.target == triggerTargetNamedWindow && query.trigger.action == triggerSelectTable {
		window, ok := e.NamedWindowInModule(query.trigger.moduleName, query.trigger.table)
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
		return newProjectionResultSchema("result:"+query.name, fields, query.selections)
	}
	if query.trigger != nil && query.trigger.target == triggerTargetNamedWindow && query.trigger.action != triggerSetVariables {
		window, ok := e.NamedWindowInModule(query.trigger.moduleName, query.trigger.table)
		if !ok {
			return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("trigger references unknown named window %q", query.trigger.table))
		}
		return window.schema, nil
	}
	if query.trigger != nil && query.trigger.action == triggerSetVariables {
		fields := make([]FieldSpec, 0, len(query.trigger.variableAssignments))
		seen := make(map[string]struct{}, len(query.trigger.variableAssignments))
		for _, assignment := range query.trigger.variableAssignments {
			if assignment.Index != nil || assignment.Apply != nil {
				// Java's array-element and call-form writes contribute no
				// output columns; the written value stays observable through
				// variable reads.
				continue
			}
			if _, exists := seen[assignment.Name]; exists {
				continue
			}
			seen[assignment.Name] = struct{}{}
			definition, ok := e.Variable(assignment.Name)
			if !ok {
				return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("variable %q is not registered", assignment.Name))
			}
			fields = append(fields, FieldSpec{Name: assignment.Name, Type: definition.Type()})
		}
		return NewSchema("result:"+query.name, fields...)
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
		return newProjectionResultSchema("result:"+query.name, fields, query.selections)
	}
	if query.trigger != nil && query.trigger.action != triggerSetVariables {
		table, ok := e.TableInModule(query.trigger.moduleName, query.trigger.table)
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
		return newProjectionResultSchema("result:"+query.name, fields, query.selections)
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
		return newProjectionResultSchema("result:"+query.name, fields, query.patternSelections)
	}
	if query.pattern != nil {
		if len(query.patternSelections) == 0 && len(patternDefinitionTagNames(query.pattern)) > 0 {
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
		return newProjectionResultSchema("result:"+query.name, fields, query.patternSelections)
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
		return newProjectionResultSchema("result:"+query.name, fields, query.aggregate.selections)
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
		projectionSelections := make([]Selection, 0, len(query.joinSelections))
		for _, selection := range query.joinSelections {
			projectionSelections = append(projectionSelections, Selection{Name: selection.Name, Expr: selection.Expr})
		}
		return newProjectionResultSchema("result:"+query.name, fields, projectionSelections)
	}
	if query.routeTarget != "" && query.aggregate == nil && query.join == nil && query.pattern == nil && query.rowRecog == nil && query.trigger == nil && len(query.selections) > 0 {
		hasTranspose := false
		for _, selection := range query.selections {
			if isTransposeExpression(selection.Expr) {
				hasTranspose = true
				break
			}
		}
		if hasTranspose {
			// A transpose route's projection is the routed target event itself,
			// so the result schema is the target schema. The transpose payload
			// expression and any companion properties still bind their fields
			// against the source, so validate them here as the projection path
			// would, then expose the target schema to the listener.
			for _, selection := range query.selections {
				if err := e.validateExprFields(query.input, selection.Expr); err != nil {
					return Schema{}, err
				}
			}
			target, _, ok := e.routeTargetSchema(query.moduleName, query.routeTarget)
			if !ok {
				return Schema{}, NewError(ErrorUnknownName, fmt.Sprintf("route target %q is not registered", query.routeTarget))
			}
			return target, nil
		}
	}
	if err := validateTransposeSelectShape(query.selections); err != nil {
		return Schema{}, err
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
	return newProjectionResultSchema("result:"+query.name, fields, query.selections)
}

// validateTransposeSelectShape rejects a top-level transpose selection whose
// payload cannot be statically typed, mirroring Java's select-clause
// validation in SelectExprProcessorHelper: transpose(null) produces the
// null-type message and a transpose with an incomplete parameter list
// produces the single-parameter message. Non-top-level transpose expressions
// (for example inside an `is null` predicate in a where or select clause)
// remain valid and are not inspected here; the insert-into route also
// validates payload/target coercion separately in validateTransposeExpression.
func validateTransposeSelectShape(selections []Selection) error {
	for _, selection := range selections {
		if !isTransposeExpression(selection.Expr) {
			continue
		}
		node := selection.Expr.node()
		if node == nil || len(node.children) != 1 {
			return fmt.Errorf("esper: transpose function requires a single parameter expression")
		}
		child := node.children[0]
		if child != nil && child.kind == "null" {
			return fmt.Errorf("cannot transpose a null-type value")
		}
	}
	return nil
}

func newProjectionResultSchema(name string, fields []FieldSpec, selections []Selection) (Schema, error) {
	options, err := subquerySchemaOptions(selections)
	if err != nil {
		return Schema{}, err
	}
	return NewSchemaWithOptions(name, fields, options...)
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

// updateStreamTargetsNamedWindow reports whether an update-istream statement
// targets a named window. Window-targeted updates attach to the window insert
// path and never observe the plain stream dispatch.
func updateStreamTargetsNamedWindow(query Query) bool {
	if query.updateStream == nil {
		return false
	}
	source, err := sourceNode(query.input)
	return err == nil && source != nil && source.kind == streamNamedWindow
}

// validateUpdateStream validates an update-istream statement: the target must
// be a plain (optionally filtered) event stream or named window without data
// windows, the set assignments reference declared properties with
// non-aggregate expressions, and the optional where clause is a boolean
// expression. Mirroring Esper's InternalEventRouter preprocessing, update
// statements do not carry their own projection, output policy or route
// target. Named-window targets attach to the window insert path like Esper's
// update strategy: every event offered to the window is preprocessed
// copy-on-write before the window and its subscribers observe it.
func (e *Environment) validateUpdateStream(query Query) error {
	definition := query.updateStream
	if query.input == nil {
		return NewError(ErrorInvalidRule, "update target stream is required")
	}
	for node := query.input; node != nil; node = node.input {
		if node.kind == streamWindow {
			return NewError(ErrorInvalidRule, "update target stream cannot declare a data window")
		}
		if node.kind != streamFilter {
			break
		}
	}
	source, err := sourceNode(query.input)
	if err != nil {
		return err
	}
	if source.kind != streamSource && source.kind != streamNamedWindow {
		return NewError(ErrorInvalidRule, "update target must be a plain event stream or named window (table, method and historical targets are not yet supported)")
	}
	if err := e.validateNode(query.input); err != nil {
		return err
	}
	if query.contextName != "" {
		return NewError(ErrorInvalidRule, "update with a statement context is not yet supported")
	}
	if query.routeTarget != "" || query.tableTarget != "" {
		return NewError(ErrorInvalidRule, "update statements cannot declare insert-into or into-table routes")
	}
	if len(query.selections) > 0 {
		return NewError(ErrorInvalidRule, "update statements do not support projections")
	}
	if len(definition.assignments) == 0 {
		return NewError(ErrorInvalidRule, "update requires at least one assignment")
	}
	schema, err := e.sourceSchema(source)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(definition.assignments))
	for _, assignment := range definition.assignments {
		if assignment.Wildcard {
			return NewError(ErrorInvalidRule, "update assignments do not support wildcard field copies")
		}
		if strings.TrimSpace(assignment.Column) == "" {
			return NewError(ErrorInvalidRule, "update assignment column is required")
		}
		if assignment.Expr == nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("update assignment %q requires an expression", assignment.Column))
		}
		if assignment.Index != nil && assignment.Key != nil {
			return NewError(ErrorInvalidRule, fmt.Sprintf("update assignment %q cannot combine an array index and a map key", assignment.Column))
		}
		field, canonicalName, lookupErr := schema.lookupField(assignment.Column)
		if lookupErr != nil {
			return WrapError(ErrorUnknownName, "update assignment", lookupErr)
		}
		// The duplicate rule keys on the full assignment target so distinct
		// array indexes or map keys on one property are allowed, matching
		// Esper's property-level write identities (arr[0] vs arr[1]).
		dupKey := canonicalName
		if assignment.Index != nil {
			dupKey = canonicalName + "[" + assignment.Index.Description() + "]"
		}
		if assignment.Key != nil {
			dupKey = canonicalName + "(" + assignment.Key.Description() + ")"
		}
		if _, exists := seen[dupKey]; exists {
			return NewError(ErrorInvalidRule, fmt.Sprintf("update assignment duplicates property %q", dupKey))
		}
		seen[dupKey] = struct{}{}
		if expressionNodeContainsAggregate(assignment.Expr.node()) {
			return NewError(ErrorInvalidRule, "aggregation functions are not supported within update-set expressions")
		}
		if expressionContainsPreviousAccess(assignment.Expr.node()) {
			return NewError(ErrorInvalidRule, "update-set expressions cannot use previous or prior access")
		}
		if err := e.validateExprFields(query.input, assignment.Expr); err != nil {
			return WrapError(ErrorInvalidRule, fmt.Sprintf("update assignment %q", assignment.Column), err)
		}
		if assignment.Index == nil && assignment.Key == nil {
			if err := validateUpdateSetAssignable(assignment.Column, field, assignment.Expr); err != nil {
				return err
			}
		}
		if assignment.Index != nil {
			if err := e.validateUpdateStreamArrayElement(query.input, assignment, field); err != nil {
				return err
			}
		}
		if assignment.Key != nil {
			if err := e.validateUpdateStreamMapEntry(query.input, assignment, field); err != nil {
				return err
			}
		}
	}
	if definition.where != nil {
		if definition.where.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "update where expression must return bool")
		}
		if expressionNodeContainsAggregate(definition.where.node()) {
			return NewError(ErrorInvalidRule, "aggregation functions are not supported within update where clauses")
		}
		if expressionContainsPreviousAccess(definition.where.node()) {
			return NewError(ErrorInvalidRule, "update where clauses cannot use previous or prior access")
		}
		if err := e.validateExprFields(query.input, definition.where); err != nil {
			return WrapError(ErrorInvalidRule, "update where", err)
		}
	}
	return nil
}

// validateUpdateSetAssignable checks a plain update-set assignment for type
// compatibility, mirroring Esper's write-access rules: the expression type
// must be assignable to the property type or widen numerically towards it
// (Esper rejects narrowing assignments such as Long to int), and a null
// literal cannot be assigned to a non-nullable property (Esper rejects the
// statement at compile time; expression-produced nulls still skip at
// runtime). Untyped (any) expressions and targets pass through, as do nested
// index/key assignments which have their own element checks.
func validateUpdateSetAssignable(column string, field FieldSpec, expression Expr) error {
	if node := expression.node(); node != nil && node.kind == "null" {
		if field.Type != nil && !nullableUpdateFieldKind(field.Type) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("update assignment of null to property %q typed as %s has a nullable type mismatch", column, field.Type))
		}
		return nil
	}
	actual := expression.Type()
	target := field.Type
	if actual == nil || target == nil || actual == typeOf[any]() || target == typeOf[any]() {
		return nil
	}
	if actual.AssignableTo(target) {
		return nil
	}
	base := target
	for base.Kind() == reflect.Pointer {
		base = base.Elem()
	}
	actualBase := actual
	for actualBase.Kind() == reflect.Pointer {
		actualBase = actualBase.Elem()
	}
	if actualBase.AssignableTo(base) {
		return nil
	}
	if numericTypes(actualBase, base) && updateSetWidensTo(actualBase, base) {
		return nil
	}
	return NewError(ErrorTypeMismatch, fmt.Sprintf("update assignment expression type %s is incompatible with property %q typed as %s, column and parameter types mismatch", actual, column, target))
}

// updateSetWidensTo reports whether a numeric value of type from widens to
// type to without narrowing, mirroring Esper's update-set assignment widener:
// int to long and int to double are accepted while long to int is rejected.
// Go's int maps to Esper's 32-bit int rank so int64 to int is a narrowing
// rejection even though both are 64-bit wide on this platform.
func updateSetWidensTo(from, to reflect.Type) bool {
	rank := func(typ reflect.Type) (int, bool) {
		switch typ.Kind() {
		case reflect.Int8, reflect.Uint8:
			return 1, true
		case reflect.Int16, reflect.Uint16:
			return 2, true
		case reflect.Int32, reflect.Uint32, reflect.Int:
			return 3, true
		case reflect.Int64, reflect.Uint, reflect.Uint64:
			return 4, true
		case reflect.Float32:
			return 5, true
		case reflect.Float64:
			return 6, true
		}
		return 0, false
	}
	fromRank, ok := rank(from)
	if !ok {
		return false
	}
	toRank, ok := rank(to)
	if !ok {
		return false
	}
	return fromRank <= toRank
}

// validateUpdateStreamArrayElement validates an update-istream array-element
// assignment (column[index] = value): the index expression must reference
// valid stream fields and return an integer, the target property must be an
// array or slice, and the value expression must be assignable to the element
// type. Mirrors Esper's write-access checks for indexed update targets.
func (e *Environment) validateUpdateStreamArrayElement(input *streamNode, assignment TableAssignment, field FieldSpec) error {
	if expressionNodeContainsAggregate(assignment.Index.node()) {
		return NewError(ErrorInvalidRule, "aggregation functions are not supported within update array index expressions")
	}
	if expressionNodeContainsSubquery(assignment.Index.node()) {
		return NewError(ErrorInvalidRule, "subqueries within update array index expressions are not yet supported")
	}
	if expressionContainsPreviousAccess(assignment.Index.node()) {
		return NewError(ErrorInvalidRule, "update array index expressions cannot use previous or prior access")
	}
	if err := e.validateExprFields(input, assignment.Index); err != nil {
		return WrapError(ErrorInvalidRule, fmt.Sprintf("update array index for %q", assignment.Column), err)
	}
	indexType := assignment.Index.Type()
	if indexType == nil || !isTriggerIntegerType(indexType) {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("update array index expression for property '%s' must return an integer", assignment.Column))
	}
	arrayType := field.Type
	for arrayType != nil && arrayType.Kind() == reflect.Pointer {
		arrayType = arrayType.Elem()
	}
	if arrayType == nil || (arrayType.Kind() != reflect.Array && arrayType.Kind() != reflect.Slice) {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("update assignment target property '%s' is not an array", assignment.Column))
	}
	if err := validateTriggerAssignmentType(arrayType.Elem(), assignment.Expr); err != nil {
		return WrapError(ErrorTypeMismatch, fmt.Sprintf("update array assignment %q", assignment.Column), err)
	}
	return nil
}

// validateUpdateStreamMapEntry validates an update-istream map-entry
// assignment (column('key') = value): the key expression must reference valid
// stream fields and return a string, the target property must be a map with
// string keys, and the value expression must be assignable to the map element
// type. Mirrors Esper's write-access checks for mapped update targets.
func (e *Environment) validateUpdateStreamMapEntry(input *streamNode, assignment TableAssignment, field FieldSpec) error {
	if expressionNodeContainsAggregate(assignment.Key.node()) {
		return NewError(ErrorInvalidRule, "aggregation functions are not supported within update map key expressions")
	}
	if expressionNodeContainsSubquery(assignment.Key.node()) {
		return NewError(ErrorInvalidRule, "subqueries within update map key expressions are not yet supported")
	}
	if expressionContainsPreviousAccess(assignment.Key.node()) {
		return NewError(ErrorInvalidRule, "update map key expressions cannot use previous or prior access")
	}
	if err := e.validateExprFields(input, assignment.Key); err != nil {
		return WrapError(ErrorInvalidRule, fmt.Sprintf("update map key for %q", assignment.Column), err)
	}
	keyType := assignment.Key.Type()
	if keyType == nil || keyType.Kind() != reflect.String {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("update map key expression for property '%s' must return a string", assignment.Column))
	}
	mapType := field.Type
	for mapType != nil && mapType.Kind() == reflect.Pointer {
		mapType = mapType.Elem()
	}
	if mapType == nil || mapType.Kind() != reflect.Map {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("update assignment target property '%s' is not a map", assignment.Column))
	}
	if mapType.Key().Kind() != reflect.String {
		return NewError(ErrorTypeMismatch, fmt.Sprintf("update assignment target property '%s' map keys are not strings", assignment.Column))
	}
	if err := validateTriggerAssignmentType(mapType.Elem(), assignment.Expr); err != nil {
		return WrapError(ErrorTypeMismatch, fmt.Sprintf("update map assignment %q", assignment.Column), err)
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
		// Esper rejects subselects within group-by. Go keeps a documented
		// extension for join aggregates, where a correlated subquery group
		// key carries the join tuple scope (JoinField correlation), so the
		// rejection applies to plain single-source aggregates only.
		if definition.join == nil && expressionNodeContainsSubquery(key.node()) {
			return NewError(ErrorInvalidRule, "subselects not allowed within group-by")
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
		if err := e.validateAggregateMultiPluginNodes(selection.Expr.node()); err != nil {
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
		if err := e.validateAggregateMultiPluginNodes(definition.having.node()); err != nil {
			return err
		}
		if err := validateFields(definition.input, definition.having); err != nil {
			return err
		}
		if err := e.validateAggregateHavingContainment(definition); err != nil {
			return err
		}
	}
	return nil
}

// validateAggregateHavingContainment mirrors Java's
// ResultSetProcessorFactoryFactory.validateHaving: for grouped aggregates,
// every non-aggregated property referenced by the HAVING clause must occur
// in the group-by clause. Aggregate input properties are exempt (they are
// the "aggregated properties"); group keys are exempt by definition.
// The diagnostic spells the first offending reference in Java's format.
func (e *Environment) validateAggregateHavingContainment(definition *aggregateDefinition) error {
	// Java calls validateHaving only when the statement is aggregated
	// (aggregates in the select, having, or order-by) and the group-by
	// property set is non-empty. A grouped select whose having carries no
	// aggregate (having intPrimitive > 5 over group by theString) takes the
	// ResultSetProcessorAggregateGrouped path with no containment rule, so
	// the plain-property having remains legal there.
	if len(definition.groupBy) == 0 || definition.having == nil {
		return nil
	}
	aggregated := expressionNodeContainsAggregate(definition.having.node())
	for _, selection := range definition.selections {
		aggregated = aggregated || expressionNodeContainsAggregate(selection.Expr.node())
	}
	if !aggregated {
		return nil
	}
	// Collect the group-by key fields: plain field references keep their
	// name; any other key form is treated as an opaque expression.
	groupKeyFields := make(map[string]struct{})
	for _, key := range definition.groupBy {
		if key == nil || key.node() == nil {
			continue
		}
		var keyFields []string
		key.node().referencedLocalFields(&keyFields)
		for _, name := range keyFields {
			groupKeyFields[name] = struct{}{}
		}
	}
	// Aggregate input properties are the properties consumed inside
	// aggregate functions; they are not "non-aggregated".
	aggregatedFields := make(map[string]struct{})
	collectAggregateInputFields(definition.having.node(), aggregatedFields)
	// Every remaining plain field reference must be a group key.
	var havingFields []string
	definition.having.node().referencedLocalFields(&havingFields)
	for _, name := range havingFields {
		if _, aggregated := aggregatedFields[name]; aggregated {
			continue
		}
		if _, grouped := groupKeyFields[name]; grouped {
			continue
		}
		return NewError(ErrorInvalidRule, "Non-aggregated property '"+name+"' in the HAVING clause must occur in the group-by clause")
	}
	return nil
}

// collectAggregateInputFields walks the expression tree and records plain
// field references that sit inside an aggregate node's arguments. Aggregate
// boundaries stop the descent into their children (their inputs are the
// aggregated properties); nested aggregate detection mirrors
// expressionNodeIsAggregate.
func collectAggregateInputFields(node *exprNode, aggregated map[string]struct{}) {
	if node == nil {
		return
	}
	if expressionNodeIsAggregate(node) {
		for _, child := range node.children {
			var fields []string
			child.referencedLocalFields(&fields)
			for _, name := range fields {
				aggregated[name] = struct{}{}
			}
		}
		return
	}
	for _, child := range node.children {
		collectAggregateInputFields(child, aggregated)
	}
}

func (e *Environment) validateJoinAggregateFields(definition *joinDefinition, expression Expr) error {
	return e.validateJoinScopedExpression(definition, expression, "join aggregate")
}

// validateFireAndForgetNoPrevious rejects Previous/Prior access in
// fire-and-forget queries: they require a retained data window that on-demand
// execution does not provide, mirroring Java's compile rejection.
func validateFireAndForgetNoPrevious(query Query) error {
	for _, selection := range query.selections {
		if expressionContainsPrevious(selection.Expr) {
			return NewError(ErrorInvalidRule, "Previous function cannot be used in this context")
		}
	}
	for _, selection := range query.joinSelections {
		if expressionContainsPrevious(selection.Expr) {
			return NewError(ErrorInvalidRule, "Previous function cannot be used in this context")
		}
	}
	for _, key := range query.orderBy {
		if expressionContainsPrevious(key.Expr) {
			return NewError(ErrorInvalidRule, "Previous function cannot be used in this context")
		}
	}
	if query.joinWhere != nil && expressionContainsPrevious(query.joinWhere) {
		return NewError(ErrorInvalidRule, "Previous function cannot be used in this context")
	}
	if query.joinHaving != nil && expressionContainsPrevious(query.joinHaving) {
		return NewError(ErrorInvalidRule, "Previous function cannot be used in this context")
	}
	// Stream-node expressions (filters, windows, pattern steps) and aggregate
	// clauses are FAF-reachable surfaces too.
	prevErr := NewError(ErrorInvalidRule, "Previous function cannot be used in this context")
	visit := func(expr Expr) error {
		if expressionContainsPrevious(expr) {
			return prevErr
		}
		return nil
	}
	if err := visitStreamNodeExpressions(query.input, visit); err != nil {
		return err
	}
	if query.aggregate != nil {
		for _, selection := range query.aggregate.selections {
			if expressionContainsPrevious(selection.Expr) {
				return prevErr
			}
		}
		if query.aggregate.where != nil && expressionContainsPrevious(query.aggregate.where) {
			return prevErr
		}
		if query.aggregate.having != nil && expressionContainsPrevious(query.aggregate.having) {
			return prevErr
		}
		for _, key := range query.aggregate.groupBy {
			if expressionContainsPrevious(key) {
				return prevErr
			}
		}
	}
	return nil
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
	moduleName, tableName := splitCatalogKey(query.tableTarget)
	definition, ok := e.TableInModule(moduleName, tableName)
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
		// Stage one mirrors Java's per-expression select-clause validation:
		// some aggregate forms are rejected outright inside into-table
		// statements, independent of the target column.
		info, aggregate := intoTableAggregateInfo(selection.Expr)
		if aggregate {
			if err := validateIntoTableExpressionForm(info); err != nil {
				return err
			}
		}
		column, exists := columnByName[selection.Name]
		if !exists {
			// An into-table aggregate may expose ordinary projections to its
			// statement listener while contributing only the aliases that are
			// actual table columns. This is the fluent equivalent of Java's
			// select c0, sum(value) as total into table ... form.
			continue
		}
		selected[selection.Name] = struct{}{}
		if column.Type != nil && column.Type != typeOf[any]() && selection.Expr.Type() != nil &&
			!column.Type.AssignableTo(selection.Expr.Type()) && !selection.Expr.Type().AssignableTo(column.Type) && !numericTypes(column.Type, selection.Expr.Type()) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("into-table column %q expects %s, aggregate returns %s", selection.Name, column.Type, selection.Expr.Type()))
		}
		// Stage two mirrors Java's table-column compatibility check between
		// the declared column aggregation and the provided aggregate.
		if column.Agg != nil && aggregate {
			_, tableName := splitCatalogKey(query.tableTarget)
			if err := validateIntoTableCompatible(tableName, column, info, intoTableProvidedBound(query)); err != nil {
				return err
			}
		}
	}
	for _, column := range columns {
		if column.PrimaryKey {
			if _, exists := selected[column.Name]; exists {
				continue
			}
			// A grouped aggregate contributes its group key to a table primary
			// key even when the key is not repeated in the selection list. The
			// runtime derives the key from the corresponding group-by expression.
			groupBy := append([]Expr(nil), query.aggregate.groupBy...)
			if len(groupBy) == 0 {
				groupBy = implicitAggregateGroupBy(query.aggregate.input)
			}
			primaryKey := definition.PrimaryKey()
			columnIndex := -1
			for index, name := range primaryKey {
				if name == column.Name {
					columnIndex = index
					break
				}
			}
			if columnIndex < 0 || columnIndex >= len(groupBy) {
				return NewError(ErrorInvalidRule, fmt.Sprintf("into-table projection must provide primary-key column %q or a matching group-by key", column.Name))
			}
			continue
		}
		if !column.Optional {
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

// intoTableAggInfo is the Java-visible classification of a provided
// into-table aggregate expression. Field names follow the pinned Esper
// diagnostics that the validation reproduces.
type intoTableAggInfo struct {
	kind     string // engine expression kind, e.g. "max", "last-ever"
	javaName string // canonical function name used by Java diagnostics
	render   string // Java-visible rendering, e.g. "max(intPrimitive)"
	nullArg  bool   // the only argument is a null literal
	zeroKey  bool   // by-ever form without an explicit sort key
}

var intoTableEngineNames = map[string]string{
	"max": "max", "min": "min",
	"max-ever": "maxever", "min-ever": "minever",
	"first-ever": "firstever", "last-ever": "lastever",
	"window": "window", "sorted": "sorted",
	"first": "first", "last": "last",
	"min-by": "minby", "max-by": "maxby",
	"min-by-ever": "minbyever", "max-by-ever": "maxbyever",
	"sum": "sum", "sum-exact": "sum", "avg": "avg", "avg-exact": "avg",
	"count": "count", "count-distinct": "count",
	"median": "median", "stddev": "stddev",
}

func intoTableAggregateInfo(expr Expr) (intoTableAggInfo, bool) {
	node := expr.node()
	if node == nil {
		return intoTableAggInfo{}, false
	}
	javaName, ok := intoTableEngineNames[node.kind]
	if !ok {
		return intoTableAggInfo{}, false
	}
	info := intoTableAggInfo{kind: node.kind, javaName: javaName}
	args := make([]string, 0, len(node.children))
	for index, child := range node.children {
		if child == nil {
			continue
		}
		switch {
		case child.kind == "event-value":
			args = append(args, "*")
		case child.kind == "literal" && child.literalValue == nil:
			args = append(args, "null")
			if index == 0 && len(node.children) == 1 {
				info.nullArg = true
			}
		default:
			args = append(args, child.description)
		}
	}
	info.zeroKey = (node.kind == "min-by-ever" || node.kind == "max-by-ever") && len(node.children) == 1
	if info.zeroKey || (node.kind == "window" && len(node.children) == 0) {
		info.render = javaName + "()"
		if node.kind == "window" {
			info.render = "window(*)"
		}
		return info, true
	}
	// By-style aggregations carry an implicit underlying-event argument;
	// Java renders only the explicit sort key.
	if len(args) > 0 && args[0] == "*" &&
		(node.kind == "min-by" || node.kind == "max-by" || node.kind == "min-by-ever" || node.kind == "max-by-ever") {
		args = args[1:]
	}
	info.render = javaName + "(" + strings.Join(args, ",") + ")"
	return info, true
}

// validateIntoTableExpressionForm rejects aggregate forms Java refuses
// outright inside into-table statements. Messages mirror the pinned suite.
func validateIntoTableExpressionForm(info intoTableAggInfo) error {
	var message string
	switch {
	case info.kind == "first" || info.kind == "last":
		message = "For into-table use 'window(*)' or 'window(stream.*)' instead"
	case info.nullArg:
		message = "Null-type is not allowed"
	case info.kind == "min-by" || info.kind == "max-by":
		message = "When specifying into-table a sort expression cannot be provided"
	default:
		return nil
	}
	return NewError(ErrorInvalidRule, fmt.Sprintf("Failed to validate select-clause expression '%s': %s", info.render, message))
}

func intoTableProvidedBound(query Query) bool {
	if query.aggregate == nil {
		return false
	}
	if query.aggregate.input != nil {
		return query.aggregate.input.window != nil
	}
	if query.aggregate.join != nil {
		return true
	}
	return false
}

// intoTableKindBound reports whether a provided aggregate of this kind can
// ever count as data-window bound. Ever-style aggregations always report
// unbound, mirroring Java's hasDataWindows=false for -ever nodes.
func intoTableKindBound(info intoTableAggInfo, providedBound bool) bool {
	switch info.kind {
	case "max-ever", "min-ever", "first-ever", "last-ever", "min-by-ever", "max-by-ever":
		return false
	}
	return providedBound
}

func intoTableDeclClass(name string) string {
	switch name {
	case "max", "min", "maxever", "minever":
		return "minmax"
	case "firstever", "lastever":
		return "firstlast-ever"
	case "window":
		return "linear"
	case "sorted", "minby", "maxby", "minbyever", "maxbyever":
		return "sorted-minmaxby"
	default:
		return "decl:" + name
	}
}

func intoTableKindClass(kind string) string {
	switch kind {
	case "max", "min", "max-ever", "min-ever":
		return "minmax"
	case "first-ever", "last-ever":
		return "firstlast-ever"
	case "window":
		return "linear"
	case "sorted", "min-by", "max-by", "min-by-ever", "max-by-ever":
		return "sorted-minmaxby"
	default:
		return "decl:" + intoTableEngineNames[kind]
	}
}

// validateIntoTableCompatible mirrors Java's AggregationServiceFactoryFactory
// into-table check: class equality first, then family-specific rules, with
// the wrapped "Incompatible aggregation function" diagnostic on failure.
func validateIntoTableCompatible(tableName string, column TableColumn, info intoTableAggInfo, providedStreamBound bool) error {
	declared := column.Agg
	if declared == nil {
		return nil
	}
	if intoTableKindClass(info.kind) != intoTableDeclClass(declared.Name) {
		sub := fmt.Sprintf("The table declares '%s' and provided is '%s'", declared.Description, info.render)
		return intoTableIncompatible(tableName, column.Name, declared.Description, info.render, sub)
	}
	switch intoTableDeclClass(declared.Name) {
	case "minmax":
		declaredMax := declared.Name == "max" || declared.Name == "maxever"
		providedMax := info.kind == "max" || info.kind == "max-ever"
		if declaredMax != providedMax {
			declaredText := "min"
			providedText := "min"
			if declaredMax {
				declaredText = "max"
			}
			if providedMax {
				providedText = "max"
			}
			sub := fmt.Sprintf("The aggregation declares %s and provided is %s", declaredText, providedText)
			return intoTableIncompatible(tableName, column.Name, declared.Description, info.render, sub)
		}
		providedBound := intoTableKindBound(info, providedStreamBound)
		if declared.Bound != providedBound {
			sub := "The table declares "
			if declared.Bound {
				sub += "use with data windows"
			} else {
				sub += "unbound"
			}
			sub += " and provided is "
			if providedBound {
				sub += "use with data windows"
			} else {
				sub += "unbound"
			}
			return intoTableIncompatible(tableName, column.Name, declared.Description, info.render, sub)
		}
	case "firstlast-ever":
		declaredFirst := declared.Name == "firstever"
		providedFirst := info.kind == "first-ever"
		if declaredFirst != providedFirst {
			declaredText, providedText := "lastever", "lastever"
			if declaredFirst {
				declaredText = "firstever"
			}
			if providedFirst {
				providedText = "firstever"
			}
			sub := fmt.Sprintf("The aggregation declares %s and provided is %s", declaredText, providedText)
			return intoTableIncompatible(tableName, column.Name, declared.Description, info.render, sub)
		}
	case "sorted-minmaxby":
		if declared.Name != info.javaName {
			sub := fmt.Sprintf("The required aggregation function name is '%s' and provided is '%s'", declared.Name, info.javaName)
			return intoTableIncompatible(tableName, column.Name, declared.Description, info.render, sub)
		}
	}
	return nil
}

func intoTableIncompatible(tableName, columnName, declaredRender, providedRender, sub string) error {
	return NewError(ErrorInvalidRule, fmt.Sprintf("Incompatible aggregation function for table '%s' column '%s', expecting '%s' and received '%s': %s", tableName, columnName, declaredRender, providedRender, sub))
}

func (e *Environment) validateOnDemand(query Query) error {
	if query.onDemand == nil {
		return NewError(ErrorInvalidRule, "on-demand definition is required")
	}
	if query.input == nil {
		return NewError(ErrorInvalidRule, "on-demand target is required")
	}
	if query.join != nil || query.aggregate != nil || query.pattern != nil || query.rowRecog != nil || query.trigger != nil || query.sourceLess {
		return NewError(ErrorInvalidRule, "on-demand mutation cannot combine with another query operator")
	}
	if query.routeTarget != "" || query.tableTarget != "" {
		return NewError(ErrorInvalidRule, "on-demand mutation cannot route or materialize into another target")
	}
	if query.input.kind != streamNamedWindow && query.input.kind != streamTable {
		return NewError(ErrorInvalidRule, "on-demand target must be a root named window or table")
	}
	if query.contextName != "" && query.input.kind == streamNamedWindow {
		window, ok := e.NamedWindowInModule(query.input.moduleName, query.input.sourceName)
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("named window %q is not registered", query.input.sourceName))
		}
		if window.Context() != query.contextName {
			return NewError(ErrorInvalidRule, fmt.Sprintf("named window %q was declared with context %q; on-demand mutation uses context %q", query.input.sourceName, window.Context(), query.contextName))
		}
	}
	targetSchema, err := e.sourceSchema(query.input)
	if err != nil {
		return err
	}
	targetKind := "table-field"
	if query.input.kind == streamNamedWindow {
		targetKind = "named-window-field"
	}
	switch query.onDemand.action {
	case onDemandInsert:
		if len(query.onDemand.rows) > 0 {
			if len(query.onDemand.assignments) != 0 {
				return NewError(ErrorInvalidRule, "on-demand insert cannot combine positional rows with assignments")
			}
			const maxRows = 1000
			if len(query.onDemand.rows) > maxRows {
				return fmt.Errorf("on-demand insert number of rows exceeds the maximum of %d rows as the query provides %d rows", maxRows, len(query.onDemand.rows))
			}
			for rowIndex, row := range query.onDemand.rows {
				if len(row.values) != len(targetSchema.fields) {
					return fmt.Errorf("failed to validate multi-row insert at row %d of %d: number of supplied values %d does not match target column count %d", rowIndex+1, len(query.onDemand.rows), len(row.values), len(targetSchema.fields))
				}
				for columnIndex, expression := range row.values {
					if expression == nil {
						return fmt.Errorf("failed to validate multi-row insert at row %d of %d: value %d is nil", rowIndex+1, len(query.onDemand.rows), columnIndex+1)
					}
					assignment := SetColumn(targetSchema.fields[columnIndex].Name, expression)
					if err := validateTriggerAssignment(e, query.input, targetSchema, assignment, ""); err != nil {
						return fmt.Errorf("failed to validate multi-row insert at row %d of %d, column %q: %w", rowIndex+1, len(query.onDemand.rows), assignment.Column, err)
					}
				}
			}
			break
		}
		if len(query.onDemand.assignments) == 0 {
			return NewError(ErrorInvalidRule, "on-demand insert requires at least one assignment or positional row")
		}
		for index, assignment := range query.onDemand.assignments {
			if assignment.Wildcard {
				return fmt.Errorf("on-demand insert assignment %d cannot be wildcard without an incoming event", index)
			}
			if err := validateTriggerAssignment(e, query.input, targetSchema, assignment, ""); err != nil {
				return fmt.Errorf("assignment %d: %w", index, err)
			}
		}
	case onDemandUpdate:
		if query.onDemand.predicate == nil {
			return NewError(ErrorInvalidRule, "on-demand update requires a predicate")
		}
		if query.onDemand.predicate.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "on-demand update predicate must return bool")
		}
		if len(query.onDemand.assignments) == 0 {
			return NewError(ErrorInvalidRule, "on-demand update requires at least one assignment")
		}
		if err := e.validateTriggerTargetExpression(query.input, targetSchema, query.onDemand.predicate, targetKind); err != nil {
			return fmt.Errorf("predicate: %w", err)
		}
		for index, assignment := range query.onDemand.assignments {
			if assignment.Wildcard {
				return fmt.Errorf("on-demand update assignment %d cannot be wildcard without an incoming event", index)
			}
			if err := validateTriggerAssignment(e, query.input, targetSchema, assignment, targetKind); err != nil {
				return fmt.Errorf("assignment %d: %w", index, err)
			}
		}
	case onDemandDelete:
		if query.onDemand.predicate == nil {
			return NewError(ErrorInvalidRule, "on-demand delete requires a predicate")
		}
		if query.onDemand.predicate.Type() != typeOf[bool]() {
			return NewError(ErrorTypeMismatch, "on-demand delete predicate must return bool")
		}
		if err := e.validateTriggerTargetExpression(query.input, targetSchema, query.onDemand.predicate, targetKind); err != nil {
			return fmt.Errorf("predicate: %w", err)
		}
		if len(query.onDemand.assignments) != 0 {
			return NewError(ErrorInvalidRule, "on-demand delete cannot have assignments")
		}
	case onDemandDeleteAll:
		if query.onDemand.predicate != nil || len(query.onDemand.assignments) != 0 {
			return NewError(ErrorInvalidRule, "on-demand delete-all cannot have a predicate or assignments")
		}
	default:
		return NewError(ErrorInvalidRule, fmt.Sprintf("unknown on-demand action %d", query.onDemand.action))
	}
	if err := e.validateFireAndForgetSubqueries(query); err != nil {
		return err
	}
	return nil
}

// validateFireAndForgetSubqueries applies the source restrictions that are
// specific to Esper fire-and-forget subqueries.  Ordinary statement
// subqueries may use a windowed event stream and a source filter; FAF
// subqueries are snapshot lookups and therefore accept only root Named Window
// or Table sources.  A context-bound Named Window must also be queried under
// the same context declaration so a partition-local snapshot cannot silently
// become a global lookup.
func (e *Environment) validateFireAndForgetSubqueries(query Query) error {
	for index, definition := range querySubqueryDefinitions(query) {
		if definition == nil || definition.source == nil {
			continue
		}
		base, err := subqueryRootSource(definition.source)
		if err != nil {
			return fmt.Errorf("failed to plan subquery number %d: %w", index+1, err)
		}
		if base.kind != streamNamedWindow && base.kind != streamTable {
			return fmt.Errorf("fire-and-forget subquery %d only allows named-window and table sources", index+1)
		}
		for node := definition.source; node != nil && node != base; node = node.input {
			if node.kind == streamFilter {
				return fmt.Errorf("fire-and-forget subquery %d does not allow source filter expressions", index+1)
			}
		}
		if base.kind != streamNamedWindow {
			continue
		}
		window, ok := e.NamedWindowInModule(base.moduleName, base.sourceName)
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("subquery references unknown named window %q", base.sourceName))
		}
		windowContext := strings.TrimSpace(window.Context())
		queryContext := strings.TrimSpace(query.contextName)
		if windowContext != "" && windowContext != queryContext {
			return fmt.Errorf("fire-and-forget subquery %d context %q does not match query context %q for named window %q", index+1, windowContext, queryContext, base.sourceName)
		}
	}
	return nil
}

func (e *Environment) validateAggregatePluginNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.kind == "aggregate-plugin-ref" || node.kind == "aggregate-plugin-factory-ref" || node.kind == "aggregate-plugin-access-ref" {
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
		if (node.kind == "aggregate-plugin-factory-ref" || node.kind == "aggregate-plugin-access-ref") && definition.factory == nil {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("aggregate plugin %q is not a stateful factory", node.pluginName))
		}
		if node.kind == "aggregate-plugin-access-ref" && !definition.access {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("aggregate plugin %q is not registered as an access plugin", node.pluginName))
		}
		if node.kind == "aggregate-plugin-factory-ref" && definition.access {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("aggregate plugin %q is registered as an access plugin", node.pluginName))
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

func (e *Environment) validateAggregateMultiPluginNodes(node *exprNode) error {
	if node == nil {
		return nil
	}
	if node.kind == "aggregate-multi-plugin" || node.kind == "aggregate-multi-plugin-ref" {
		if strings.TrimSpace(node.aggregateMultiPluginName) == "" {
			return NewError(ErrorInvalidRule, "aggregate multi plugin provider is required")
		}
		if strings.TrimSpace(node.aggregateMultiPluginMethod) == "" {
			return NewError(ErrorInvalidRule, "aggregate multi plugin method is required")
		}
		if len(node.children) > 1 || (len(node.children) == 1 && node.children[0] == nil) {
			return NewError(ErrorInvalidRule, fmt.Sprintf("aggregate multi plugin %q accepts at most one non-nil input expression", node.aggregateMultiPluginName))
		}
		var methods map[string]AggregateMultiPluginMethod
		if node.kind == "aggregate-multi-plugin-ref" {
			if node.aggregateMultiPluginEnvironment == nil || node.aggregateMultiPluginEnvironment != e {
				return NewError(ErrorDependency, fmt.Sprintf("aggregate multi plugin %q belongs to a different environment", node.aggregateMultiPluginName))
			}
			e.mu.RLock()
			definition, ok := e.aggregateMultiPlugins[node.aggregateMultiPluginName]
			e.mu.RUnlock()
			if !ok {
				return NewError(ErrorUnknownName, fmt.Sprintf("aggregate multi plugin %q is not registered", node.aggregateMultiPluginName))
			}
			methods = definition.methods
		} else {
			if !node.aggregateMultiPluginReady || node.aggregateMultiPluginFactory == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("aggregate multi plugin %q has no factory", node.aggregateMultiPluginName))
			}
			methods = node.aggregateMultiMethods
		}
		method, ok := methods[node.aggregateMultiPluginMethod]
		if !ok {
			return NewError(ErrorUnknownName, fmt.Sprintf("aggregate multi plugin %q method %q is not declared", node.aggregateMultiPluginName, node.aggregateMultiPluginMethod))
		}
		if method.ResultType != nil && node.typ != nil && method.ResultType != node.typ &&
			!method.ResultType.AssignableTo(node.typ) && !node.typ.AssignableTo(method.ResultType) && !numericTypes(method.ResultType, node.typ) {
			return NewError(ErrorTypeMismatch, fmt.Sprintf("aggregate multi plugin %q.%s returns %s, expression expects %s", node.aggregateMultiPluginName, node.aggregateMultiPluginMethod, method.ResultType, node.typ))
		}
		node.aggregateMultiStateKey = method.StateKey
		node.aggregateMultiStateShared = method.StateKey != ""
	}
	for _, child := range node.children {
		if err := e.validateAggregateMultiPluginNodes(child); err != nil {
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
	if node.kind == "aggregate-plugin" || node.kind == "aggregate-plugin-ref" || node.kind == "aggregate-plugin-factory" || node.kind == "aggregate-plugin-factory-ref" || node.kind == "aggregate-plugin-access-ref" {
		if strings.TrimSpace(node.pluginName) == "" {
			return NewError(ErrorInvalidRule, "plugin aggregate name is required")
		}
		if (node.kind == "aggregate-plugin" || node.kind == "aggregate-plugin-factory") && !node.pluginReady {
			return NewError(ErrorInvalidRule, fmt.Sprintf("plugin aggregate %q has no evaluator", node.pluginName))
		}
		if (node.kind == "aggregate-plugin-factory" || node.kind == "aggregate-plugin-factory-ref" || node.kind == "aggregate-plugin-access-ref") && len(node.children) > 1 {
			return NewError(ErrorInvalidRule, "plugin aggregate factory accepts at most one input expression")
		}
	}
	if node.kind == "aggregate-multi-plugin" || node.kind == "aggregate-multi-plugin-ref" {
		if strings.TrimSpace(node.aggregateMultiPluginName) == "" {
			return NewError(ErrorInvalidRule, "aggregate multi plugin provider is required")
		}
		if strings.TrimSpace(node.aggregateMultiPluginMethod) == "" {
			return NewError(ErrorInvalidRule, "aggregate multi plugin method is required")
		}
		if len(node.children) > 1 || (len(node.children) == 1 && node.children[0] == nil) {
			return NewError(ErrorInvalidRule, "aggregate multi plugin accepts at most one non-nil input expression")
		}
	}
	if node.kind == "aggregate-plugin-inputs" {
		for index, child := range node.children {
			if child == nil {
				return NewError(ErrorInvalidRule, fmt.Sprintf("aggregate plugin input %d is nil", index))
			}
			if expressionNodeContainsAggregate(child) {
				return NewError(ErrorInvalidRule, "aggregate plugin input cannot contain an aggregate expression")
			}
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
	if node.kind == "aggregate-distinct" {
		if len(node.children) < 1 || len(node.children) > 2 || node.children[0] == nil {
			return NewError(ErrorInvalidRule, "distinct aggregate requires an aggregate expression")
		}
		if len(node.children) == 2 && node.children[1] == nil {
			return NewError(ErrorInvalidRule, "distinct aggregate input expression is nil")
		}
		if len(node.children) == 2 && expressionNodeContainsAggregate(node.children[1]) {
			return NewError(ErrorInvalidRule, "distinct aggregate input cannot contain an aggregate expression")
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
	if expressionNodeIsAggregate(node) {
		return true
	}
	for _, child := range node.children {
		if expressionNodeContainsAggregate(child) {
			return true
		}
	}
	return false
}

// expressionNodeIsAggregate reports whether the node itself is an aggregate
// boundary.  It is deliberately separate from expressionNodeContainsAggregate:
// callers that inspect scalar dependencies must not descend into the input
// fields consumed by an aggregate function.
func expressionNodeIsAggregate(node *exprNode) bool {
	if node == nil {
		return false
	}
	if node.kind == "linear-regression" || strings.HasPrefix(node.kind, "linear-regression-") || node.kind == "univariate-statistics" || strings.HasPrefix(node.kind, "univariate-statistics-") {
		return true
	}
	if node.kind == "sorted-access" || strings.HasPrefix(node.kind, "sorted-access-") || node.kind == "window-access" || strings.HasPrefix(node.kind, "window-access-") {
		return true
	}
	switch node.kind {
	case "tag-sum", "tag-avg", "tag-min", "tag-max", "tag-first", "tag-last":
		return true
	}
	switch node.kind {
	case "aggregate-filter", "aggregate-local-group", "aggregate-distinct", "aggregate-plugin", "aggregate-plugin-ref", "aggregate-plugin-factory", "aggregate-plugin-factory-ref", "aggregate-plugin-access-ref", "aggregate-multi-plugin", "aggregate-multi-plugin-ref", "count-min-sketch", "count-min-frequency", "count-min-total", "rate-timestamp", "rate-quantity-timestamp", "leaving", "count", "count-ever-invalid", "sum", "sum-exact", "avg", "avg-exact", "min", "min-exact", "max", "first", "last", "nth", "count-distinct", "median", "stddev", "stddev-pop", "variance", "avedev", "weighted-avg", "correlation", "rate", "min-by", "max-by", "min-by-ever", "max-by-ever", "window", "set", "sorted", "count-ever", "first-ever", "last-ever", "max-ever", "min-ever":
		return true
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
	case "count", "sum", "sum-exact", "avg", "avg-exact", "min", "min-exact", "max", "max-exact", "first", "last", "first-ever", "last-ever", "max-ever", "min-ever", "count-ever", "count-distinct", "median", "stddev", "stddev-pop", "variance", "avedev", "weighted-avg", "correlation", "rate", "min-by", "max-by", "min-by-ever", "max-by-ever", "window", "sorted", "set", "count-min-frequency", "count-min-total":
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
		if definition.everyDistinct != nil {
			if err := e.validateExprFields(definition.input, definition.everyDistinct); err != nil {
				return fmt.Errorf("every-distinct key: %w", err)
			}
		}
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
	if len(selections) == 0 && len(patternDefinitionTagNames(definition)) > 0 {
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
		if err := e.validatePatternExpressionFields(eventInput, node.predicate, tagSources, requireTags); err != nil {
			return err
		}
		return validatePatternFilterExpression(node.predicate)
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
	case patternGuardWhileNode:
		if node.guardExpr != nil {
			if err := e.validateExprFields(input, node.guardExpr); err != nil {
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

// validatePatternFilterExpression enforces Esper's evaluation boundary for a
// pattern event filter. Filters run against one incoming event and captured
// tags, so aggregate state and view-relative previous/prior access have no
// valid evaluation context. Keeping this check at the pattern node boundary
// prevents a scalar wrapper from hiding either invalid expression family.
func validatePatternFilterExpression(expression Expr) error {
	if expression == nil || expression.node() == nil {
		return NewError(ErrorInvalidRule, "pattern filter expression is required")
	}
	if expressionNodeContainsAggregate(expression.node()) {
		return NewError(ErrorInvalidRule, "aggregation functions not allowed within filters")
	}
	if expressionContainsPreviousAccess(expression.node()) {
		return NewError(ErrorInvalidRule, "previous or prior functions cannot be used in pattern filters")
	}
	return nil
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
		if err := e.validateExpressionReferences(selection.Expr.node(), make(map[string]bool)); err != nil {
			return err
		}
		if err := e.validateExprVariables(selection.Expr); err != nil {
			return err
		}
		if err := validateBitwiseExpressionNodes(selection.Expr.node()); err != nil {
			return err
		}
		if err := validateCoalesceExpressionNodes(selection.Expr.node()); err != nil {
			return err
		}
		if err := validateMethodNodes(selection.Expr.node()); err != nil {
			return err
		}
		if err := e.validateScriptNodes(selection.Expr.node()); err != nil {
			return err
		}
		var fields []string
		selection.Expr.node().referencedFields(&fields)
		if len(fields) > 0 {
			return NewError(ErrorDependency, "source-less query cannot reference event fields")
		}
		if err := e.validateExpressionSubqueries(selection.Expr.node()); err != nil {
			return err
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
	if (policy.Kind == OutputFirstPolicy || policy.Kind == OutputEveryPolicy || policy.Kind == OutputFirstEveryEventsPolicy || policy.Kind == OutputLastEveryEventsPolicy || policy.Kind == OutputAllEveryEventsPolicy) && policy.Count <= 0 {
		return NewError(ErrorInvalidRule, "output count must be positive")
	}
	if (policy.Kind == OutputEveryTimePolicy || policy.Kind == OutputFirstEveryTimePolicy || policy.Kind == OutputLastEveryTimePolicy || policy.Kind == OutputAllEveryTimePolicy) && policy.Interval <= 0 {
		return NewError(ErrorInvalidRule, "time-based output interval must be positive")
	}
	if policy.Kind > OutputAllEveryEventsPolicy {
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
	if len(policy.TerminationThen) > 0 && policy.TerminationWhen == nil && policy.Termination == OutputNoTermination {
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
			return NewError(ErrorState, fmt.Sprintf("Variable by name '%s' is declared constant and may not be set", name))
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
			return NewError(ErrorState, fmt.Sprintf("Variable by name '%s' is declared constant and may not be set", name))
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
