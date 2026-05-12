# NewAPI + BEpusdt 集成架构说明

## 整体架构

```
┌────────────────────────────────────────────────────┐
│                   用户/浏览器                        │
└──────────┬──────────────────────────────┬──────────┘
           │ ① 选择USDT充值              │ ⑧ 打开payment_url
           ▼                              ▼
┌──────────────────────┐    ┌──────────────────────────┐
│    NewAPI 前端        │    │  BEpusdt 收银台页面       │
│  (React / Console)    │    │  (托管在BEpusdt)          │
└──────────┬───────────┘    └──────────────────────────┘
           │ ② POST /api/user/bepusdt/pay
           ▼
┌──────────────────────┐    ┌──────────────────────────┐
│    NewAPI 后端        │    │  BEpusdt API 服务         │
│  (Gin Web Framework) │    │  (你的服务器)              │
│                      │    │                          │
│  controller/         │    │  POST /api/v1/order/     │
│   topup_bepusdt.go   │───→│   create-transaction     │
│                      │    │                          │
│  pkg/bepusdt/        │←───│  返回 payment_url        │
│   client.go          │    │                          │
│                      │    │                          │
│  setting/            │    │  链上确认后回调:          │
│   payment_bepusdt.go │←───│  POST /callback          │
│                      │    └──────────────────────────┘
│  回调处理:            │
│  验签→查订单→加余额   │
└──────────────────────┘
```

## 文件职责

### src/controller/topup_bepusdt.go

| 函数 | 路由 | 职责 |
|------|------|------|
| `RequestBEpusdt` | `POST /api/user/bepusdt/pay` | 创建充值订单，调 BEpusdt API 获取 payment_url |
| `BEpusdtCallback` | `POST /api/payment/bepusdt/callback` | 接收支付结果回调，验签后给用户加额度 |

### src/pkg/bepusdt/bepusdt.go

| 类型/函数 | 职责 |
|-----------|------|
| `Client` | BEpusdt API 客户端 |
| `CreateTransaction` | 调用 BEpusdt 创建交易 API，生成签名 |
| `VerifySignature` | 验证 BEpusdt 回调签名 |
| `generateSignature` | MD5 签名算法（与 BEpusdt 平台一致） |

### src/setting/payment_bepusdt.go

定义所有 BEpusdt 相关配置项，从环境变量读取。

## 数据流

### 充值流程

```
请求: POST /api/user/bepusdt/pay {amount: 10}
  │
  ├─ 1. 解析金额、获取用户分组
  ├─ 2. 计算法币金额（payMoney = quotaToMoney(amount, group)）
  ├─ 3. 计算 USDT 金额（usdtAmount = payMoney / exchangeRate）
  ├─ 4. 调用 BEpusdt CreateTransaction：
  │     POST /api/v1/order/create-transaction
  │     {
  │       order_id: "USDT2NOxxx",
  │       amount: 1.37,
  │       trade_type: "usdt.trc20",
  │       fiat: "CNY",
  │       address: "<YOUR_TRC20_WALLET>"  // 指定地址模式！
  │     }
  ├─ 5. 保存订单到数据库（status=pending）
  └─ 6. 返回 payment_url
```

### 回调流程

```
请求: POST /api/payment/bepusdt/callback
{
  trade_id: "...",
  order_id: "USDT2NOxxx",
  amount: 10.735,
  actual_amount: 1.59,
  token: "",
  block_transaction_id: "...",
  status: 1,
  signature: "md5..."
}
  │
  ├─ 1. 解析请求体到 NotifyRequest
  ├─ 2. 构造 params map（必须含 block_transaction_id!）
  ├─ 3. VerifySignature(params)：计算签名比对
  │    算法: MD5(排序key=value&... + Token)
  ├─ 4. 检查 status == "1"
  ├─ 5. LockOrder(orderID) 幂等锁
  ├─ 6. 查数据库订单（GetTopUpByTradeNo）
  ├─ 7. 校验 PaymentProvider 和 Status
  ├─ 8. 金额校验（1% 误差）
  ├─ 9. 更新订单状态为 success
  └─ 10. IncreaseUserQuota 加余额
```

## 关键设计决策

### 为什么用 topup_bepusdt.go 而不是 payment_bepusdt.go？

| 对比项 | topup_bepusdt.go ✅ (已选) | payment_bepusdt.go ❌ (废弃) |
|--------|---------------------------|----------------------------|
| 依赖 | pkg/bepusdt 客户端包 | 直接 HTTP 调用 |
| 签名验证 | 全参数 MD5（安全） | 仅 3 参数（有漏洞） |
| 配置方式 | setting.BEpusdt* 全局变量 | getBEpusdtConfig() 读 env |
| Address 支持 | ✅ 已验证通过 | ✅ |
| 代码量 | 275 行 | 648 行 |
| 订阅支付 | ❌ | ✅（本仓库不涉及） |

### 为什么不用两套实现？

两套 Controller 是历史遗留问题。本仓库只保留一套经过验证的实现（`topup_bepusdt.go`）。

## 依赖关系

```
topup_bepusdt.go ───┬─── pkg/bepusdt/bepusdt.go (API客户端)
                    ├─── setting/payment_bepusdt.go (配置)
                    ├─── NewAPI model (TopUp, GetTopUpByTradeNo, IncreaseUserQuota)
                    ├─── NewAPI common (QuotaPerUnit, GetRandomString, TopUpStatus*)
                    ├─── NewAPI logger
                    └─── Gin (gin.Context)
```
