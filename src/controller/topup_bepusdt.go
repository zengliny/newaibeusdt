package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bepusdt"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// BEpusdtRequest BEpusdt 充值请求
type BEpusdtRequest struct {
	Amount int64 `json:"amount"`
}

// GetBEpusdtClient 获取 BEpusdt 客户端
func GetBEpusdtClient() *bepusdt.Client {
	if !setting.BEpusdtEnabled {
		return nil
	}
	if setting.BEpusdtAPIToken == "" {
		return nil
	}
	return bepusdt.NewClient(
		setting.BEpusdtAPIURL,
		setting.BEpusdtAPIToken,
		setting.BEpusdtAPIURL+"/api/payment/bepusdt/callback",
		setting.BEpusdtAPIURL+"/console/log",
	)
}

// RequestBEpusdt BEpusdt 充值请求
// 路由: POST /api/user/bepusdt/pay (需用户认证)
// 功能: 创建 BEpusdt 订单，返回 payment_url
func RequestBEpusdt(c *gin.Context) {
	var req BEpusdtRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	if !setting.BEpusdtEnabled {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "BEpusdt 支付未启用"})
		return
	}
	if req.Amount < int64(setting.BEpusdtMinTopUp) {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", setting.BEpusdtMinTopUp)})
		return
	}

	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	payMoney := getPayMoney(req.Amount, group)
	if payMoney < 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	client := GetBEpusdtClient()
	if client == nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "BEpusdt 支付未配置"})
		return
	}

	// 生成订单号
	tradeNo := fmt.Sprintf("USDT%dNO%s", id, common.GetRandomString(6)+strconv.FormatInt(time.Now().Unix(), 10))
	usdtAmount := payMoney / setting.BEpusdtExchangeRate

	// 创建 BEpusdt 订单（指定收款地址 = 钱直接进你的钱包）
	resp, err := client.CreateTransaction(&bepusdt.CreateTransactionRequest{
		OrderID:     tradeNo,
		Amount:      usdtAmount,
		TradeType:   "usdt.trc20",
		Fiat:        "CNY",
		Rate:        fmt.Sprintf("%.1f", setting.BEpusdtExchangeRate),
		Timeout:     1200,
		Name:        fmt.Sprintf("AI-KEY充值 %d", req.Amount),
		Address:     setting.BEpusdtWalletAddress, // 关键：指定收款地址
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt 创建订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, tradeNo, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建支付订单失败"})
		return
	}

	// 保存充值记录
	amount := req.Amount
	if common.QuotaPerUnit > 0 {
		dAmount := decimal.NewFromInt(int64(amount))
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		amount = dAmount.Div(dQuotaPerUnit).IntPart()
	}

	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   "bepusdt",
		PaymentProvider: model.PaymentProviderBEpusdt,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := topUp.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt 保存订单失败 user_id=%d trade_no=%s error=%q", id, tradeNo, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "保存订单失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt 订单创建成功 user_id=%d trade_no=%s order_id=%s amount=%.2f USDT payment_url=%s", id, tradeNo, resp.Data.OrderID, usdtAmount, resp.Data.PaymentURL))

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"trade_no":      tradeNo,
			"payment_url":   resp.Data.PaymentURL,
			"usdt_amount":   usdtAmount,
			"cny_amount":    payMoney,
			"trade_id":      resp.Data.TradeID,
			"expiration":    resp.Data.ExpirationTime,
		},
	})
}

// BEpusdtCallback BEpusdt 回调通知
// 路由: POST /api/payment/bepusdt/callback (公开)
// 功能: 接收 BEpusdt 支付结果回调，验签后加余额
func BEpusdtCallback(c *gin.Context) {
	var req bepusdt.NotifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 参数解析失败 client_ip=%s error=%q", c.ClientIP(), err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 收到请求 client_ip=%s body=%q", c.ClientIP(), common.GetJsonString(req)))

	client := GetBEpusdtClient()
	if client == nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt client 未初始化 client_ip=%s", c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	// 构造签名参数 — 必须包含 BEpusdt 返回的所有非空字段！
	params := map[string]string{
		"order_id":             req.OrderID,
		"trade_id":             req.TradeID,
		"amount":               fmt.Sprintf("%v", req.Amount),
		"actual_amount":        fmt.Sprintf("%v", req.ActualAmount),
		"token":                req.Token,
		"status":               fmt.Sprintf("%v", req.Status),
		"block_transaction_id": req.BlockTransactionID, // 关键：必须包含，否则验签失败
		"signature":            req.Signature,
	}

	if !client.VerifySignature(params) {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt webhook 验签失败 order_id=%s trade_id=%s signature=%s client_ip=%s", req.OrderID, req.TradeID, req.Signature, c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	// 检查支付状态
	status, ok := req.Status.(string)
	if !ok {
		if f, ok := req.Status.(float64); ok {
			status = strconv.FormatFloat(f, 'f', 0, 64)
		} else {
			status = "1"
		}
	}

	if status != "1" {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt 订单未支付完成 order_id=%s status=%s", req.OrderID, status))
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	LockOrder(req.OrderID)
	defer UnlockOrder(req.OrderID)

	topUp := model.GetTopUpByTradeNo(req.OrderID)
	if topUp == nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt 回调订单不存在 order_id=%s trade_id=%s client_ip=%s", req.OrderID, req.TradeID, c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	if topUp.PaymentProvider != model.PaymentProviderBEpusdt {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt 订单支付网关不匹配 order_id=%s order_provider=%s client_ip=%s", req.OrderID, topUp.PaymentProvider, c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	if topUp.Status == common.TopUpStatusSuccess {
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt 订单已完成 order_id=%s trade_id=%s client_ip=%s", req.OrderID, req.TradeID, c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	// 金额校验（1% 误差容忍）
	actualAmount, ok := req.ActualAmount.(float64)
	if !ok {
		if s, ok := req.ActualAmount.(string); ok {
			var err error
			actualAmount, err = strconv.ParseFloat(s, 64)
			if err != nil {
				logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt 回调金额解析失败 order_id=%s actual_amount=%v client_ip=%s", req.OrderID, req.ActualAmount, c.ClientIP()))
				c.JSON(http.StatusOK, gin.H{"message": "success"})
				return
			}
		} else {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt 回调金额类型错误 order_id=%s actual_amount=%v client_ip=%s", req.OrderID, req.ActualAmount, c.ClientIP()))
			c.JSON(http.StatusOK, gin.H{"message": "success"})
			return
		}
	}

	expectedUSDT := topUp.Money / setting.BEpusdtExchangeRate
	tolerance := expectedUSDT * 0.01
	if actualAmount < expectedUSDT-tolerance || actualAmount > expectedUSDT+tolerance {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("BEpusdt 回调金额不匹配 order_id=%s expected_usdt=%.4f actual_usdt=%.4f tolerance=%.4f client_ip=%s", req.OrderID, expectedUSDT, actualAmount, tolerance, c.ClientIP()))
		c.JSON(http.StatusOK, gin.H{"message": "success"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt 金额校验通过 order_id=%s expected_usdt=%.4f actual_usdt=%.4f", req.OrderID, expectedUSDT, actualAmount))

	// 计算配额：用法币金额 / 汇率 = USDT金额 x QuotaPerUnit
	// 不能用 topUp.Amount（它已被除以 QuotaPerUnit 可能为0）
	quotaToAdd := int(float64(topUp.Money) / setting.BEpusdtExchangeRate * common.QuotaPerUnit)

	topUp.TradeNo = req.TradeID
	topUp.Status = common.TopUpStatusSuccess
	topUp.CompleteTime = time.Now().Unix()
	if err := topUp.Update(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt 更新订单状态失败 order_id=%s error=%q", req.OrderID, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "更新失败"})
		return
	}

	if err := model.IncreaseUserQuota(topUp.UserId, quotaToAdd, true); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("BEpusdt 发放配额失败 user_id=%d order_id=%s amount=%d error=%q", topUp.UserId, req.OrderID, topUp.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "发放配额失败"})
		return
	}

	logger.LogInfo(c.Request.Context(), fmt.Sprintf("BEpusdt 充值成功 user_id=%d order_id=%s trade_id=%s amount=%d money=%.2f client_ip=%s", topUp.UserId, req.OrderID, req.TradeID, topUp.Amount, topUp.Money, c.ClientIP()))
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}
