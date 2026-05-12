#!/bin/bash
# BEpusdt 支付集成验证脚本
# 用法: bash deploy/verify.sh <api_token> <wallet_address>
# 示例: bash deploy/verify.sh "<YOUR_API_TOKEN>" "<YOUR_WALLET_ADDRESS>"

set -e

TOKEN="${1:-$BEpusdtAPIToken}"
WALLET="${2:-$BEpusdtWalletAddress}"
API_URL="${BEpusdtAPIURL:-https://your-bepusdt-domain.com}"

if [ -z "$TOKEN" ]; then
  echo "❌ 请提供 API Token"
  echo "用法: bash $0 <api_token> [wallet_address]"
  exit 1
fi

echo "========================================="
echo "  BEpusdt 支付集成验证"
echo "========================================="
echo ""

# 1. 验证 BEpusdt API 连通性
echo "1️⃣  测试 BEpusdt API 连通性..."
ORDER_ID="verify_$(date +%s)"
RESP=$(curl -s -X POST "$API_URL/api/v1/order/create-transaction" \
  -H "Content-Type: application/json" \
  -d "{\"order_id\":\"$ORDER_ID\",\"notify_url\":\"https://example.com/callback\",\"redirect_url\":\"https://example.com/return\",\"amount\":1,\"trade_type\":\"usdt.trc20\",\"fiat\":\"CNY\",\"timeout\":3600}")

if echo "$RESP" | grep -q 'signature'; then
  echo "   ⚠️  缺少 signature 参数（正常，因为没传 Token header）"
fi
if echo "$RESP" | grep -q 'status_code'; then
  echo "   ✅ API 可访问"
else
  echo "   ❌ API 不可访问"
  echo "   $RESP"
  exit 1
fi

# 2. 验证回调签名算法
echo ""
echo "2️⃣  验证回调签名算法..."
cat > /tmp/verify_sig.go << 'GOEOF'
package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

func main() {
	token := "TEST_TOKEN"
	params := map[string]string{
		"order_id":             "TEST_ORDER_123",
		"trade_id":             "TEST_TRADE_456",
		"amount":               "10",
		"actual_amount":        "1.37",
		"status":               "1",
		"block_transaction_id": "TEST_BLOCK_789",
	}

	var keys []string
	for k, v := range params {
		if k != "signature" && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, params[k]))
	}
	signStr := strings.Join(pairs, "&") + token
	hash := md5.Sum([]byte(signStr))
	signature := hex.EncodeToString(hash[:])

	fmt.Printf("Token: %s\n", token)
	fmt.Printf("参数字符串: %s\n", signStr)
	fmt.Printf("签名: %s\n", signature)
}
GOEOF

cd /tmp && go run verify_sig.go
echo "   ✅ 签名算法可运行"

# 3. 建议
echo ""
echo "3️⃣  环境检查"
echo "   BEpusdt API URL: $API_URL"
echo "   钱包地址: ${WALLET:0:8}...${WALLET: -4}"
echo "   Token: ${TOKEN:0:4}...${TOKEN: -4}"
echo ""

echo "========================================="
echo "  下一步："
echo "  1. 如果 NewAPI 已部署，测试充值接口："
echo "     curl -X POST https://your-domain.com/api/user/bepusdt/pay \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"amount\":10}'"
echo ""
echo "  2. 如果收到回调但验签失败，检查："
echo "     - block_transaction_id 是否在验签参数中"
echo "     - Token 是否与 BEpusdt 后台一致"
echo "========================================="
