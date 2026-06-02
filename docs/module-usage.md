# 模块使用说明

本文档说明如何选择和删除不需要的服务模块。

## 模块化设计

本项目采用模块化设计，支持三种服务类型：

- **HTTP 服务**：提供 HTTP/HTTPS RESTful API 服务
- **TCP 服务**：支持 TCP 长连接服务
- **UDP 服务**：支持 UDP 数据报服务

每种服务类型的代码完全独立，可以根据实际需求选择启用或禁用。

## 如何选择服务类型

### 通过配置文件启用/禁用服务

在 `configs/config.yaml` 中配置：

```yaml
http:
  enabled: true   # 启用 HTTP 服务
  host: 0.0.0.0
  port: 8080

tcp:
  enabled: false  # 禁用 TCP 服务
  host: 0.0.0.0
  port: 9000

udp:
  enabled: false  # 禁用 UDP 服务
  host: 0.0.0.0
  port: 9001
```

### 仅使用 HTTP 服务

如果只需要 HTTP 服务，可以删除以下文件和目录：

**删除的目录**：
- `internal/handler/tcp/`
- `internal/handler/udp/`
- `internal/protocol/`（TCP/UDP 协议编解码）

**删除的文件**：
- `internal/server/tcp.go`
- `internal/server/udp.go`

**保留**：
- `internal/handler/http/`
- `internal/server/http.go`
- `internal/router/router.go`
- 共享的 `service/`、`repository/`、`model/` 层

**配置文件**：
```yaml
http:
  enabled: true
tcp:
  enabled: false
udp:
  enabled: false
```

### 仅使用 TCP 服务

如果只需要 TCP 服务，可以删除以下文件和目录：

**删除的目录**：
- `internal/handler/http/`
- `internal/handler/udp/`
- `internal/router/`
- `internal/middleware/http/`

**删除的文件**：
- `internal/server/http.go`
- `internal/server/udp.go`

**保留**：
- `internal/handler/tcp/`
- `internal/server/tcp.go`
- `internal/protocol/`
- 共享的 `service/`、`repository/`、`model/` 层

**配置文件**：
```yaml
http:
  enabled: false
tcp:
  enabled: true
udp:
  enabled: false
```

### 仅使用 UDP 服务

如果只需要 UDP 服务，可以删除以下文件和目录：

**删除的目录**：
- `internal/handler/http/`
- `internal/handler/tcp/`
- `internal/router/`
- `internal/middleware/http/`

**删除的文件**：
- `internal/server/http.go`
- `internal/server/tcp.go`

**保留**：
- `internal/handler/udp/`
- `internal/server/udp.go`
- `internal/protocol/`
- 共享的 `service/`、`repository/`、`model/` 层

**配置文件**：
```yaml
http:
  enabled: false
tcp:
  enabled: false
udp:
  enabled: true
```

### HTTP + TCP 组合

如果需要 HTTP 和 TCP 服务，可以删除以下文件和目录：

**删除的目录**：
- `internal/handler/udp/`

**删除的文件**：
- `internal/server/udp.go`

**保留**：
- `internal/handler/http/`
- `internal/handler/tcp/`
- `internal/server/http.go`
- `internal/server/tcp.go`
- `internal/protocol/`
- `internal/router/router.go`
- 共享的 `service/`、`repository/`、`model/` 层

**配置文件**：
```yaml
http:
  enabled: true
tcp:
  enabled: true
udp:
  enabled: false
```

### 使用全部服务

保留所有模块，配置文件中启用所有服务：

```yaml
http:
  enabled: true
tcp:
  enabled: true
udp:
  enabled: true
```

## 模块依赖关系

### HTTP 服务依赖

- `internal/handler/http/` - HTTP 处理器
- `internal/server/http.go` - HTTP 服务器
- `internal/router/` - 路由注册（`Handlers` 聚合 + `RegisterRoutes`）
- `internal/middleware/http/` - HTTP 中间件
- 共享的 `service/`、`repository/`、`model/` 层

### TCP 服务依赖

- `internal/handler/tcp/` - TCP 处理器
- `internal/server/tcp.go` - TCP 服务器
- `internal/protocol/` - 协议编解码
- 共享的 `service/`、`repository/`、`model/` 层

### UDP 服务依赖

- `internal/handler/udp/` - UDP 处理器
- `internal/server/udp.go` - UDP 服务器
- `internal/protocol/` - 协议编解码
- 共享的 `service/`、`repository/`、`model/` 层

### 共享层

以下层被所有服务类型共享，删除服务类型时**不要删除**：

- `internal/service/` - 业务逻辑层
- `internal/repository/` - 数据访问层
- `internal/model/` - 数据模型
- `internal/dto/` - 数据传输对象
- `internal/config/` - 配置管理
- `internal/database/` - 数据库连接
- `internal/cache/` - 缓存
- `internal/event/` - 事件系统
- `internal/logger/` - 日志
- `internal/errors/` - 错误处理
- `internal/response/` - 统一响应

## 删除模块后的清理步骤

删除模块后，需要执行以下清理步骤：

### 1. 更新 go.mod

删除模块后，运行以下命令清理未使用的依赖：

```bash
go mod tidy
```

### 2. 更新依赖组装

在 `internal/app/` 中，删除或注释对应服务的 wiring 逻辑（如 `wireTCP()`、`wireUDP()` 中的组装代码）。

### 3. 更新 Makefile

如果删除了不需要的服务，可以删除相关的构建命令。

### 4. 更新文档

更新 README.md 和其他文档，移除对已删除模块的说明。

## 常见问题

### Q: 删除模块后编译失败怎么办？

A: 检查以下内容：
1. 确保已删除所有相关文件和目录
2. 运行 `go mod tidy` 清理依赖
3. 检查 `internal/app/` 中的组装逻辑
4. 检查配置文件中是否禁用了对应服务

### Q: 可以只删除部分中间件吗？

A: 可以。中间件在 `internal/middleware/` 目录下，可以单独删除不需要的中间件文件。但需要注意：
- 删除中间件后，需要在路由注册中移除对它的引用
- 某些中间件可能有依赖关系，删除时需要一并处理

### Q: 删除 TCP/UDP 服务后，protocol 目录还需要吗？

A: 如果只使用 HTTP 服务，可以删除 `internal/protocol/` 目录，因为 HTTP 服务不使用自定义协议编解码。

### Q: 如何重新启用已删除的模块？

A: 从项目模板中恢复相关文件和目录，然后：
1. 运行 `go mod tidy` 恢复依赖
2. 在 `internal/app/` 中恢复对应 wiring
3. 在配置文件中启用对应服务
4. 重新编译

## 示例代码

### 示例 1：仅 HTTP 服务的最小配置

```yaml
# configs/config.yaml
app:
  name: my-app
  version: 1.0.0

http:
  enabled: true
  host: 0.0.0.0
  port: 8080

tcp:
  enabled: false

udp:
  enabled: false

database:
  enabled: true
  driver: sqlite
  database: ./data/app.db

redis:
  enabled: false
```

### 示例 2：HTTP + TCP 服务配置

```yaml
# configs/config.yaml
app:
  name: my-app
  version: 1.0.0

http:
  enabled: true
  host: 0.0.0.0
  port: 8080

tcp:
  enabled: true
  host: 0.0.0.0
  port: 9000
  codec: json

udp:
  enabled: false

database:
  enabled: true
  driver: sqlite
  database: ./data/app.db

redis:
  enabled: false
```

## 总结

- 模块化设计允许灵活选择服务类型
- 通过配置文件控制服务启用/禁用
- 删除不需要的模块可以减小应用体积
- 删除模块后需要清理依赖和配置
- 共享层（service、repository、model）不要删除
