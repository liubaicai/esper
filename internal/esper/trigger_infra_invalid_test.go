package esper

import (
	"reflect"
	"testing"
)

func TestTriggerInfraInvalidBuildCases(t *testing.T) {
	for _, namedWindow := range []bool{true, false} {
		t.Run(map[bool]string{true: "named-window", false: "table"}[namedWindow], func(t *testing.T) {
			env := NewEnvironment()
			const (
				sourceName = "TriggerInfraInvalidSource"
				targetName = "TriggerInfraInvalidTarget"
			)
			if _, err := RegisterStruct[triggerInfraFlowEvent](env, sourceName); err != nil {
				t.Fatal(err)
			}
			if namedWindow {
				targetSchema, err := RegisterStruct[triggerInfraFlowEvent](env, targetName)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := CreateNamedWindow(env, targetName, targetSchema, NamedWindowRetention(KeepAll())); err != nil {
					t.Fatal(err)
				}
			} else if _, err := CreateTable(env, targetName, []TableColumn{
				PrimaryKeyColumn[string]("theString"),
				TableColumnOf[int]("intPrimitive"),
				TableColumnOf[bool]("boolPrimitive"),
			}); err != nil {
				t.Fatal(err)
			}

			source := From[triggerInfraFlowEvent](env, sourceName)
			theString := Field[triggerInfraFlowEvent, string]("theString")
			intPrimitive := Field[triggerInfraFlowEvent, int]("intPrimitive")
			var targetString Expression[string]
			if namedWindow {
				targetString = NamedWindowField[string]("theString")
			} else {
				targetString = TableField[string]("theString")
			}

			build := func(name string, clauses ...TableMergeClause) error {
				if namedWindow {
					return func() error {
						_, err := env.Build(OnEvent(source).MergeIntoNamedWindowWhen(
							targetName,
							Equal[string](targetString, theString),
							clauses...,
						).Query(StatementName(name)))
						return err
					}()
				}
				_, err := env.Build(OnEvent(source).MergeIntoTableWhen(
					targetName,
					[]Expr{theString},
					clauses...,
				).Query(StatementName(name)))
				return err
			}
			invalid := []struct {
				name  string
				build func() error
			}{
				{
					name: "missing-target-column",
					build: func() error {
						return build("invalid-"+"missing-target-column",
							WhenNotMatchedAny(SetColumn("missing", intPrimitive)))
					},
				},
				{
					name: "not-matched-target-scope",
					build: func() error {
						return build("invalid-"+"not-matched-target-scope",
							WhenNotMatched(Equal[string](targetString, theString),
								SetColumn("theString", theString)))
					},
				},
				{
					name: "incompatible-assignment-type",
					build: func() error {
						return build("invalid-"+"incompatible-assignment-type",
							WhenMatchedAny(SetColumn("intPrimitive", theString)))
					},
				},
				{
					name: "indexed-assignment-to-scalar",
					build: func() error {
						return build("invalid-"+"indexed-assignment-to-scalar",
							WhenMatchedAny(SetArrayElement("theString", Literal(0), intPrimitive)))
					},
				},
				{
					name: "missing-clause",
					build: func() error {
						return build("invalid-" + "missing-clause")
					},
				},
				{
					name: "not-matched-delete",
					build: func() error {
						return build("invalid-"+"not-matched-delete",
							TableMergeClause{Delete: true, Condition: Literal(true)},
							WhenMatchedAny(SetColumn("intPrimitive", intPrimitive)))
					},
				},
				{
					name: "malformed-wildcard",
					build: func() error {
						return build("invalid-"+"malformed-wildcard",
							TableMergeClause{Assignments: []TableAssignment{{Wildcard: true, Column: "bad"}}})
					},
				},
				{
					name: "matched-update-action-on-not-matched-branch",
					build: func() error {
						return build("invalid-"+"matched-update-action-on-not-matched-branch",
							WhenNotMatchedActions(ThenUpdate(Literal(true), SetColumn("intPrimitive", intPrimitive))))
					},
				},
				{
					name: "target-side-stream-missing",
					build: func() error {
						return build("invalid-"+"target-side-stream-missing",
							WhenMatchedActions(ThenInsertInto("MissingInfraStream", Alias("value", intPrimitive))))
					},
				},
				{
					name: "insert-target-action-on-matched-branch",
					build: func() error {
						return build("invalid-"+"insert-target-action-on-matched-branch",
							WhenMatchedActions(ThenInsertIntoTarget(SetColumn("intPrimitive", intPrimitive))))
					},
				},
			}
			for _, testCase := range invalid {
				t.Run(testCase.name, func(t *testing.T) {
					if err := testCase.build(); err == nil {
						t.Fatalf("InfraInvalid analogue %q was accepted", testCase.name)
					}
				})
			}

			// The Java case also rejects assigning an event of an unrelated
			// declared type to a composite target property. Keep this check
			// explicit in Go by using distinct struct types rather than relying
			// on a same-shaped map, whose dynamic values are intentionally valid.
			type composite struct {
				C0 int `esper:"c0"`
			}
			type other struct {
				C1 int `esper:"c1"`
			}
			type outer struct {
				SO other `esper:"so"`
			}
			compositeSchema, err := RegisterStruct[composite](env, "TriggerInvalidComposite")
			if err != nil {
				t.Fatal(err)
			}
			otherSchema, err := RegisterStruct[other](env, "TriggerInvalidOther")
			if err != nil {
				t.Fatal(err)
			}
			outerSource, err := RegisterStruct[outer](env, "TriggerInvalidOuter")
			if err != nil {
				t.Fatal(err)
			}
			_ = otherSchema
			_ = outerSource
			compositeTarget, err := RegisterMap(env, "TriggerInvalidCompositeTarget", []FieldSpec{
				FieldDef("c", reflect.TypeOf(map[string]any{})),
			}, WithNestedPropertySchema("c", compositeSchema))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CreateNamedWindow(env, "TriggerInvalidCompositeTarget", compositeTarget, NamedWindowRetention(KeepAll())); err != nil {
				t.Fatal(err)
			}
			compositeSource := From[outer](env, "TriggerInvalidOuter")
			if _, err := env.Build(OnEvent(compositeSource).InsertIntoNamedWindow(
				"TriggerInvalidCompositeTarget",
				SetColumn("c", Field[outer, other]("so")),
			).Query(StatementName("invalid-composite-event-assignment"))); err == nil {
				t.Fatal("unrelated nested event assignment was accepted")
			}
		})
	}
}
