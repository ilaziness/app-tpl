# 目录结构

```shell
app-tpl/
├── main.go                          # 程序入口，调用 cmd.Execute()
├── Makefile                         # 构建、测试、运行等快捷命令
├── go.mod / go.sum                  # 模块依赖
├── cmd/                             # CLI 命令层（Cobra：serve、config、migrate、gen 等）
├── configs/                         # 配置文件（config.yaml、model-relations.yaml 及环境覆盖）
├── migrations/                      # 数据库迁移脚本（*.up.sql / *.down.sql）
├── docs/                            # 项目文档
│   ├── agents/                      # Agent 专题文档（从 AGENTS.md 拆分）
│   └── swagger/                     # Swagger 生成代码与 OpenAPI 规范（make swagger）
├── scripts/                         # 构建、部署、测试脚本
├── data/                            # 本地运行时数据（如 SQLite 文件）
├── test/                            # 集成测试
├── pkg/                             # 可复用的通用包（不依赖 internal）
│   ├── cache/                       # 通用缓存原语
│   ├── captcha/                     # 验证码抽象（PNG / 可扩展）
│   ├── event/                       # 通用事件总线
│   ├── lock/                        # 分布式锁（Redis / 内存）
│   ├── objectstorage/               # 对象存储（Aliyun/Tencent/Qiniu）
│   └── payment/                     # 支付抽象（Alipay/WeChat）
├── internal/                        # 私有业务代码（外部不可导入）
│   ├── app/                         # 应用组装与生命周期（手动 DI 唯一入口）
│   ├── auth/                        # JWT 认证与令牌管理
│   ├── cache/                       # 业务缓存集中管理（KeySpec + Manager）
│   ├── config/                      # 配置结构体、加载与校验
│   ├── constant/                    # 应用级常量
│   ├── crypto/                      # 密码哈希等加密工具
│   ├── database/                    # 数据库连接与 Bun ORM；migrate / gen
│   ├── dto/                         # 请求/响应数据传输对象
│   ├── errcode/                     # 业务错误码定义
│   ├── event/                       # 业务事件定义与发布
│   ├── handler/                     # 协议层请求处理器
│   │   ├── http/                    # HTTP Handler（REST API）
│   │   ├── tcp/                     # TCP 协议 Handler
│   │   └── udp/                     # UDP 协议 Handler
│   ├── health/                      # 健康检查
│   ├── lock/                        # 业务锁 key 辅助
│   ├── logger/                      # Zap 日志封装
│   ├── metrics/                     # Prometheus 指标采集与暴露
│   ├── middleware/                  # 协议层中间件
│   │   ├── http/                    # HTTP 中间件（认证、限流、安全头等）
│   │   ├── tcp/                     # TCP 中间件
│   │   └── udp/                     # UDP 中间件
│   ├── model/                       # Bun 数据模型（优先 *.gen.go）
│   ├── protocol/                    # 协议编解码（JSON、Binary、自定义）
│   ├── repository/                  # 数据访问层
│   ├── response/                    # 统一 HTTP 响应结构
│   ├── router/                      # Gin 路由注册
│   ├── scheduler/                   # 定时任务（cron + 可选 Redis 选主）
│   ├── server/                      # HTTP / TCP / UDP 服务器实现
│   ├── service/                     # 业务逻辑层
│   ├── tracing/                     # OpenTelemetry 链路追踪
│   ├── types/                       # 跨包共享类型
│   ├── utils/                       # 通用工具（协程等）
│   └── validator/                   # 请求参数校验器
└── build/                           # 编译产物目录（gitignore）
```

## 相关文档

- Agent 文档索引：[agents/README.md](README.md)
- 模块启用/删除：[module-usage.md](../module-usage.md)
- TCP/UDP 扩展：[tcp-udp-guide.md](../tcp-udp-guide.md)
