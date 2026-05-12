package setting

import (
	"fmt"
	"os"
)

var (
	// BEpusdtEnabled 启用 BEpusdt 支付
	BEpusdtEnabled bool = getEnvBool("BEpusdtEnabled", true)
	// BEpusdtAPIURL BEpusdt API 地址
	BEpusdtAPIURL string = getEnv("BEpusdtAPIURL", "https://your-bepusdt-domain.com")
	// BEpusdtAPIToken 对接令牌
	BEpusdtAPIToken string = getEnv("BEpusdtAPIToken", "")
	// BEpusdtPID 商户 ID (固定为 1000)
	BEpusdtPID string = "1000"
	// BEpusdtMinTopUp 最小充值金额（法币单位）
	BEpusdtMinTopUp int = getEnvInt("BEpusdtMinTopUp", 10)
	// BEpusdtExchangeRate 汇率 (CNY per USDT)
	BEpusdtExchangeRate float64 = getEnvFloat("BEpusdtExchangeRate", 7.3)
	// BEpusdtWalletAddress 收款地址 (TRC20 USDT) — 指定地址模式
	BEpusdtWalletAddress string = getEnv("BEpusdtWalletAddress", "")
)

func getEnv(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

func getEnvBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val == "true" || val == "1"
}

func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	var result int
	if _, err := fmt.Sscanf(val, "%d", &result); err != nil {
		return defaultVal
	}
	return result
}

func getEnvFloat(key string, defaultVal float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	var result float64
	if _, err := fmt.Sscanf(val, "%f", &result); err != nil {
		return defaultVal
	}
	return result
}
