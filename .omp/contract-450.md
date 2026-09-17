# Draft 4.450 contract — infra-namedwindow-on-delete-indexes

Scope: `InfraNamedWindowOnDelete.java` ordinals 1–4. Java commit
`9e1b9f1cc9117fea4bf33ab043762c045d73839c` (verified via `git -C /root/app/esper rev-parse HEAD`).
Frozen by read-only scouts JavaContract450 + GoSurface450.

## Executions

| ord | name | runtime ID | static ID | flags |
|-----|------|-----------|-----------|-------|
| 1 | InfraStaggeredNamedWindow | java-runtime-0dcb2b72f505c7931a45 | java-06bf0eb71230b3119293 | [STATICHOOK] |
| 2 | InfraCoercionKeyMultiPropIndexes | java-runtime-c4c336036fdd92d803f7 | java-06bf0eb71230b3119293 | [] |
| 3 | InfraCoercionRangeMultiPropIndexes | java-runtime-a58ae70ca908579af50a | java-06bf0eb71230b3119293 | [] |
| 4 | InfraCoercionKeyAndRangeMultiPropIndexes | java-runtime-2d6d018e91b5664f3734 | java-06bf0eb71230b3119293 | [] |

Ord 0 already DV (`case.named-window-mutation-firstunique`); ords 5/6 DV via 4.449.
STATICHOOK has no consumer in the repo (RegressionRunner reads only
EXCLUDEWHENINSTRUMENTED) — record-only no-op flag.

## Index semantics (what assertIndexCount pins)

`assertIndexCount` = `instance.getIndexDescriptors().length`
(SupportInfraUtil.getIndexCountNoContext) — runtime introspection, not a
listener record. Implicit index identity = IndexMultiKey{unique,
ordered hashIndexedProps[], ordered rangeIndexedProps[]};
IndexedPropDesc.equals compares indexPropName + coercionType.

Observable rules:
- `=` conjunct on a window prop → hash prop; `between` conjunct → range prop.
- `<=` and `not between` create NO new index (reuse-or-scan).
- Same prop + same coercion ⇒ reuse; different coercion ⇒ new index.
- Multi-prop hash ORDER is significant ({int,double} ≠ {double,int}).
- Mixed hash+range live in ONE index.
- Undeploying the owning statement drops its index (ref-counted: shared
  index survives while another statement still uses it).
- on-SELECT triggers also create implicit indexes.
- Coercion type = common boxed numeric type of the two compared operand
  types (int vs Integer → Integer; int vs Double → Double).

## Observable record shapes

createOne/createTwo listeners: new rows on insert, OLD rows on delete
(assertPropsOld). Iterator = window contents in insertion order
(assertPropsPerRowIterator; null = empty). `assertListenerNotInvoked` =
zero deliveries since last reset. Window row count (ord 1) and index
count (ords 2–4) are introspection assertions → `index-count` op with
`count` field (new op; statement field = window name). No virtual time,
no multithreading, no FAF.

## Ord 1 — InfraStaggeredNamedWindow (staggered)

Java loops EventRepresentationChoice (6 reps); Go replays the
DEFAULT-equivalent single iteration (approved-difference precedent:
infra_table_insert_into / infra_named_window_join).

EPL (one RegressionPath, five statements):
```
@name('createOne') @public create window MyWindowSTAG#keepall as select theString as a1, intPrimitive as b1 from SupportBean
 @name('createTwo') @public create window MyWindowSTAGTwo#keepall as select theString as a2, intPrimitive as b2 from SupportBean
@name('delete') on MyWindowSTAG delete from MyWindowSTAGTwo where a1 = a2
@name('insert') insert into MyWindowSTAG select theString as a1, intPrimitive as b1 from SupportBean(intPrimitive > 0)
@name('insertTwo') insert into MyWindowSTAGTwo select theString as a2, intPrimitive as b2 from SupportBean(intPrimitive < 0)
```
Sends (SupportBean 2-arg: theString, intPrimitive):
1. ("E1",-10) → createTwo new {a2:E1,b2:-10}; createOne NOT invoked
2. ("E2",5) → createOne new {a1:E2,b1:5}; createTwo NOT invoked
3. ("E3",-1) → createTwo new {E3,-1}; createOne NOT invoked
4. ("E3",1) → createOne new {E3,1}; createTwo OLD {E3,-1} (cross-window
   delete fired: insert into MyWindowSTAG triggered delete from
   MyWindowSTAGTwo where a1=a2)
Then undeployModuleContaining: delete, insert, insertTwo, createOne,
createTwo (in that order).

## Ord 2 — InfraCoercionKeyMultiPropIndexes (coercion-key)

Phase A — deploy order:
```
@name('createOne') @public create window MyWindowCK#keepall as select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean
```
Then SEVEN separate deployments, each followed by index-count on MyWindowCK:
- d1: `on SupportBean(theString='DB') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.doubleBoxed` → 1
- d2: `on SupportBean(theString='DP') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.doublePrimitive` → 1 (reuses d1: same prop, same coercion Double)
- d3: `on SupportBean(theString='IB') as s0 delete from MyWindowCK where MyWindowCK.intPrimitive = s0.intBoxed` → 2 (coercion Integer≠Double)
- d4: `on SupportBean(theString='IPDP') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.intPrimitive and win.doublePrimitive = s0.doublePrimitive` → 3
- d5: `on SupportBean(theString='IPDP2') as s0 delete from MyWindowCK as win where win.doublePrimitive = s0.doublePrimitive and win.intPrimitive = s0.intPrimitive` → 4 (order matters)
- d6: `on SupportBean(theString='IPDPIB') as s0 delete from MyWindowCK as win where win.doublePrimitive = s0.doublePrimitive and win.intPrimitive = s0.intPrimitive and win.intBoxed = s0.intBoxed` → 5
- d7: `on SupportBean(theString='CAST') as s0 delete from MyWindowCK as win where win.intBoxed = s0.intPrimitive and win.doublePrimitive = s0.doubleBoxed and win.intPrimitive = s0.intBoxed` → 6
```
insert into MyWindowCK select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean(theString like 'E%')
```
Sends (SupportBean 5-arg: theString, intPrimitive, intBoxed:Integer,
doublePrimitive, doubleBoxed:Double). ALL SupportBean events evaluate
every live on-delete; only 'E%' inserts.
- E1(1,10,100d,1000d) E2(2,20,200d,2000d) E3(3,30,300d,3000d) E4(4,40,400d,4000d) → createOne listenerReset
- ("DB",0,0,0d,null) → NOT invoked (null boxed key)
- ("DB",0,0,0d,3d) → old {E3}
- ("DP",0,0,5d,null) → NOT invoked
- ("DP",0,0,4d,null) → old {E4}
- ("IB",0,-1,0d,null) → NOT invoked
- ("IB",0,1,0d,null) → old {E1}
- E5(5,50,500d,5000d) E6(6,60,600d,6000d) E7(7,70,700d,7000d) → listenerReset
- ("IPDP",5,0,500d,null) → old {E5}
- ("IPDP2",6,0,600d,null) → old {E6}
- ("IPDPIB",7,70,0d,null) → NOT invoked (doublePrimitive 0≠700)
- ("IPDPIB",7,70,700d,null) → old {E7}
- E8(8,80,800d,8000d) → listenerReset
- ("CAST",80,8,0,800d) → old {E8}
- undeployModuleContaining d1..d7 → deploy d0:
  `on SupportBean(theString='LAST') as s0 delete from MyWindowCK as win where win.intPrimitive = s0.intPrimitive and win.doublePrimitive = s0.doublePrimitive` (no index-count assert)
- ("LAST",2,20,200,2000d) → old {E2}; iterator null (empty)
- undeployModuleContaining d0 → index-count 0; undeployAll.

Phase B (fresh path, on-SELECT implicit indexes, no sends):
```
@name('createTwo') @public create window WinOne#keepall as SupportBean
on SupportBean_ST0 select * from WinOne where theString = key0                    → count 1
on SupportBean_ST0 select * from WinOne where theString = key0 and intPrimitive = p00  → count 2
undeployAll
```

## Ord 3 — InfraCoercionRangeMultiPropIndexes (coercion-range)

```
@name('createOne') @public create window MyWindowCR#keepall as select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean
insert into MyWindowCR select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean
```
Sends E1(1,10,100d,1000d) E2(2,20,200d,2000d) E3(3,30,3d,30d) E4(4,40,4d,40d)
E5(5,50,500d,5000d) E6(6,60,600d,6000d) → listenerReset.
Deletes deployed interleaved with sends (all stay live until end);
trigger SupportBeanTwo(stringTwo, intPrimitiveTwo, intBoxedTwo:Integer,
doublePrimitiveTwo, doubleBoxedTwo:Double):
- d0: `on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive between s2.doublePrimitiveTwo and s2.doubleBoxedTwo` → 1
  - sendTwo("T",0,0,0d,null) → NOT invoked (null bound)
  - sendTwo("T",0,0,-1d,1d) → old {E1}
- d1: `on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo` → 2 (Integer vs d0's Double)
  - sendTwo("T",-2,2,0d,0d) → old {E2}
- d2: `on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo and win.doublePrimitive between s2.intPrimitiveTwo and s2.intBoxedTwo` → 3
  - sendTwo("T",-3,3,-3d,3d) → old {E3}
- d3: `on SupportBeanTwo as s2 delete from MyWindowCR as win where win.doublePrimitive between s2.intPrimitiveTwo and s2.intPrimitiveTwo and win.intPrimitive between s2.intPrimitiveTwo and s2.intPrimitiveTwo` → 4 (range order differs)
  - sendTwo("T",-4,4,-4,4d) → old {E4} — d3 itself does NOT match
    (doublePrimitive 4 ∉ [-4,-4]); still-live d2 deletes E4
- d4: `on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive <= doublePrimitiveTwo` → 4 (no new index)
  - sendTwo("T",0,0,5,1d) → old {E5}
- d5: `on SupportBeanTwo as s2 delete from MyWindowCR as win where win.intPrimitive not between s2.intPrimitiveTwo and s2.intBoxedTwo` → 4 (no new index)
  - sendTwo("T",100,200,0,0d) → old {E6}
undeployModuleContaining d0..d5 → index-count 0; undeployAll.

## Ord 4 — InfraCoercionKeyAndRangeMultiPropIndexes (coercion-key-range)

```
@name('createOne') @public create window MyWindowCKR#keepall as select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean
insert into MyWindowCKR select theString, intPrimitive, intBoxed, doublePrimitive, doubleBoxed from SupportBean
```
Sends E1(1,10,100d,1000d) E2(2,20,200d,2000d) E3(3,30,300d,3000d) E4(4,40,400d,4000d) → listenerReset.
- d0: `on SupportBeanTwo delete from MyWindowCKR where theString = stringTwo and intPrimitive between doublePrimitiveTwo and doubleBoxedTwo` → 1
  - sendTwo("T",0,0,1d,200d) → NOT invoked
  - sendTwo("E1",0,0,1d,200d) → old {E1}
- d1: `on SupportBeanTwo delete from MyWindowCKR where theString = stringTwo and intPrimitive = intPrimitiveTwo and intBoxed between doublePrimitiveTwo and doubleBoxedTwo` → 2
  - sendTwo("E2",2,0,19d,21d) → old {E2}
- d2: `on SupportBeanTwo delete from MyWindowCKR where intBoxed between doubleBoxedTwo and doublePrimitiveTwo and intPrimitive = intPrimitiveTwo and theString = stringTwo ` → 3 (hash order differs; note trailing space in EPL)
  - sendTwo("E3",3,0,29d,34d) → old {E3} — d2's range is REVERSED [34,29]
    ⇒ no match; live d1 deletes E3
- d3: `on SupportBeanTwo delete from MyWindowCKR where intBoxed between intBoxedTwo and intBoxedTwo and intPrimitive = intPrimitiveTwo and theString = stringTwo` → 4 (Integer vs d2's Double)
  - sendTwo("E4",4,40,0d,null) → old {E4}
undeployModuleContaining d0..d3 → index-count 0; undeployAll.

## Go-side gaps (shared core — primary agent)

1. `index-count` op in internal/compat/scenario.go (Count field, statement
   = window name).
2. Implicit index inference at named-window trigger deploy for
   DeleteFromNamedWindow/DeleteAllFromNamedWindow/SelectFromNamedWindow
   where clauses: walk the `where` exprNode AST — "and" recurses;
   "equal-of" with a "named-window-field" operand → hash prop
   {name, coercionKey}; non-negated "between-of" with named-window-field
   value → range prop {name, coercionKey}; all other kinds contribute
   nothing. Coercion key = canonical common numeric type of the two
   operand reflect.Types (pointer-unwrap; wider wins: Double>Float>
   Long>Integer; non-numeric → the other operand's type).
3. Ref-counted implicit index registry on namedWindowRuntime; drop when
   the owning statement undeploys and refcount hits 0.
4. Observable: `NamedWindow.IndexCount()` (or Definition().IndexCount())
   = declared + implicit count, matching getIndexDescriptors().length.

## Pitfalls

- Multiple simultaneous on-delete statements: a send may match several;
  only resulting old rows are asserted (ord3 send 4, ord4 send 3 rely on
  an earlier still-live statement).
- Null boxed key/bound ⇒ no match, no error; reversed between ⇒ no match.
- insert-into filters ('E%', intPrimitive>0/<0) gate inserts but NOT
  on-delete triggers.
- Ord 2 phase B: on-SELECT implicit indexes asserted with zero sends.
- Ord 1 'delete' listener attached but never asserted in Java; Go records
  it only if it fires (it does fire — deleted rows as new data; keep the
  listener attached and record like ords 5/6).
- New beans: SupportBeanTwo (stringTwo, intPrimitiveTwo, intBoxedTwo,
  doublePrimitiveTwo, doubleBoxedTwo), SupportBean_ST0 (id, key0, p00).
