package esper

import "strings"

// ModuleVisibility controls whether a module is part of the engine-wide
// catalog or materialized only while its owning deployment is active.
type ModuleVisibility uint8

const (
	ModulePublic ModuleVisibility = iota
	ModuleProtected
)

func (v ModuleVisibility) String() string {
	if v == ModuleProtected {
		return "protected"
	}
	return "public"
}

type moduleDefinition struct {
	visibility ModuleVisibility
}

type moduleConfig struct {
	visibility ModuleVisibility
}

// ModuleOption configures a typed module namespace.
type ModuleOption func(*moduleConfig)

// ProtectedModule gives every deployment of this module its own lifecycle.
// The module's tables, Named Windows, variables and contexts are materialized
// atomically at deploy time and removed again at undeploy time.
func ProtectedModule() ModuleOption {
	return func(config *moduleConfig) { config.visibility = ModuleProtected }
}

// catalogKey is the internal identity for a named catalog object. The empty
// module keeps the original unqualified Go API behavior; a non-empty module
// allows two modules to expose the same logical Table or Named Window name
// without making callers encode a Java-style qualified name in a rule.
func catalogKey(moduleName, objectName string) string {
	moduleName = strings.TrimSpace(moduleName)
	objectName = strings.TrimSpace(objectName)
	if moduleName == "" {
		return objectName
	}
	return moduleName + "::" + objectName
}

func normalizeModuleName(name string) string { return strings.TrimSpace(name) }

func (e *Environment) protectedModuleForQualifiedNameLocked(name string) (string, bool) {
	if e == nil {
		return "", false
	}
	moduleName, owned := e.moduleObjects[name]
	if !owned {
		return "", false
	}
	definition, exists := e.modules[moduleName]
	if exists && definition.visibility == ModuleProtected {
		return moduleName, true
	}
	return "", false
}

func (e *Environment) recordModuleObject(moduleName, identity string) {
	if e == nil || moduleName == "" || identity == "" {
		return
	}
	e.mu.Lock()
	e.moduleObjects[identity] = moduleName
	e.mu.Unlock()
}

func (e *Environment) moduleDefinition(name string) (moduleDefinition, bool) {
	if e == nil {
		return moduleDefinition{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.modules[normalizeModuleName(name)]
	return definition, ok
}

// Module is a Go-native catalog namespace. It models the name-resolution
// portion of an Esper module while keeping registration and source selection
// explicit instead of parsing module/EPL text.
type Module struct {
	env        *Environment
	name       string
	visibility ModuleVisibility
}

func (m Module) Name() string                 { return m.name }
func (m Module) Visibility() ModuleVisibility { return m.visibility }
func (m Module) Protected() bool              { return m.visibility == ModuleProtected }

// RegisterModule creates a named catalog namespace. Modules are immutable
// namespace identities; their contained definitions can be registered while
// an Environment is being assembled.
func (e *Environment) RegisterModule(name string, options ...ModuleOption) (Module, error) {
	if e == nil {
		return Module{}, NewError(ErrorDependency, "nil environment")
	}
	name = normalizeModuleName(name)
	if name == "" {
		return Module{}, NewError(ErrorInvalidRule, "module name is required")
	}
	config := moduleConfig{visibility: ModulePublic}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.modules[name]; exists {
		return Module{}, NewError(ErrorDependency, "module "+name+" is already registered")
	}
	definition := moduleDefinition{visibility: config.visibility}
	e.modules[name] = definition
	return Module{env: e, name: name, visibility: definition.visibility}, nil
}

// Module returns a previously registered namespace.
func (e *Environment) Module(name string) (Module, bool) {
	if e == nil {
		return Module{}, false
	}
	name = normalizeModuleName(name)
	e.mu.RLock()
	defer e.mu.RUnlock()
	definition, ok := e.modules[name]
	if !ok {
		return Module{}, false
	}
	return Module{env: e, name: name, visibility: definition.visibility}, true
}

// RegisterModule is also available as a package-level constructor for code
// that prefers the same style as RegisterSchema/RegisterTable.
func RegisterModule(env *Environment, name string, options ...ModuleOption) (Module, error) {
	if env == nil {
		return Module{}, NewError(ErrorDependency, "nil environment")
	}
	return env.RegisterModule(name, options...)
}

// Build binds the immutable plan to this module. Protected plans can only be
// deployed as one deployment-local namespace, and the binding participates in
// canonical plan identity.
func (m Module) Build(query Query) (Plan, error) {
	if m.env == nil {
		return Plan{}, NewError(ErrorDependency, "module has no environment")
	}
	if query.env != m.env {
		return Plan{}, NewError(ErrorDependency, "query belongs to a different or nil environment")
	}
	query.moduleName = m.name
	return m.env.Build(query)
}

// QualifiedName returns the private catalog identity for a module-local
// schema, variable, context or declared expression.
func (m Module) QualifiedName(name string) string {
	return catalogKey(m.name, name)
}

// RegisterVariable declares a module-local runtime variable.
func (m Module) RegisterVariable(name string, initial any, options ...VariableOption) error {
	if m.env == nil {
		return NewError(ErrorDependency, "module has no environment")
	}
	identity := m.QualifiedName(name)
	if err := m.env.RegisterVariable(identity, initial, options...); err != nil {
		return err
	}
	m.env.recordModuleObject(m.name, identity)
	return nil
}

// VariableRef creates a typed reference to a module-local variable. Go does
// not support generic methods, therefore the type parameter is supplied by
// the top-level helper.
func ModuleVariableRef[T any](module Module, name string) Expression[T] {
	return VariableRef[T](module.QualifiedName(name))
}

// DefineExpression declares a module-local named expression.
func (m Module) DefineExpression(name string, expression Expr) error {
	if m.env == nil {
		return NewError(ErrorDependency, "module has no environment")
	}
	identity := m.QualifiedName(name)
	if err := m.env.DefineExpression(identity, expression); err != nil {
		return err
	}
	m.env.recordModuleObject(m.name, identity)
	return nil
}

// ModuleExpressionRef creates a typed reference to a module-local declared
// expression.
func ModuleExpressionRef[T any](module Module, name string, arguments ...Expr) Expression[T] {
	return ExpressionRef[T](module.env, module.QualifiedName(name), arguments...)
}

// RegisterMap registers a module-local map event type. The returned schema's
// stable internal name is suitable for InsertInto and FromAny.
func (m Module) RegisterMap(name string, fields []FieldSpec, options ...SchemaOption) (Schema, error) {
	if m.env == nil {
		return Schema{}, NewError(ErrorDependency, "module has no environment")
	}
	identity := m.QualifiedName(name)
	schema, err := RegisterMap(m.env, identity, fields, options...)
	if err != nil {
		return Schema{}, err
	}
	m.env.recordModuleObject(m.name, identity)
	return schema, nil
}

// EventType returns the private event-type identity used by fluent routing.
func (m Module) EventType(name string) string { return m.QualifiedName(name) }

// Stream selects a module-local dynamic event type.
func (m Module) Stream(name string) RecordStream {
	return FromAny(m.env, m.EventType(name))
}

// RegisterContextDefinition registers a context under this module's private
// identity. The supplied definition remains immutable; only its logical name
// is rebound.
func (m Module) RegisterContextDefinition(name string, definition ContextDefinition) (ContextDefinition, error) {
	if m.env == nil {
		return ContextDefinition{}, NewError(ErrorDependency, "module has no environment")
	}
	definition.name = m.QualifiedName(name)
	registered, err := m.env.registerContextDefinition(definition)
	if err != nil {
		return ContextDefinition{}, err
	}
	m.env.recordModuleObject(m.name, definition.name)
	return registered, nil
}

// Context returns the private context identity used with WithContext.
func (m Module) Context(name string) string { return m.QualifiedName(name) }

func (m Module) RegisterTable(name string, columns []TableColumn, options ...TableOption) (TableDefinition, error) {
	if m.env == nil {
		return TableDefinition{}, NewError(ErrorDependency, "module has no environment")
	}
	return m.env.RegisterTableInModule(m.name, name, columns, options...)
}

func (m Module) RegisterNamedWindow(name string, schema Schema, options ...NamedWindowOption) (NamedWindowDefinition, error) {
	if m.env == nil {
		return NamedWindowDefinition{}, NewError(ErrorDependency, "module has no environment")
	}
	return m.env.RegisterNamedWindowInModule(m.name, name, schema, options...)
}

// NamedWindow returns a source bound to this module's logical name.
func (m Module) NamedWindow(name string) RecordStream {
	return FromNamedWindowInModule(m.env, m.name, name)
}

// Table returns a source bound to this module's logical name.
func (m Module) Table(name string) RecordStream {
	return FromTableInModule(m.env, m.name, name)
}
