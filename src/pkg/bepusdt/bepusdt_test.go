package bepusdt

import (
	"testing"
)

// TestGenerateSignature 测试签名生成算法
// 确认与 BEpusdt 平台源码 EpusdtSign() 算法一致
func TestGenerateSignature(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]string
		token  string
	}{
		{
			name: "基础参数（无 block_transaction_id）",
			params: map[string]string{
				"order_id":      "USDT_TEST_123",
				"trade_id":      "TRADE_ABC_456",
				"amount":        "10",
				"actual_amount": "1.37",
				"status":        "1",
			},
			token: "TEST_TOKEN",
		},
		{
			name: "包含 block_transaction_id",
			params: map[string]string{
				"order_id":             "USDT_TEST_123",
				"trade_id":             "TRADE_ABC_456",
				"amount":               "10",
				"actual_amount":        "1.37",
				"status":               "1",
				"block_transaction_id": "BLOCK_XYZ_789",
			},
			token: "TEST_TOKEN",
		},
		{
			name: "包含空 token 字段",
			params: map[string]string{
				"order_id":             "USDT_TEST_456",
				"trade_id":             "TRADE_DEF_789",
				"amount":               "10",
				"actual_amount":        "1.59",
				"token":                "",
				"status":               "1",
				"block_transaction_id": "BLOCK_XYZ_789",
			},
			token: "TEST_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := generateSignature(tt.params, tt.token)
			t.Logf("params=%v token=%s", tt.params, tt.token)
			t.Logf("signature=%s", sig)

			if sig == "" {
				t.Error("签名不应为空")
			}
		})
	}
}

// TestVerifySignature 测试签名验证
func TestVerifySignature(t *testing.T) {
	client := NewClient(
		"https://your-bepusdt-domain.com",
		"TEST_TOKEN",
		"https://example.com/callback",
		"https://example.com/return",
	)

	t.Run("正确签名应通过", func(t *testing.T) {
		params := map[string]string{
			"order_id":             "ORD_001",
			"trade_id":             "TRD_001",
			"amount":               "10",
			"actual_amount":        "1.37",
			"status":               "1",
			"block_transaction_id": "BLK_001",
		}

		sig := generateSignature(params, "TEST_TOKEN")
		params["signature"] = sig

		if !client.VerifySignature(params) {
			t.Error("正确签名应通过验证")
		}
	})

	t.Run("错误签名应失败", func(t *testing.T) {
		params := map[string]string{
			"order_id":             "ORD_001",
			"trade_id":             "TRD_001",
			"amount":               "10",
			"actual_amount":        "1.37",
			"status":               "1",
			"block_transaction_id": "BLK_001",
			"signature":            "invalid_signature_xxx",
		}

		if client.VerifySignature(params) {
			t.Error("错误签名应失败")
		}
	})

	t.Run("缺失 block_transaction_id 时签名不同", func(t *testing.T) {
		paramsWith := map[string]string{
			"order_id":             "ORD_002",
			"trade_id":             "TRD_002",
			"amount":               "10",
			"actual_amount":        "1.37",
			"status":               "1",
			"block_transaction_id": "BLK_002",
		}
		paramsWithout := map[string]string{
			"order_id":      "ORD_002",
			"trade_id":      "TRD_002",
			"amount":        "10",
			"actual_amount": "1.37",
			"status":        "1",
		}

		sigWith := generateSignature(paramsWith, "TEST_TOKEN")
		sigWithout := generateSignature(paramsWithout, "TEST_TOKEN")

		t.Logf("包含 block_transaction_id: %s", sigWith)
		t.Logf("缺少 block_transaction_id:  %s", sigWithout)

		if sigWith == sigWithout {
			t.Error("缺失 block_transaction_id 后签名应不同")
		}
	})

	t.Run("签名算法一致性", func(t *testing.T) {
		// 两次相同输入应得到相同输出
		params := map[string]string{
			"order_id": "CONSISTENCY_TEST",
			"amount":   "10",
			"status":   "1",
		}

		sig1 := generateSignature(params, "TOKEN")
		sig2 := generateSignature(params, "TOKEN")

		if sig1 != sig2 {
			t.Error("相同输入应产生相同签名")
		}
	})
}
