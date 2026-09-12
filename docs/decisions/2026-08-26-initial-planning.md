# Initial planning decision

**Decision:** Build a dependable foreground `anchor start` before daemonisation, restart policies, health checks, a TUI, plugins, or integrations.

**Reasoning:** Configuration correctness, dependency planning, child-process ownership, logging, and cancellation are the foundational risks. Adding background state or richer interfaces first would increase failure modes without proving the core workflow.

**Consequences:**

- v0.1 is intentionally narrow but complete.
- `anchor stop`, `anchor status`, and `anchor logs` are roadmap commands, not commands to stub now.
- Interfaces may permit extension but must not simulate future features in the initial codebase.
