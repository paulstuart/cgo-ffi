# Claude Code Project Guide

## Agent Philosophy

You are a capable engineering assistant. Your training has a knowledge cutoff, so remain open to better approaches discovered since then. Prefer explicit verification over assumptions. When using versioned software from your training, know how to check if there are updates to use instead, and knowing to track even that step (last checked/updated)

---

## Dual Agent Architecture

### Primary Agent (Initializer)

- Defines the project structure and success criteria
- Creates reproducible tooling (`init.sh`) that rebuilds the project from scratch
- Maintains the questions, intentions, and acceptance criteria
- Runs ONCE per project setup or reset

### Worker Agent (Session Agent)

- Follows the plan established by Primary
- Handles daily development tasks
- Works incrementally, one feature per session
- Maintains progress state for resumability

---

## Session Protocol

### 1. Session Startup (ALWAYS do these steps)

```text
1. Read .claude/state/progress.md for current status
2. Read recent git log: git log --oneline -10
3. Read docs/FEATURES.md to identify next priority
4. Run basic tests to catch undocumented regressions
5. Verify working directory matches expected project
```

### 2. During Session

- Work on ONE feature/task at a time
- Log each significant action to `.claude/state/session.log`
- Commit frequently with descriptive messages, only commit if all tests pass
- If blocked, document in docs/ISSUES.md before attempting fixes

### 3. Session End (ALWAYS do these steps)

```text
1. Run tests to verify nothing broke
2. Commit all changes with clear message
3. Update .claude/state/progress.md with:
   - What was completed
   - What remains
   - Any blockers discovered
4. Update docs/FEATURES.md if feature status changed
```

---

## State Management

### Directory Structure

```text
.claude/
  state/
    progress.md      # Current status, next steps, blockers
    sessions.db      # SQLite database for session/feature tracking
  logs/
    plans.md         # Plan create/revise/execute/done
    errors.md        # Errors and resolution attempts
```

### Core Project Files

```text
init.sh              # Reproducible project setup
README.md            # Project overview
CLAUDE.md            # This file - agent instructions
docs/
  GUIDE.md           # Engineering guidelines/conventions
  GOALS.md           # Success criteria and acceptance tests
  FEATURES.md        # Feature list with status (asked/done dates)
  ISSUES.md          # Known bugs, concerns, technical debt
  DONE.md            # Completed items with completion dates
```

### SQLite Session Logging

The `init.sh` script manages a SQLite database at `.claude/state/sessions.db` with these tables:

- **sessions**: Tracks each agent session with status and linked feature
- **session_log**: Append-only event log with actions (START, PLAN, EXEC, ERROR, RESOLVE, COMMIT, END)
- **features**: Machine-readable feature tracking with acceptance tests
- **errors**: Detailed error tracking with resolution attempts
- **versions**: Dependency version tracking with last-checked timestamps

Usage via init.sh:

```bash
./init.sh start F001          # Start session linked to feature F001
./init.sh log PLAN "design auth system"
./init.sh log EXEC "created middleware"
./init.sh log COMMIT "add auth"
./init.sh end completed       # End session
./init.sh status              # Show current state
```

---

## Feature Tracking

Use `docs/FEATURES.md` with explicit pass/fail status:

```markdown
## Features

|  ID  | Feature | Status | Asked | Done |
|------|---------|--------|-------|------|
| F001 | User can compile project | DONE | 2025-01-20 | 2025-01-21 |
| F002 | Tests pass with coverage >80% | PENDING | 2025-01-22 | - |
| F003 | WASM build produces valid output | IN_PROGRESS | 2025-01-25 | - |
```

**Critical**: Never declare victory without explicit feature verification. Each feature needs a concrete acceptance test.

---

## Error Recovery

When resuming after interruption:

1. Read `progress.md` for last known state
2. Check `session.log` for incomplete actions
3. Run `git status` and `git diff` to see uncommitted work
4. Either complete the interrupted task or revert and restart cleanly

When encountering errors:

1. Log to `errors.md` with full context
2. Document attempted solutions
3. If unresolved after 3 attempts, add to `ISSUES.md` and move on
4. Never silently swallow errors

---

## Principles

1. **Deterministic Resumability**: Any interruption should be recoverable from logged state
2. **Incremental Progress**: Small commits, frequent checkpoints
3. **Explicit Verification**: Test claims, don't assume success
4. **Clean Slate**: Each completed task leaves environment ready for next
5. **Minimal Context**: Log enough to resume, not so much it becomes noise

---

## Quick Reference Commands

```bash
# Session start
./init.sh status                    # Check current state
git log --oneline -10               # Recent commits
cat .claude/state/progress.md       # Progress notes
./init.sh start F001                # Start session for feature

# During session
./init.sh log PLAN "description"    # Log planning
./init.sh log EXEC "description"    # Log execution
./init.sh log COMMIT "message"      # Log commit

# Session end
git add -A && git commit -m "msg"   # Commit changes
./init.sh end completed             # Close session

# Recovery
git status
./init.sh status
sqlite3 .claude/state/sessions.db "SELECT * FROM v_recent_activity;"
```

---

## References

- [Anthropic: Effective Harnesses for Long-Running Agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
