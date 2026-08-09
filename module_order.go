package esper

import (
	"fmt"
	"sort"
	"strings"
)

// ModuleOrderItem couples dependency metadata to an arbitrary immutable
// deployment artifact. Value may be a Module, ModulePath, Plan slice or an
// application-owned deployment descriptor.
type ModuleOrderItem[T any] struct {
	Name  string
	Uses  []string
	Value T
}

// NewModuleOrderItem constructs detached dependency metadata for one module.
// A blank name represents an anonymous module; dependencies are still honored.
func NewModuleOrderItem[T any](name string, value T, uses ...string) ModuleOrderItem[T] {
	return ModuleOrderItem[T]{Name: strings.TrimSpace(name), Uses: normalizeModuleUses(uses), Value: value}
}

// DependencyNames returns a detached, normalized dependency list.
func (i ModuleOrderItem[T]) DependencyNames() []string {
	return normalizeModuleUses(i.Uses)
}

type moduleOrderConfig struct {
	checkUses     bool
	checkCircular bool
}

// ModuleOrderOption configures OrderModules. Missing dependencies and cycles
// are rejected by default.
type ModuleOrderOption func(*moduleOrderConfig)

// AllowUnresolvedModuleUses keeps unknown uses declarations as ordering
// metadata but does not fail the operation. Dependencies matching a proposed
// or already-deployed module are still honored.
func AllowUnresolvedModuleUses() ModuleOrderOption {
	return func(config *moduleOrderConfig) { config.checkUses = false }
}

// AllowCircularModuleUses disables cycle rejection. OrderModules then applies
// the same deterministic first-listed fallback used for otherwise rootless
// dependency components.
func AllowCircularModuleUses() ModuleOrderOption {
	return func(config *moduleOrderConfig) { config.checkCircular = false }
}

// OrderModules returns a deterministic dependency-first deployment order.
// Dependencies satisfied by deployedModules do not create edges among the
// proposed items. Duplicate names depend on every matching item except self;
// self-use is allowed. Returned Uses slices are detached from the input.
func OrderModules[T any](modules []ModuleOrderItem[T], deployedModules []string, options ...ModuleOrderOption) ([]ModuleOrderItem[T], error) {
	config := moduleOrderConfig{checkUses: true, checkCircular: true}
	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}
	proposed := make([]ModuleOrderItem[T], len(modules))
	for index, module := range modules {
		proposed[index] = ModuleOrderItem[T]{
			Name:  strings.TrimSpace(module.Name),
			Uses:  normalizeModuleUses(module.Uses),
			Value: module.Value,
		}
	}
	if len(proposed) == 0 {
		return []ModuleOrderItem[T]{}, nil
	}

	indicesByName := make(map[string][]int)
	available := make(map[string]struct{})
	for index, module := range proposed {
		if module.Name == "" {
			continue
		}
		available[module.Name] = struct{}{}
		indicesByName[module.Name] = append(indicesByName[module.Name], index)
	}
	deployed := make(map[string]struct{}, len(deployedModules))
	for _, name := range deployedModules {
		name = strings.TrimSpace(name)
		if name != "" {
			deployed[name] = struct{}{}
		}
	}
	if config.checkUses {
		for _, module := range proposed {
			for _, dependency := range module.Uses {
				if _, exists := available[dependency]; exists {
					continue
				}
				if _, exists := deployed[dependency]; exists {
					continue
				}
				owner := "anonymous module"
				if module.Name != "" {
					owner = fmt.Sprintf("module %q", module.Name)
				}
				return nil, NewError(ErrorDependency, fmt.Sprintf("%s requires unavailable module %q", owner, dependency))
			}
		}
	}

	dependencies := make([][]int, len(proposed))
	for index, module := range proposed {
		seen := make(map[int]struct{})
		for _, dependency := range module.Uses {
			for _, dependencyIndex := range indicesByName[dependency] {
				if dependencyIndex == index {
					continue
				}
				seen[dependencyIndex] = struct{}{}
			}
		}
		for dependencyIndex := range seen {
			dependencies[index] = append(dependencies[index], dependencyIndex)
		}
		sort.Ints(dependencies[index])
	}
	if config.checkCircular {
		if cycle := firstModuleDependencyCycle(dependencies); len(cycle) > 0 {
			names := make([]string, len(cycle))
			for index, moduleIndex := range cycle {
				name := proposed[moduleIndex].Name
				if name == "" {
					name = "<anonymous>"
				}
				names[index] = name
			}
			return nil, NewError(ErrorDependency, "circular module dependency: "+strings.Join(names, " -> "))
		}
	}

	ignored := make([]bool, len(proposed))
	reverseOrder := make([]int, 0, len(proposed))
	for len(reverseOrder) < len(proposed) {
		roots := make([]int, 0)
		for candidate := range proposed {
			if ignored[candidate] {
				continue
			}
			dependedOn := false
			for dependent := range proposed {
				if ignored[dependent] {
					continue
				}
				if sortedIntsContain(dependencies[dependent], candidate) {
					dependedOn = true
					break
				}
			}
			if !dependedOn {
				roots = append(roots, candidate)
			}
		}
		if len(roots) == 0 {
			for index := range proposed {
				if !ignored[index] {
					roots = append(roots, index)
					break
				}
			}
		}
		sort.Sort(sort.Reverse(sort.IntSlice(roots)))
		for _, root := range roots {
			ignored[root] = true
			reverseOrder = append(reverseOrder, root)
		}
	}

	ordered := make([]ModuleOrderItem[T], len(proposed))
	for outputIndex := range reverseOrder {
		inputIndex := reverseOrder[len(reverseOrder)-1-outputIndex]
		ordered[outputIndex] = proposed[inputIndex]
		ordered[outputIndex].Uses = append([]string(nil), proposed[inputIndex].Uses...)
	}
	return ordered, nil
}

func normalizeModuleUses(uses []string) []string {
	result := make([]string, 0, len(uses))
	seen := make(map[string]struct{}, len(uses))
	for _, use := range uses {
		use = strings.TrimSpace(use)
		if use == "" {
			continue
		}
		if _, exists := seen[use]; exists {
			continue
		}
		seen[use] = struct{}{}
		result = append(result, use)
	}
	return result
}

func firstModuleDependencyCycle(dependencies [][]int) []int {
	for start := range dependencies {
		stack := []int{start}
		if moduleDependencyCycleFrom(dependencies, start, &stack) {
			return append([]int(nil), stack...)
		}
	}
	return nil
}

func moduleDependencyCycleFrom(dependencies [][]int, current int, stack *[]int) bool {
	for _, dependency := range dependencies[current] {
		if intsContain(*stack, dependency) {
			return true
		}
		*stack = append(*stack, dependency)
		if moduleDependencyCycleFrom(dependencies, dependency, stack) {
			return true
		}
		*stack = (*stack)[:len(*stack)-1]
	}
	return false
}

func intsContain(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sortedIntsContain(values []int, target int) bool {
	index := sort.SearchInts(values, target)
	return index < len(values) && values[index] == target
}
