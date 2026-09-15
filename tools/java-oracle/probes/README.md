# JDK dispatch-order probes

Standalone probes compiled against the pinned Esper checkout's prebuilt
classes (javac -cp common/compiler/runtime target/classes, no maven run) that
pinned the same-instant scheduled-callback dispatch order used by
`reverseStatementGroups` (internal/esper/runtime.go):

- OrderProbe.java — two `#time` view-expiry statements (4 s deployed first as
  s0, 3 s second as s1): at the shared 5000 ms tick the second-deployed
  statement delivers first (`s1:E2` before `s0:E1`).
- OrderProbe2.java — two identical windows deployed in order (zz then aa):
  `aa` delivers before `zz`, independent of statement names.
- OrderProbe3.java — two `timer:at` pattern statements: `psecond` before
  `pfirst`, extending the rule to pattern-scheduled callbacks.

Event-driven deliveries are not covered by these probes; registration order
there is pinned by the differential chains.
