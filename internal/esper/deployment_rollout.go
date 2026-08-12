package esper

import (
	"context"
	"fmt"
	"reflect"
	"sort"
)

// DeploymentState identifies a committed deployment lifecycle transition.
type DeploymentState uint8

const (
	DeploymentStateDeployed DeploymentState = iota + 1
	DeploymentStateUndeployed
)

// DeploymentStateEvent is a detached lifecycle notification. RolloutItemIndex
// is zero-based for Rollout and -1 for ordinary Deploy or Undeploy calls.
type DeploymentStateEvent struct {
	State            DeploymentState
	RuntimeURI       string
	DeploymentID     string
	ModuleName       string
	Statements       []*Statement
	RolloutItemIndex int
}

// DeploymentStateListener observes committed deployment lifecycle changes.
// Callbacks run outside the Engine mutex and may safely call back into Engine.
type DeploymentStateListener interface {
	OnDeploymentState(DeploymentStateEvent)
}

// DeploymentStateListenerFunc adapts a function to DeploymentStateListener.
type DeploymentStateListenerFunc func(DeploymentStateEvent)

func (listener DeploymentStateListenerFunc) OnDeploymentState(event DeploymentStateEvent) {
	if listener != nil {
		listener(event)
	}
}

// AddDeploymentStateListener registers one Engine-local lifecycle observer.
func (e *Engine) AddDeploymentStateListener(listener DeploymentStateListener) error {
	if e == nil {
		return NewError(ErrorDependency, "nil engine")
	}
	if deploymentStateListenerIsNil(listener) {
		return NewError(ErrorInvalidRule, "deployment state listener is nil")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return NewError(ErrorState, "engine is closed")
	}
	e.deploymentStateListeners = append(e.deploymentStateListeners, listener)
	return nil
}

// RemoveDeploymentStateListener removes the first matching listener.
func (e *Engine) RemoveDeploymentStateListener(listener DeploymentStateListener) {
	if e == nil || listener == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for index, candidate := range e.deploymentStateListeners {
		if sameDeploymentStateListener(candidate, listener) {
			e.deploymentStateListeners = append(e.deploymentStateListeners[:index], e.deploymentStateListeners[index+1:]...)
			return
		}
	}
}

// DeploymentStateListeners returns a registration-order snapshot.
func (e *Engine) DeploymentStateListeners() []DeploymentStateListener {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]DeploymentStateListener(nil), e.deploymentStateListeners...)
}

// RemoveDeploymentStateListeners removes all deployment lifecycle observers.
func (e *Engine) RemoveDeploymentStateListeners() {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.deploymentStateListeners = nil
	e.mu.Unlock()
}

func sameDeploymentStateListener(left, right DeploymentStateListener) bool {
	if left == nil || right == nil {
		return left == right
	}
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	if leftValue.Type() != rightValue.Type() {
		return false
	}
	if leftValue.Type().Comparable() {
		return left == right
	}
	if leftValue.Kind() == reflect.Pointer || leftValue.Kind() == reflect.Func {
		return leftValue.Pointer() == rightValue.Pointer()
	}
	return false
}

func deploymentStateListenerIsNil(listener DeploymentStateListener) bool {
	if listener == nil {
		return true
	}
	value := reflect.ValueOf(listener)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (e *Engine) dispatchDeploymentState(event DeploymentStateEvent) {
	if e == nil {
		return
	}
	event.Statements = append([]*Statement(nil), event.Statements...)
	e.mu.Lock()
	listeners := append([]DeploymentStateListener(nil), e.deploymentStateListeners...)
	e.mu.Unlock()
	for _, listener := range listeners {
		if listener != nil {
			detached := event
			detached.Statements = append([]*Statement(nil), event.Statements...)
			listener.OnDeploymentState(detached)
		}
	}
}

// DeploymentRolloutItem describes one independently managed deployment in an
// atomic rollout. Multiple Plans in one item become one Deployment.
type DeploymentRolloutItem struct {
	plans   []Plan
	options []DeploymentOption
}

// RolloutPlans starts a fluent rollout item from one or more typed Plans.
func RolloutPlans(plans ...Plan) DeploymentRolloutItem {
	return DeploymentRolloutItem{plans: append([]Plan(nil), plans...)}
}

// WithOptions returns a detached rollout item with deployment-time options.
func (item DeploymentRolloutItem) WithOptions(options ...DeploymentOption) DeploymentRolloutItem {
	item.plans = append([]Plan(nil), item.plans...)
	item.options = append(append([]DeploymentOption(nil), item.options...), options...)
	return item
}

// Plans returns the item's immutable Plan snapshot.
func (item DeploymentRolloutItem) Plans() []Plan {
	return append([]Plan(nil), item.plans...)
}

// DeploymentRolloutResult is one ordered rollout result item.
type DeploymentRolloutResult struct {
	deployment *Deployment
}

func (result DeploymentRolloutResult) Deployment() *Deployment {
	return result.deployment
}

// DeploymentRollout contains committed result items in input order.
type DeploymentRollout struct {
	items []DeploymentRolloutResult
}

func (rollout *DeploymentRollout) Items() []DeploymentRolloutResult {
	if rollout == nil {
		return nil
	}
	return append([]DeploymentRolloutResult(nil), rollout.items...)
}

func (rollout *DeploymentRollout) Deployments() []*Deployment {
	if rollout == nil {
		return nil
	}
	deployments := make([]*Deployment, len(rollout.items))
	for index, item := range rollout.items {
		deployments[index] = item.deployment
	}
	return deployments
}

// DeploymentRolloutError reports the zero-based failing rollout item while
// preserving the underlying typed Esper error for errors.Is/errors.As.
type DeploymentRolloutError struct {
	ItemIndex int
	Err       error
}

// UndeployPreconditionError reports that an active deployment still depends
// on the requested deployment. The dependency must be removed first.
type UndeployPreconditionError struct {
	DeploymentID string
	ReferencedBy string
	// Resource is set when the blocking dependency is a module-owned catalog
	// object (named window, table, variable, context, event type or declared
	// expression) rather than a typed module-use edge.
	Resource *DeploymentResource
}

func (err *UndeployPreconditionError) Error() string {
	if err == nil {
		return "esper: undeploy precondition failed"
	}
	if err.Resource != nil {
		return fmt.Sprintf("esper: %s: %s %q cannot be un-deployed as it is referenced by deployment %q",
			ErrorDependency, err.Resource.Kind.label(), err.Resource.Name, err.ReferencedBy)
	}
	return fmt.Sprintf("esper: %s: deployment %q cannot be undeployed; referenced by active deployment %q",
		ErrorDependency, err.DeploymentID, err.ReferencedBy)
}

func (err *UndeployPreconditionError) Is(target error) bool {
	return err != nil && target == ErrorDependency
}

// DeployPreconditionError reports that a deployment references a module-owned
// catalog object whose provider module has no active deployment, mirroring
// Esper's EPDeployPreconditionException from path-dependency resolution.
// Deploy the provider module first, then deploy the consumer. RolloutItemIndex
// is -1 for ordinary Deploy calls; rollout failures surface the failing item
// through DeploymentRolloutError.RolloutItemIndex.
type DeployPreconditionError struct {
	Kind       DeploymentResourceKind
	Name       string
	ModuleName string
	// RolloutItemIndex mirrors EPDeployPreconditionException's rollout item
	// number and is -1 for non-rollout deployments.
	RolloutItemIndex int
}

func (err *DeployPreconditionError) Error() string {
	if err == nil {
		return "esper: deploy precondition failed"
	}
	message := fmt.Sprintf("Required dependency %s '%s'", err.Kind.dependencyLabel(), err.Name)
	if err.ModuleName != "" {
		message += fmt.Sprintf(" module '%s'", err.ModuleName)
	}
	message += " cannot be found"
	return fmt.Sprintf("esper: %s: A precondition is not satisfied: %s", ErrorDependency, message)
}

func (err *DeployPreconditionError) Is(target error) bool {
	return err != nil && target == ErrorDependency
}

func (err *DeploymentRolloutError) Error() string {
	if err == nil {
		return "esper: rollout failed"
	}
	return fmt.Sprintf("esper: rollout item %d: %v", err.ItemIndex, err.Err)
}

func (err *DeploymentRolloutError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

func (err *DeploymentRolloutError) RolloutItemIndex() int {
	if err == nil {
		return -1
	}
	return err.ItemIndex
}

type preparedRolloutItem struct {
	requests         []deploymentRequest
	config           deploymentConfig
	deploymentModule string
}

// Rollout atomically activates multiple deployments. Engine readers and event
// processing cannot observe a prefix because the Engine mutex is held through
// all item activations; any failure reverses prior items before releasing it.
func (e *Engine) Rollout(ctx context.Context, items ...DeploymentRolloutItem) (*DeploymentRollout, error) {
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	if e == nil || e.env == nil {
		return nil, NewError(ErrorDependency, "engine has no environment")
	}
	if len(items) == 0 {
		return nil, NewError(ErrorInvalidRule, "rollout requires at least one item")
	}
	prepared := make([]preparedRolloutItem, len(items))
	explicitIDs := make(map[string]int, len(items))
	for itemIndex, item := range items {
		config := deploymentConfig{}
		for _, option := range item.options {
			if option != nil {
				option(&config)
			}
		}
		if config.deploymentID != "" {
			if previous, exists := explicitIDs[config.deploymentID]; exists {
				return nil, rolloutError(itemIndex, NewError(ErrorDeployment,
					fmt.Sprintf("deployment id %q occurs multiple times in rollout (first item %d)", config.deploymentID, previous)))
			}
			explicitIDs[config.deploymentID] = itemIndex
		}
		requests := make([]deploymentRequest, len(item.plans))
		for index, plan := range item.plans {
			requests[index] = deploymentRequest{plan: plan}
		}
		var err error
		requests, prepared[itemIndex].deploymentModule, err = e.prepareDeploymentRequests(ctx, requests, config)
		if err != nil {
			return nil, rolloutError(itemIndex, err)
		}
		prepared[itemIndex].requests = requests
		prepared[itemIndex].config = config
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil, NewError(ErrorState, "engine is closed")
	}
	nextID := e.nextID
	activations := make([]deploymentActivation, 0, len(prepared))
	for itemIndex, item := range prepared {
		if err := e.validateRolloutDependenciesLocked(item.requests); err != nil {
			rolledBack := e.rollbackRolloutLocked(activations, nextID)
			e.mu.Unlock()
			closeRolledBackDeployments(rolledBack)
			return nil, rolloutError(itemIndex, err)
		}
		activation, err := e.deployPreparedRequestsLocked(ctx, item.requests, item.config, item.deploymentModule)
		if err != nil {
			rolledBack := e.rollbackRolloutLocked(activations, nextID)
			e.mu.Unlock()
			closeRolledBackDeployments(rolledBack)
			return nil, rolloutError(itemIndex, err)
		}
		activations = append(activations, activation)
	}
	e.mu.Unlock()

	results := make([]DeploymentRolloutResult, len(activations))
	for itemIndex, activation := range activations {
		if err := e.dispatchDeploymentActivation(ctx, activation, itemIndex); err != nil {
			return nil, rolloutError(itemIndex, err)
		}
		results[itemIndex] = DeploymentRolloutResult{deployment: activation.deployment}
	}
	return &DeploymentRollout{items: results}, nil
}

func rolloutError(itemIndex int, err error) error {
	if err == nil {
		return nil
	}
	return &DeploymentRolloutError{ItemIndex: itemIndex, Err: err}
}

func (e *Engine) validateRolloutDependenciesLocked(requests []deploymentRequest) error {
	uses := make(map[string]struct{})
	for _, request := range requests {
		for _, moduleName := range request.plan.query.moduleUses {
			moduleName = normalizeModuleName(moduleName)
			if moduleName != "" {
				uses[moduleName] = struct{}{}
			}
		}
	}
	if len(uses) == 0 {
		return nil
	}
	active := make(map[string]struct{})
	for _, deployment := range e.deployments {
		if deployment != nil && deployment.moduleName != "" {
			active[normalizeModuleName(deployment.moduleName)] = struct{}{}
		}
	}
	names := make([]string, 0, len(uses))
	for moduleName := range uses {
		names = append(names, moduleName)
	}
	sort.Strings(names)
	for _, moduleName := range names {
		if _, exists := active[moduleName]; !exists {
			return NewError(ErrorDependency, fmt.Sprintf("required deployment module %q is not active", moduleName))
		}
	}
	return nil
}

func (e *Engine) rollbackRolloutLocked(activations []deploymentActivation, nextID uint64) []*Deployment {
	rolledBack := make([]*Deployment, 0, len(activations))
	for index := len(activations) - 1; index >= 0; index-- {
		deployment := activations[index].deployment
		if deployment == nil {
			continue
		}
		delete(e.deployments, deployment.id)
		e.removeDeploymentResourceDependentsLocked(deployment.id)
		for _, statement := range deployment.statements {
			e.removeStatementMetricsLocked(statement)
			delete(e.statements, statement.id)
			statement.markClosedLocked()
		}
		if deployment.moduleName != "" {
			if definition, ok := e.env.moduleDefinition(deployment.moduleName); ok && definition.visibility == ModuleProtected {
				e.deactivateProtectedModuleLocked(deployment.moduleName)
			}
		}
		deployment.mu.Lock()
		deployment.closed = true
		deployment.mu.Unlock()
		rolledBack = append(rolledBack, deployment)
	}
	e.nextID = nextID
	e.pendingContextEvents = nil
	e.pendingAuditRecords = nil
	return rolledBack
}

func closeRolledBackDeployments(deployments []*Deployment) {
	for _, deployment := range deployments {
		_ = closeDeploymentSinks(deployment)
	}
}
