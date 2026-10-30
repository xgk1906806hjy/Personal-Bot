package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type MessagingPlatform string
type ModelChannel string
type QQTransport string

const (
	PlatformFeishu MessagingPlatform = "feishu"
	PlatformWechat MessagingPlatform = "wechat"
	PlatformQQ     MessagingPlatform = "qq"

	ModelGatewayAPI  ModelChannel = "gateway_api"
	ModelCodexLocal  ModelChannel = "codex_local"
	ModelCursorLocal ModelChannel = "cursor_local"

	QQTransportWebsocket QQTransport = "websocket"
	QQTransportWebhook   QQTransport = "webhook"
	QQTransportBoth      QQTransport = "both"
)

type Config struct {
	Port string
	Host string

	MessagingChannels []MessagingPlatform
	ModelChannel      ModelChannel

	FeishuAppID             string
	FeishuAppSecret         string
	FeishuEncryptKey        string
	FeishuVerificationToken string

	QQAppID      string
	QQAppSecret  string
	QQAPIBaseURL string
	QQTransport  QQTransport

	CodexWorkingDirectory string
	CodexBin              string

	GatewayAPIBaseURL          string
	GatewayAPIKey              string
	GatewayAPIModel            string
	GatewayAPIExtraHeaderName  string
	GatewayAPIExtraHeaderValue string

	CursorAPIKey      string
	CursorModel       string
	CursorNodeBin     string
	CursorAgentScript string // absolute path to cursor-agent.mjs

	SessionTTLHours      int
	SessionCompressAfter int
	SessionKeepRecent    int
	BotSystemPrompt      string

	ToolsEnabled          bool
	ToolWorkspace         string
	ToolCommandTimeoutSec int
	ToolMaxRounds         int
	ToolMaxOutputBytes    int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:                       getenv("PORT", "8787"),
		Host:                       getenv("HOST", "0.0.0.0"),
		ModelChannel:               ModelChannel(getenv("MODEL_CHANNEL", string(ModelGatewayAPI))),
		FeishuAppID:                os.Getenv("FEISHU_APP_ID"),
		FeishuAppSecret:            os.Getenv("FEISHU_APP_SECRET"),
		FeishuEncryptKey:           os.Getenv("FEISHU_ENCRYPT_KEY"),
		FeishuVerificationToken:    os.Getenv("FEISHU_VERIFICATION_TOKEN"),
		QQAppID:                    os.Getenv("QQ_APP_ID"),
		QQAppSecret:                os.Getenv("QQ_APP_SECRET"),
		QQAPIBaseURL:               strings.TrimRight(getenv("QQ_API_BASE_URL", "https://api.bot.qq.com"), "/"),
		QQTransport:                QQTransport(getenv("QQ_TRANSPORT", string(QQTransportWebsocket))),
		CodexWorkingDirectory:      getenv("CODEX_WORKING_DIRECTORY", "."),
		CodexBin:                   os.Getenv("CODEX_BIN"),
		GatewayAPIBaseURL:          strings.TrimRight(getenv("GATEWAY_API_BASE_URL", "https://api.openai.com/v1"), "/"),
		GatewayAPIKey:              os.Getenv("GATEWAY_API_KEY"),
		GatewayAPIModel:            getenv("GATEWAY_API_MODEL", "gpt-4o-mini"),
		GatewayAPIExtraHeaderName:  os.Getenv("GATEWAY_API_EXTRA_HEADER_NAME"),
		GatewayAPIExtraHeaderValue: os.Getenv("GATEWAY_API_EXTRA_HEADER_VALUE"),
		CursorAPIKey:               os.Getenv("CURSOR_API_KEY"),
		CursorModel:                getenv("CURSOR_MODEL", "composer-2.5"),
		CursorNodeBin:              getenv("CURSOR_NODE_BIN", "node"),
		SessionTTLHours:            getenvInt("SESSION_TTL_HOURS", 2),
		SessionCompressAfter:       getenvInt("SESSION_COMPRESS_AFTER", 30),
		SessionKeepRecent:          getenvInt("SESSION_KEEP_RECENT", 16),
		BotSystemPrompt: getenv(
			"BOT_SYSTEM_PROMPT",
			"You are my personal exclusive assistant. Be concise, practical, and clear. If you are unsure, say what is missing instead of guessing.",
		),
		ToolsEnabled:          getenvBool("TOOLS_ENABLED", true),
		ToolCommandTimeoutSec: getenvInt("TOOL_COMMAND_TIMEOUT_SEC", 60),
		ToolMaxRounds:         getenvInt("TOOL_MAX_ROUNDS", 8),
		ToolMaxOutputBytes:    getenvInt("TOOL_MAX_OUTPUT_BYTES", 100_000),
	}

	channels, err := parseMessagingChannels(getenv("MESSAGING_CHANNELS", "qq"))
	if err != nil {
		return nil, err
	}
	cfg.MessagingChannels = channels

	workspace := os.Getenv("TOOL_WORKSPACE")
	if workspace == "" {
		workspace = cfg.CodexWorkingDirectory
	}
	if workspace == "" {
		workspace = "."
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("TOOL_WORKSPACE: %w", err)
	}
	cfg.ToolWorkspace = absWorkspace

	script := os.Getenv("CURSOR_AGENT_SCRIPT")
	if script == "" {
		script = filepath.Join("scripts", "cursor-agent", "cursor-agent.mjs")
	}
	absScript, err := filepath.Abs(script)
	if err != nil {
		return nil, fmt.Errorf("CURSOR_AGENT_SCRIPT: %w", err)
	}
	cfg.CursorAgentScript = absScript

	switch cfg.ModelChannel {
	case ModelGatewayAPI, ModelCodexLocal, ModelCursorLocal:
	default:
		return nil, fmt.Errorf("unknown MODEL_CHANNEL: %s", cfg.ModelChannel)
	}
	switch cfg.QQTransport {
	case QQTransportWebsocket, QQTransportWebhook, QQTransportBoth:
	default:
		return nil, fmt.Errorf("unknown QQ_TRANSPORT: %s", cfg.QQTransport)
	}

	for _, p := range cfg.MessagingChannels {
		switch p {
		case PlatformFeishu:
			if cfg.FeishuAppID == "" || cfg.FeishuAppSecret == "" {
				return nil, fmt.Errorf("FEISHU_APP_ID and FEISHU_APP_SECRET are required when MESSAGING_CHANNELS includes feishu")
			}
		case PlatformQQ:
			if cfg.QQAppID == "" || cfg.QQAppSecret == "" {
				return nil, fmt.Errorf("QQ_APP_ID and QQ_APP_SECRET are required when MESSAGING_CHANNELS includes qq")
			}
		case PlatformWechat:
			// Placeholder adapter exists; Mount() returns a clear not-implemented error.
		}
	}

	if cfg.ModelChannel == ModelGatewayAPI && cfg.GatewayAPIKey == "" {
		return nil, fmt.Errorf("GATEWAY_API_KEY is required when MODEL_CHANNEL=gateway_api")
	}
	if cfg.ModelChannel == ModelCursorLocal && cfg.CursorAPIKey == "" {
		return nil, fmt.Errorf("CURSOR_API_KEY is required when MODEL_CHANNEL=cursor_local")
	}

	return cfg, nil
}

func parseMessagingChannels(raw string) ([]MessagingPlatform, error) {
	parts := strings.Split(raw, ",")
	out := make([]MessagingPlatform, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		switch MessagingPlatform(p) {
		case PlatformFeishu, PlatformWechat, PlatformQQ:
			out = append(out, MessagingPlatform(p))
		default:
			return nil, fmt.Errorf("unknown messaging platform: %s", p)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("MESSAGING_CHANNELS is empty")
	}
	return out, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func getenvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
