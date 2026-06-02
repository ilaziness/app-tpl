# TCP/UDP 服务使用指南

本指南介绍如何使用 app-tpl 项目中的 TCP/UDP 服务功能。

## 概述

本项目支持同时运行 HTTP、TCP 和 UDP 三种服务。TCP/UDP 服务是可选模块，可以通过配置文件启用或禁用。

## 自定义协议设计

### 协议格式

TCP/UDP 服务使用自定义二进制协议，协议头格式如下：

| 字段 | 大小 | 说明 |
|------|------|------|
| 魔数 | 4字节 | `0x54435053` ("TCPS")，用于协议识别 |
| 版本 | 1字节 | 协议版本号，当前为 0x01 |
| 消息类型 | 1字节 | 请求(0x01)/响应(0x02)/心跳(0x03)/错误(0x04) |
| 消息ID | 4字节 | 用于请求响应匹配 |
| 序列化类型 | 1字节 | 0=JSON，1=二进制(Gob) |
| 保留字段 | 1字节 | 预留扩展 |
| 数据长度 | 4字节 | 负载长度 |
| 校验和 | 4字节 | CRC32 校验 |
| 负载数据 | 变长 | 实际业务数据 |

### 协议设计要点

- **魔数**：快速识别协议，防止误解析
- **版本号**：支持协议升级和兼容性处理
- **消息类型**：区分不同业务消息
- **消息ID**：支持异步请求响应匹配
- **序列化类型**：支持多种序列化方式切换
- **校验和**：检测数据传输错误
- **长度字段**：解决粘包/半包问题

## 配置

### 启用 TCP/UDP 服务

在 `configs/config.yaml` 中配置 TCP/UDP 服务：

```yaml
# TCP server configuration
tcp:
  enabled: true
  host: 0.0.0.0
  port: 9000
  read_timeout: 30
  write_timeout: 30
  heartbeat_interval: 30
  heartbeat_timeout: 90
  shutdown_timeout: 30
  codec: json  # json or binary
  max_connections: 10000

# UDP server configuration
udp:
  enabled: true
  host: 0.0.0.0
  port: 9001
  read_buffer_size: 4096
  write_buffer_size: 4096
  shutdown_timeout: 30
  codec: json  # json or binary
  worker_pool_size: 100
```

### 配置说明

#### TCP 配置项

- `enabled`: 是否启用 TCP 服务
- `host`: 监听地址
- `port`: 监听端口
- `read_timeout`: 读超时时间（秒）
- `write_timeout`: 写超时时间（秒）
- `heartbeat_interval`: 心跳间隔（秒）
- `heartbeat_timeout`: 心跳超时（秒），超时后断开连接
- `shutdown_timeout`: 优雅关闭超时（秒）
- `codec`: 编解码类型，`json` 或 `binary`
- `max_connections`: 最大连接数

#### UDP 配置项

- `enabled`: 是否启用 UDP 服务
- `host`: 监听地址
- `port`: 监听端口
- `read_buffer_size`: 读缓冲区大小（字节）
- `write_buffer_size`: 写缓冲区大小（字节）
- `shutdown_timeout`: 优雅关闭超时（秒）
- `codec`: 编解码类型，`json` 或 `binary`
- `worker_pool_size`: Worker pool 大小

## 编解码器

### JSON 编解码

JSON 编解码器使用标准库 `encoding/json`，适合需要人类可读的场景。

```go
codec := protocol.NewJSONCodec()
```

### 二进制编解码

二进制编解码器使用标准库 `encoding/gob`，适合需要高性能和紧凑传输的场景。

```go
codec := protocol.NewBinaryCodec()
```

## 中间件

### TCP 中间件

项目提供以下 TCP 中间件：

#### 认证中间件

```go
auth := middleware.NewAuthMiddleware(logger, "your-token")
if auth.Authenticate(conn, authToken) {
    // 认证通过
}
```

#### 限流中间件

```go
ratelimit := middleware.NewRateLimitMiddleware(logger, 100, true)
if ratelimit.Allow(conn) {
    // 允许处理
}
```

#### 日志中间件

```go
logger := middleware.NewLoggerMiddleware(logger)
logger.LogConnection(conn, "connected")
logger.LogMessage(conn, "request", len(data))
```

#### 超时中间件

```go
timeout := middleware.NewTimeoutMiddleware(logger, 30, 30)
timeout.SetTimeouts(conn)
```

### UDP 中间件

项目提供以下 UDP 中间件：

#### 认证中间件

```go
auth := middleware.NewAuthMiddleware(logger, "your-token")
if auth.Authenticate(remoteAddr, authToken) {
    // 认证通过
}
```

#### 限流中间件

```go
ratelimit := middleware.NewRateLimitMiddleware(logger, 100, true)
if ratelimit.Allow(remoteAddr) {
    // 允许处理
}
```

#### 日志中间件

```go
logger := middleware.NewLoggerMiddleware(logger)
logger.LogPacket(remoteAddr, "request", len(data))
```

## 示例业务

### TCP 示例

#### Echo 服务

Echo 服务原样返回接收到的消息，在 `internal/app/tcp.go` 中组装为 TCP Server 的默认 handler。

#### 聊天室服务

聊天室服务支持多客户端广播，内部采用注册表模式分发消息类型（示例代码在 `handler/tcp/`，待 TCP Dispatcher 后接入 `app/tcp.go`）。

聊天消息格式：

```json
{
  "type": "join|message|leave",
  "from": "connection-id",
  "content": "message content"
}
```

### UDP 示例

#### 时间查询服务

时间查询服务返回当前服务器时间，通过 `Dispatcher` 按 `"type"` 字段路由，无需手动调用。

请求格式：

```json
{
  "type": "time"
}
```

响应格式：

```json
{
  "type": "time",
  "timestamp": "2026-05-22T17:00:00Z",
  "unix": 1716384000
}
```

#### 统计服务

统计服务返回服务器统计信息，通过 `Dispatcher` 按 `"type"` 字段路由，无需手动调用。

请求格式：

```json
{
  "type": "stats"
}
```

响应格式：

```json
{
  "type": "stats",
  "packets_received": 1000,
  "packets_sent": 950,
  "bytes_received": 102400,
  "bytes_sent": 97280,
  "sessions": 10
}
```

## 运行服务

### 启动服务

```bash
# 使用默认配置
./build/app-tpl serve

# 指定配置文件
./build/app-tpl serve --config configs/config.yaml

# 指定环境
./build/app-tpl serve --env dev
```

### 验证服务

启动后，应用会输出所有服务的监听地址：

```
INFO  Application started  name=app-tpl version=1.0.0 env=dev http_addr=:8080 tcp_addr=:9000 udp_addr=:9001
```

## 客户端示例

### TCP 客户端示例（Go）

```go
package main

import (
    "encoding/json"
    "net"
)

func main() {
    conn, err := net.Dial("tcp", "localhost:9000")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // 发送消息
    msg := map[string]string{"type": "message", "content": "hello"}
    data, _ := json.Marshal(msg)
    conn.Write(data)

    // 接收响应
    buf := make([]byte, 1024)
    n, _ := conn.Read(buf)
    println(string(buf[:n]))
}
```

### UDP 客户端示例（Go）

```go
package main

import (
    "encoding/json"
    "net"
)

func main() {
    conn, err := net.Dial("udp", "localhost:9001")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // 发送时间查询请求
    req := map[string]string{"type": "time"}
    data, _ := json.Marshal(req)
    conn.Write(data)

    // 接收响应
    buf := make([]byte, 1024)
    n, _, _ := conn.ReadFromUDP(buf)
    println(string(buf[:n]))
}
```

## 性能特性

### TCP 服务

- 支持 ≥ 10000 并发连接
- 连接池管理
- 心跳机制（可配置）
- 优雅关闭
- 读写超时控制

### UDP 服务

- 高并发数据包处理
- Worker pool 处理
- CRC32 校验
- 会话管理
- 优雅关闭

## 故障排查

### TCP 服务无法启动

1. 检查端口是否被占用
2. 检查配置文件中的端口范围（1-65535）
3. 检查编解码类型是否为 `json` 或 `binary`

### UDP 服务无法启动

1. 检查端口是否被占用
2. 检查配置文件中的端口范围（1-65535）
3. 检查 worker pool size 是否为正数

### 连接被频繁断开

1. 检查心跳超时配置
2. 检查读写超时配置
3. 查看日志中的超时警告

## 最佳实践

1. **生产环境建议使用二进制编解码**：JSON 编解码可读性好但性能较低，二进制编解码性能更高
2. **合理设置心跳间隔**：根据网络延迟和业务需求调整心跳间隔和超时
3. **监控连接数**：定期检查连接数，避免资源耗尽
4. **使用中间件**：合理使用认证、限流、日志中间件增强服务安全性
5. **优雅关闭**：确保服务在关闭时正确处理现有连接和数据包

## 扩展开发

### 添加新的 TCP 消息类型

项目采用注册表模式（Sub-Handler 模式），新增消息类型**不需要修改任何已有文件**：

1. 在 `internal/handler/tcp/` 下创建新文件，实现 `SubMessageHandler` 接口：

```go
type SubMessageHandler interface {
    MessageType() string
    Handle(conn *types.Connection, msg *ChatMessage) error
}
```

2. 在 `ChatHandler.NewChatHandler()` 中调用 `h.register(...)` 注册新的子 Handler。

### 添加新的 UDP 消息类型

1. 在 `internal/handler/udp/` 下创建新文件，实现 `SubPacketHandler` 接口：

```go
type SubPacketHandler interface {
    MessageType() string
    Handle(packet *types.UDPPacket) ([]byte, error)
}
```

2. 在 `NewDispatcher()` 的参数列表中加入新 Handler，并调用 `d.register(...)` 完成注册。

3. 在 `internal/app/udp.go` 的 `wireUDP()` 中组装新 Handler，并在 `NewDispatcher` 中注册。

### 添加新的中间件

1. 实现中间件逻辑
2. 在 `internal/middleware/tcp/` 或 `internal/middleware/udp/` 下创建新文件
3. 在连接/数据包处理流程中调用中间件

## 相关文档

- [项目 README](../README.md)
- [PRD 文档](PRD_v1.md)
- [配置指南](../configs/config.yaml)
