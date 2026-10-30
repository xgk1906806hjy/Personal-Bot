package app

import (
	"fmt"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging/feishu"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging/qq"
	"github.com/xgk1906806hjy/Personal-Bot/internal/messaging/wechat"
)

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

func ReadyMessagingPlatforms() []config.MessagingPlatform {
	return []config.MessagingPlatform{config.PlatformQQ, config.PlatformFeishu}
}
