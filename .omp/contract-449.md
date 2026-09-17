# Draft 4.449 contract — infra-namedwindow-on-delete-silent

Scope: `InfraNamedWindowOnDelete.java` ordinals 5/6 (silent-delete cluster).
Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c` (verified via `git -C /root/app/esper rev-parse HEAD`).

## Executions

| ord | name | runtime ID | static ID | flags |
|-----|------|-----------|-----------|-------|
| 5 | InfraNamedWindowSilentDeleteOnDelete | java-runtime-38dd6f716920e46075c7 | java-06bf0eb71230b3119293 | [] |
| 6 | InfraNamedWindowSilentDeleteOnDeleteMany | java-runtime-6bce45980de6b3f7aa9f | java-06bf0eb71230b3119293 | [] |

Deferred: ord 1 (STATICHOOK flag), ords 2–4 (assertIndexCount needs implicit-index
inference — separate unit), ord 0 already referenced.

## EPL (byte-exact)

Ord 5 (`silent-delete`):
```
@name('create') create window MyWindow#length(2) as SupportBean;
insert into MyWindow select * from SupportBean;
@name('delete') @hint('silent_delete') on SupportBean_S0 delete from MyWindow where p00 = theString;
@name('count') select count(*) as cnt from MyWindow;
```

Ord 6 (`silent-delete-many`):
```
@name('create') create window MyWindow#groupwin(theString)#length(2) as SupportBean;
insert into MyWindow select * from SupportBean;
@name('delete') @hint('silent_delete') on SupportBean_S0 delete from MyWindow;
@name('count') select count(*) as cnt from MyWindow;
```

## Observable contract

Java `OnExprViewNamedWindowDelete`: `@hint('silent_delete')` calls
`rootView.clearDeliveriesRemoveStream(matchingEvents)` — the deleted rows are
stripped from the named window's own-statement (direct child) delivery, while
the on-delete output (deleted rows as NEW data) and tail-view consumers
(`count`) still observe the delta.

### silent-delete (ord 5)

- send SupportBean(E1,1) → count cnt=1; create new {E1}
- send S0(0,"E1") → count cnt=0; delete new {E1}; create NOT invoked (silent)
- send SupportBean(E2,2) → count cnt=1; create new {E2}
- send SupportBean(E3,3) → count cnt=2; create new {E3}
- send SupportBean(E4,4) → count cnt=2; create IR pair new {E4} old {E2} (length-2 expiry)
- send S0(0,"E4") → count cnt=1; delete new {E4}; create NOT invoked
- send S0(0,"E3") → count cnt=0; delete new {E3}; create NOT invoked
- send S0(0,"EX") → no listener invoked at all

### silent-delete-many (ord 6)

- send SupportBean(A,1),(A,2),(B,3),(B,4) → count fires per insert; 4th delivery cnt=4
- send S0(0) → count cnt=0; delete new rows {A,1},{A,2},{B,3},{B,4} (assertPropsPerRowLastNew); create NOT invoked

## Engine gap

`HintSilentDelete` exists in statement_metadata.go but is never consumed.
Fix in `queueNamedWindowDeltaLocked` (runtime.go): when owner statement has
HintSilentDelete and delta.Old non-empty, strip Old from the direct-dispatch
delta only (consumers + pendingNamedWindowDispatches unchanged).

## Files

- scenario: testdata/parity/infra-namedwindow-on-delete-silent.json
- oracle: tools/java-oracle/InfraNamedWindowOnDeleteSilentScenarioOracle.java + run script
- runner: internal/app/parity/infra_namedwindow_on_delete_silent.go + run.go wiring
- engine: internal/esper/runtime.go (queueNamedWindowDeltaLocked + statementHasHint)
- tests: run_test.go six-test family
- manifest: case.infra-namedwindow-on-delete-silent → infra.namedwindow.views
