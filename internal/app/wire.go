// Package app 负责依赖装配（wiring），当前主要是按配置创建消息适配器。
package app

import (
	"fmt"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging/feishu"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging/qq"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging/wechat"
)

// CreateMessagingAdapters 按 MESSAGING_CHANNELS 顺序创建适配器实例。
func CreateMessagingAdapters(ids []config.MessagingPlatform) ([]messaging.Adapter, error) {
	out := make([]messaging.Adapter, 0, len(ids))
	for _, id := range ids {
		a, err := createMessagingAdapter(id)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func createMessagingAdapter(id config.MessagingPlatform) (messaging.Adapter, error) {
	switch id {
	case config.PlatformQQ:
		return qq.NewAdapter(), nil
	case config.PlatformFeishu:
		return feishu.NewAdapter(), nil
	case config.PlatformWechat:
		return wechat.NewAdapter(), nil
	default:
		return nil, fmt.Errorf("unknown messaging platform: %s", id)
	}
}

// ReadyMessagingPlatforms 返回已真正实现（非占位）的消息平台。
func ReadyMessagingPlatforms() []config.MessagingPlatform {
	return []config.MessagingPlatform{config.PlatformQQ, config.PlatformFeishu}
}
