package esper

import (
	"fmt"
	"sort"
	"strings"
)

// ModuleVisibility controls whether a module is part of the engine-wide
// catalog or materialized only while its owning deployment is active.
type ModuleVisibility uint8

const (
	ModulePrivate ModuleVisibility = iota
	ModuleProtected
	ModulePublic
)

func (v ModuleVisibility) String() string {
	switch v {
	case ModuleProtected:
		return "protected"
	case ModulePublic:
		return "public"
	default:
		return "private"
	}
}

type moduleDefinition struct {
	visibility ModuleVisibility
	metadata   ModuleMetadata
}

type moduleConfig struct {
	visibility         ModuleVisibility
	visibilitySet      bool
	visibilityConflict bool
	metadata           ModuleMetadata
	err                error
}

// ModuleMetadata is detached compile/deployment metadata for one typed module.
// URI, archive name and user object are application-owned labels. Uses
// participates in the module's default typed resolution path; Imports is
// retained for diagnostics only because Go package imports are resolved by the
// Go compiler rather than by a runtime rule parser.
type ModuleMetadata struct {
	URI         string
	ArchiveName string
	UserObject  any
	Uses        []string
	Imports     []string
}

func cloneModuleMetadata(metadata ModuleMetadata) ModuleMetadata {
	metadata.Uses = append([]string(nil), metadata.Uses...)
	metadata.Imports = append([]string(nil), metadata.Imports...)
	return metadata
}

// ModuleOption configures a typed module namespace.
type ModuleOption func(*moduleConfig)

func setModuleVisibility(visibility ModuleVisibility) ModuleOption {
	return func(config *moduleConfig) {
		if config.visibilitySet && config.visibility != visibility {
			config.visibilityConflict = true
			return
		}
		config.visibility = visibility
		config.visibilitySet = true
	}
}

// PrivateModule limits definitions to rules built for the same module. This
// is the default, matching Esper's default name-access modifier.
func PrivateModule() ModuleOption { return setModuleVisibility(ModulePrivate) }

// ProtectedModule gives every deployment of this module its own lifecycle.
// The module's tables, Named Windows, variables and contexts are materialized
// atomically at deploy time and removed again at undeploy time.
func ProtectedModule() ModuleOption {
	return setModuleVisibility(ModuleProtected)
}

// PublicModule exports definitions to every module path in the Environment.
// A Uses dependency can select one public module when several export the same
// logical name.
func PublicModule() ModuleOption { return setModuleVisibility(ModulePublic) }

// WithModuleURI attaches an application URI to the typed module and its
// deployments.
func WithModuleURI(uri string) ModuleOption {
	return func(config *moduleConfig) { config.metadata.URI = uri }
}

// WithModuleArchiveName attaches an archive/source label to the typed module.
func WithModuleArchiveName(name string) ModuleOption {
	return func(config *moduleConfig) { config.metadata.ArchiveName = name }
}

// WithModuleUserObject attaches one opaque application value to the module.
func WithModuleUserObject(value any) ModuleOption {
	return func(config *moduleConfig) { config.metadata.UserObject = value }
}

// WithModuleUses declares the module names available through the module's
// default typed resolution path. Names preserve first-declaration order.
func WithModuleUses(names ...string) ModuleOption {
	return func(config *moduleConfig) {
		if config.err != nil {
			return
		}
		uses, err := normalizeModuleMetadataNames("uses", names)
		if err != nil {
			config.err = err
			return
		}
		config.metadata.Uses = uses
	}
}

// WithModuleImports records source-language import labels for diagnostics.
// They do not alter Go symbol resolution.
func WithModuleImports(names ...string) ModuleOption {
	return func(config *moduleConfig) {
		if config.err != nil {
			return
		}
		imports, err := normalizeModuleMetadataNames("import", names)
		if err != nil {
			config.err = err
			return
		}
		config.metadata.Imports = imports
	}
}

func normalizeModuleMetadataNames(kind string, names []string) ([]string, error) {
	result := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for index, name := range names {
		name = normalizeModuleName(name)
		if name == "" {
			return nil, NewError(ErrorInvalidRule, fmt.Sprintf("module %s name %d is blank", kind, index))
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result, nil
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
	definition.metadata = cloneModuleMetadata(definition.metadata)
	return definition, ok
}

// Module is a Go-native catalog namespace. It models the name-resolution
// portion of an Esper module while keeping registration and source selection
// explicit instead of parsing module/EPL text.
type Module struct {
	env        *Environment
	name       string
	visibility ModuleVisibility
	metadata   ModuleMetadata
}

func (m Module) Name() string                 { return m.name }
func (m Module) Visibility() ModuleVisibility { return m.visibility }
func (m Module) Private() bool                { return m.visibility == ModulePrivate }
func (m Module) Protected() bool              { return m.visibility == ModuleProtected }
func (m Module) Public() bool                 { return m.visibility == ModulePublic }

// Metadata returns a detached snapshot of module compile/deployment metadata.
func (m Module) Metadata() ModuleMetadata { return cloneModuleMetadata(m.metadata) }

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
	config := moduleConfig{visibility: ModulePrivate}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	if config.visibilityConflict {
		return Module{}, NewError(ErrorInvalidRule, "module cannot be private, protected and public at the same time")
	}
	if config.err != nil {
		return Module{}, config.err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.modules[name]; exists {
		return Module{}, NewError(ErrorDependency, "module "+name+" is already registered")
	}
	definition := moduleDefinition{visibility: config.visibility, metadata: cloneModuleMetadata(config.metadata)}
	e.modules[name] = definition
	return Module{env: e, name: name, visibility: definition.visibility, metadata: cloneModuleMetadata(definition.metadata)}, nil
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
	return Module{env: e, name: name, visibility: definition.visibility, metadata: cloneModuleMetadata(definition.metadata)}, true
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
	query.moduleUses = append([]string(nil), m.metadata.Uses...)
	return m.env.Build(query)
}

// ModulePath is an immutable compiler-path equivalent for fluent Go rules.
// The owning module can always see its own definitions. Other modules are
// visible only when public; Uses selects an explicit public dependency and
// disambiguates otherwise-identical exported names.
type ModulePath struct {
	env        *Environment
	moduleName string
	uses       []string
	selected   bool
	err        error
}

// Path returns the same-module visibility scope without dependencies.
func (m Module) Path() ModulePath {
	if m.env == nil {
		return ModulePath{err: NewError(ErrorDependency, "module has no environment")}
	}
	return ModulePath{env: m.env, moduleName: m.name, uses: append([]string(nil), m.metadata.Uses...), selected: len(m.metadata.Uses) > 0}
}

// Uses constructs a same-module scope with explicit public dependencies.
func (m Module) Uses(modules ...Module) ModulePath { return m.Path().Uses(modules...) }

// Path returns an unowned resolution scope. It can see preconfigured global
// definitions and unambiguous public module exports.
func (e *Environment) Path() ModulePath { return ModulePath{env: e} }

// Uses constructs an unowned resolution scope with explicit public module
// dependencies.
func (e *Environment) Uses(modules ...Module) ModulePath { return e.Path().Uses(modules...) }

// Uses appends explicit public dependencies while preserving declaration
// order for diagnostics. Canonical plan identity sorts a detached copy.
func (p ModulePath) Uses(modules ...Module) ModulePath {
	if p.err != nil {
		return p
	}
	if p.env == nil {
		p.err = NewError(ErrorDependency, "module path has no environment")
		return p
	}
	p.selected = true
	seen := make(map[string]struct{}, len(p.uses)+len(modules))
	for _, name := range p.uses {
		seen[name] = struct{}{}
	}
	for _, module := range modules {
		if module.env == nil || module.env != p.env {
			p.err = NewError(ErrorDependency, "used module belongs to a different or nil environment")
			return p
		}
		name := normalizeModuleName(module.name)
		if name == "" {
			p.err = NewError(ErrorInvalidRule, "used module name is required")
			return p
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		p.uses = append(p.uses, name)
	}
	return p
}

// UsesNames appends module dependency declarations without requiring those
// modules to be registered in this Environment. Unknown names are ignored for
// symbol resolution unless a referenced object actually requires them. This
// supports separately assembled/deployed module graphs while preserving the
// declarations in Query.ModuleUses and Plan canonical identity.
func (p ModulePath) UsesNames(names ...string) ModulePath {
	if p.err != nil {
		return p
	}
	if p.env == nil {
		p.err = NewError(ErrorDependency, "module path has no environment")
		return p
	}
	p.selected = true
	seen := make(map[string]struct{}, len(p.uses)+len(names))
	for _, name := range p.uses {
		seen[name] = struct{}{}
	}
	for _, name := range names {
		name = normalizeModuleName(name)
		if name == "" {
			p.err = NewError(ErrorInvalidRule, "used module name is required")
			return p
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		p.uses = append(p.uses, name)
	}
	return p
}

// Build binds module identity and dependency selection to an immutable Plan.
func (p ModulePath) Build(query Query) (Plan, error) {
	if p.err != nil {
		return Plan{}, p.err
	}
	if p.env == nil {
		return Plan{}, NewError(ErrorDependency, "module path has no environment")
	}
	if query.env != p.env {
		return Plan{}, NewError(ErrorDependency, "query belongs to a different or nil environment")
	}
	query.moduleName = p.moduleName
	query.moduleUses = append([]string(nil), p.uses...)
	return p.env.Build(query)
}

type moduleObjectKind uint8

const (
	moduleObjectEventType moduleObjectKind = iota
	moduleObjectVariable
	moduleObjectContext
	moduleObjectNamedWindow
	moduleObjectTable
	moduleObjectExpression
	moduleObjectScript
)

func (k moduleObjectKind) label() string {
	switch k {
	case moduleObjectVariable:
		return "variable"
	case moduleObjectContext:
		return "context"
	case moduleObjectNamedWindow:
		return "named window"
	case moduleObjectTable:
		return "table"
	case moduleObjectExpression:
		return "declared expression"
	case moduleObjectScript:
		return "script"
	default:
		return "event type"
	}
}

func (e *Environment) hasModuleObjectLocked(kind moduleObjectKind, identity string) bool {
	switch kind {
	case moduleObjectVariable:
		_, ok := e.variables[identity]
		return ok
	case moduleObjectContext:
		_, ok := e.contexts[identity]
		return ok
	case moduleObjectNamedWindow:
		_, ok := e.namedWindows[identity]
		return ok
	case moduleObjectTable:
		_, ok := e.tables[identity]
		return ok
	case moduleObjectExpression:
		_, ok := e.expressions[identity]
		return ok
	case moduleObjectScript:
		_, ok := e.scripts[identity]
		return ok
	default:
		_, ok := e.schemas[identity]
		return ok
	}
}

func (p ModulePath) resolve(kind moduleObjectKind, logicalName string) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	if p.env == nil {
		return "", NewError(ErrorDependency, "module path has no environment")
	}
	logicalName = strings.TrimSpace(logicalName)
	if logicalName == "" {
		return "", NewError(ErrorInvalidRule, kind.label()+" name is required")
	}
	p.env.mu.RLock()
	defer p.env.mu.RUnlock()

	global := p.env.hasModuleObjectLocked(kind, logicalName)
	if p.moduleName != "" {
		own := catalogKey(p.moduleName, logicalName)
		if p.env.hasModuleObjectLocked(kind, own) {
			return own, nil
		}
	}

	if p.selected || len(p.uses) > 0 {
		candidates := make([]string, 0, len(p.uses))
		for _, moduleName := range p.uses {
			definition, exists := p.env.modules[moduleName]
			if !exists {
				// UsesNames permits dependency declarations for modules assembled or
				// deployed elsewhere. Such names participate in Plan identity and
				// deployment ordering but do not contribute local catalog candidates.
				continue
			}
			if definition.visibility != ModulePublic && moduleName != p.moduleName {
				return "", NewError(ErrorUnknownName, fmt.Sprintf("module %q is not public", moduleName))
			}
			identity := catalogKey(moduleName, logicalName)
			if p.env.hasModuleObjectLocked(kind, identity) {
				candidates = append(candidates, identity)
			}
		}
		if len(candidates) == 1 {
			return candidates[0], nil
		}
		if len(candidates) > 1 {
			return "", NewError(ErrorAmbiguous, fmt.Sprintf("%s %q is exported by multiple used modules", kind.label(), logicalName))
		}
		if global {
			return logicalName, nil
		}
		return "", NewError(ErrorUnknownName, fmt.Sprintf("%s %q is not visible through the selected modules", kind.label(), logicalName))
	}

	candidates := make([]string, 0)
	for moduleName, definition := range p.env.modules {
		if definition.visibility != ModulePublic {
			continue
		}
		identity := catalogKey(moduleName, logicalName)
		if p.env.hasModuleObjectLocked(kind, identity) {
			candidates = append(candidates, identity)
		}
	}
	sort.Strings(candidates)
	if global && len(candidates) > 0 {
		return "", NewError(ErrorAmbiguous, fmt.Sprintf("%s %q is ambiguous between the preconfigured catalog and a module path", kind.label(), logicalName))
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) > 1 {
		return "", NewError(ErrorAmbiguous, fmt.Sprintf("%s %q is exported by multiple modules", kind.label(), logicalName))
	}
	if global {
		return logicalName, nil
	}
	return "", NewError(ErrorUnknownName, fmt.Sprintf("%s %q is not visible", kind.label(), logicalName))
}

func (p ModulePath) EventType(name string) (string, error) {
	return p.resolve(moduleObjectEventType, name)
}

func (p ModulePath) Variable(name string) (string, error) {
	return p.resolve(moduleObjectVariable, name)
}

func (p ModulePath) Context(name string) (string, error) {
	return p.resolve(moduleObjectContext, name)
}

func (p ModulePath) NamedWindow(name string) (RecordStream, error) {
	identity, err := p.resolve(moduleObjectNamedWindow, name)
	if err != nil {
		return RecordStream{}, err
	}
	moduleName, objectName := splitCatalogKey(identity)
	return FromNamedWindowInModule(p.env, moduleName, objectName), nil
}

func (p ModulePath) Table(name string) (RecordStream, error) {
	identity, err := p.resolve(moduleObjectTable, name)
	if err != nil {
		return RecordStream{}, err
	}
	moduleName, objectName := splitCatalogKey(identity)
	return FromTableInModule(p.env, moduleName, objectName), nil
}

func splitCatalogKey(identity string) (string, string) {
	parts := strings.SplitN(identity, "::", 2)
	if len(parts) != 2 {
		return "", identity
	}
	return parts[0], parts[1]
}

func ModulePathVariableRef[T any](path ModulePath, name string) (Expression[T], error) {
	identity, err := path.resolve(moduleObjectVariable, name)
	if err != nil {
		return nil, err
	}
	return VariableRef[T](identity), nil
}

func ModulePathExpressionRef[T any](path ModulePath, name string, arguments ...Expr) (Expression[T], error) {
	identity, err := path.resolve(moduleObjectExpression, name)
	if err != nil {
		return nil, err
	}
	return ExpressionRef[T](path.env, identity, arguments...), nil
}

func ModulePathScriptCall[T any](path ModulePath, name string, arguments ...Expr) (Expression[T], error) {
	identity, err := path.resolve(moduleObjectScript, name)
	if err != nil {
		return nil, err
	}
	return ScriptCall[T](path.env, identity, arguments...), nil
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

// RegisterModuleScript registers a Go provider under a module-local script
// identity. Generic methods are not available in Go, therefore result typing
// is expressed by this top-level helper.
func RegisterModuleScript[T any](module Module, name, dialect string, provider any, options ...ScriptOption) error {
	if module.env == nil {
		return NewError(ErrorDependency, "module has no environment")
	}
	identity := module.QualifiedName(name)
	if err := RegisterScript[T](module.env, identity, dialect, provider, options...); err != nil {
		return err
	}
	module.env.recordModuleObject(module.name, identity)
	return nil
}

// RegisterMap registers a module-local map event type. The returned schema's
// stable internal name is suitable for InsertInto and FromAny.
func (m Module) RegisterMap(name string, fields []FieldSpec, options ...SchemaOption) (Schema, error) {
	if m.env == nil {
		return Schema{}, NewError(ErrorDependency, "module has no environment")
	}
	identity := m.QualifiedName(name)
	schema, err := NewMapSchema(identity, fields, options...)
	if err != nil {
		return Schema{}, err
	}
	if schema.BusVisible() && m.visibility != ModulePublic {
		return Schema{}, NewError(ErrorInvalidRule, fmt.Sprintf("event type %q with bus visibility requires a public module", name))
	}
	if err := m.env.RegisterSchema(schema); err != nil {
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
