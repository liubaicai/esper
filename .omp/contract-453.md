# Draft 4.453 frozen contract — ExprFilterInAndBetween

Oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c
Source: regression-lib/.../suite/expr/filter/ExprFilterInAndBetween.java
Shared static id: java-17cece2bf9c2df0b27f1 (all ords)

## Harness semantics
- compileDeploy(epl): one EPL string = one module = one deployment; addListener("sN") AFTER deploy.
- compileDeployAddListenerMile(epl,name,m): compile+deploy+addListener, then milestone(m) unless m==-1.
- milestone(n): ordering marker only; NO virtual time anywhere in this file.
- assertListenerInvoked(name): at-least-one delivery since last reset, then RESET.
- assertListenerNotInvoked(name): no delivery, NO reset.
- assertListenerInvokedFlag(name,expected): equals check, always resets.
- assertEventNew(name): exactly one new event, then reset (assertor is no-op in ords 7/8).
- undeployModuleContaining(name): undeploy the whole module holding statement `name`.
- tryInvalidCompile(epl,"skip"): compile must throw; message NOT asserted.
- Beans: SupportBean(theString,intPrimitive) ctor; defaults theString=null,intBoxed=null,longBoxed=null,boolPrimitive=false,intPrimitive=0,longPrimitive=0,bytePrimitive=0. SupportBeanNumeric(intOne,intTwo). SupportBean_S0(id,p00,p01,p02,p03).

## Executions (runtime IDs)
- ord 0 ExprFilterInDynamic  java-runtime-0d06d25e978384a0b008  flags []
- ord 4 ExprFilterInInvalid  java-runtime-9472ab3c76f8e25de952  flags [] (INVALIDITY is descriptive only; body gated on FilterIndexPlanning>=BASIC — Go has no in-list index planning; register implemented, NOT DV)
- ord 5 ExprFilterReuse      java-runtime-d16fe1fd7693643a5001  flags [OBSERVEROPS]
- ord 6 ExprFilterReuseNot   java-runtime-9245458815076f0baee3  flags [OBSERVEROPS]
- ord 7 ExprFilterInMultipleNonMatchingFirst java-runtime-a3336b696b1ae9b2c831 flags []
- ord 8 ExprFilterInMultipleWithBool         java-runtime-ed506bfbf3734b4fff03 flags []

## ord 0 ExprFilterInDynamic (two phases, undeployAll between)
Phase A EPL (byte-exact):
  @name('s0') select * from pattern [a=SupportBeanNumeric -> every b=SupportBean(intPrimitive in (a.intOne, a.intTwo))]
compileDeployAddListenerMile(epl,"s0",0). Sends:
  SupportBeanNumeric(intOne=10,intTwo=20); SupportBean intPrimitive=10 → s0 INVOKED;
  intPrimitive=11 → NOT; intPrimitive=20 → INVOKED. undeployAll.
Phase B EPL:
  @name('s0') select * from pattern [a=SupportBean_S0 -> every b=SupportBean(theString in (a.p00, a.p01, a.p02))]
compileDeployAddListenerMile(epl,"s0",1). Sends:
  SupportBean_S0(1,"a","b","c","d"); theString="a"→Y; "x"→N; "b"→Y; "c"→Y; "d"→N (p03 not in set). undeployAll.

## ord 4 ExprFilterInInvalid — compile-failure only, gated on filter index planning >= BASIC
Four EPLs must fail compile (message unchecked):
  select * from SupportBean(intPrimitive in (1L, 10L))
  select * from SupportBean(intPrimitive in (1, 10L))
  select * from SupportBean(intPrimitive in (1, 'x'))
  select * from pattern [a=SupportBean -> b=SupportBean(intPrimitive in (a.longPrimitive, a.longBoxed))]
Java rationale "we do not coerce": index planning requires in-list element types to match the property
type exactly. Under NONE the same filters compile with coercion. Go coerces (= NONE behavior) → ord 4
is intentionally-different-adjacent: register implemented with a note, not DV.

## ord 5 ExprFilterReuse — tryReuse protocol over 8 groups
Protocol per group: deploy each stmt as its OWN module via compileDeploy("@name('s"+i+"')"+epl).addListener("s"+i);
one milestone; sendBean(intBoxed=3) → EVERY listener invoked-and-reset; then for toStop=0..n-1:
undeployModuleContaining("s"+toStop), send intBoxed=3, assert listeners[0..toStop] NOT invoked and
listeners[toStop+1..n-1] invoked-and-reset; final send intBoxed=3 → ALL not invoked.
All EPLs TRUE at intBoxed=3 with Java defaults. Groups (@name prefix added by harness, not in epl):
 G1 (2 identical): select * from SupportBean(intBoxed in [2:4])
 G2 (2 identical): select * from SupportBean(intBoxed in (1, 2, 3))
 G3: s0=select * from SupportBean(intBoxed in (2:3]) ; s1=select * from SupportBean(intBoxed in (1:3])
 G4: s0=select * from SupportBean(intBoxed in (2, 3, 4)) ; s1=select * from SupportBean(intBoxed in (1, 3))
 G5 (3): in (2, 3, 4) ; in (1, 3) ; in (8, 3)
 G6 (3): in (3, 1, 3) ; in (3, 3) ; in (1, 3)
 G7 (3, comma=AND): s0=SupportBean(boolPrimitive=false, intBoxed in (1, 2, 3)) ;
    s1=SupportBean(boolPrimitive=false, intBoxed in (3, 4)) ; s2=SupportBean(boolPrimitive=false, intBoxed in (3))
 G8 (3, mixed 2nd cond): s0=SupportBean(intBoxed in (1, 2, 3), longPrimitive >= 0) ;
    s1=SupportBean(intBoxed in (3, 4), intPrimitive >= 0) ; s2=SupportBean(intBoxed in (3), bytePrimitive < 1)

## ord 6 ExprFilterReuseNot — same protocol, 4 groups, all TRUE at intBoxed=3
 G1 (2 identical): select * from SupportBean(intBoxed not in [1:2])
 G2 (3): s0=SupportBean(intBoxed in (3, 1, 3)) ; s1=SupportBean(intBoxed not in (2, 1)) ;
    s2=SupportBean(intBoxed not between 0 and -3)   // endpoints normalize to [-3,0]; 3 outside → true
 G3 (3): s0=SupportBean(intBoxed not in (1, 4, 5)) ; s1=same text ; s2=SupportBean(intBoxed not in (4, 5, 1))
 G4 (3): s0=SupportBean(intBoxed not in (3:4)) ; s1=SupportBean(intBoxed not in [1:3)) ;
    s2=SupportBean(intBoxed not in (1,1,1,33))
Note: `not in (3:4)` true at 3 — open BOTH ends.

## ord 7 ExprFilterInMultipleNonMatchingFirst — deploy order matters
FIRST: @name('A') select * from SupportBean(intPrimitive in (0,0,1) and theString like 'X%') + addListener('A')
SECOND: @name('B') select * from SupportBean(intPrimitive in (0,1) and theString like 'A%') + addListener('B')
milestone(0). Send SupportBean(theString="A", intPrimitive=0): B delivers exactly one new event; A NOT invoked. undeployAll.

## ord 8 ExprFilterInMultipleWithBool
Deploy @name('s1') select * from SupportBean(intPrimitive in (0) and theString like 'X%') + listener; milestone(0);
deploy @name('s2') select * from SupportBean(intPrimitive in (0,1) and theString like 'A%') + listener; milestone(1).
Send SupportBean("A",1): s2 one new event; s1 NOT invoked. undeployAll.

## Edge cases
- between NORMALIZES endpoints (between 0 and -3 = [-3,0]); range literals [a:b] (a:b] [a:b) (a:b).
- in-list duplicates legal; comma inside filter = AND.
- Ord 5/6 rely on Java defaults: only intBoxed=3 set; boolPrimitive=false, long/int/bytePrimitive=0.
- invoked/invokedFlag/assertEventNew RESET; notInvoked does NOT — assert ordering matters.
- No virtual time, no old-stream, no iterators, no null sends.

## Go surface (scout NextGoSurface453)
- API complete: InOf/NotInOf/BetweenOf/BetweenRangeOf/NotBetweenOf/NotBetweenRangeOf/InSlice/In/Between,
  And/Or/Not/Like; Stream.Filter (stream.go:1151); dynamic in-sets over pattern tags proven
  (expr_filter_optimizable_lookupable_limited_parity_test.go:186).
- Invalid filters surface as ErrorInvalidRule at env.Build (plan.go validateExpressionConfiguration).
- sharedFilterIndex only guards field=literal equality — in/between never index-shared; reuse needs no core change.
- Runner template: internal/app/parity/expr_filter_optimizable.go (efoJavaSources/efoJavaRuntimeIDs vars,
  runEfoScenario dispatch, per-statement Subscribe → makeRecord). Mode registered in run.go.
- SupportBean_S0 bean exists in expr_filter_optimizable_value_limited.go (efovS0: id,p00-p03,value).
- Ord 4 precedent: compile-rejected executions → Go Build-rejection unit tests; javaFlags=[] for ord 4.
