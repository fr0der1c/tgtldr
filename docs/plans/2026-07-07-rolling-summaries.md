# Rolling Summaries Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add configurable today rolling summaries with Telegram Bot delivery while preserving the existing next-day daily summary behavior.

**Architecture:** Extend the summary data model with a `summary_type` and optional time window, then add a second scheduler path for rolling summaries. Daily summaries keep the existing date-based flow; rolling summaries create independent rows for today's partial-day windows and deliver through the existing Bot service.

**Tech Stack:** Go backend, PostgreSQL migrations, Next.js/React frontend, existing Telegram Bot delivery service.

---

### Task 1: Add Backend Model And Migration

**Files:**
- Modify: `app/internal/model/types.go`
- Create: `app/internal/store/migrations/013_rolling_summaries.sql`

**Steps:**
1. Add `SummaryTypeDaily` and `SummaryTypeRolling`.
2. Add summary fields: `SummaryType`, `WindowStart`, `WindowEnd`.
3. Add chat fields: `RollingSummaryEnabled`, `RollingSummaryIntervalMinutes`, `RollingSummaryMaxPerDay`, `RollingSummaryBotEnabled`.
4. Add migration columns and indexes.
5. Run `go test ./internal/model ./internal/store`.

### Task 2: Add Repository Support

**Files:**
- Modify: `app/internal/store/chats_repo.go`
- Modify: `app/internal/store/summaries_repo.go`
- Modify: `app/internal/store/summaries_search.go`
- Modify: `app/internal/store/summaries_stats.go`

**Steps:**
1. Update chat SELECT/UPDATE/INSERT scans.
2. Update summary SELECT/scan/save logic for type and windows.
3. Add repository helpers for latest rolling summary and rolling count per day.
4. Run focused store tests.

### Task 3: Add Scheduler Eligibility Tests

**Files:**
- Modify: `app/internal/scheduler/service_test.go`

**Steps:**
1. Write failing tests for rolling due logic.
2. Verify tests fail before production code changes.
3. Implement helper functions only after red tests exist.

### Task 4: Implement Rolling Scheduler

**Files:**
- Modify: `app/internal/scheduler/service.go`
- Modify: `app/internal/summary/service.go`

**Steps:**
1. Add a window-based summary runner.
2. Keep `RunDailySummary` as a wrapper.
3. Add rolling scheduler pass after the daily pass.
4. Add Bot title formatting for daily vs rolling.
5. Run `go test ./internal/scheduler ./internal/summary ./internal/store`.

### Task 5: Expose API And UI Fields

**Files:**
- Modify: `app/internal/api/router.go`
- Modify: `web/lib/types.ts`
- Modify: `web/components/chats-panel.tsx`
- Modify: `web/components/summaries-panel-sections.tsx`
- Modify: `web/components/summary-detail-drawer.tsx`

**Steps:**
1. Include rolling fields in chat save payloads.
2. Add rolling controls in the chat editor.
3. Show daily/rolling labels in summary rows and detail views.
4. Run frontend build or lint.

### Task 6: Final Verification

**Steps:**
1. Run `go test ./...` in `app`.
2. Run `npm ci` if dependencies are missing, then `npm run build` in `web`.
3. Check `git diff`.
4. Summarize files changed and any remaining deployment steps.

