package esper

import "reflect"

// CompilerProvider compiles one typed fluent query. Providers can observe,
// route, cache or wrap compilation without introducing EPL text or depending
// on an external language toolchain.
type CompilerProvider interface {
	Compile(CompilerRequest) (Plan, error)
}

// CompilerProviderFunc adapts a function to CompilerProvider.
type CompilerProviderFunc func(CompilerRequest) (Plan, error)

func (provider CompilerProviderFunc) Compile(request CompilerRequest) (Plan, error) {
	if provider == nil {
		return Plan{}, NewError(ErrorDependency, "compiler provider function is nil")
	}
	return provider(request)
}

// NativeCompilerProvider delegates to Environment.Build. It is the default
// Go-native compiler implementation and requires no external compiler binary.
type NativeCompilerProvider struct{}

func (NativeCompilerProvider) Compile(request CompilerRequest) (Plan, error) {
	return request.CompileDefault()
}

// CompilerRequest is the immutable input passed to a CompilerProvider.
// CompileDefault invokes the ordinary fluent compiler with the original
// Environment, Query and detached CompileOption slice.
type CompilerRequest struct {
	environment *Environment
	query       Query
	options     []CompileOption
}

// Environment returns the compile-time catalog supplied by the caller.
func (request CompilerRequest) Environment() *Environment {
	return request.environment
}

// Query returns the typed fluent query supplied by the caller.
func (request CompilerRequest) Query() Query {
	return request.query
}

// Description returns the query's stable typed description without compiling
// it or resolving catalog names.
func (request CompilerRequest) Description() string {
	return request.query.description()
}

// CompileDefault invokes the native Environment.Build compiler.
func (request CompilerRequest) CompileDefault() (Plan, error) {
	if request.environment == nil {
		return Plan{}, NewError(ErrorDependency, "compiler request has no environment")
	}
	return request.environment.Build(request.query, append([]CompileOption(nil), request.options...)...)
}

// CompileWithProvider compiles query through provider. A provider must return
// a valid Plan owned by env; provider errors are reported at the compiler
// provider boundary.
func CompileWithProvider(env *Environment, query Query, provider CompilerProvider, options ...CompileOption) (Plan, error) {
	if env == nil {
		return Plan{}, NewError(ErrorDependency, "compiler provider requires an environment")
	}
	if compilerProviderIsNil(provider) {
		return Plan{}, NewError(ErrorDependency, "compiler provider is nil")
	}
	plan, err := provider.Compile(CompilerRequest{
		environment: env,
		query:       query,
		options:     append([]CompileOption(nil), options...),
	})
	if err != nil {
		return Plan{}, WrapError(ErrorInvalidRule, "compiler provider", err)
	}
	if plan.Hash() == "" {
		return Plan{}, NewError(ErrorInvalidRule, "compiler provider returned an empty plan")
	}
	if plan.query.env != env {
		return Plan{}, NewError(ErrorDependency, "compiler provider returned a plan for a different environment")
	}
	return plan, nil
}

func compilerProviderIsNil(provider CompilerProvider) bool {
	if provider == nil {
		return true
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
