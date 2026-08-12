package esper

import "fmt"

// PlanCompatibilityError reports that an otherwise valid immutable Plan was
// produced for a different plan-schema or fluent-compiler contract. A batch
// deployment may additionally identify the zero-based plan that failed.
type PlanCompatibilityError struct {
	RuntimeSchemaVersion   string
	RuntimeCompilerVersion string
	PlanSchemaVersion      string
	PlanCompilerVersion    string
	planIndex              int
	hasPlanIndex           bool
}

func (e *PlanCompatibilityError) Error() string {
	if e == nil {
		return "esper: <nil plan compatibility error>"
	}
	message := fmt.Sprintf(
		"esper: %s: plan compatibility mismatch; runtime schema is %q and compiler is %q, plan schema is %q and compiler is %q",
		ErrorDependency,
		e.RuntimeSchemaVersion,
		e.RuntimeCompilerVersion,
		e.PlanSchemaVersion,
		e.PlanCompilerVersion,
	)
	if e.hasPlanIndex {
		message += fmt.Sprintf(" (deployment plan %d)", e.planIndex)
	}
	return message
}

// Is categorizes version mismatches as dependency errors: the Plan is valid
// data, but it cannot execute under this runtime contract.
func (e *PlanCompatibilityError) Is(target error) bool {
	return e != nil && target == ErrorDependency
}

// DeploymentPlanIndex returns the zero-based plan index reported by a
// DeployPlans preflight. Single-plan Deploy and fire-and-forget errors do not
// carry an index.
func (e *PlanCompatibilityError) DeploymentPlanIndex() (int, bool) {
	if e == nil || !e.hasPlanIndex {
		return 0, false
	}
	return e.planIndex, true
}

func newPlanCompatibilityError(schemaVersion, compilerVersion string, planIndex *int) error {
	failure := &PlanCompatibilityError{
		RuntimeSchemaVersion:   planSchemaVersion,
		RuntimeCompilerVersion: CompilerVersion,
		PlanSchemaVersion:      schemaVersion,
		PlanCompilerVersion:    compilerVersion,
	}
	if planIndex != nil {
		failure.planIndex = *planIndex
		failure.hasPlanIndex = true
	}
	return failure
}

func validatePlanCompatibility(plan Plan, planIndex *int) error {
	if plan.schemaVersion == planSchemaVersion && plan.compilerVersion == CompilerVersion {
		return nil
	}
	return newPlanCompatibilityError(plan.schemaVersion, plan.compilerVersion, planIndex)
}

func (e *Engine) validateOwnedPlan(plan Plan, planIndex *int) error {
	if e == nil || e.env == nil {
		return NewError(ErrorDependency, "engine has no environment")
	}
	if plan.query.env != e.env || plan.schemaVersion == "" || plan.compilerVersion == "" {
		return NewError(ErrorDependency, "plan does not belong to this engine")
	}
	return validatePlanCompatibility(plan, planIndex)
}
