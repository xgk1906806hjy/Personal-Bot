package qq

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/xgk1906806hjy/Personal-Bot/internal/orchestrator"
)

const (
	opHTTPCallbackACK = 12
	opCallbackVerify  = 13
)

type webhookPayload struct {
	Op int             `json:"op"`
	T  string          `json:"t"`
	D  json.RawMessage `json:"d"`
}

func mountWebhook(mux *http.ServeMux, api *APIClient, orch *orchestrator.Orchestrator, botSecret string) {
	mux.HandleFunc("/qq/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body failed", http.StatusBadRequest)
			return
		}

		var payload webhookPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if payload.Op == opCallbackVerify {
			var d struct {
				PlainToken string `json:"plain_token"`
				EventTs    string `json:"event_ts"`
			}
			_ = json.Unmarshal(payload.D, &d)
			if d.PlainToken == "" || d.EventTs == "" {
				http.Error(w, "invalid validation payload", http.StatusBadRequest)
				return
			}
			sig := SignValidationPayload(botSecret, d.EventTs, d.PlainToken)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"plain_token": d.PlainToken,
				"signature":   sig,
			})
			return
		}

		sig := r.Header.Get("X-Signature-Ed25519")
		ts := r.Header.Get("X-Signature-Timestamp")
		if sig != "" && ts != "" {
			if !VerifyCallbackSignature(botSecret, ts, raw, sig) {
				log.Printf("[qq] signature verify failed")
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// ACK ASAP
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"op": opHTTPCallbackACK, "d": 0})

		if payload.Op != opDispatch {
			return
		}
		var d map[string]any
		_ = json.Unmarshal(payload.D, &d)
		go handleInbound(payload.T, d, api, orch)
	})
	log.Printf("[boot] messaging adapter mounted: qq → POST /qq/webhook (ports allowed by QQ: 80/443/8080/8443)")
}
