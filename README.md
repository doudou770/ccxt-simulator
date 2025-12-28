# CCXT Simulator

基于真实市场价格的加密货币合约交易模拟器。

![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-Apache%202.0-blue)
![Exchanges](https://img.shields.io/badge/Exchanges-5-blue)

## ✨ 功能特性

- 🔄 **实时价格** - WebSocket 连接 5 大交易所获取实时标记价格
- 🔐 **完全兼容** - 与原交易所 API 100% 兼容，只需修改 URL
- 💰 **多交易所** - 支持 Binance、OKX、Bybit、Bitget、Hyperliquid
- 📊 **完整交易** - 开仓、平仓、杠杆、止损止盈
- ⚡ **高性能** - Go 原生实现，延迟 < 50ms
- 🛡️ **签名验证** - 模拟真实交易所签名算法

---

## 📊 支持的交易所

| 交易所 | 兼容 API 路径 | WebSocket | 状态 |
|--------|---------------|-----------|------|
| **Binance** | `/fapi/v1/*`, `/fapi/v2/*` | ✅ 654 交易对 | 🟢 完整支持 |
| **OKX** | `/api/v5/*` | ✅ 270 交易对 | 🟢 完整支持 |
| **Bybit** | `/v5/*` | ✅ 500 交易对 | 🟢 完整支持 |
| **Bitget** | `/api/v2/mix/*` | ✅ | 🟢 完整支持 |
| **Hyperliquid** | `/info`, `/exchange` | ✅ | 🟢 完整支持 |

---

## 🚀 快速开始

### 环境要求

- Go 1.23+
- PostgreSQL 15+
- Redis 7+

### 安装依赖

```bash
# 启动 PostgreSQL
docker run -d --name postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=123456 \
  -e POSTGRES_DB=ccxt_simulator \
  -p 5432:5432 postgres:15

# 启动 Redis
docker run -d --name redis \
  -p 6379:6379 redis:7
```

### 配置文件

编辑 `config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 11188

database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "123456"
  dbname: "ccxt_simulator"

redis:
  host: "localhost"
  port: 6379
  password: "123456"

jwt:
  secret: "your-super-secret-jwt-key"
  expire_hours: 24

encryption:
  aes_key: "ccxt-simulator-32bytes-aes-key!!"  # 必须 32 字节
```

### 运行项目

```bash
# 下载依赖
go mod download

# 运行服务
go run ./cmd/server

# 或编译后运行
go build -o bin/server.exe ./cmd/server
./bin/server.exe
```

---

## 🐳 Docker 部署

### 快速部署 (推荐)

使用 docker-compose 一键部署（包含 PostgreSQL + Redis）:

```bash
# 克隆项目
git clone https://github.com/doudou770/ccxt-simulator.git
cd ccxt-simulator

# 创建环境变量文件
cat > .env << EOF
DB_PASSWORD=your_secure_password
REDIS_PASSWORD=your_redis_password
JWT_SECRET=your-super-secret-jwt-key
AES_KEY=ccxt-simulator-32bytes-aes-key!!
EOF

# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f ccxt-simulator

# 停止服务
docker-compose down
```

### 仅部署应用

如果已有 PostgreSQL 和 Redis，可单独部署应用:

```bash
# 拉取镜像
docker pull ghcr.io/doudou770/ccxt-simulator:latest

# 运行容器
docker run -d \
  --name ccxt-simulator \
  -p 11188:11188 \
  -v /opt/ccxt-simulator/config.yaml:/app/config.yaml:ro \
  -v /opt/ccxt-simulator/logs:/app/logs \
  -e DATABASE_HOST=your_postgres_host \
  -e REDIS_HOST=your_redis_host \
  ghcr.io/doudou770/ccxt-simulator:latest
```

### 本地构建镜像

```bash
# 构建镜像
docker build -t ccxt-simulator:local \
  --build-arg VERSION=v1.0.0 \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  .

# 运行本地镜像
docker run -d -p 11188:11188 ccxt-simulator:local
```

### 健康检查

```bash
# 检查服务状态
curl http://localhost:11188/health
```

响应示例:
```json
{
  "status": "ok",
  "version": "v1.0.0",
  "commit": "abc1234",
  "build_time": "2024-12-28T00:00:00Z",
  "time": 1703721600,
  "exchanges": {
    "binance": true,
    "okx": true,
    "bybit": true,
    "bitget": true,
    "hyperliquid": true
  }
}
```

### GitHub Actions 自动构建

推送代码到 GitHub 后会自动:
1. 运行测试
2. 构建 Docker 镜像
3. 推送到 GitHub Container Registry
4. 自动递增版本号

| 触发条件 | 版本格式 |
|----------|----------|
| Tag 推送 (`v1.0.0`) | `v1.0.0` |
| main 分支推送 | `v1.0.1-abc1234` |
| 手动触发 | 自定义版本 |

拉取最新镜像:
```bash
docker pull ghcr.io/doudou770/ccxt-simulator:latest
```

---

## 📡 API 使用指南

### 1. 管理 API（用户认证）

#### 注册用户
```bash
curl -X POST http://localhost:11188/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "trader", "email": "trader@example.com", "password": "password123"}'
```

#### 登录获取 Token
```bash
curl -X POST http://localhost:11188/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "trader", "password": "password123"}'
```

#### 创建模拟账户
```bash
curl -X POST http://localhost:11188/api/v1/accounts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your_token>" \
  -d '{"exchange_type": "binance", "initial_balance": 10000}'
```

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 1,
    "exchange_type": "binance",
    "api_key": "mkNF2p4zmgBHWmrHs0BOxxxxxxxxxxxx",
    "api_secret": "xxxxxxxxxxxxxxxxxxxxxxxx",
    "balance_usdt": 10000,
    "endpoint_url": "https://sim-binance.yourdomain.com"
  }
}
```

### 2. 交易 API（内部简化版）

#### 开多仓
```bash
curl -X POST http://localhost:11188/api/v1/trading/1/open-long \
  -H "Authorization: Bearer <token>" \
  -d '{"symbol": "BTCUSDT", "quantity": 0.01, "leverage": 10}'
```

#### 平仓
```bash
curl -X POST http://localhost:11188/api/v1/trading/1/close-long \
  -H "Authorization: Bearer <token>" \
  -d '{"symbol": "BTCUSDT"}'
```

#### 查看余额
```bash
curl http://localhost:11188/api/v1/trading/1/balance \
  -H "Authorization: Bearer <token>"
```

### 3. 交易所兼容 API

**无需修改代码，只需替换 URL！**

#### Binance 兼容
```diff
- base_url: https://fapi.binance.com
+ base_url: http://localhost:11188

# 使用创建账户时获得的 API Key
api_key: mkNF2p4zmgBHWmrHs0BOxxxx
api_secret: xxxxxxxxxxxxxxxxxxxx
```

```bash
# 获取余额
curl "http://localhost:11188/fapi/v2/balance?timestamp=1234567890&signature=xxx" \
  -H "X-MBX-APIKEY: <your_api_key>"

# 下单
curl -X POST "http://localhost:11188/fapi/v1/order" \
  -H "X-MBX-APIKEY: <your_api_key>" \
  -d "symbol=BTCUSDT&side=BUY&type=MARKET&quantity=0.01&timestamp=xxx&signature=xxx"
```

#### OKX 兼容
```bash
curl "http://localhost:11188/api/v5/account/balance" \
  -H "OK-ACCESS-KEY: <api_key>" \
  -H "OK-ACCESS-SIGN: <signature>" \
  -H "OK-ACCESS-TIMESTAMP: <timestamp>" \
  -H "OK-ACCESS-PASSPHRASE: <passphrase>"
```

#### Bybit 兼容
```bash
curl "http://localhost:11188/v5/account/wallet-balance?accountType=UNIFIED" \
  -H "X-BAPI-API-KEY: <api_key>" \
  -H "X-BAPI-SIGN: <signature>" \
  -H "X-BAPI-TIMESTAMP: <timestamp>"
```

#### Bitget 兼容
```bash
curl "http://localhost:11188/api/v2/mix/account/account?marginCoin=USDT" \
  -H "ACCESS-KEY: <api_key>" \
  -H "ACCESS-SIGN: <signature>" \
  -H "ACCESS-TIMESTAMP: <timestamp>"
```

#### Hyperliquid 兼容
```bash
# 获取元数据
curl -X POST "http://localhost:11188/info" \
  -H "Content-Type: application/json" \
  -d '{"type": "meta"}'

# 获取所有价格
curl -X POST "http://localhost:11188/info" \
  -d '{"type": "allMids"}'
```

---

## 📁 项目结构

```
ccxt-simulator/
├── cmd/server/              # 应用入口
│   └── main.go
├── internal/
│   ├── config/              # 配置管理
│   ├── models/              # 数据模型 (User, Account, Position, Order, Trade)
│   ├── repository/          # 数据访问层
│   │   ├── user_repo.go
│   │   ├── account_repo.go
│   │   ├── position_repo.go
│   │   ├── order_repo.go
│   │   └── trade_repo.go
│   ├── service/             # 业务逻辑
│   │   ├── auth_service.go
│   │   ├── account_service.go
│   │   ├── price_service.go
│   │   └── trading_service.go
│   ├── handler/             # API 处理器
│   │   ├── auth_handler.go
│   │   ├── account_handler.go
│   │   ├── price_handler.go
│   │   ├── trading_handler.go
│   │   └── exchange/        # 交易所兼容处理器
│   │       ├── binance/
│   │       ├── okx/
│   │       ├── bybit/
│   │       ├── bitget/
│   │       └── hyperliquid/
│   ├── middleware/          # 中间件
│   │   ├── auth.go          # JWT 认证
│   │   └── exchange_auth.go # 交易所签名验证
│   └── exchange/            # WebSocket 客户端
│       ├── interface.go
│       ├── binance/
│       ├── okx/
│       ├── bybit/
│       ├── bitget/
│       └── hyperliquid/
├── pkg/                     # 公共工具包
│   ├── crypto/              # 加密工具
│   ├── keygen/              # API 密钥生成
│   └── response/            # 统一响应格式
├── migrations/              # 数据库迁移
└── docs/                    # 文档
```

---

## 🔧 交易功能

### 支持的订单类型

| 类型 | 说明 |
|------|------|
| Market | 市价单，立即成交 |
| Limit | 限价单 |
| Stop Loss | 止损单 |
| Take Profit | 止盈单 |

### 仓位管理

- ✅ 双向持仓模式 (Hedge Mode)
- ✅ 全仓保证金 (Cross Margin)
- ✅ 逐仓保证金 (Isolated Margin)
- ✅ 杠杆 1-125x
- ✅ 自动爆仓计算

### 手续费

| 交易所 | Taker | Maker |
|--------|-------|-------|
| Binance | 0.04% | 0.02% |
| OKX | 0.05% | 0.02% |
| Bybit | 0.06% | 0.01% |
| Bitget | 0.06% | 0.02% |
| Hyperliquid | 0.035% | 0.01% |

---

## 📊 API 端点汇总

### 管理 API (需要 JWT)
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/register` | 用户注册 |
| POST | `/api/v1/auth/login` | 用户登录 |
| GET | `/api/v1/accounts` | 获取所有账户 |
| POST | `/api/v1/accounts` | 创建账户 |

### 交易 API (需要 JWT)
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/trading/:id/open-long` | 开多仓 |
| POST | `/api/v1/trading/:id/open-short` | 开空仓 |
| POST | `/api/v1/trading/:id/close-long` | 平多仓 |
| POST | `/api/v1/trading/:id/close-short` | 平空仓 |
| GET | `/api/v1/trading/:id/balance` | 查询余额 |
| GET | `/api/v1/trading/:id/positions` | 查询持仓 |
| POST | `/api/v1/trading/:id/leverage` | 设置杠杆 |

### Binance 兼容 API
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/fapi/v1/time` | 服务器时间 |
| GET | `/fapi/v1/exchangeInfo` | 交易所信息 (缓存) |
| GET | `/fapi/v2/account` | 完整账户信息 |
| GET | `/fapi/v2/balance` | 账户余额 |
| GET | `/fapi/v2/positionRisk` | 持仓风险 |
| GET | `/fapi/v2/ticker/price` | 价格行情 |
| POST | `/fapi/v1/order` | 下单 |
| GET | `/fapi/v1/order` | 查询订单 |
| DELETE | `/fapi/v1/order` | 撤单 |
| GET | `/fapi/v1/openOrders` | 获取挂单 |
| DELETE | `/fapi/v1/allOpenOrders` | 撤销所有挂单 |
| POST | `/fapi/v1/leverage` | 设置杠杆 |
| POST | `/fapi/v1/marginType` | 设置保证金模式 |
| POST | `/fapi/v1/algoOrder` | **创建 SL/TP 委托** |
| DELETE | `/fapi/v1/algoOrder` | **取消 SL/TP 委托** |
| GET | `/fapi/v1/openAlgoOrders` | **获取 SL/TP 挂单** |
| DELETE | `/fapi/v1/allOpenAlgoOrders` | **取消所有 SL/TP** |

### OKX 兼容 API
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v5/public/time` | 服务器时间 |
| GET | `/api/v5/public/instruments` | 产品信息 (缓存) |
| GET | `/api/v5/public/mark-price` | 标记价格 |
| GET | `/api/v5/market/tickers` | 所有行情 |
| GET | `/api/v5/account/balance` | 账户余额 |
| GET | `/api/v5/account/positions` | 持仓 |
| POST | `/api/v5/account/set-leverage` | 设置杠杆 |
| POST | `/api/v5/trade/order` | 下单 |
| POST | `/api/v5/trade/cancel-order` | 撤单 |
| POST | `/api/v5/trade/cancel-batch-orders` | 批量撤单 |
| GET | `/api/v5/trade/orders-pending` | 获取挂单 |
| POST | `/api/v5/trade/order-algo` | **创建 SL/TP 委托** |
| POST | `/api/v5/trade/cancel-algos` | **取消 SL/TP 委托** |
| GET | `/api/v5/trade/orders-algo-pending` | **获取 SL/TP 挂单** |

### Bybit 兼容 API
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/v5/market/time` | 服务器时间 |
| GET | `/v5/market/instruments-info` | 产品信息 (缓存) |
| GET | `/v5/market/tickers` | 行情 |
| GET | `/v5/account/wallet-balance` | 钱包余额 |
| GET | `/v5/position/list` | 持仓列表 |
| POST | `/v5/position/set-leverage` | 设置杠杆 |
| POST | `/v5/position/trading-stop` | **设置 SL/TP** |
| POST | `/v5/order/create` | 创建订单 |
| POST | `/v5/order/cancel` | 取消订单 |
| POST | `/v5/order/cancel-all` | 取消所有订单 |
| GET | `/v5/order/realtime` | 获取挂单 |

### Bitget 兼容 API
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v2/public/time` | 服务器时间 |
| GET | `/api/v2/mix/market/contracts` | 合约信息 (缓存) |
| GET | `/api/v2/mix/market/ticker` | 行情 |
| GET | `/api/v2/mix/account/account` | 账户信息 |
| POST | `/api/v2/mix/account/set-leverage` | 设置杠杆 |
| GET | `/api/v2/mix/position/all-position` | 所有持仓 |
| POST | `/api/v2/mix/order/place-order` | 下单 |
| POST | `/api/v2/mix/order/cancel-order` | 撤单 |
| POST | `/api/v2/mix/order/cancel-all-orders` | 撤销所有订单 |
| GET | `/api/v2/mix/order/orders-pending` | 获取挂单 |
| POST | `/api/v2/mix/order/place-plan-order` | **创建 SL/TP 委托** |
| POST | `/api/v2/mix/order/cancel-plan-order` | **取消 SL/TP 委托** |
| GET | `/api/v2/mix/order/orders-plan-pending` | **获取 SL/TP 挂单** |

### Hyperliquid 兼容 API
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/info` | 查询信息 (allMids/meta/clearinghouseState/**openOrders**) |
| POST | `/exchange` | 交易操作 (order/cancel/updateLeverage/**TP/SL trigger**) |

---

## 🔒 安全说明

- API 密钥使用 AES-256 加密存储
- 所有 API 请求需要签名验证
- JWT Token 有效期 24 小时
- 支持 HTTPS（生产环境推荐）

---

## 📈 性能指标

| 指标 | 数值 |
|------|------|
| API 响应延迟 | < 50ms |
| WebSocket 价格延迟 | < 100ms |
| 并发订单处理 | > 1000 TPS |
| 交易对总数 | 1424+ |

---

## License

Apache version 2.0
