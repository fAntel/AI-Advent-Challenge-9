package agent

import "time"

const (
	FlashModel = "deepseek-v4-flash"
	ProModel   = "deepseek-v4-pro"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Settings struct {
	Model       string   `json:"model"`
	Reasoning   string   `json:"reasoning"`
	Temperature *float64 `json:"temperature,omitempty"`
	Approach    string   `json:"approach"`
	Roles       []string `json:"roles,omitempty"`
	Format      string   `json:"format,omitempty"`
	Length      string   `json:"length,omitempty"`
	Stop        string   `json:"stop,omitempty"`
	Stats       bool     `json:"stats,omitempty"`
	Debug       bool     `json:"debug,omitempty"`
}

type Usage struct {
	PromptTokens          int `json:"prompt_tokens"`
	PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	CompletionTokens      int `json:"completion_tokens"`
	ReasoningTokens       int `json:"reasoning_tokens"`
	TotalTokens           int `json:"total_tokens"`
}

type Metrics struct {
	Requests int           `json:"requests"`
	Duration time.Duration `json:"duration"`
	Usage    Usage         `json:"usage"`
	CostUSD  float64       `json:"cost_usd"`
}

type Operation struct {
	ID          string     `json:"id"`
	State       string     `json:"state"`
	Settings    Settings   `json:"settings"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	Diagnostics string     `json:"diagnostics,omitempty"`
	Metrics     Metrics    `json:"metrics"`
	BaseCount   int        `json:"base_count"`
}

type Session struct {
	ID           string     `json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	SystemPrompt string     `json:"system_prompt,omitempty"`
	Settings     Settings   `json:"settings"`
	Messages     []Message  `json:"messages"`
	Operation    *Operation `json:"operation,omitempty"`
	Attached     bool       `json:"attached,omitempty"`
	Preview      string     `json:"preview,omitempty"`
}

type CreateSessionRequest struct {
	SystemPrompt string    `json:"system_prompt"`
	Settings     *Settings `json:"settings,omitempty"`
}

type MessageRequest struct {
	Content  string    `json:"content"`
	Settings *Settings `json:"settings,omitempty"`
}

type LeaseResponse struct {
	Token string `json:"token"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
