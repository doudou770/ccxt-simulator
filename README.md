# CCXT Simulator

基于真实市场价格的加密货币交易模拟器。

## 功能特性

- 🔄 通过 WebSocket 连接交易所获取实时价格
- 🔐 与原交易所 API 完全兼容
- 💰 支持多种交易所：Binance、OKX、Bybit、Bitget、Hyperliquid
- 📊 完整的交易功能：开仓、平仓、止损、止盈

## 快速开始

### 环境要求

- Go 1.21+
- PostgreSQL 15+
- Redis 7+

### 安装依赖

```bash
# 启动 PostgreSQL
docker run -d --name postgres \
  -e POSTGRES_USER=ccxt \
  -e POSTGRES_PASSWORD=ccxt123 \
  -e POSTGRES_DB=ccxt_simulator \
  -p 5432:5432 postgres:15

# 启动 Redis
docker run -d --name redis \
  -p 6379:6379 redis:7
```

### 运行项目

```bash
# 下载依赖
go mod download

# 运行服务
go run ./cmd/server

# 或使用 make
make run
```

### 测试 API

```bash
# 注册用户
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "email": "test@example.com", "password": "password123"}'

# 登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "password123"}'

# 创建模拟账户（需要 Bearer Token）
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your_token>" \
  -d '{"exchange_type": "binance", "initial_balance": 10000}'
```

## 项目结构

```
├── cmd/server/          # 应用入口
├── internal/
│   ├── config/          # 配置管理
│   ├── models/          # 数据模型
│   ├── repository/      # 数据访问层
│   ├── service/         # 业务逻辑
│   ├── handler/         # API 处理器
│   ├── middleware/      # 中间件
│   └── exchange/        # 交易所适配器
├── pkg/                 # 公共工具包
├── migrations/          # 数据库迁移
└── docs/                # 文档
```

## License

MIT
