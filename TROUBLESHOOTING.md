# 故障排查指南

## 1. 充值接口返回 404

**原因：** 新 API 的路由没注册。

**解决：** 检查 `router/api-router.go` 是否添加了：
```go
selfRoute.POST("/bepusdt/pay", middleware.CriticalRateLimit(), controller.RequestBEpusdt)
```

## 2. 创建订单成功但收银台没有收款地址

**原因：** 创建订单时没有传 `address` 参数（非指定地址模式）。

**解决：** 确认环境变量 `BEpusdtWalletAddress` 已配置，且代码中 `CreateTransactionRequest` 传入了 `Address` 字段。

验证：在 BEpusdt 后台查看订单详情，如果有"收款地址"字段说明已生效。

## 3. 回调验签失败

**日志：** `BEpusdt webhook 验签失败`

**原因：** 参与签名的参数列表与 BEpusdt 平台不一致。

**排查步骤：**

1. 确认 `params` map 包含了 `block_transaction_id`（必须！）
2. 确认 API Token 与 BEpusdt 后台的一致
3. 比对签名算法：
   ```
   sign = MD5(key1=value1&key2=value2&...&keyN=valueN + Token)
   ```
   其中 keys 按字母排序，排除空值和 `signature` 字段

## 4. 回调收到但"订单不存在"

**日志：** `BEpusdt 回调订单不存在`

**原因：** 回调的 `order_id` 在 NewAPI 的数据库中找不到。

**排查：**
1. 确认 `order_id` = 创建订单返回的 `trade_no`
2. 在数据库中查询该订单是否存在
```bash
# SQLite
sqlite3 /app/data/new-api.db "SELECT * FROM top_ups WHERE trade_no='xxx'"
# PostgreSQL
psql -d new-api -c "SELECT * FROM top_ups WHERE trade_no='xxx'"
```

## 5. 用户支付成功但余额未增加

**可能原因：**
1. 回调验签失败 → 看日志确认
2. 金额校验不通过 → 检查实际到账 USDT 是否在 1% 误差内
3. 订单已过期 → BEpusdt 订单状态是"交易过期"，不会触发回调

## 6. 回调签名手动计算

```bash
TOKEN="your_api_token"
# 按字母排序，排除空值
SIGN_STR="actual_amount=1.59&amount=10.735&block_transaction_id=xxx&order_id=xxx&status=1&trade_id=xxx${TOKEN}"
echo -n "$SIGN_STR" | md5sum
# 比对结果是否等于回调中的 signature
```

## 7. 前端充值页面显示"尚未启用在线充值"

**现象：** 后端代码都配好了，充值接口也能调通，但前端充值页面显示"尚未启用在线充值"，看不到 USDT 支付选项。

**原因：** 前端 `recharge-form-card.tsx` 中判断是否显示充值表单的 `hasConfigurableTopup` 变量没有包含 `enable_bepusdt_topup`。

**文件：** `web/default/src/features/wallet/components/recharge-form-card.tsx`

**修复（第127-132行）：**
```tsx
// 修改前
const hasConfigurableTopup =
    topupInfo?.enable_online_topup ||
    topupInfo?.enable_stripe_topup ||
    enableWaffoTopup ||
    enableWaffoPancakeTopup

// 修改后 - 加一行 enable_bepusdt_topup
const hasConfigurableTopup =
    topupInfo?.enable_online_topup ||
    topupInfo?.enable_stripe_topup ||
    topupInfo?.enable_bepusdt_topup ||   // ← 新增
    enableWaffoTopup ||
    enableWaffoPancakeTopup
```

**修复后重新构建前端：**
```bash
cd web/default
bun run build
```

## 8. 充值成功但余额没增加（配额为 0）

**现象：** 回调日志显示"充值成功"，但用户余额不变。

**原因：** `topup_bepusdt.go:104-109` 把充值金额除以 `QuotaPerUnit(500000)`，10/500000=0。

**修复文件：** `controller/topup_bepusdt.go` 第 265 行

**修改前：**
```go
err = model.IncreaseUserQuota(topUp.UserId, int(topUp.Amount), true)
// topUp.Amount = 0 → 加0配额
```

**修改后：**
```go
// 用法币金额 × QuotaPerUnit / 汇率 重新计算
quotaToAdd := int(float64(topUp.Money) / setting.BEpusdtExchangeRate * common.QuotaPerUnit)
err = model.IncreaseUserQuota(topUp.UserId, quotaToAdd, true)
```
