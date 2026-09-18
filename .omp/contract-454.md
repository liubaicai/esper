# Draft 4.454 frozen contract — EPLVariablesCreate

Oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c
Source: regression-lib/.../suite/epl/variable/EPLVariablesCreate.java
Shared inventory id: java-6a445f399d99f5d4785d

## Harness semantics
- compileDeploy(epl): one EPL string = one module; addListener after deploy.
- milestone(n): NO-OP ordering marker (no virtual time).
- tryInvalidCompile(epl,msg): compile must throw; msg is startsWith prefix; "skip" = failure only.
- assertPropsIRPair: one invocation, exactly 1 new + 1 old event.
- assertPropsPerRowIterator: statement safeIterator rows.
- assertListenerNotInvoked: no reset.
- SupportBean: sendSupportBean(theString, intPrimitive); defaults as usual.
- undeployModuleContaining(name): undeploy whole module holding statement.

## Runtime IDs
- ord 0 EPLVariableOM                  java-runtime-67e6441b95a4ca30917b  flags []
- ord 1 EPLVariableCompileStartStop    java-runtime-74285cbcf0d7aeabf02f  flags []
- ord 2 EPLVariableSubscribeAndIterate java-runtime-06466d91981a6c410b39  flags []
- ord 3 EPLVariableDeclarationAndSelect java-runtime-75cb3e01ac92790b5194 flags []
- ord 4 EPLVariableInvalid             java-runtime-b2c51a16c93eaf0cb126  flags []
- ord 5 EPLVariableDimensionAndPrimitive java-runtime-71775e7db7d1ffd875ba flags [RUNTIMEOPS]
- ord 6 EPLVariableGenericType         java-runtime-4ac4b7b111d10a20f74d  flags []

## ord 0 EPLVariableOM — SODA object-model path
Model1: @public create variable long var1OMCreate (no init) → toEPL byte-exact
"@public create variable long var1OMCreate". Model2: @public create variable string
var2OMCreate = "abc" → toEPL renders DOUBLE quotes:
'@public create variable string var2OMCreate = "abc"'.
Then EPL deploy on same path: @name('s0') select var1OMCreate, var2OMCreate from SupportBean
+ listener. sendSupportBean("E1",10) → new=[null,"abc"] (uninitialized long var = null,
boxed Long, NOT 0). Then compileDeploy("create variable double[] arrdouble = {1.0d,2.0d}").
undeployAll. PORT: SODA/toEPL assertions are approved-difference (no object model);
deployable semantics = module-scoped vars + select + read.

## ord 1 EPLVariableCompileStartStop — eplToModel round-trip + lifecycle
Deploy "@public create variable long var1CSS" then "@public create variable string var2CSS = \"abc\""
(double-quoted). Deploy @name('s0') select var1CSS, var2CSS from SupportBean + listener.
sendSupportBean("E1",10) → new=[null,"abc"].
ESPER-545: deploy "@name('create') @public create variable int FOO = 0"; deploy
"on pattern [every SupportBean] set FOO = FOO + 1"; send SupportBean →
getVariableValue(deploymentId("create"),"FOO") == 1. undeployAll; redeploy create text alone
→ FOO == 0 (redeploy reinitializes). Cleanup: compileDeploy "@private create variable int x = 123";
tryInvalidCompile("select missingScript(x) from SupportBean","skip"); recompile same @private
create succeeds (failed compile leaves no residue). undeployAll.
PORT: eplToModel is approved-difference; on-pattern-every → OnEvent(From) equivalent;
module-scoped vars give redeploy-reset.

## ord 2 EPLVariableSubscribeAndIterate — IR pairs + iterator + reset
Deploy "@name('create-one') @public create variable long var1SAI = null" + listener.
Statement props: STATEMENTTYPE=CREATE_VARIABLE, CREATEOBJECTNAME="var1SAI"; iterator yields
1 row {var1SAI:null}; listener NOT invoked on deploy; event type propertyType=Long boxed,
underlyingType=Map, propertyNames=[var1SAI].
Deploy "@name('create-two') @public create variable long var2SAI = 20" + listener →
iterator {var2SAI:20L}, listener not invoked.
Deploy "@name('set') on SupportBean set var1SAI = intPrimitive * 2, var2SAI = var1SAI + 1"
— assignments SEQUENTIAL: var2SAI sees NEW var1SAI.
sendSupportBean("E1",100): create-one IR new=200L old=null; create-two new=201L old=20L;
iterators {200L},{201L}. milestone(0). sendSupportBean("E2",200): create-one new=400L
old=200L; create-two new=401L old=201L; iterators {400L},{401L}.
undeployModuleContaining("set"); undeployModuleContaining("create-two"); redeploy
create-two text (no path): create-one iterator still {400L} (survives); create-two reset
to {20L}. undeployAll.
PORT: VariableChangeListener Old/New = IR pairs; read-variable = iterator row;
statement-type/event-type assertions are API-only (approved-difference).

## ord 3 EPLVariableDeclarationAndSelect — 29-var typing/coercion matrix
Each var its own module "@public create variable <type> <name>[ = <init>]" on shared path
(order matters: varX5 init references varX1). Table (name,type,init,expected):
varX1 int `1`→1; varX2 int `'2'`→2 (string→int); varX3 INTEGER ` 3+2 `→5;
varX4 bool ` true|false `→true; varX5 boolean ` varX1=1 `→true (= is equality);
varX6 double ` 1.11 `→1.11; varX7 double ` 1.20d `→1.20; varX8 Double ` ' 1.12 ' `→1.12;
varX9 float ` 1.13f*2f `→2.26f; varX10 FLOAT ` -1.14f `→-1.14f;
varX11 string ` ' XXXX ' `→" XXXX " (spaces kept); varX12 string ` "a" `→"a";
varX13 character `'a'`→'a'; varX14 char `'x'`→'x'; varX15 short ` 20 `→(short)20;
varX16 SHORT ` ' 9 ' `→(short)9; varX17 long ` 20*2 `→40L; varX18 LONG ` ' 9 ' `→9L;
varX19 byte ` 20*2 `→(byte)40; varX20 BYTE `9+1`→(byte)10;
varX21..varX29 (int,bool,double,float,string,char,short,long,BYTE) no init → all null.
milestone(0). One select: @name('s0') select varX1,...,varX29 from SupportBean + listener.
sendSupportBean("E1",1) → one new event, each property = expected. undeployAll.
PORT: pre-fold initializers (runner computes Go values); byte→int8, short→int16,
char→int32(rune), float→float32; uninitialized → null.

## ord 4 EPLVariableInvalid — compile failures (startsWith messages)
(a) "create variable somedummy myvar = 10" → "Cannot create variable 'myvar', type
'somedummy' is not a recognized type ["
(b) "create variable string myvar = 5" → "Variable 'myvar' of declared type String cannot
be initialized by a value of type Integer ["
(c) fresh path, deploy "@public create variable string myvar = 'a'", then recompile same →
"A variable by name 'myvar' has already been declared"
(d) "select * from SupportBean output every somevar events" → "Failed to validate the
output rate limiting clause: Variable named 'somevar' has not been declared ["
(e) "create variable SupportBean<Integer> sb" → "Cannot create variable 'sb', type
'SupportBean' cannot be declared as an array type and cannot receive type parameters as
it is an event type"
PORT: implemented-only probes — duplicate registration (duplicateModuleObjectError) and
undeclared variable in output expr (ErrorUnknownName at Build via
OutputLastEveryEventsExpr(VariableRef)) coverable; unrecognized-type/init-mismatch/
event-type-params cases have no Go trigger.

## ord 5 EPLVariableDimensionAndPrimitive [RUNTIMEOPS] — variable service set/get
One module: create variable int[primitive] int_prim = null; create variable int[]
int_boxed = null; create variable java.lang.Object[] objectarray = null; create variable
java.lang.Object[][] objectarray_2dim = null. id=deploymentId("vars").
Per var: setVariableValue(id,name,value) then getVariableValue → exact-order equals;
then setVariableValue wrong-typed array → VariableValueException.
int_prim: int[]{1,2} OK; String[0] → reject.
int_boxed: Integer[]{1,2} OK; int[0] → reject (primitive rejected for boxed!).
objectarray: Integer[]{1,2} OK (covariance); int[0] → reject.
objectarray_2dim: Object[][]{{1,2}} OK; int[0] → reject.
undeployAll.
PORT: set-variable/read-variable ops; []int32 (int[]), []*int32 or []any
(Integer[]/Object[]), [][]any (Object[][]); rejections fall out of coerce assignability.

## ord 6 EPLVariableGenericType — List<String> + enum method
Module: "@name('var') create variable List<String> mylist = Arrays.asList('a', 'b');
@name('s0') select mylist as c0, mylist.where(v => v = 'a') as c1 from SupportBean;"
compileDeploy + listener s0. assertStmtTypes: c0=List<String> parameterized,
c1=Collection<String> (where widens List→Collection). milestone(0).
getVariableValue(deploymentId("var"),"mylist") → ["a","b"] exact order.
sendEventBean(new SupportBean()) → new: c0=["a","b"], c1=["a"]. undeployAll.
PORT: c0=[]string{a,b}, c1=EnumWhere→{a}; parameterized-type assertion is
approved-difference (Go type is []string).

## Edge cases
- Uninitialized primitive-typed vars read null (boxed), not zero.
- On-set multi-assignment sequential: var2SAI=var1SAI+1 sees NEW var1SAI.
- Create-variable stmts iterable (1 row current value); IR pair on mutation
  new=current/old=previous; deploy does NOT invoke listener.
- Redeploy resets module vars to initializers; undeploying only the on-set module leaves
  values intact.
- Declaration coercion: '2'→int, ' 1.12 '→double, 3+2→5, varX1=1→bool; case-insensitive
  type names.
- int[primitive] vs int[] distinct: boxed var rejects primitive array and vice versa.
- Object[] accepts Integer[] (covariance) but rejects int[].
- VariableService keyed by deploymentId of the module containing the create statement.

## Go surface (scout NextGoSurface454)
- Module.RegisterVariable (module.go:631) = deployment-scoped create-variable analog:
  activate/reset on deploy, delete on undeploy. Use module-scoped vars for ords 1,2.
- env.RegisterVariable (plan.go:293) engine-lifetime; duplicate → duplicateModuleObjectError.
- VariableRef[T] reads; Engine.SetVariable/SetVariables atomic+coerced with Java-parity
  error strings; GetVariable/Variables snapshot.
- VariableChangeListener (variable_service.go) Old/New per committed write = IR pair.
- OnEvent(stream).SetVariables sequential working-map = sequential assignment semantics.
- EnumWhere(EnumElement) covers mylist.where(v => v='a').
- OutputLastEveryEventsExpr(VariableRef) reaches undeclared-variable Build error.
- compat ops: set-variable (needs handler), read-variable, types, snapshot.
- Runner template: internal/app/parity/variables_use.go (fixtures{registrations,builders,
  probes}); variables_onset.go for on-set; variable_deploy.go for lifecycle.
- Gaps (approved-difference, not DV blockers): no SODA/toEPL/eplToModel; no create-variable
  statement type (statement props/event-type API-only); no expression initializers
  (pre-fold); no pattern-triggered on-set (OnEvent equivalent); no parameterized-type
  metadata; no 'unrecognized type' concept.
