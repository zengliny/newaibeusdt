# NewAPI + BEpusdt 支付集成

> 让 NewAPI 支持 USDT (TRC20) 支付的完整集成方案

## 概述

本仓库提供将 **BEpusdt**（个人 USDT 收款网关）集成到 **NewAPI**（AI API 网关）的完整方案。

| 组件 | 说明 |
|------|------|
| NewAPI | AI API 网关，支持用户管理、计费、额度 |
| BEpusdt | 开源的 USDT (TRC20) 自动收款网关 |
| 本集成 | 用户在 NewAPI 选择 USDT 充值 → 跳转 BEpusdt 付款 → 回调自动加额度 |

## 架构

```
用户 → NewAPI 前端选择 USDT 支付
        ↓
   POST /api/user/bepusdt/pay
        ↓
   BEpusdt API (create-transaction, 指定地址模式)
        ↓
   用户扫码转账 USDT → 链上确认
        ↓
   BEpusdt 回调 POST /api/payment/bepusdt/callback
        ↓
   验签 → 金额校验 → 幂等检查 → 加余额
        ↓
   用户看到余额增加 ✅
```

## 前提条件

1. **已部署的 BEpusdt 服务**（参考 [BEpusdt 部署指南](https://github.com/v03413/BEpusdt)）
2. **运行中的 NewAPI 服务**
3. **TRC20 USDT 钱包地址**（用于收款）

## 快速集成（3 步）

### 第 1 步：复制代码

将 `src/` 目录下的文件复制到 NewAPI 项目对应位置：

```bash
# 从本仓库复制到你的 NewAPI 项目
cp src/controller/topup_bepusdt.go    /path/to/new-api/controller/
cp src/setting/payment_bepusdt.go      /path/to/new-api/setting/
cp -r src/pkg/bepusdt                  /path/to/new-api/pkg/
```

### 第 2 步：配置环境变量

在 NewAPI 的运行环境中添加以下变量：

```bash
# 启用 BEpusdt 支付
BEpusdtEnabled=true

# BEpusdt 服务地址
BEpusdtAPIURL=https://bepusdt.ai-key.top

# BEpusdt 对接令牌（从 BEpusdt 后台获取）
BEpusdtAPIToken=your_api_token_here

# 你的 USDT TRC20 收款地址（指定地址模式，钱直接进你钱包）
BEpusdtWalletAddress=TZGm3kFjQpEimu6i632M77KpiMip3J9gws

# 交易类型（默认 usdt.trc20）
BEpusdtTradeType=usdt.trc20

# 法币（默认 CNY）
BEpusdtFiat=CNY

# 最小充值金额（法币单位，默认 10）
BEpusdtMinTopUp=10

# 汇率（1 USDT = 多少法币，默认 7.3）
BEpusdtExchangeRate=7.3
```

### 第 3 步：注册路由 + 修改前端判断

**① 注册后端路由**

在 NewAPI 的路由文件（如 `router/api-router.go`）中添加以下两处：

**公开回调路由**（放在无需认证的公共路由区）：

```go
apiRouter.POST("/payment/bepusdt/callback", controller.BEpusdtCallback)
// 备用路由（可选）：
apiRouter.POST("/bepusdt/webhook", controller.BEpusdtCallback)
```

**用户端充值路由**（放在需要用户认证的路由组中）：

```go
selfRoute.POST("/bepusdt/pay", middleware.CriticalRateLimit(), controller.RequestBEpusdt)
```

**② 修改前端充值页面判断条件**

**文件：** `web/default/src/features/wallet/components/recharge-form-card.tsx`

在 `hasConfigurableTopup` 中添加 `enable_bepusdt_topup`，否则前端充值页面会显示"尚未启用在线充值"：

```tsx
// 修改前（第127行）
const hasConfigurableTopup =
    topupInfo?.enable_online_topup ||
    topupInfo?.enable_stripe_topup ||
    enableWaffoTopup ||
    enableWaffoPancakeTopup

// 修改后 - 加一行
const hasConfigurableTopup =
    topupInfo?.enable_online_topup ||
    topupInfo?.enable_stripe_topup ||
    topupInfo?.enable_bepusdt_topup ||   // ← 新增！否则 USDT 按钮不显示
    enableWaffoTopup ||
    enableWaffoPancakeTopup
```

修改后重新构建前端：
```bash
cd web/default
bun run build
```

## 集成内容清单

| 文件 | 说明 | 行数 |
|------|------|------|
| `src/controller/topup_bepusdt.go` | 充值请求 + 回调处理 Controller | 275 |
| `src/setting/payment_bepusdt.go` | 环境变量配置定义 | 67 |
| `src/pkg/bepusdt/bepusdt.go` | BEpusdt API 客户端（创建交易、验签） | 190 |

## 关键知识点

### 指定地址模式（重要）

BEpusdt 支持两种收款模式：

| 模式 | 说明 | 风险 |
|------|------|------|
| **指定地址模式** ✅ | 创建订单时传 `address` 参数，USDT 直接进你的钱包 | 零信任风险 |
| 非指定地址模式 | USDT 进 BEpusdt 平台钱包，需要手动提现 | BEpusdt 跑路风险 |

**本集成默认使用指定地址模式**（配置 `BEpusdtWalletAddress` 即可）。

### 回调签名验证

BEpusdt 的签名算法（与 BEpusdt 源码 `EpusdtSign` 一致）：

```
sign = MD5(按字母排序 key1=value1&key2=value2&...&keyN=valueN + Token)
```

**关键：** 参与签名的参数必须包含 BEpusdt 回调返回的**所有非空字段**，特别是 `block_transaction_id`。

### 安全防护

| 防护措施 | 实现 |
|---------|------|
| 签名验证 | MD5 全参数签名比对 |
| 金额校验 | 1% 误差容忍，防止汇率波动 |
| 幂等处理 | `LockOrder` 锁 + 订单状态检查 |
| 订单归属验证 | 校验 PaymentProvider 是否匹配 |
| 防重复加余额 | 仅处理 `pending` 状态的订单 |

## 验证测试

### 1. 验证充值接口

```bash
curl -X POST https://your-domain.com/api/user/bepusdt/pay \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <user_token>" \
  -d '{"amount":10}'

# 期望返回：
# {"message":"success","data":{"payment_url":"...","trade_no":"...","usdt_amount":1.37}}
```

### 2. 验证回调签名

```bash
# 进入 test/ 目录运行测试
cd test
go test -v ./...

# 或手动计算签名验证：
TOKEN="your_api_token"
SIGN_STR="actual_amount=10&amount=10&block_transaction_id=xxx&order_id=xxx&status=1&trade_id=xxx${TOKEN}"
echo -n "$SIGN_STR" | md5sum
```

### 3. 真实转账测试

1. 用测试用户登录 NewAPI
2. 选择 USDT TRC20 支付
3. 打开 payment_url，扫码转账少量 USDT
4. 等待链上确认（数秒~数分钟）
5. 检查回调日志：`docker logs new-api | grep bepusdt`
6. 确认用户余额增加

## 故障排查

参见 [TROUBLESHOOTING.md](TROUBLESHOOTING.md) 或 [完整对接文档](https://github.com/zengliny/newaibeusdt/blob/main/docs/ARCHITECTURE.md)

## 参考链接

- [BEpusdt 项目](https://github.com/v03413/BEpusdt)
- [BEpusdt API 文档](https://github.com/v03413/BEpusdt/blob/main/docs/api/api.md)
- [NewAPI 项目](https://github.com/QuantumNous/new-api)
