# Rolling Summaries Design

Date: 2026-07-07

## Goal

Keep the existing daily summary behavior intact, and add a separate "today rolling summary" flow that can periodically summarize today's messages and push each rolling result to Telegram.

## Requirements

- Daily summaries continue to summarize yesterday after the configured daily summary time.
- Rolling summaries summarize today from local midnight to the current time.
- Each chat can configure:
  - rolling summary enabled or disabled
  - rolling interval in minutes
  - max rolling sends per day
  - whether rolling summaries are pushed through the Telegram Bot
- Rolling summaries must not overwrite or block daily summaries.
- Rolling summaries must skip runs when:
  - the daily max count has been reached
  - the interval since the previous rolling run has not elapsed
  - no new messages arrived since the last rolling run
  - there are no messages today

## Data Model

Add `summary_type` to summaries:

- `daily`: the existing formal daily summary.
- `rolling`: today's periodic rolling summary.

Add `window_start` and `window_end` to summaries so rolling rows can represent a partial-day window. Daily rows can also store their full-day window, but existing UI can continue showing `summary_date`.

Add chat-level rolling settings:

- `rolling_summary_enabled`
- `rolling_summary_interval_minutes`
- `rolling_summary_max_per_day`
- `rolling_summary_bot_enabled`

## Scheduler

The scheduler still wakes every minute.

Daily flow:

- unchanged: find summary-enabled chats, check daily due time, summarize yesterday.

Rolling flow:

- find chats with rolling summaries enabled.
- calculate today's local window `[00:00, now)`.
- find the latest rolling summary for today.
- enforce interval and max-per-day.
- count messages since the previous rolling `window_end`, or since midnight for the first run.
- generate and save a new rolling summary row.
- if the row succeeds and rolling Bot delivery is enabled, send it to Telegram.

## UI

Group settings gain a "Today rolling summary" section visible when AI summary is enabled:

- enable/disable rolling summary
- interval minutes
- max sends per day
- push rolling summaries to Telegram

The summaries list shows the summary type so daily and rolling records are distinguishable.

## Testing

Backend tests cover:

- rolling interval eligibility
- max-per-day eligibility
- requiring new messages after the previous rolling summary
- daily target date remains yesterday
- rolling target window is today

Frontend verification covers:

- TypeScript compile/lint or build
- settings form can carry the new fields through existing chat save flow

