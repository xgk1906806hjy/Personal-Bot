package wechat

import (
	"fmt"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging"
)

// Adapter is a placeholder for WeChat / WeCom.
type Adapter struct{}

func NewAdapter() *Adapter { return &Adapter{} }

func (a *Adapter) ID() config.MessagingPlatform { return config.PlatformWechat }

func (a *Adapter) Mount(_ messaging.AdapterContext) error {
	return fmt.Errorf(`messaging adapter "wechat" is not implemented yet; add webhook parsing + reply in internal/messaging/wechat`)
}

func (a *Adapter) Close() error { return nil }
