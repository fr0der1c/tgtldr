# Changelog

[中文版](CHANGELOG.md)

## 2026-09-06

### Changed

- Chat summaries and Daily Digests now retry up to four times with delays of 1, 3, 5, and 10 minutes. Upgrades set enabled retry policies to four attempts while preserving disabled retries.
- Chat delivery now offers “Include in Daily Digest” without the hint that misaligned form fields.
- Telegram Daily Digests omit web source markers while web references remain available.

- Enabling Daily Digest now automatically includes all chats with AI summaries enabled, including web-only chats. Activation copy explains this scope, and individual chats can still be excluded afterward.

## 2026-09-05

### Fixed

- Fixed the Daily Digest description flashing when entering the Summaries page while settings load asynchronously; the page no longer assumes the feature is disabled during loading.

## 2026-09-04

### Added

- Added Daily Digest delivery. After every chat participating in Bot delivery finishes its daily summary, TGTLDR combines them into one cross-chat digest and sends it to Telegram.
- Added Daily Digest history and details to the Summaries page, including source summaries, omitted or inactive chats, manual regeneration, and delivery retries.

### Changed

- Per-chat Telegram settings now determine participation in delivery, while system settings choose between separate chat messages and one combined Daily Digest. Mode changes apply starting with messages from the day they are saved.
- Daily Digest and Catch Up now share the same context budgeting, adaptive chunking, and multi-level merge implementation for multi-source summaries.
- The Summaries page now shows an enable action when Daily Digest is inactive and uses a confirmation dialog to explain participation, the effective date, and whether Telegram Bot will also be enabled.
- The README now contains the complete Chinese and English documentation in one file instead of maintaining a separate English README.

## 2026-08-23

### Added

- Added Catch Up to the Summaries page. It can create a source-linked period review for the last 7, 14, or 30 days, or for a custom range of up to 90 days, using chats with AI summaries enabled.
- Catch Up supports background generation, history and detail drawers, default Telegram delivery when the Bot is configured, and independent retries for failed delivery.

### Changed

- Daily summaries and Catch Up now prefer the complete input based on the model context window and actual token counts. Chunking occurs only when required by capacity or an upstream context-limit response, with multi-level merge fallback when needed.
- Summary engine settings now support automatic context-window detection and a manual override for custom models and OpenAI-compatible providers.

## 2026-07-21

### Added

- Added global chat-history search to the chat list for finding messages, senders, and usernames across all chats. Selecting a result opens that chat on the matching day and positions the message in context.
- Chat history now downloads and displays static WebP, video WebM, and animated TGS stickers. Existing sticker resources are automatically discovered when account listeners start after an upgrade.

### Changed

- Photos, files, and stickers without text no longer inject “without text” placeholders into summary transcripts or chat message bodies.

### Fixed

- Fixed real-time monitoring ignoring group messages sent by the connected Telegram account itself.
- Fixed messages sent by anonymous administrators or as a chat/channel appearing as `Unknown`. Existing history is repaired in background batches after an upgrade, with the chat title used as a display fallback until repair completes.

## 2026-07-17

### Fixed

- Fixed web content no longer appearing behind iOS Safari's floating bottom address bar while preserving the mobile bottom navigation's safe spacing and shadow.
- Fixed chat activity and account deletion tooltips widening mobile pages and allowing users to drag into a blank area on the right.
- Fixed the login page canvas not extending behind iOS Safari's floating bottom address bar.
- Fixed the dashboard's transparent outer layer stopping before iOS Safari's floating bottom address bar.

### Changed

- Added a separate web bind-address setting so the web UI can be exposed to the local network while the backend API remains local-only.
- Summary metric cards can now clear existing criteria and switch directly to all, succeeded, processing, or failed summaries.

## 2026-07-16

### Added

- Added per-chat message history with daily browsing, loading for earlier messages, in-chat search, filtering based on chat settings, and direct access to each day's summary; also improved the mobile browsing experience.
- Chat history now automatically downloads and displays photos, videos, audio, voice messages, and regular files inline, together with available sender and chat avatars. Files over 100 MB can be downloaded after explicit confirmation, and failed downloads can be retried.

### Changed

- Chat names and public usernames are now consistently aligned to the right of the chat icon. A default group icon is shown when no avatar is available or when loading fails, preventing list misalignment.
- Added an “Automatically download message attachments” preference. When disabled, new and queued attachments wait for manual download while avatar downloads remain unaffected.

## 2026-07-14

### Changed

- Raw prompts now open in a dedicated drawer; closing it returns to the existing summary details, with prompt content directly accessible and scrollable on both desktop and mobile.
- Summary details and raw prompts now animate in and out: they slide from the right on desktop and rise from the bottom on mobile. Opening a raw prompt also reuses the existing backdrop instead of adding more blur.

### Fixed

- Fixed the raw-prompt window appearing behind the summary drawer backdrop and requiring page scrolling before it became visible.
- Fixed a client-side exception when opening raw prompts for zero-message summaries and other empty-context cases.

## 2026-07-13

### Changed

- The web app now adapts to phones: the dashboard uses bottom navigation, chat lists become touch-friendly cards, and summary filters and detail reading are rearranged for narrow screens.

### Fixed

- Fixed the desktop summary-detail drawer expanding beyond its intended width when it contains long content.

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
