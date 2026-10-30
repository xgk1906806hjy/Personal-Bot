package qq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
)

type ReplyTarget struct {
	Kind        string // group | c2c
	GroupOpenID string
	UserOpenID  string
}

type APIClient struct {
	cfg    *config.Config
	client *http.Client

	tokenMu     sync.Mutex
	accessToken string
	expiresAt   time.Time

	seqMu sync.Mutex
	seq   map[string]int
}

func NewAPIClient(cfg *config.Config) *APIClient {
	return &APIClient{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		seq:    make(map[string]int),
	}
}

func (c *APIClient) GetAccessToken() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.expiresAt.Add(-60*time.Second)) {
		return c.accessToken, nil
	}

	body, _ := json.Marshal(map[string]string{
		"appId":        c.cfg.QQAppID,
		"clientSecret": c.cfg.QQAppSecret,
	})
	resp, err := c.client.Post(c.cfg.QQAPIBaseURL+"/app/getAppAccessToken", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   any    `json:"expires_in"`
		Code        int    `json:"code"`
		Message     string `json:"message"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if parsed.AccessToken == "" {
		return "", fmt.Errorf("QQ getAppAccessToken failed: %s (%d)", parsed.Message, parsed.Code)
	}
	expiresSec := 7200
	switch v := parsed.ExpiresIn.(type) {
	case float64:
		expiresSec = int(v)
	case string:
		fmt.Sscanf(v, "%d", &expiresSec)
	}
	c.accessToken = parsed.AccessToken
	c.expiresAt = time.Now().Add(time.Duration(expiresSec) * time.Second)
	return c.accessToken, nil
}

func (c *APIClient) GetGatewayURL() (string, error) {
	token, err := c.GetAccessToken()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, c.cfg.QQAPIBaseURL+"/gateway", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		URL     string `json:"url"`
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if parsed.URL == "" {
		return "", fmt.Errorf("QQ get gateway failed: %s (%d)", parsed.Message, parsed.Code)
	}
	return parsed.URL, nil
}

func (c *APIClient) ReplyText(target ReplyTarget, messageID, text string) error {
	token, err := c.GetAccessToken()
	if err != nil {
		return err
	}
	seq := c.nextSeq(messageID)
	var path string
	switch target.Kind {
	case "group":
		path = "/v2/groups/" + target.GroupOpenID + "/messages"
	case "c2c":
		path = "/v2/users/" + target.UserOpenID + "/messages"
	default:
		return fmt.Errorf("unknown reply target kind: %s", target.Kind)
	}

	payload, _ := json.Marshal(map[string]any{
		"content":  truncate(text, 4000),
		"msg_type": 0,
		"msg_id":   messageID,
		"msg_seq":  seq,
	})
	req, err := http.NewRequest(http.MethodPost, c.cfg.QQAPIBaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "QQBot "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		ErrCode int    `json:"err_code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode >= 300 || (parsed.ErrCode != 0 && parsed.Message != "") {
		msg := parsed.Message
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("QQ reply failed: %s", msg)
	}
	return nil
}

func (c *APIClient) nextSeq(messageID string) int {
	c.seqMu.Lock()
	defer c.seqMu.Unlock()
	c.seq[messageID]++
	if len(c.seq) > 5000 {
		for k := range c.seq {
			delete(c.seq, k)
			break
		}
	}
	return c.seq[messageID]
}

func truncate(text string, max int) string {
	if len([]rune(text)) <= max {
		return text
	}
	runes := []rune(text)
	return string(runes[:max-20]) + "\n…(truncated)"
}

func cleanText(content string) string {
	content = strings.TrimSpace(content)
	// Strip <@!openid> style mentions.
	for {
		start := strings.Index(content, "<@")
		if start < 0 {
			break
		}
		end := strings.Index(content[start:], ">")
		if end < 0 {
			break
		}
		content = content[:start] + " " + content[start+end+1:]
	}
	return strings.Join(strings.Fields(content), " ")
}
