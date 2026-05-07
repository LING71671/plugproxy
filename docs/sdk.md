# Go SDK 接入

plugproxy 提供两种 Go 接入方式：

- `pkg/client`：推荐方式，连接已经运行的 plugproxy HTTP 服务。
- `pkg/plugproxy`：嵌入式方式，在业务进程内启动一个轻量 plugproxy 服务。

## HTTP Client SDK

适合已经通过 `plugproxy run` 启动代理池服务的项目。

```go
package main

import (
	"context"
	"fmt"

	"github.com/LING71671/plugproxy/pkg/client"
	"github.com/LING71671/plugproxy/pkg/model"
)

func main() {
	ctx := context.Background()
	c := client.New("http://127.0.0.1:8899")

	proxy, err := c.GetProxy(ctx, client.GetProxyOptions{
		Strategy: "fastest",
		Protocol: model.ProtocolHTTP,
		Healthy:  true,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(proxy.URL())
}
```

常用方法：

```go
c.GetProxy(ctx, client.GetProxyOptions{Healthy: true})
c.ListProxies(ctx, client.ListOptions{Status: model.HealthHealthy, Limit: 20})
c.Stats(ctx)
c.Sources(ctx)
c.Metrics(ctx)
c.TriggerRefresh(ctx)
c.RefreshStatus(ctx)
c.CancelRefresh(ctx)
```

## 嵌入式 SDK

适合希望业务进程自己托管 plugproxy 的场景。

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/LING71671/plugproxy/pkg/client"
	"github.com/LING71671/plugproxy/pkg/model"
	"github.com/LING71671/plugproxy/pkg/plugproxy"
)

func main() {
	ctx := context.Background()
	svc := plugproxy.New(plugproxy.Config{
		Addr:      "127.0.0.1:0",
		SkipCheck: true,
		Refresh:   false,
	})

	if err := svc.Start(ctx); err != nil {
		panic(err)
	}
	defer svc.Close(context.Background())

	proxy, err := svc.GetProxy(ctx, client.GetProxyOptions{
		Strategy: "fastest",
		Protocol: model.ProtocolHTTP,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(proxy.Address)
	fmt.Println(svc.URL())
	_, _ = svc.Refresh(ctx)
	time.Sleep(time.Second)
}
```

嵌入式 SDK 暴露启动、关闭、获取代理、列出代理、触发刷新和取消刷新。更细的源管理、检测策略和管理面板接口后续再逐步开放。
