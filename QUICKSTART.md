# 5 分钟快速部署

## 如果 BEpusdt 和 NewAPI 都已经在运行

```bash
# 1. 复制源码到 NewAPI 项目
cp src/controller/topup_bepusdt.go /path/to/new-api/controller/
cp src/setting/payment_bepusdt.go /path/to/new-api/setting/
cp -r src/pkg/bepusdt /path/to/new-api/pkg/

# 2. 重启 NewAPI（Docker 方式）
docker restart new-api

# 3. 测试充值接口
curl -X POST https://your-domain.com/api/user/bepusdt/pay \
  -H "Content-Type: application/json" \
  -d '{"amount":10}'
# 应该返回 payment_url
```

## 如果从零开始（全新部署）

```bash
# 1. 启动 BEpusdt
docker run -d --name bepusdt -p 8080:8080 \
  -e AUTH_TOKEN=your_token \
  -e APP_URI=https://bepusdt.your-domain.com \
  v03413/bepusdt:latest

# 2. 登录 BEpusdt 后台配置钱包
# 访问 https://bepusdt.your-domain.com/ 配好 TRC20 地址

# 3. 把本仓库 src/ 文件复制到 NewAPI
# 同上

# 4. 重启 NewAPI，搞定
```
