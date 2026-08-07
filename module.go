package esper

import "strings"

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

// Module is a Go-native catalog namespace. It models the name-resolution
// portion of an Esper module while keeping registration and source selection
// explicit instead of parsing module/EPL text.
type Module struct {
	env  *Environment
	name string
}

func (m Module) Name() string { return m.name }

// RegisterModule creates a named catalog namespace. Modules are immutable
// namespace identities; their contained definitions can be registered while
// an Environment is being assembled.
func (e *Environment) RegisterModule(name string) (Module, error) {
	if e == nil {
		return Module{}, NewError(ErrorDependency, "nil environment")
	}
	name = normalizeModuleName(name)
	if name == "" {
		return Module{}, NewError(ErrorInvalidRule, "module name is required")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.modules[name]; exists {
		return Module{}, NewError(ErrorDependency, "module "+name+" is already registered")
	}
	e.modules[name] = struct{}{}
	return Module{env: e, name: name}, nil
}

// Module returns a previously registered namespace.
func (e *Environment) Module(name string) (Module, bool) {
	if e == nil {
		return Module{}, false
	}
	name = normalizeModuleName(name)
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.modules[name]
	if !ok {
		return Module{}, false
	}
	return Module{env: e, name: name}, true
}

// RegisterModule is also available as a package-level constructor for code
// that prefers the same style as RegisterSchema/RegisterTable.
func RegisterModule(env *Environment, name string) (Module, error) {
	if env == nil {
		return Module{}, NewError(ErrorDependency, "nil environment")
	}
	return env.RegisterModule(name)
}

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
