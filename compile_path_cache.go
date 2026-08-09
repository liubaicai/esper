package esper

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// CompilePathCache caches immutable typed catalog-resolution snapshots. It is
// the Go counterpart of Esper's compiler path cache without carrying JVM
// bytecode or class-loader state. A catalog change produces a new fingerprint;
// existing snapshots remain immutable and safe for concurrent use.
type CompilePathCache struct {
	mu      sync.RWMutex
	entries map[string]*compilePathSnapshot
	hits    uint64
	misses  uint64
}

type compilePathSnapshot struct {
	fingerprint string
	resolutions map[moduleObjectKind]map[string]string
}

// CompilePathCacheStats is a detached point-in-time cache summary.
type CompilePathCacheStats struct {
	Hits    uint64
	Misses  uint64
	Entries int
}

// NewCompilePathCache creates an application-owned cache. The zero value is
// also ready to use. Explicit ownership avoids the process-global mutable
// singleton used by the Java compiler.
func NewCompilePathCache() *CompilePathCache {
	return &CompilePathCache{entries: make(map[string]*compilePathSnapshot)}
}

// Stats returns a detached cache summary.
func (c *CompilePathCache) Stats() CompilePathCacheStats {
	if c == nil {
		return CompilePathCacheStats{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return CompilePathCacheStats{Hits: c.hits, Misses: c.misses, Entries: len(c.entries)}
}

// Clear removes all snapshots and resets counters.
func (c *CompilePathCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries = make(map[string]*compilePathSnapshot)
	c.hits = 0
	c.misses = 0
	c.mu.Unlock()
}

// CachedModulePath is an immutable catalog snapshot backed by a
// CompilePathCache entry.
type CachedModulePath struct {
	path     ModulePath
	snapshot *compilePathSnapshot
}

// Prepare snapshots and validates every object visible through path. Reusing
// an unchanged path returns the cached snapshot; ambiguous duplicate exports
// fail before a query Plan is produced.
func (c *CompilePathCache) Prepare(path ModulePath) (CachedModulePath, error) {
	if c == nil {
		return CachedModulePath{}, NewError(ErrorDependency, "nil compile path cache")
	}
	fingerprint, resolutions, err := buildCompilePathSnapshot(path)
	if err != nil {
		return CachedModulePath{}, err
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[string]*compilePathSnapshot)
	}
	if snapshot, ok := c.entries[fingerprint]; ok {
		c.hits++
		c.mu.Unlock()
		return CachedModulePath{path: path, snapshot: snapshot}, nil
	}
	c.mu.Unlock()

	snapshot := &compilePathSnapshot{fingerprint: fingerprint, resolutions: resolutions}

	c.mu.Lock()
	if existing, ok := c.entries[fingerprint]; ok {
		c.hits++
		snapshot = existing
	} else {
		c.entries[fingerprint] = snapshot
		c.misses++
	}
	c.mu.Unlock()
	return CachedModulePath{path: path, snapshot: snapshot}, nil
}

func buildCompilePathSnapshot(path ModulePath) (string, map[moduleObjectKind]map[string]string, error) {
	if path.err != nil {
		return "", nil, path.err
	}
	if path.env == nil {
		return "", nil, NewError(ErrorDependency, "module path has no environment")
	}
	visibleModules := make(map[string]struct{})
	path.env.mu.RLock()
	if path.moduleName != "" {
		visibleModules[path.moduleName] = struct{}{}
	}
	if path.selected || len(path.uses) > 0 {
		for _, moduleName := range path.uses {
			definition, exists := path.env.modules[moduleName]
			if !exists {
				continue
			}
			if definition.visibility != ModulePublic && moduleName != path.moduleName {
				path.env.mu.RUnlock()
				return "", nil, NewError(ErrorUnknownName, fmt.Sprintf("module %q is not public", moduleName))
			}
			visibleModules[moduleName] = struct{}{}
		}
	} else {
		for moduleName, definition := range path.env.modules {
			if definition.visibility == ModulePublic {
				visibleModules[moduleName] = struct{}{}
			}
		}
	}

	identities := make(map[moduleObjectKind][]string)
	owners := make(map[string]string)
	appendVisible := func(kind moduleObjectKind, identity string) {
		owner := path.env.moduleObjects[identity]
		if owner == "" {
			identities[kind] = append(identities[kind], identity)
			owners[identity] = ""
			return
		}
		if _, visible := visibleModules[owner]; visible {
			identities[kind] = append(identities[kind], identity)
			owners[identity] = owner
		}
	}
	for identity := range path.env.schemas {
		appendVisible(moduleObjectEventType, identity)
	}
	for identity := range path.env.variables {
		appendVisible(moduleObjectVariable, identity)
	}
	for identity := range path.env.contexts {
		appendVisible(moduleObjectContext, identity)
	}
	for identity := range path.env.namedWindows {
		appendVisible(moduleObjectNamedWindow, identity)
	}
	for identity := range path.env.tables {
		appendVisible(moduleObjectTable, identity)
	}
	for identity := range path.env.expressions {
		appendVisible(moduleObjectExpression, identity)
	}
	for identity := range path.env.scripts {
		appendVisible(moduleObjectScript, identity)
	}
	path.env.mu.RUnlock()

	resolutions := make(map[moduleObjectKind]map[string]string, len(identities))
	parts := []string{fmt.Sprintf("env=%p", path.env), "module=" + path.moduleName, fmt.Sprintf("selected=%t", path.selected), "uses=" + strings.Join(path.uses, ",")}
	kinds := []moduleObjectKind{
		moduleObjectEventType,
		moduleObjectVariable,
		moduleObjectContext,
		moduleObjectNamedWindow,
		moduleObjectTable,
		moduleObjectExpression,
		moduleObjectScript,
	}
	for _, kind := range kinds {
		values := identities[kind]
		sort.Strings(values)
		seen := make(map[string]struct{}, len(values))
		for _, identity := range values {
			_, logicalName := splitCatalogKey(identity)
			seen[logicalName] = struct{}{}
		}
		names := make([]string, 0, len(seen))
		for name := range seen {
			names = append(names, name)
		}
		sort.Strings(names)
		resolved := make(map[string]string, len(names))
		identitySet := make(map[string]struct{}, len(values))
		for _, identity := range values {
			identitySet[identity] = struct{}{}
		}
		for _, name := range names {
			identity, resolveErr := resolveCompilePathSnapshot(path, kind, name, identitySet, owners)
			if resolveErr != nil {
				return "", nil, resolveErr
			}
			resolved[name] = identity
		}
		resolutions[kind] = resolved
		parts = append(parts, fmt.Sprintf("%d=%s", kind, strings.Join(values, ",")))
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return fmt.Sprintf("%x", digest[:]), resolutions, nil
}

func resolveCompilePathSnapshot(path ModulePath, kind moduleObjectKind, logicalName string, identities map[string]struct{}, owners map[string]string) (string, error) {
	globalIdentity := logicalName
	_, global := identities[globalIdentity]
	if path.moduleName != "" {
		own := catalogKey(path.moduleName, logicalName)
		if _, exists := identities[own]; exists {
			return own, nil
		}
	}
	if path.selected || len(path.uses) > 0 {
		candidates := make([]string, 0, len(path.uses))
		for _, moduleName := range path.uses {
			identity := catalogKey(moduleName, logicalName)
			if _, exists := identities[identity]; exists {
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
			return globalIdentity, nil
		}
		return "", NewError(ErrorUnknownName, fmt.Sprintf("%s %q is not visible through the selected modules", kind.label(), logicalName))
	}
	candidates := make([]string, 0)
	for identity := range identities {
		if owners[identity] == "" {
			continue
		}
		_, name := splitCatalogKey(identity)
		if name == logicalName {
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
		return globalIdentity, nil
	}
	return "", NewError(ErrorUnknownName, fmt.Sprintf("%s %q is not visible", kind.label(), logicalName))
}

// Fingerprint returns the stable in-process identity of this cached catalog.
func (p CachedModulePath) Fingerprint() string {
	if p.snapshot == nil {
		return ""
	}
	return p.snapshot.fingerprint
}

// Path returns the underlying immutable module selection.
func (p CachedModulePath) Path() ModulePath { return p.path }

func (p CachedModulePath) resolve(kind moduleObjectKind, name string) (string, error) {
	if p.snapshot == nil {
		return "", NewError(ErrorDependency, "compile path cache snapshot is empty")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", NewError(ErrorInvalidRule, kind.label()+" name is required")
	}
	if identity, ok := p.snapshot.resolutions[kind][name]; ok {
		return identity, nil
	}
	return "", NewError(ErrorUnknownName, fmt.Sprintf("%s %q is not visible through the cached path", kind.label(), name))
}

func (p CachedModulePath) EventType(name string) (string, error) {
	return p.resolve(moduleObjectEventType, name)
}

func (p CachedModulePath) Variable(name string) (string, error) {
	return p.resolve(moduleObjectVariable, name)
}

func (p CachedModulePath) Context(name string) (string, error) {
	return p.resolve(moduleObjectContext, name)
}

func (p CachedModulePath) NamedWindow(name string) (RecordStream, error) {
	identity, err := p.resolve(moduleObjectNamedWindow, name)
	if err != nil {
		return RecordStream{}, err
	}
	moduleName, objectName := splitCatalogKey(identity)
	return FromNamedWindowInModule(p.path.env, moduleName, objectName), nil
}

func (p CachedModulePath) Table(name string) (RecordStream, error) {
	identity, err := p.resolve(moduleObjectTable, name)
	if err != nil {
		return RecordStream{}, err
	}
	moduleName, objectName := splitCatalogKey(identity)
	return FromTableInModule(p.path.env, moduleName, objectName), nil
}

// Build binds the cached path's module and uses identity to a typed query.
func (p CachedModulePath) Build(query Query, options ...CompileOption) (Plan, error) {
	if p.snapshot == nil {
		return Plan{}, NewError(ErrorDependency, "compile path cache snapshot is empty")
	}
	if query.env != p.path.env {
		return Plan{}, NewError(ErrorDependency, "query belongs to a different or nil environment")
	}
	query.moduleName = p.path.moduleName
	query.moduleUses = append([]string(nil), p.path.uses...)
	return p.path.env.Build(query, options...)
}

func CachedPathVariableRef[T any](path CachedModulePath, name string) (Expression[T], error) {
	identity, err := path.resolve(moduleObjectVariable, name)
	if err != nil {
		return nil, err
	}
	return VariableRef[T](identity), nil
}

func CachedPathExpressionRef[T any](path CachedModulePath, name string, arguments ...Expr) (Expression[T], error) {
	identity, err := path.resolve(moduleObjectExpression, name)
	if err != nil {
		return nil, err
	}
	return ExpressionRef[T](path.path.env, identity, arguments...), nil
}

func CachedPathScriptCall[T any](path CachedModulePath, name string, arguments ...Expr) (Expression[T], error) {
	identity, err := path.resolve(moduleObjectScript, name)
	if err != nil {
		return nil, err
	}
	return ScriptCall[T](path.path.env, identity, arguments...), nil
}
