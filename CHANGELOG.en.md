# Changelog

[中文版](CHANGELOG.md)

## 2026-07-12

### Added

- Added support for connecting and managing multiple Telegram accounts in one TGTLDR instance. Each Telegram chat still has a single record, and you can choose which account receives its messages and loads its message history.

### Changed

- Reorganized the settings page into categorized tabs and refined multi-account management, Bot target-conversation binding, and page-transition interactions; the chat list now shows the active account for chats shared by multiple accounts.

### Fixed

- Fixed an issue where the sign-in card could collapse after sending a verification code while adding a Telegram account.

## 2026-07-11

### Added

- Chat names in the chat list now link directly to the summaries page with that chat automatically selected.

## 2026-06-14

### Added

- The chat list now shows a 30-day daily message activity strip under each chat, with a custom hover prompt for each day's message count.
- The summary list now shows the source message count next to each summary's date and model.

### Fixed

- Telegram user sessions now clear invalid session data and mark the account disconnected after permanent authentication errors such as `AUTH_KEY_DUPLICATED`, preventing listeners and history backfills from reusing a bad session.

## 2026-06-06

### Changed

- Docker Compose now restores the `postgres`, `app`, and `web` services by default after the Docker daemon or host restarts.

## 2026-05-10

### Added

- OpenAI configuration now supports connection testing, so you can validate the endpoint during setup or in system settings before saving.
- Added streaming request support to handle relay services that do not work well with non-streaming requests and may otherwise time out.
- Added automatic retry controls for failed summary generation, including retry limit, initial delay, and backoff multiplier.
- Failed summary details now let you view and copy the OpenAI request parameters, System prompt, and User prompt for easier debugging.

### Changed

- Failed summary details now show the retry status and next retry time, making it easier to understand what will happen next.
- Optimized the Docker image build flow to avoid unnecessary rebuilds and speed up publishing.
- Added bilingual date-based changelogs and linked them from both READMEs.

### Fixed

- Added a fallback copy implementation for failed summary details, so copy still works more often when `navigator.clipboard` is unavailable and shows a clearer failure state when it does not succeed.

## 2026-05-08

### Added

- Added `socks5://` and `socks5h://` proxy support for the Telegram user client, making login, group sync, and message listening easier in restricted network environments.

### Changed

- Expanded the Telegram proxy documentation and Docker usage notes to make self-hosted deployment easier to configure.
