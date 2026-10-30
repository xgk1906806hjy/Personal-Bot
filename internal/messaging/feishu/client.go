package feishu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

type Client struct {
	cfg    *config.Config
	http   *http.Client
	tokenMu sync.Mutex
	token   string
	expires time.Time
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) getTenantAccessToken() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Now().Before(c.expires.Add(-60*time.Second)) {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{
		"app_id":     c.cfg.FeishuAppID,
		"app_secret": c.cfg.FeishuAppSecret,
	})
	resp, err := c.http.Post(
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		"application/json; charset=utf-8",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if parsed.Code != 0 || parsed.TenantAccessToken == "" {
		return "", fmt.Errorf("feishu token failed: %s (%d)", parsed.Msg, parsed.Code)
	}
	c.token = parsed.TenantAccessToken
	expire := parsed.Expire
	if expire <= 0 {
		expire = 7200
	}
	c.expires = time.Now().Add(time.Duration(expire) * time.Second)
	return c.token, nil
}

func (c *Client) ReplyText(messageID, text string) error {
	token, err := c.getTenantAccessToken()
	if err != nil {
		return err
	}
	content, _ := json.Marshal(map[string]string{"text": truncateRunes(text, 4000)})
	payload, _ := json.Marshal(map[string]string{
		"content":  string(content),
		"msg_type": "text",
	})
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages/%s/reply", messageID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode >= 300 || parsed.Code != 0 {
		msg := parsed.Msg
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("feishu reply failed: %s", msg)
	}
	return nil
}

func truncateRunes(text string, max int) string {
	if utf8.RuneCountInString(text) <= max {
		return text
	}
	runes := []rune(text)
	return string(runes[:max-20]) + "\n…(truncated)"
}
