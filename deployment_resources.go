package esper

import (
	"fmt"
	"sort"
	"strings"
)

// DeploymentResourceKind classifies the module-owned catalog objects whose
// provider/consumer edges block an unsafe undeploy. The declaration order
// mirrors Esper's undeploy precondition check order (named windows, tables,
// variables, contexts, event types, declared expressions); scripts, indexes
// and application classes are separate follow-up slices.
type DeploymentResourceKind uint8

const (
	DeploymentResourceNamedWindow DeploymentResourceKind = iota
	DeploymentResourceTable
	DeploymentResourceVariable
	DeploymentResourceContext
	DeploymentResourceEventType
	DeploymentResourceExpression
	DeploymentResourceScript
)

// label matches the object-type wording Esper uses in its undeploy
// precondition messages ("Named window 'W' cannot be un-deployed ...").
func (k DeploymentResourceKind) label() string {
	switch k {
	case DeploymentResourceNamedWindow:
		return "Named window"
	case DeploymentResourceTable:
		return "Table"
	case DeploymentResourceVariable:
		return "Variable"
	case DeploymentResourceContext:
		return "Context"
	case DeploymentResourceEventType:
		return "Event type"
	case DeploymentResourceExpression:
		return "Declared-expression"
	case DeploymentResourceScript:
		return "Script"
	default:
		return "Resource"
	}
}

// dependencyLabel matches the lowercase object-type wording Esper's
// PathRegistryObjectType uses in deploy precondition messages ("Required
// dependency named window 'W' module 'M' cannot be found").
func (k DeploymentResourceKind) dependencyLabel() string {
	switch k {
	case DeploymentResourceNamedWindow:
		return "named window"
	case DeploymentResourceTable:
		return "table"
	case DeploymentResourceVariable:
		return "variable"
	case DeploymentResourceContext:
		return "context"
	case DeploymentResourceEventType:
		return "event type"
	case DeploymentResourceExpression:
		return "declared-expression"
	case DeploymentResourceScript:
		return "script"
	default:
		return "resource"
	}
}

// moduleObjectKind maps a resource kind to the catalog bucket consulted for
// the registration half of the deploy provider check.
func (k DeploymentResourceKind) moduleObjectKind() (moduleObjectKind, bool) {
	switch k {
	case DeploymentResourceNamedWindow:
		return moduleObjectNamedWindow, true
	case DeploymentResourceTable:
		return moduleObjectTable, true
	case DeploymentResourceVariable:
		return moduleObjectVariable, true
	case DeploymentResourceContext:
		return moduleObjectContext, true
	case DeploymentResourceEventType:
		return moduleObjectEventType, true
	case DeploymentResourceExpression:
		return moduleObjectExpression, true
	case DeploymentResourceScript:
		return moduleObjectScript, true
	default:
		return 0, false
	}
}

// deployPreconditionPathOrder is the path-object check order of Esper's
// DeployerHelperResolver.resolveDependencies: named windows, tables, event
// types, variables, contexts, declared expressions, then scripts. The first
// unsatisfied reference in this order determines the reported precondition.
var deployPreconditionPathOrder = []DeploymentResourceKind{
	DeploymentResourceNamedWindow,
	DeploymentResourceTable,
	DeploymentResourceEventType,
	DeploymentResourceVariable,
	DeploymentResourceContext,
	DeploymentResourceExpression,
	DeploymentResourceScript,
}

// DeploymentResource names one module-owned catalog object in an undeploy
// precondition report. Name is the logical object name as registered inside
// its module; ModuleName is the owning module.
type DeploymentResource struct {
	Kind       DeploymentResourceKind
	Name       string
	ModuleName string
}

// deploymentResourceRef is the internal catalog-identity form of a
// DeploymentResource. The identity keeps the "module::name" catalog key so
// two modules may expose the same logical name without collision.
type deploymentResourceRef struct {
	kind     DeploymentResourceKind
	identity string
}

func (r deploymentResourceRef) moduleName() string {
	moduleName, _ := splitCatalogKey(r.identity)
	return moduleName
}

func (r deploymentResourceRef) resource() DeploymentResource {
	moduleName, name := splitCatalogKey(r.identity)
	return DeploymentResource{Kind: r.kind, Name: name, ModuleName: moduleName}
}

// deploymentResourceCollector gathers the module-owned catalog references of
// one query. References to objects owned by the referencing deployment's own
// module are internal only while that deployment provides the module catalog;
// the deploy path decides which collected references become edges.
type deploymentResourceCollector struct {
	env  *Environment
	seen map[deploymentResourceRef]struct{}
	refs []deploymentResourceRef
}

func newDeploymentResourceCollector(env *Environment) *deploymentResourceCollector {
	return &deploymentResourceCollector{env: env, seen: make(map[deploymentResourceRef]struct{})}
}

func (c *deploymentResourceCollector) add(kind DeploymentResourceKind, identity string) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return
	}
	moduleName, _ := splitCatalogKey(identity)
	if moduleName == "" {
		// Preconfigured (module-less) catalog objects are not deployment-owned
		// and therefore never block an undeploy.
		return
	}
	ref := deploymentResourceRef{kind: kind, identity: identity}
	if _, exists := c.seen[ref]; exists {
		return
	}
	c.seen[ref] = struct{}{}
	c.refs = append(c.refs, ref)
}

// eventType records an event-type reference together with the direct nested
// property schemas of the referenced schema. Esper records schema-property
// event types as deployment dependencies of the referencing deployment (see
// ClientDeployListDependencyStar); Go resolves the same direct references
// from the immutable schema metadata. Module-less nested schemas are filtered
// by add because they are not deployment-owned.
func (c *deploymentResourceCollector) eventType(identity string) {
	c.add(DeploymentResourceEventType, identity)
	if c.env == nil {
		return
	}
	identity = strings.TrimSpace(identity)
	c.env.mu.RLock()
	schema, ok := c.env.schemas[identity]
	c.env.mu.RUnlock()
	if !ok {
		return
	}
	for _, nestedName := range schema.NestedSchemaNames() {
		nested, ok := schema.NestedSchema(nestedName)
		if !ok {
			continue
		}
		c.add(DeploymentResourceEventType, nested.Name())
	}
}

func (c *deploymentResourceCollector) stream(node *streamNode) {
	for current := node; current != nil; current = current.input {
		switch current.kind {
		case streamNamedWindow:
			c.add(DeploymentResourceNamedWindow, catalogKey(current.moduleName, current.sourceName))
		case streamTable:
			c.add(DeploymentResourceTable, catalogKey(current.moduleName, current.sourceName))
		case streamSource:
			if !current.isAlias {
				c.eventType(current.sourceName)
			}
		}
		if current.pattern != nil {
			for _, input := range patternDefinitionInputs(current.pattern) {
				c.stream(input)
			}
		}
	}
}

func (c *deploymentResourceCollector) expression(expression Expr) {
	if expression == nil || expression.node() == nil {
		return
	}
	c.expressionNode(expression.node())
}

func (c *deploymentResourceCollector) expressionNode(node *exprNode) {
	if node == nil {
		return
	}
	switch node.kind {
	case "variable":
		c.add(DeploymentResourceVariable, node.variableName)
	case "expression-ref":
		c.add(DeploymentResourceExpression, node.expressionName)
	case "script":
		c.add(DeploymentResourceScript, node.scriptName)
	}
	for _, child := range node.children {
		c.expressionNode(child)
	}
	if node.subquery != nil {
		c.stream(node.subquery.source)
		c.expression(node.subquery.predicate)
		c.expression(node.subquery.projection)
		for _, selection := range node.subquery.columns {
			c.expression(selection.Expr)
		}
		c.expression(node.subquery.groupBy)
		c.expression(node.subquery.having)
		for _, order := range node.subquery.orderBy {
			c.expression(order.Expression)
		}
	}
}

// collectDeploymentResourceReferences walks one immutable plan and returns
// the module-owned catalog objects the plan references, in first-observed
// order. Same-module ownership and dependent filtering happen at record time.
func collectDeploymentResourceReferences(query Query) []deploymentResourceRef {
	collector := newDeploymentResourceCollector(query.env)
	collector.stream(query.input)
	if query.aggregate != nil {
		collector.stream(query.aggregate.input)
	}
	if query.join != nil {
		for _, source := range joinDefinitionSources(query.join) {
			collector.stream(source)
		}
	}
	if query.pattern != nil {
		for _, input := range patternDefinitionInputs(query.pattern) {
			collector.stream(input)
		}
	}
	if query.rowRecog != nil {
		collector.stream(query.rowRecog.input)
	}
	if query.trigger != nil {
		collector.stream(query.trigger.input)
		switch query.trigger.target {
		case triggerTargetTable:
			collector.add(DeploymentResourceTable, catalogKey(query.trigger.moduleName, query.trigger.table))
		case triggerTargetNamedWindow:
			collector.add(DeploymentResourceNamedWindow, catalogKey(query.trigger.moduleName, query.trigger.table))
		}
		if query.trigger.sourceTable != "" {
			collector.add(DeploymentResourceNamedWindow, catalogKey(query.trigger.moduleName, query.trigger.sourceTable))
		}
		for _, assignment := range query.trigger.variableAssignments {
			collector.add(DeploymentResourceVariable, assignment.Name)
		}
	}
	if query.routeTarget != "" {
		collector.eventType(query.routeTarget)
	}
	if query.tableTarget != "" {
		collector.add(DeploymentResourceTable, query.tableTarget)
	}
	if query.contextName != "" {
		collector.add(DeploymentResourceContext, query.contextName)
	}
	_ = visitQueryExpressions(nil, query, func(expression Expr) error {
		collector.expression(expression)
		return nil
	})
	return collector.refs
}

// recordDeploymentResourceDependentsLocked indexes the catalog references of a
// freshly activated deployment. References to module-less (preconfigured)
// objects are not deployment-owned and never become edges. References to the
// deployment's own module are internal while the deployment provides the
// module catalog (it is the earliest active deployment of that module); a
// later same-module deployment depends on the provider instead, mirroring
// Java's per-deployment path registry dependency entries. The engine mutex
// must be held.
func (e *Engine) recordDeploymentResourceDependentsLocked(deployment *Deployment, requests []deploymentRequest) {
	if e == nil || deployment == nil {
		return
	}
	moduleName := normalizeModuleName(deployment.moduleName)
	provider := e.moduleProviderDeploymentLocked(moduleName)
	for _, request := range requests {
		for _, ref := range collectDeploymentResourceReferences(request.plan.query) {
			if ref.moduleName() == "" {
				continue
			}
			if ref.moduleName() == moduleName && provider != nil && provider.id == deployment.id {
				// The provider's references to its own module catalog are internal.
				continue
			}
			dependents := e.resourceDependents[ref]
			if dependents == nil {
				dependents = make(map[string]struct{})
				e.resourceDependents[ref] = dependents
			}
			dependents[deployment.id] = struct{}{}
		}
	}
}

// moduleProviderDeploymentLocked returns the earliest active deployment that
// provides the module's catalog, or nil when the module has no active
// deployment. The engine mutex must be held.
func (e *Engine) moduleProviderDeploymentLocked(moduleName string) *Deployment {
	moduleName = normalizeModuleName(moduleName)
	if e == nil || moduleName == "" {
		return nil
	}
	var provider *Deployment
	for _, deployment := range e.deployments {
		if deployment == nil || normalizeModuleName(deployment.moduleName) != moduleName {
			continue
		}
		if provider == nil || deployment.order < provider.order {
			provider = deployment
		}
	}
	return provider
}

// removeDeploymentResourceDependentsLocked drops every dependent edge owned by
// one deployment. The engine mutex must be held.
func (e *Engine) removeDeploymentResourceDependentsLocked(deploymentID string) {
	if e == nil || deploymentID == "" {
		return
	}
	for ref, dependents := range e.resourceDependents {
		delete(dependents, deploymentID)
		if len(dependents) == 0 {
			delete(e.resourceDependents, ref)
		}
	}
}

// deploymentResourcePreconditionLocked reports the first module-owned catalog
// object, in Esper's check order, that still has an active external dependent.
// A deployment provides its module's catalog while it is the earliest active
// deployment of that module; a later same-module deployment leaves provision
// to the earlier one. The engine mutex must be held.
func (e *Engine) deploymentResourcePreconditionLocked(deployment *Deployment) *UndeployPreconditionError {
	if e == nil || deployment == nil {
		return nil
	}
	moduleName := normalizeModuleName(deployment.moduleName)
	if moduleName == "" {
		return nil
	}
	for _, other := range e.deployments {
		if other != nil && other.id != deployment.id && normalizeModuleName(other.moduleName) == moduleName && other.order < deployment.order {
			return nil
		}
	}
	blocked := make([]deploymentResourceRef, 0)
	for ref, dependents := range e.resourceDependents {
		if ref.moduleName() != moduleName {
			continue
		}
		hasExternal := false
		for dependentID := range dependents {
			if dependentID != deployment.id {
				hasExternal = true
				break
			}
		}
		if hasExternal {
			blocked = append(blocked, ref)
		}
	}
	if len(blocked) == 0 {
		return nil
	}
	sort.Slice(blocked, func(left, right int) bool {
		if blocked[left].kind != blocked[right].kind {
			return blocked[left].kind < blocked[right].kind
		}
		return blocked[left].identity < blocked[right].identity
	})
	ref := blocked[0]
	dependent := e.earliestResourceDependentLocked(ref, deployment.id)
	resource := ref.resource()
	if ref.kind == DeploymentResourceScript {
		resource.Name = e.scriptResourceNameLocked(ref)
	}
	return &UndeployPreconditionError{DeploymentID: deployment.id, ReferencedBy: dependent, Resource: &resource}
}

// scriptResourceNameLocked renders Esper's NameAndParamNum identity form
// ("myscript (1 parameters)") when the registered definition declares its
// argument types; providers without declared argument types keep the plain
// logical name. The engine mutex must be held.
func (e *Engine) scriptResourceNameLocked(ref deploymentResourceRef) string {
	_, name := splitCatalogKey(ref.identity)
	if e == nil || e.env == nil {
		return name
	}
	e.env.mu.RLock()
	definition, ok := e.env.scripts[ref.identity]
	e.env.mu.RUnlock()
	if !ok || !definition.argumentTypesSet {
		return name
	}
	return fmt.Sprintf("%s (%d parameters)", name, len(definition.argumentTypes))
}

// earliestResourceDependentLocked returns the earliest active deployment, in
// deployment order, that still references the catalog object.
func (e *Engine) earliestResourceDependentLocked(ref deploymentResourceRef, excludeID string) string {
	dependents := e.resourceDependents[ref]
	var candidates []*Deployment
	for dependentID := range dependents {
		if dependentID == excludeID {
			continue
		}
		if deployment, ok := e.deployments[dependentID]; ok && deployment != nil {
			candidates = append(candidates, deployment)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].order != candidates[right].order {
			return candidates[left].order < candidates[right].order
		}
		return candidates[left].id < candidates[right].id
	})
	return candidates[0].id
}

// deploymentPathPreconditionLocked resolves every cross-module catalog
// reference of a pending deployment against the active provider deployments,
// mirroring Esper's DeployerHelperResolver.resolveDependencies. A reference
// is satisfied when its owning module has an active deployment and the object
// is registered in the environment catalog; references owned by the pending
// deployment's own module are provided by the deployment itself. The engine
// mutex must be held.
func (e *Engine) deploymentPathPreconditionLocked(requests []deploymentRequest, deploymentModule string) *DeployPreconditionError {
	if e == nil || e.env == nil {
		return nil
	}
	refsByKind := make(map[DeploymentResourceKind]map[string]struct{})
	for _, request := range requests {
		for _, ref := range collectDeploymentResourceReferences(request.plan.query) {
			moduleName := ref.moduleName()
			if moduleName == "" || moduleName == deploymentModule {
				// Preconfigured (module-less) objects are validated when the
				// plan is built; same-module references are provided by this
				// deployment.
				continue
			}
			identities := refsByKind[ref.kind]
			if identities == nil {
				identities = make(map[string]struct{})
				refsByKind[ref.kind] = identities
			}
			identities[ref.identity] = struct{}{}
		}
	}
	if len(refsByKind) == 0 {
		return nil
	}
	for _, kind := range deployPreconditionPathOrder {
		identities := refsByKind[kind]
		if len(identities) == 0 {
			continue
		}
		sorted := make([]string, 0, len(identities))
		for identity := range identities {
			sorted = append(sorted, identity)
		}
		sort.Strings(sorted)
		for _, identity := range sorted {
			moduleName, name := splitCatalogKey(identity)
			if e.hasActiveModuleDeploymentLocked(moduleName) && e.env.hasModuleObject(kind, identity) {
				continue
			}
			return &DeployPreconditionError{Kind: kind, Name: name, ModuleName: moduleName, RolloutItemIndex: -1}
		}
	}
	return nil
}

// hasActiveModuleDeploymentLocked reports whether any active deployment
// provides the module's catalog, matching the path-registry provider lookup
// Esper performs at deploy time. The engine mutex must be held.
func (e *Engine) hasActiveModuleDeploymentLocked(moduleName string) bool {
	if e == nil || moduleName == "" {
		return false
	}
	for _, deployment := range e.deployments {
		if deployment != nil && normalizeModuleName(deployment.moduleName) == moduleName {
			return true
		}
	}
	return false
}
