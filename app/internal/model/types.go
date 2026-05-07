package model

import "time"

type DeliveryMode string

const (
	DeliveryModeDashboard DeliveryMode = "dashboard"
	DeliveryModeBot       DeliveryMode = "bot"
)

type SummaryStatus string

const (
	SummaryStatusPending   SummaryStatus = "pending"
	SummaryStatusRunning   SummaryStatus = "running"
	SummaryStatusSucceeded SummaryStatus = "succeeded"
	SummaryStatusFailed    SummaryStatus = "failed"
)

type OutputMode string
type Language string

const (
	OutputModeAuto       OutputMode = "auto"
	OutputModeManual     OutputMode = "manual"
	LanguageZhCN         Language   = "zh-CN"
	LanguageEN           Language   = "en"
	DefaultOpenAIBaseURL            = "https://api.openai.com/v1"
)

func NormalizeLanguage(language Language) Language {
	if language == LanguageEN {
		return LanguageEN
	}
	return LanguageZhCN
}

type AppSettings struct {
	ID                   int64      `json:"id"`
	TelegramAPIID        int        `json:"telegramApiId"`
	TelegramAPIHash      string     `json:"telegramApiHash,omitempty"`
	OpenAIBaseURL        string     `json:"openAIBaseUrl"`
	OpenAIAPIKey         string     `json:"openAIApiKey,omitempty"`
	OpenAIModel          string     `json:"openAIModel"`
	OpenAITemperature    float64    `json:"openAITemperature"`
	OpenAIOutputMode     OutputMode `json:"openAIOutputMode"`
	OpenAIMaxOutputToken int        `json:"openAIMaxOutputTokens"`
	SummaryParallelism   int        `json:"summaryParallelism"`
	DefaultTimezone      string     `json:"defaultTimezone"`
	Language             Language   `json:"language"`
	BotEnabled           bool       `json:"botEnabled"`
	BotToken             string     `json:"botToken,omitempty"`
	BotTargetChatID      string     `json:"botTargetChatId"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

func (s AppSettings) Sanitized() AppSettings {
	s.TelegramAPIHash = redactSecret(s.TelegramAPIHash)
	s.OpenAIAPIKey = redactSecret(s.OpenAIAPIKey)
	s.BotToken = redactSecret(s.BotToken)
	return s
}

type LocalAuth struct {
	PasswordHash      string    `json:"-"`
	PasswordUpdatedAt time.Time `json:"passwordUpdatedAt"`
	SessionVersion    int       `json:"sessionVersion"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type LocalSession struct {
	ID             int64     `json:"id"`
	SessionID      string    `json:"sessionId"`
	SessionVersion int       `json:"sessionVersion"`
	ExpiresAt      time.Time `json:"expiresAt"`
	LastSeenAt     time.Time `json:"lastSeenAt"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type TelegramAuth struct {
	ID              int64     `json:"id"`
	PhoneNumber     string    `json:"phoneNumber"`
	TelegramUserID  int64     `json:"telegramUserId"`
	TelegramName    string    `json:"telegramName"`
	TelegramHandle  string    `json:"telegramHandle"`
	SessionData     []byte    `json:"-"`
	Status          string    `json:"status"`
	LastConnectedAt time.Time `json:"lastConnectedAt"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Chat struct {
	ID               int64        `json:"id"`
	TelegramChatID   int64        `json:"telegramChatId"`
	TelegramAccess   int64        `json:"telegramAccessHash"`
	Title            string       `json:"title"`
	Username         string       `json:"username"`
	ChatType         string       `json:"chatType"`
	Enabled          bool         `json:"enabled"`
	SummaryEnabled   bool         `json:"summaryEnabled"`
	SummaryContext   string       `json:"summaryContext"`
	SummaryPrompt    string       `json:"summaryPrompt"`
	SummaryTimeLocal string       `json:"summaryTimeLocal"`
	SummaryTimezone  string       `json:"summaryTimezone"`
	DeliveryMode     DeliveryMode `json:"deliveryMode"`
	ModelOverride    string       `json:"modelOverride"`
	KeepBotMessages  bool         `json:"keepBotMessages"`
	FilteredSenders  []string     `json:"filteredSenders"`
	FilteredKeywords []string     `json:"filteredKeywords"`
	AlertEnabled     bool         `json:"alertEnabled"`
	AlertKeywords    []string     `json:"alertKeywords"`
	CreatedAt        time.Time    `json:"createdAt"`
	UpdatedAt        time.Time    `json:"updatedAt"`
}

type Message struct {
	ID                int64     `json:"id"`
	ChatID            int64     `json:"chatId"`
	TelegramMessageID int       `json:"telegramMessageId"`
	TelegramSenderID  int64     `json:"telegramSenderId"`
	SenderName        string    `json:"senderName"`
	SenderUsername    string    `json:"senderUsername"`
	SenderIsBot       bool      `json:"senderIsBot"`
	TextContent       string    `json:"textContent"`
	Caption           string    `json:"caption"`
	MessageType       string    `json:"messageType"`
	MediaKind         string    `json:"mediaKind"`
	ReplyToMessageID  int       `json:"replyToMessageId"`
	MessageTime       time.Time `json:"messageTime"`
	RawJSON           string    `json:"rawJson"`
	CreatedAt         time.Time `json:"createdAt"`
}

func (m Message) SummaryText() string {
	text := m.TextContent
	if text == "" {
		text = m.Caption
	}
	return text
}

type Summary struct {
	ID                 int64         `json:"id"`
	ChatID             int64         `json:"chatId"`
	SummaryDate        string        `json:"summaryDate"`
	Status             SummaryStatus `json:"status"`
	Content            string        `json:"content"`
	Model              string        `json:"model"`
	SourceMessageCount int           `json:"sourceMessageCount"`
	ChunkCount         int           `json:"chunkCount"`
	GeneratedAt        time.Time     `json:"generatedAt"`
	DeliveredAt        *time.Time    `json:"deliveredAt,omitempty"`
	DeliveryError      string        `json:"deliveryError"`
	ErrorMessage       string        `json:"errorMessage"`
	MatchSnippet       string        `json:"matchSnippet,omitempty"`
	MatchedFields      []string      `json:"matchedFields,omitempty"`
	CreatedAt          time.Time     `json:"createdAt"`
	UpdatedAt          time.Time     `json:"updatedAt"`
}

type SummaryListResponse struct {
	Items    []Summary `json:"items"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
}

type SummaryStats struct {
	Total           int `json:"total"`
	SuccessCount    int `json:"successCount"`
	ProcessingCount int `json:"processingCount"`
	FailedCount     int `json:"failedCount"`
}

type SummaryContextPreview struct {
	SummaryID        int64                 `json:"summaryId"`
	ChatID           int64                 `json:"chatId"`
	SummaryDate      string                `json:"summaryDate"`
	Model            string                `json:"model"`
	SystemPrompt     string                `json:"systemPrompt"`
	FinalPrompt      string                `json:"finalPrompt"`
	MessageCount     int                   `json:"messageCount"`
	ChunkCount       int                   `json:"chunkCount"`
	Chunks           []SummaryContextChunk `json:"chunks"`
	FinalInputNotice string                `json:"finalInputNotice"`
	PreviewNotice    string                `json:"previewNotice"`
}

type SummaryContextChunk struct {
	Index        int    `json:"index"`
	MessageCount int    `json:"messageCount"`
	Content      string `json:"content"`
}

type HistoryBackfillStatus string

const (
	HistoryBackfillStatusPending   HistoryBackfillStatus = "pending"
	HistoryBackfillStatusRunning   HistoryBackfillStatus = "running"
	HistoryBackfillStatusSucceeded HistoryBackfillStatus = "succeeded"
	HistoryBackfillStatusFailed    HistoryBackfillStatus = "failed"
)

type HistoryBackfillTask struct {
	ID            string                `json:"id"`
	ChatID        int64                 `json:"chatId"`
	ChatTitle     string                `json:"chatTitle"`
	FromDate      string                `json:"fromDate"`
	ToDate        string                `json:"toDate"`
	Status        HistoryBackfillStatus `json:"status"`
	ImportedCount int                   `json:"importedCount"`
	ErrorMessage  string                `json:"errorMessage"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
	CompletedAt   *time.Time            `json:"completedAt,omitempty"`
}

type Bootstrap struct {
	SettingsConfigured bool          `json:"settingsConfigured"`
	PasswordConfigured bool          `json:"passwordConfigured"`
	Authenticated      bool          `json:"authenticated"`
	TelegramAuthorized bool          `json:"telegramAuthorized"`
	EnabledChatCount   int           `json:"enabledChatCount"`
	BotEnabled         bool          `json:"botEnabled"`
	Settings           AppSettings   `json:"settings"`
	Auth               *TelegramAuth `json:"auth,omitempty"`
}

type AuthStep string

const (
	AuthStepIdle     AuthStep = "idle"
	AuthStepCode     AuthStep = "code"
	AuthStepPassword AuthStep = "password"
	AuthStepDone     AuthStep = "done"
)

type AuthSessionState struct {
	Step        AuthStep  `json:"step"`
	PhoneNumber string    `json:"phoneNumber"`
	CodeHash    string    `json:"-"`
	Deadline    time.Time `json:"deadline"`
}

func redactSecret(secret string) string {
	if len(secret) <= 4 {
		return ""
	}
	return secret[:2] + "****" + secret[len(secret)-2:]
}
