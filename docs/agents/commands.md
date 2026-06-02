# 常用命令

构建产物路径：`build/app-tpl`（Windows 为 `build/app-tpl.exe`）。以下 CLI 示例使用 Unix 路径。

```bash
# 构建和运行
make build          # 构建应用
make run            # 运行应用
make run-dev        # 使用开发配置运行

# 测试
make test           # 运行测试
make test-coverage  # 运行测试并生成覆盖率报告

# 代码质量
make fmt            # 格式化代码
make vet            # 运行 go vet
make lint           # 运行 linter
```

## 数据库

默认 [`configs/config.yaml`](../../configs/config.yaml) 使用 SQLite；示例迁移 SQL 亦为 **SQLite 语法**。

```bash
./build/app-tpl migrate create <name>   # 创建迁移
./build/app-tpl migrate up              # 执行迁移
./build/app-tpl migrate down            # 回滚迁移
./build/app-tpl migrate status          # 查看迁移状态
./build/app-tpl gen model               # 从数据库生成模型
```
