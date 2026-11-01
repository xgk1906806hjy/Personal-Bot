package qq

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xgk1906806hjy/Personal-Bot/internal/orchestrator"
)

// QQ 网关 WebSocket 操作码与 Intent（群 + C2C）。
const (
	opDispatch        = 0
	opHeartbeat       = 1
	opIdentify        = 2
	opResume          = 6
	opReconnect       = 7
	opInvalidSession  = 9
	opHello           = 10
	opHeartbeatACK    = 11
	intentGroupAndC2C = 1 << 25
)

type gatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  *int            `json:"s"`
	T  string          `json:"t"`
}

type WSClient struct {
	api  *APIClient
	orch *orchestrator.Orchestrator

	mu       sync.Mutex
	conn     *websocket.Conn
	stopped  bool
	session  string
	lastSeq  *int
	resume   bool
	attempt  int
	hbEvery  time.Duration
	hbStop   chan struct{}
	reconnCh chan struct{}
}

func NewWSClient(api *APIClient, orch *orchestrator.Orchestrator) *WSClient {
	return &WSClient{
		api:      api,
		orch:     orch,
		hbEvery:  45 * time.Second,
		reconnCh: make(chan struct{}, 1),
	}
}

func (c *WSClient) Start() {
	c.mu.Lock()
	c.stopped = false
	c.mu.Unlock()
	go c.loop()
}

func (c *WSClient) Stop() {
	c.mu.Lock()
	c.stopped = true
	if c.hbStop != nil {
		close(c.hbStop)
		c.hbStop = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
}

func (c *WSClient) loop() {
	for {
		c.mu.Lock()
		stopped := c.stopped
		c.mu.Unlock()
		if stopped {
			return
		}
		if err := c.connectOnce(); err != nil {
			log.Printf("[qq:ws] connect failed: %v", err)
		}
		c.mu.Lock()
		stopped = c.stopped
		attempt := c.attempt
		c.attempt++
		c.mu.Unlock()
		if stopped {
			return
		}
		shift := attempt
		if shift > 5 {
			shift = 5
		}
		delay := time.Second << shift
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		log.Printf("[qq:ws] reconnect in %s (attempt %d)", delay, attempt+1)
		time.Sleep(delay)
	}
}

func (c *WSClient) connectOnce() error {
	if _, err := c.api.GetAccessToken(); err != nil {
		return err
	}
	url, err := c.api.GetGatewayURL()
	if err != nil {
		return err
	}
	log.Printf("[qq:ws] connecting %s", url)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	log.Printf("[qq:ws] socket open, waiting for Hello")

	defer func() {
		c.stopHeartbeat()
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		_ = conn.Close()
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[qq:ws] read closed: %v", err)
			return nil
		}
		if err := c.handleFrame(conn, data); err != nil {
			log.Printf("[qq:ws] frame error: %v", err)
		}
	}
}

func (c *WSClient) handleFrame(conn *websocket.Conn, data []byte) error {
	var payload gatewayPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("[qq:ws] non-json frame ignored")
		return nil
	}
	if payload.S != nil {
		c.mu.Lock()
		c.lastSeq = payload.S
		c.mu.Unlock()
	}

	switch payload.Op {
	case opHello:
		var d struct {
			HeartbeatInterval int `json:"heartbeat_interval"`
		}
		_ = json.Unmarshal(payload.D, &d)
		if d.HeartbeatInterval > 0 {
			c.hbEvery = time.Duration(d.HeartbeatInterval) * time.Millisecond
		}
		c.mu.Lock()
		resume := c.resume && c.session != "" && c.lastSeq != nil
		c.mu.Unlock()
		if resume {
			return c.sendResume(conn)
		}
		return c.sendIdentify(conn)
	case opDispatch:
		var d map[string]any
		_ = json.Unmarshal(payload.D, &d)
		switch payload.T {
		case "READY":
			if sid, ok := d["session_id"].(string); ok {
				c.mu.Lock()
				c.session = sid
				c.resume = true
				c.attempt = 0
				c.mu.Unlock()
				c.startHeartbeat(conn)
				log.Printf("[qq:ws] READY session=%s", sid)
			}
		case "RESUMED":
			c.mu.Lock()
			c.attempt = 0
			c.mu.Unlock()
			c.startHeartbeat(conn)
			log.Printf("[qq:ws] RESUMED")
		default:
			go handleInbound(payload.T, d, c.api, c.orch)
		}
	case opHeartbeatACK:
		// ok
	case opReconnect:
		log.Printf("[qq:ws] server requested reconnect")
		c.mu.Lock()
		c.resume = true
		c.mu.Unlock()
		_ = conn.Close()
	case opInvalidSession:
		var resumable bool
		_ = json.Unmarshal(payload.D, &resumable)
		log.Printf("[qq:ws] invalid session resumable=%v", resumable)
		if !resumable {
			c.mu.Lock()
			c.session = ""
			c.lastSeq = nil
			c.resume = false
			c.mu.Unlock()
		}
		_ = conn.Close()
	default:
		log.Printf("[qq:ws] unhandled op=%d t=%s", payload.Op, payload.T)
	}
	return nil
}

func (c *WSClient) sendIdentify(conn *websocket.Conn) error {
	token, err := c.api.GetAccessToken()
	if err != nil {
		return err
	}
	payload := map[string]any{
		"op": opIdentify,
		"d": map[string]any{
			"token":   "QQBot " + token,
			"intents": intentGroupAndC2C,
			"shard":   []int{0, 1},
			"properties": map[string]string{
				"$os":      "windows",
				"$browser": "personal-bot",
				"$device":  "personal-bot",
			},
		},
	}
	if err := conn.WriteJSON(payload); err != nil {
		return err
	}
	log.Printf("[qq:ws] Identify sent")
	return nil
}

func (c *WSClient) sendResume(conn *websocket.Conn) error {
	token, err := c.api.GetAccessToken()
	if err != nil {
		return err
	}
	c.mu.Lock()
	session := c.session
	seq := 0
	if c.lastSeq != nil {
		seq = *c.lastSeq
	}
	c.mu.Unlock()
	if session == "" {
		return c.sendIdentify(conn)
	}
	payload := map[string]any{
		"op": opResume,
		"d": map[string]any{
			"token":      "QQBot " + token,
			"session_id": session,
			"seq":        seq,
		},
	}
	if err := conn.WriteJSON(payload); err != nil {
		return err
	}
	log.Printf("[qq:ws] Resume sent session=%s seq=%d", session, seq)
	return nil
}

func (c *WSClient) startHeartbeat(conn *websocket.Conn) {
	c.stopHeartbeat()
	stop := make(chan struct{})
	c.mu.Lock()
	c.hbStop = stop
	every := c.hbEvery
	c.mu.Unlock()

	send := func() {
		c.mu.Lock()
		seq := c.lastSeq
		c.mu.Unlock()
		_ = conn.WriteJSON(map[string]any{"op": opHeartbeat, "d": seq})
	}
	send()
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				send()
			}
		}
	}()
}

func (c *WSClient) stopHeartbeat() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hbStop != nil {
		close(c.hbStop)
		c.hbStop = nil
	}
}
