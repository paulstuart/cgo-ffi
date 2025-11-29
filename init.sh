#!/bin/bash
#
# init.sh - Project initialization and session management
# Run once to set up project structure, or with flags for session ops
#

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLAUDE_DIR="${PROJECT_ROOT}/.claude"
STATE_DIR="${CLAUDE_DIR}/state"
LOGS_DIR="${CLAUDE_DIR}/logs"
DOCS_DIR="${PROJECT_ROOT}/docs"
DB_FILE="${STATE_DIR}/sessions.db"

# Generate session ID: sess_YYYYMMDD_NNN
generate_session_id() {
    local today=$(date +%Y%m%d)
    local count=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) + 1 FROM sessions WHERE date(created_at) = date('now');")
    printf "sess_%s_%03d" "$today" "$count"
}

# Initialize SQLite database
init_database() {
    sqlite3 "$DB_FILE" <<'SQL'
-- Sessions table: one row per agent session
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    status TEXT DEFAULT 'active' CHECK(status IN ('active', 'completed', 'interrupted', 'failed')),
    feature_id TEXT,
    summary TEXT
);

-- Session log: append-only event log
CREATE TABLE IF NOT EXISTS session_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    action TEXT NOT NULL CHECK(action IN ('START', 'PLAN', 'EXEC', 'ERROR', 'RESOLVE', 'COMMIT', 'END')),
    message TEXT,
    metadata JSON,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- Features: machine-readable feature tracking
CREATE TABLE IF NOT EXISTS features (
    id TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'in_progress', 'done', 'blocked')),
    asked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    done_at TIMESTAMP,
    acceptance_test TEXT,
    notes TEXT
);

-- Errors: detailed error tracking
CREATE TABLE IF NOT EXISTS errors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    error_type TEXT,
    message TEXT,
    context TEXT,
    resolution TEXT,
    attempts INTEGER DEFAULT 1,
    resolved INTEGER DEFAULT 0,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- Version tracking for dependencies
CREATE TABLE IF NOT EXISTS versions (
    name TEXT PRIMARY KEY,
    current_version TEXT,
    last_checked TIMESTAMP,
    source_url TEXT,
    notes TEXT
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_session_log_session ON session_log(session_id);
CREATE INDEX IF NOT EXISTS idx_session_log_timestamp ON session_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_features_status ON features(status);
CREATE INDEX IF NOT EXISTS idx_errors_session ON errors(session_id);

-- View: recent activity
CREATE VIEW IF NOT EXISTS v_recent_activity AS
SELECT
    sl.timestamp,
    sl.session_id,
    sl.action,
    sl.message,
    s.status as session_status
FROM session_log sl
JOIN sessions s ON sl.session_id = s.id
ORDER BY sl.timestamp DESC
LIMIT 50;

-- View: feature progress
CREATE VIEW IF NOT EXISTS v_feature_progress AS
SELECT
    status,
    COUNT(*) as count,
    GROUP_CONCAT(id, ', ') as feature_ids
FROM features
GROUP BY status;
SQL
    echo "Database initialized: $DB_FILE"
}

# Create directory structure
init_directories() {
    mkdir -p "$STATE_DIR" "$LOGS_DIR" "$DOCS_DIR"

    # Initialize state files if they don't exist
    [[ -f "${STATE_DIR}/progress.md" ]] || cat > "${STATE_DIR}/progress.md" <<'EOF'
# Progress

## Current Status
Project initialized. No work started yet.

## Next Steps
- Review docs/FEATURES.md for priorities
- Begin first feature

## Blockers
None
EOF

    # Initialize docs if they don't exist
    [[ -f "${DOCS_DIR}/FEATURES.md" ]] || cat > "${DOCS_DIR}/FEATURES.md" <<'EOF'
# Features

| ID | Feature | Status | Asked | Done |
|----|---------|--------|-------|------|
| F001 | Project compiles successfully | PENDING | - | - |

## Feature Details

### F001: Project compiles successfully
**Acceptance Test**: `go build ./...` exits with code 0
EOF

    [[ -f "${DOCS_DIR}/GOALS.md" ]] || cat > "${DOCS_DIR}/GOALS.md" <<'EOF'
# Project Goals

## Success Criteria
Define what "done" looks like for this project.

## Acceptance Tests
List concrete, verifiable tests that prove success.
EOF

    [[ -f "${DOCS_DIR}/ISSUES.md" ]] || cat > "${DOCS_DIR}/ISSUES.md" <<'EOF'
# Issues

Track bugs, concerns, and technical debt here.

| ID | Issue | Severity | Status | Added |
|----|-------|----------|--------|-------|
EOF

    [[ -f "${DOCS_DIR}/GUIDE.md" ]] || cat > "${DOCS_DIR}/GUIDE.md" <<'EOF'
# Engineering Guide

## Code Style
Document project-specific conventions here.

## Testing
How to run tests, coverage requirements, etc.

## Build
Build commands and requirements.
EOF

    [[ -f "${LOGS_DIR}/plans.md" ]] || echo "# Plans Log" > "${LOGS_DIR}/plans.md"
    [[ -f "${LOGS_DIR}/errors.md" ]] || echo "# Errors Log" > "${LOGS_DIR}/errors.md"

    echo "Directory structure initialized"
}

# Start a new session
start_session() {
    local feature_id="${1:-}"
    local session_id=$(generate_session_id)

    sqlite3 "$DB_FILE" "INSERT INTO sessions (id, feature_id) VALUES ('$session_id', '$feature_id');"
    sqlite3 "$DB_FILE" "INSERT INTO session_log (session_id, action, message) VALUES ('$session_id', 'START', 'Session started');"

    echo "$session_id"
}

# Log an action to current session
log_action() {
    local session_id="$1"
    local action="$2"
    local message="${3:-}"
    local metadata="${4:-{}}"

    sqlite3 "$DB_FILE" "INSERT INTO session_log (session_id, action, message, metadata) VALUES ('$session_id', '$action', '$message', '$metadata');"
}

# End a session
end_session() {
    local session_id="$1"
    local status="${2:-completed}"
    local summary="${3:-}"

    sqlite3 "$DB_FILE" "UPDATE sessions SET ended_at = CURRENT_TIMESTAMP, status = '$status', summary = '$summary' WHERE id = '$session_id';"
    sqlite3 "$DB_FILE" "INSERT INTO session_log (session_id, action, message) VALUES ('$session_id', 'END', 'Session ended: $status');"
}

# Get current/last session
get_session() {
    sqlite3 "$DB_FILE" "SELECT id FROM sessions WHERE status = 'active' ORDER BY created_at DESC LIMIT 1;"
}

# Show session status
show_status() {
    echo "=== Recent Sessions ==="
    sqlite3 -header -column "$DB_FILE" "SELECT id, status, feature_id, created_at FROM sessions ORDER BY created_at DESC LIMIT 5;"

    echo ""
    echo "=== Feature Progress ==="
    sqlite3 -header -column "$DB_FILE" "SELECT * FROM v_feature_progress;"

    echo ""
    echo "=== Recent Activity ==="
    sqlite3 -header -column "$DB_FILE" "SELECT timestamp, action, message FROM session_log ORDER BY timestamp DESC LIMIT 10;"
}

# Export session log to markdown (for compatibility)
export_session_log() {
    local session_id="$1"
    sqlite3 "$DB_FILE" "SELECT printf('[%s] [%s] [%s] %s', session_id, timestamp, action, message) FROM session_log WHERE session_id = '$session_id' ORDER BY timestamp;"
}

# Add or update a feature
add_feature() {
    local id="$1"
    local description="$2"
    local acceptance_test="${3:-}"

    sqlite3 "$DB_FILE" "INSERT OR REPLACE INTO features (id, description, acceptance_test) VALUES ('$id', '$description', '$acceptance_test');"
}

# Update feature status
update_feature() {
    local id="$1"
    local status="$2"

    if [[ "$status" == "done" ]]; then
        sqlite3 "$DB_FILE" "UPDATE features SET status = '$status', done_at = CURRENT_TIMESTAMP WHERE id = '$id';"
    else
        sqlite3 "$DB_FILE" "UPDATE features SET status = '$status' WHERE id = '$id';"
    fi
}

# Log an error
log_error() {
    local session_id="$1"
    local error_type="$2"
    local message="$3"
    local context="${4:-}"

    sqlite3 "$DB_FILE" "INSERT INTO errors (session_id, error_type, message, context) VALUES ('$session_id', '$error_type', '$message', '$context');"
}

# Track dependency version
track_version() {
    local name="$1"
    local version="$2"
    local source_url="${3:-}"

    sqlite3 "$DB_FILE" "INSERT OR REPLACE INTO versions (name, current_version, last_checked, source_url) VALUES ('$name', '$version', CURRENT_TIMESTAMP, '$source_url');"
}

# Show help
show_help() {
    cat <<EOF
Usage: $0 [command] [args...]

Commands:
  init              Initialize project structure and database (default)
  start [feature]   Start a new session, optionally linked to a feature
  end [status]      End current session (completed|interrupted|failed)
  log <action> <msg> Log an action to current session
  status            Show current status
  feature <id> <desc> [test]  Add/update a feature
  feature-status <id> <status>  Update feature status
  error <type> <msg> [ctx]  Log an error
  version <name> <ver> [url]  Track a dependency version
  export <session>  Export session log to markdown format
  help              Show this help

Actions for log: START, PLAN, EXEC, ERROR, RESOLVE, COMMIT, END

Examples:
  $0 init
  $0 start F001
  $0 log PLAN "implementing user auth"
  $0 log EXEC "created middleware"
  $0 log COMMIT "add auth middleware"
  $0 end completed
  $0 feature F002 "User can login" "curl returns 200 with valid token"
  $0 feature-status F002 done
EOF
}

# Main
main() {
    local cmd="${1:-init}"
    shift || true

    case "$cmd" in
        init)
            init_directories
            init_database
            echo "Project initialized. Run '$0 start' to begin a session."
            ;;
        start)
            start_session "$@"
            ;;
        end)
            local session=$(get_session)
            [[ -n "$session" ]] && end_session "$session" "$@"
            ;;
        log)
            local session=$(get_session)
            [[ -n "$session" ]] && log_action "$session" "$@"
            ;;
        status)
            show_status
            ;;
        feature)
            add_feature "$@"
            ;;
        feature-status)
            update_feature "$@"
            ;;
        error)
            local session=$(get_session)
            [[ -n "$session" ]] && log_error "$session" "$@"
            ;;
        version)
            track_version "$@"
            ;;
        export)
            export_session_log "$@"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            echo "Unknown command: $cmd"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
