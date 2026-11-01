// Package messaging 定义消息平台适配器接口。
// 各子包（qq / feishu / wechat）实现 Adapter，把平台事件转为 orchestrator 入站消息。
package messaging

import (
	"net/http"

	"github.com/xgk1906806hjy/Personal-Bot/internal/config"
	"github.com/xgk1906806hjy/Personal-Bot/internal/orchestrator"
)

// AdapterContext 是 Mount 时注入的共享依赖。
type AdapterContext struct {
	Config       *config.Config
	Orchestrator *orchestrator.Orchestrator
	Mux          *http.ServeMux
}

// Adapter 挂载平台传输（HTTP webhook 和/或长连接），并在 Close 时释放资源。
type Adapter interface {
	ID() config.MessagingPlatform
	Mount(ctx AdapterContext) error
	Close() error
}
