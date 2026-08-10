package esper

import "fmt"

// DuplicateModuleObjectError reports that a module-owned catalog object is
// registered while an object with the same catalog identity already exists.
// It is the Go registration-time equivalent of Esper's deploy-time
// PathExceptionAlreadyRegistered surfaced as EPDeployPreconditionException:
// Go materializes EPL create-window/table/schema/variable/expression/script/
// context statements as Environment or Module registrations, so the duplicate
// precondition is enforced at the registration boundary instead of at deploy
// time. The message preserves Esper's final text so parity tests can compare
// against the Java assertion verbatim.
type DuplicateModuleObjectError struct {
	Kind       DeploymentResourceKind
	Name       string
	ModuleName string
}

func (err *DuplicateModuleObjectError) Error() string {
	if err == nil {
		return "esper: duplicate module object"
	}
	moduleName := err.ModuleName
	if moduleName == "" {
		// Java renders the module-less bucket through
		// StringValue.unnamedWhenNullOrEmpty.
		moduleName = "unnamed"
	}
	return fmt.Sprintf("esper: %s: A precondition is not satisfied: %s by name '%s' has already been created for module '%s'",
		ErrorDependency, err.Kind.duplicateLabel(), err.Name, moduleName)
}

// Is classifies the duplicate as a dependency-category error, matching the
// existing classification of deploy-time path preconditions.
func (err *DuplicateModuleObjectError) Is(target error) bool {
	return err != nil && target == ErrorDependency
}

// duplicateLabel returns the article-prefixed object-type wording Esper's
// PathRegistryObjectType uses in already-registered messages ("A named
// window", "An event type").
func (k DeploymentResourceKind) duplicateLabel() string {
	switch k {
	case DeploymentResourceNamedWindow:
		return "A named window"
	case DeploymentResourceTable:
		return "A table"
	case DeploymentResourceVariable:
		return "A variable"
	case DeploymentResourceContext:
		return "A context"
	case DeploymentResourceEventType:
		return "An event type"
	case DeploymentResourceExpression:
		return "A declared-expression"
	case DeploymentResourceScript:
		return "A script"
	default:
		return "A resource"
	}
}

// duplicateModuleObjectError builds the typed duplicate error for one catalog
// identity. The identity carries the owning module so the reported module
// name matches the Java module that attempted the duplicate registration.
func duplicateModuleObjectError(kind DeploymentResourceKind, identity string) *DuplicateModuleObjectError {
	moduleName, name := splitCatalogKey(identity)
	return &DuplicateModuleObjectError{Kind: kind, Name: name, ModuleName: moduleName}
}

// duplicateScriptError renders the duplicate-script error with Esper's
// NameAndParamNum identity form ("myscript (1 parameters)") when the
// attempted definition declares its argument types, matching the identity
// Java's script path registry derives from the incoming module object.
func duplicateScriptError(identity string, definition scriptDefinition) *DuplicateModuleObjectError {
	dup := duplicateModuleObjectError(DeploymentResourceScript, identity)
	if definition.argumentTypesSet {
		_, name := splitCatalogKey(identity)
		dup.Name = fmt.Sprintf("%s (%d parameters)", name, len(definition.argumentTypes))
	}
	return dup
}
