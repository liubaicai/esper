package esper

import (
	"fmt"
	"sort"
	"strings"
)

// DeploymentDependencyConsumedItem names one provider deployment and the
// module-owned catalog object consumed from it, mirroring Esper's
// EPDeploymentDependencyConsumed.Item.
type DeploymentDependencyConsumedItem struct {
	DeploymentID string
	Kind         DeploymentResourceKind
	Name         string
}

// DeploymentDependencyConsumed is the detached, deterministically ordered
// list of catalog objects one deployment consumes from other deployments.
type DeploymentDependencyConsumed struct {
	Items []DeploymentDependencyConsumedItem
}

// DeploymentDependencyProvidedItem names one module-owned catalog object the
// deployment provides and the consumer deployments referencing it, mirroring
// Esper's EPDeploymentDependencyProvided.Item.
type DeploymentDependencyProvidedItem struct {
	Kind        DeploymentResourceKind
	Name        string
	ConsumerIDs []string
}

// DeploymentDependencyProvided is the detached, deterministically ordered
// list of catalog objects one deployment provides to other deployments. Only
// objects with at least one consumer are listed, matching Esper.
type DeploymentDependencyProvided struct {
	Items []DeploymentDependencyProvidedItem
}

// DeploymentDependenciesConsumed lists the provider deployments and objects
// one active deployment consumes. The boolean result is false for unknown or
// blank deployment IDs, matching Esper's null return for unknown deployments;
// Go has no null deployment IDs, so blank IDs also report false.
func (e *Engine) DeploymentDependenciesConsumed(deploymentID string) (DeploymentDependencyConsumed, bool) {
	if e == nil {
		return DeploymentDependencyConsumed{}, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	deployment, ok := e.deployments[strings.TrimSpace(deploymentID)]
	if !ok || deployment == nil {
		return DeploymentDependencyConsumed{}, false
	}
	items := make([]DeploymentDependencyConsumedItem, 0)
	for ref, dependents := range e.resourceDependents {
		if _, isConsumer := dependents[deployment.id]; !isConsumer {
			continue
		}
		provider := e.moduleProviderDeploymentLocked(ref.moduleName())
		if provider == nil || provider.id == deployment.id {
			continue
		}
		items = append(items, DeploymentDependencyConsumedItem{
			DeploymentID: provider.id,
			Kind:         ref.kind,
			Name:         e.dependencyObjectNameLocked(ref),
		})
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Kind != items[right].Kind {
			return items[left].Kind < items[right].Kind
		}
		if items[left].Name != items[right].Name {
			return items[left].Name < items[right].Name
		}
		return items[left].DeploymentID < items[right].DeploymentID
	})
	return DeploymentDependencyConsumed{Items: items}, true
}

// DeploymentDependenciesProvided lists the catalog objects one active
// deployment provides to consumers. A deployment provides its module's
// catalog while it is the earliest active deployment of that module; other
// deployments report an empty list. The boolean result is false for unknown
// or blank deployment IDs.
func (e *Engine) DeploymentDependenciesProvided(deploymentID string) (DeploymentDependencyProvided, bool) {
	if e == nil {
		return DeploymentDependencyProvided{}, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	deployment, ok := e.deployments[strings.TrimSpace(deploymentID)]
	if !ok || deployment == nil {
		return DeploymentDependencyProvided{}, false
	}
	items := make([]DeploymentDependencyProvidedItem, 0)
	moduleName := normalizeModuleName(deployment.moduleName)
	if provider := e.moduleProviderDeploymentLocked(moduleName); provider != nil && provider.id == deployment.id {
		for ref, dependents := range e.resourceDependents {
			if ref.moduleName() != moduleName || len(dependents) == 0 {
				continue
			}
			consumers := make([]string, 0, len(dependents))
			for consumerID := range dependents {
				consumers = append(consumers, consumerID)
			}
			sort.Strings(consumers)
			items = append(items, DeploymentDependencyProvidedItem{
				Kind:        ref.kind,
				Name:        e.dependencyObjectNameLocked(ref),
				ConsumerIDs: consumers,
			})
		}
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Kind != items[right].Kind {
			return items[left].Kind < items[right].Kind
		}
		return items[left].Name < items[right].Name
	})
	return DeploymentDependencyProvided{Items: items}, true
}

// dependencyObjectNameLocked renders the dependency-list object name. Script
// identities use Esper's name#paramNum form ("MyScript#1") when the
// registered definition declares its argument types; every other kind keeps
// the logical object name. The engine mutex must be held.
func (e *Engine) dependencyObjectNameLocked(ref deploymentResourceRef) string {
	_, name := splitCatalogKey(ref.identity)
	if ref.kind != DeploymentResourceScript || e == nil || e.env == nil {
		return name
	}
	e.env.mu.RLock()
	definition, ok := e.env.scripts[ref.identity]
	e.env.mu.RUnlock()
	if !ok || !definition.argumentTypesSet {
		return name
	}
	return fmt.Sprintf("%s#%d", name, len(definition.argumentTypes))
}
