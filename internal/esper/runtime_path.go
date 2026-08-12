package esper

import "sort"

// RuntimePath is an immutable compiler-path snapshot derived from the public
// modules exported by active Engine deployments. Preconfigured global catalog
// entries remain visible through the embedded ModulePath. A snapshot keeps its
// module set even if deployments later change; call Engine.RuntimePath again to
// observe the new runtime catalog.
type RuntimePath struct {
	path          ModulePath
	moduleNames   []string
	deploymentIDs []string
}

// RuntimePath returns the current active public-module compiler path. Private
// and protected deployment namespaces are intentionally not exported.
func (e *Engine) RuntimePath() RuntimePath {
	if e == nil || e.env == nil {
		return RuntimePath{path: ModulePath{err: NewError(ErrorDependency, "engine has no environment")}}
	}
	e.mu.Lock()
	moduleSet := make(map[string]struct{})
	deploymentIDs := make([]string, 0, len(e.deployments))
	for id, deployment := range e.deployments {
		if deployment == nil {
			continue
		}
		deploymentIDs = append(deploymentIDs, id)
		moduleName := normalizeModuleName(deployment.moduleName)
		if moduleName != "" {
			moduleSet[moduleName] = struct{}{}
		}
	}
	e.mu.Unlock()
	sort.Strings(deploymentIDs)

	moduleNames := make([]string, 0, len(moduleSet))
	modules := make([]Module, 0, len(moduleSet))
	for moduleName := range moduleSet {
		module, exists := e.env.Module(moduleName)
		if !exists || !module.Public() {
			continue
		}
		moduleNames = append(moduleNames, moduleName)
		modules = append(modules, module)
	}
	sort.Slice(modules, func(left, right int) bool { return modules[left].Name() < modules[right].Name() })
	sort.Strings(moduleNames)
	path := ModulePath{env: e.env, selected: true}
	if len(modules) > 0 {
		path = path.Uses(modules...)
	}
	return RuntimePath{path: path, moduleNames: moduleNames, deploymentIDs: deploymentIDs}
}

// Path returns the immutable ModulePath used for typed symbol resolution.
func (p RuntimePath) Path() ModulePath { return p.path }

// ModuleNames returns active public modules included in this snapshot.
func (p RuntimePath) ModuleNames() []string { return append([]string(nil), p.moduleNames...) }

// DeploymentIDs returns all active deployments observed by the snapshot,
// including deployments that do not export a public module.
func (p RuntimePath) DeploymentIDs() []string { return append([]string(nil), p.deploymentIDs...) }

func (p RuntimePath) EventType(name string) (string, error) { return p.path.EventType(name) }
func (p RuntimePath) Variable(name string) (string, error)  { return p.path.Variable(name) }
func (p RuntimePath) Context(name string) (string, error)   { return p.path.Context(name) }
func (p RuntimePath) NamedWindow(name string) (RecordStream, error) {
	return p.path.NamedWindow(name)
}
func (p RuntimePath) Table(name string) (RecordStream, error) { return p.path.Table(name) }

// Build compiles a typed query against the runtime-path snapshot.
func (p RuntimePath) Build(query Query, options ...CompileOption) (Plan, error) {
	if p.path.err != nil {
		return Plan{}, p.path.err
	}
	if p.path.env == nil {
		return Plan{}, NewError(ErrorDependency, "runtime path has no environment")
	}
	if query.env != p.path.env {
		return Plan{}, NewError(ErrorDependency, "query belongs to a different or nil environment")
	}
	query.moduleName = ""
	query.moduleUses = append([]string(nil), p.path.uses...)
	return p.path.env.Build(query, options...)
}

// RuntimePathVariableRef resolves one typed variable through a runtime path.
func RuntimePathVariableRef[T any](path RuntimePath, name string) (Expression[T], error) {
	return ModulePathVariableRef[T](path.path, name)
}

// RuntimePathExpressionRef resolves one declared expression through a runtime
// path snapshot.
func RuntimePathExpressionRef[T any](path RuntimePath, name string, arguments ...Expr) (Expression[T], error) {
	return ModulePathExpressionRef[T](path.path, name, arguments...)
}

// RuntimePathScriptCall resolves one registered Go script through a runtime
// path snapshot.
func RuntimePathScriptCall[T any](path RuntimePath, name string, arguments ...Expr) (Expression[T], error) {
	return ModulePathScriptCall[T](path.path, name, arguments...)
}
