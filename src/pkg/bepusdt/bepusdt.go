package bepusdt

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
)

// Config BEpusdt 配置
type Config struct {
	APIURL       string // API 地址，如 https://bepusdt.ai-key.top
	APIToken     string // 对接令牌
	NotifyURL    string // 回调通知地址
	RedirectURL  string // 支付成功跳转地址
}

// Client BEpusdt 客户端
type Client struct {
	config Config
}

// NewClient 创建客户端
func NewClient(apiURL, apiToken, notifyURL, redirectURL string) *Client {
	return &Client{
		config: Config{
			APIURL:      strings.TrimRight(apiURL, "/"),
			APIToken:    apiToken,
			NotifyURL:   notifyURL,
			RedirectURL: redirectURL,
		},
	}
}

// CreateTransactionRequest 创建交易请求
type CreateTransactionRequest struct {
	OrderID     string  `json:"order_id"`
	NotifyURL   string  `json:"notify_url"`
	RedirectURL string  `json:"redirect_url"`
	Amount      float64 `json:"amount,omitempty"`
	TradeType   string  `json:"trade_type,omitempty"`
	Fiat        string  `json:"fiat,omitempty"`
	Address     string  `json:"address,omitempty"` // 指定收款地址（重要!）
	Name        string  `json:"name,omitempty"`
	Timeout     int     `json:"timeout,omitempty"`
	Rate        string  `json:"rate,omitempty"`
	Signature   string  `json:"signature"`
}

// CreateTransactionResponse 创建交易响应
type CreateTransactionResponse struct {
	StatusCode int             `json:"status_code"`
	Message    string          `json:"message"`
	Data       TransactionData `json:"data"`
}

// TransactionData 交易数据
type TransactionData struct {
	Fiat           string      `json:"fiat"`
	TradeID        string      `json:"trade_id"`
	OrderID        string      `json:"order_id"`
	Amount         string      `json:"amount"`
	ActualAmount   interface{} `json:"actual_amount"`
	Status         interface{} `json:"status"`
	Token          string      `json:"token"`
	ExpirationTime int         `json:"expiration_time"`
	PaymentURL     string      `json:"payment_url"`
}

// NotifyRequest 回调通知请求
type NotifyRequest struct {
	TradeID            string      `json:"trade_id"`
	OrderID            string      `json:"order_id"`
	Amount             interface{} `json:"amount"`
	ActualAmount       interface{} `json:"actual_amount"`
	Token              string      `json:"token"`
	BlockTransactionID string      `json:"block_transaction_id"`
	Signature          string      `json:"signature"`
	Status             interface{} `json:"status"`
}

// generateSignature 生成签名
// 算法：MD5(所有非空参数按字母排序 key=value&key2=value2&... + token)
// 对应 BEpusdt 源码：utils.EpusdtSign()
func generateSignature(params map[string]string, token string) string {
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
	signatureStr := strings.Join(pairs, "&") + token
	hash := md5.Sum([]byte(signatureStr))
	return hex.EncodeToString(hash[:])
}

// CreateTransaction 创建交易订单
func (c *Client) CreateTransaction(req *CreateTransactionRequest) (*CreateTransactionResponse, error) {
	if req.NotifyURL == "" {
		req.NotifyURL = c.config.NotifyURL
	}
	if req.RedirectURL == "" {
		req.RedirectURL = c.config.RedirectURL
	}
	if req.TradeType == "" {
		req.TradeType = "usdt.trc20"
	}
	if req.Fiat == "" {
		req.Fiat = "CNY"
	}

	params := map[string]string{
		"order_id":     req.OrderID,
		"notify_url":   req.NotifyURL,
		"redirect_url": req.RedirectURL,
	}
	if req.Amount > 0 {
		params["amount"] = fmt.Sprintf("%v", req.Amount)
	}
	if req.TradeType != "" {
		params["trade_type"] = req.TradeType
	}
	if req.Fiat != "" {
		params["fiat"] = req.Fiat
	}
	if req.Address != "" {
		params["address"] = req.Address
	}
	if req.Name != "" {
		params["name"] = req.Name
	}
	if req.Timeout > 0 {
		params["timeout"] = fmt.Sprintf("%d", req.Timeout)
	}
	if req.Rate != "" {
		params["rate"] = req.Rate
	}

	req.Signature = generateSignature(params, c.config.APIToken)

	resp, err := postJSON(c.config.APIURL+"/api/v1/order/create-transaction", req)
	if err != nil {
		return nil, err
	}

	log.Printf("[BEpusdt DEBUG] raw response: %s", string(resp))

	var result CreateTransactionResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if result.StatusCode != 200 {
		return nil, fmt.Errorf("BEpusdt API error: %s", result.Message)
	}

	return &result, nil
}

// VerifySignature 验证回调签名
// 注意：params 必须包含回调的所有非空字段（包括 block_transaction_id）
func (c *Client) VerifySignature(params map[string]string) bool {
	signature := params["signature"]
	if signature == "" {
		return false
	}
	delete(params, "signature")
	calculated := generateSignature(params, c.config.APIToken)
	return strings.EqualFold(signature, calculated)
}

func postJSON(url string, data interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
