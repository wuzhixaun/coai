# 支付模块改造（支付宝/微信扫码，移除 Stripe/PayPal/EPay）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把点数充值的支付方式收敛为支付宝当面付 + 微信 Native 扫码（官方直连），并彻底移除 Stripe、PayPal、易支付(EPay)。

**Architecture:** 先把 `auth/paypal.go` 里混入的全站共享支付主干迁到 `auth/payment_core.go`（零行为变化）；再删除三家 provider；最后接入支付宝/微信 SDK，复用 `insertPaymentOrder`/`completePaymentOrder`/payment 表/quota 表，确认支付用「异步回调 + 轮询主动查单」双重保障；前端把扫码方式改为弹二维码 + 轮询。

**Tech Stack:** Go (gin), `github.com/smartwalle/alipay/v3`, `github.com/wechatpay-apiv3/wechatpay-go`；React/Vite + `qrcode.react`。

## Global Constraints

- 金额换算固定：`amount(元) = form.Quota * 0.1`；`completePaymentOrder` 内 `quota = round(amount*10)`（即 10 点 = ¥1）。**不改这两个公式。**
- quota 范围校验沿用 `1 ~ 99999`。
- 商户密钥只写入 gitignored `config.yaml`，**严禁提交**；`config.example.yaml` 只放占位空值。
- 私钥 / APIv3 key 不得出现在日志或返回前端。
- `service` 列取值：支付宝=`"alipay"`，微信=`"wxpay"`。
- notify 必须验签通过后才入账；入账统一走 `completePaymentOrder`（已幂等：`state=FALSE` 才加点 + 事务）。
- **不动** `auth/payment.go`（Deeptrain + `(u *User).Pay` + `BuyQuota`，订阅依赖它）、订阅流程、兑换码流程。
- 每个 Go 任务以 `go build ./...` 通过为最低门槛；前端任务以 `cd app && npx tsc --noEmit` 通过为门槛。

## 共享函数签名（迁移后保持不变）

```go
type CreatePaymentForm struct { Type string; Quota int; Domain string; Name string; Device string }
func normalizePaymentDomain(c *gin.Context, domain string) string
func insertPaymentOrder(db *sql.DB, user *User, form CreatePaymentForm, orderID string, service string) error
func completePaymentOrder(db *sql.DB, orderID string, userID int64, amount float32) error
func CreatePaymentAPI(c *gin.Context)   // 路由 POST /payment/create
func CheckPaymentAPI(c *gin.Context)    // 路由 GET  /payment/check/:order
```

---

## Phase 1 — 重构：抽出共享支付主干（零行为变化）

### Task 1: 新建 `auth/payment_core.go`，迁入共享主干

**Files:**
- Create: `auth/payment_core.go`
- Modify: `auth/paypal.go`（删除被迁走的函数，仅留 PayPal 专属代码）

**Interfaces:**
- Produces: 上节「共享函数签名」全部符号，迁移后包内可见性与签名不变。

- [ ] **Step 1: 把共享符号剪切到新文件**

把 `auth/paypal.go` 中以下符号**整段剪切**到 `auth/payment_core.go`（package auth，import 按需）：`CreatePaymentForm`、`normalizePaymentDomain`、`insertPaymentOrder`、`completePaymentOrder`、`CreatePaymentAPI`、`CheckPaymentAPI`。

`auth/payment_core.go` 顶部 import 至少包含：
```go
package auth

import (
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strings"

	"chat/channel"
	"chat/globals"
	"chat/utils"

	"github.com/gin-gonic/gin"
)
```

- [ ] **Step 2: `paypal.go` 只留 PayPal 专属代码**

保留：`paypalService` 常量、`paypal*` 类型、`paypalBaseURL`、`paypalApprovalURL`、`paypalHTTP`、`getPayPalAccessToken`、`createPayPalOrder`、`capturePayPalOrder`。其余已迁走。

- [ ] **Step 3: 构建验证（此时三家仍在，行为不变）**

Run: `go build ./...`
Expected: 通过（仅 webp 的 C 告警可忽略）。

- [ ] **Step 4: Commit**

```bash
git add auth/payment_core.go auth/paypal.go
git commit -m "refactor(auth): extract shared payment core out of paypal.go"
```

---

## Phase 2 — 移除 Stripe / PayPal / EPay（编译绿，无可用 provider 的中间态）

### Task 2: 删除三家后端 provider + 修正引用

**Files:**
- Delete: `auth/stripe.go`, `auth/epay.go`, `auth/paypal.go`
- Modify: `auth/router.go`, `channel/system.go`, `auth/payment_core.go`, `config.example.yaml`

**Interfaces:**
- Produces: `CreatePaymentAPI` / `CheckPaymentAPI` 重写为仅识别 `alipay`/`wxpay`（本任务先返回 "unsupported"，Phase 4/5 填实现）。

- [ ] **Step 1: 删除三个 provider 文件**

```bash
git rm auth/stripe.go auth/epay.go auth/paypal.go
```

- [ ] **Step 2: `auth/router.go` 删除三家路由**

删除这三行：
```go
app.Any("/payment/epay/notify", EPayNotifyAPI)
app.Any("/payment/epay/return", EPayReturnAPI)
app.POST("/payment/stripe/webhook", StripeWebhookAPI)
```

- [ ] **Step 3: `channel/system.go` 删除三家 state 并修正 paymentState**

删除类型 `payPalState`、`stripeState`、`ePayState` 及其全部方法（`IsValid`/`Normalize`/`GetMethods`/`GetCurrency`/`Get*Key`/`Accepts` 等）。把 `paymentState` 改为：
```go
type paymentState struct {
	Alipay    alipayState    `json:"alipay" mapstructure:"alipay"`
	WechatPay wechatPayState `json:"wechatpay" mapstructure:"wechatpay"`
}
```
（`alipayState`/`wechatPayState` 在 Task 4 定义；本任务可先加空结构体占位以便编译，Task 4 再补字段。占位：）
```go
type alipayState struct{ Enabled bool `json:"enabled" mapstructure:"enabled"` }
type wechatPayState struct{ Enabled bool `json:"enabled" mapstructure:"enabled"` }
func (s alipayState) IsValid() bool    { return s.Enabled }
func (s wechatPayState) IsValid() bool { return s.Enabled }
func (s *alipayState) Normalize()      {}
func (s *wechatPayState) Normalize()   {}
```

- [ ] **Step 4: `channel/system.go` 修正 `AsInfo()` 支付方式列表**

把 L177-186 的三家判断替换为：
```go
payment := make([]string, 0)
if c.Payment.Alipay.IsValid() {
	payment = append(payment, "alipay")
}
if c.Payment.WechatPay.IsValid() {
	payment = append(payment, "wxpay")
}
```

- [ ] **Step 5: `channel/system.go` 修正 `Normalize()`**

把 L363-365 替换为：
```go
p.Alipay.Normalize()
p.WechatPay.Normalize()
```

- [ ] **Step 6: 重写 `CreatePaymentAPI` 分发（payment_core.go）**

把 quota 校验之后的整段 provider 分发替换为（先骨架，Phase 4/5 填实现）：
```go
paymentType := strings.ToLower(strings.TrimSpace(form.Type))
form.Type = paymentType

switch paymentType {
case alipayService:
	qrcode, orderID, err := createAlipayOrder(c, user, form)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"qrcode": qrcode, "params": gin.H{"order": orderID}}})
case wechatPayService:
	codeURL, orderID, err := createWechatOrder(c, user, form)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "data": gin.H{"qrcode": codeURL, "params": gin.H{"order": orderID}}})
default:
	c.JSON(http.StatusOK, gin.H{"status": false, "error": "unsupported payment provider"})
}
```
> `alipayService`/`wechatPayService` 常量与 `createAlipayOrder`/`createWechatOrder` 函数在 Phase 4/5 定义。本任务为让编译通过，先在 payment_core.go 顶部加临时桩：
```go
const alipayService = "alipay"
const wechatPayService = "wxpay"
func createAlipayOrder(c *gin.Context, user *User, form CreatePaymentForm) (string, string, error) {
	return "", "", fmt.Errorf("alipay not configured")
}
func createWechatOrder(c *gin.Context, user *User, form CreatePaymentForm) (string, string, error) {
	return "", "", fmt.Errorf("wechat pay not configured")
}
```
（Phase 4/5 会把这两个桩移到各自文件并实现；届时删除此处桩。）

- [ ] **Step 7: 重写 `CheckPaymentAPI` 主动查单段（payment_core.go）**

把 `if state { ... return }` 早返回**之后、直到函数结尾**的全部内容（原 `isEPayService` 段、stripe 段、`if service != paypalService` 段、以及结尾的 `capturePayPalOrder` 尾巴）整体替换为下面这个 switch（替换后函数即在此 switch 结束）：
```go
switch service {
case alipayService:
	paid, err := queryAlipayOrder(db, orderID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": false, "error": err.Error(), "order_state": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "order_state": paid})
case wechatPayService:
	paid, err := queryWechatOrder(db, orderID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": false, "error": err.Error(), "order_state": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": true, "order_state": paid})
default:
	c.JSON(http.StatusOK, gin.H{"status": false, "error": "unsupported payment provider", "order_state": false})
}
```
> 临时桩（Phase 4/5 替换）：
```go
func queryAlipayOrder(db *sql.DB, orderID string) (bool, error) { return false, nil }
func queryWechatOrder(db *sql.DB, orderID string) (bool, error) { return false, nil }
```

- [ ] **Step 8: `config.example.yaml` 删除 paypal/stripe/epay 段**

删除 `payment:` 下的 `paypal:`/`stripe:`/`epay:` 三段（Task 4 再加 alipay/wechatpay）。

- [ ] **Step 9: 构建验证**

Run: `go build ./...`
Expected: 通过。若报未定义符号，检查是否有遗漏的三家引用。

- [ ] **Step 10: Commit**

```bash
git add -A
git commit -m "feat(auth): remove Stripe/PayPal/EPay providers, scaffold alipay/wxpay dispatch"
```

### Task 3: 前端移除三家入口与回跳逻辑

**Files:**
- Modify: `app/src/routes/wallet/WalletQuotaBox.tsx`

**Interfaces:**
- Consumes: 后端 `/payment/create` 现仅接受 `alipay`/`wxpay`。

- [ ] **Step 1: 充值方式数组只留两家**

把方式数组（L31-36）改为：
```ts
const paymentMethods = ["alipay", "wxpay"];
```

- [ ] **Step 2: 删除 Stripe/PayPal/EPay 回跳处理 useEffect**

删除处理 `?payment=`/`provider`/`session_id` 跳转返回的整个 useEffect（约 L55-92）及 `canceledProvider`、`payment.notify-stripe/paypal/epay` 相关分支。扫码无跳转，不需要 URL 回跳处理。

- [ ] **Step 3: 类型检查**

Run: `cd app && npx tsc --noEmit`
Expected: 无 `WalletQuotaBox` 报错（QR 弹窗在 Phase 6 加；本步只删，不应引入新引用）。

- [ ] **Step 4: Commit**

```bash
git add app/src/routes/wallet/WalletQuotaBox.tsx
git commit -m "feat(wallet): drop Stripe/PayPal/EPay entries from recharge UI"
```

---

## Phase 3 — 配置与依赖

### Task 4: 加 SDK 依赖 + 完整 alipay/wechatpay 配置结构

**Files:**
- Modify: `channel/system.go`（用完整字段替换 Task 2 的占位结构体）, `config.example.yaml`, `go.mod`, `go.sum`

**Interfaces:**
- Produces: `alipayState{Enabled,AppID,PrivateKey,AlipayPublicKey,IsProd}`、`wechatPayState{Enabled,AppID,MchID,APIv3Key,MchCertSerialNo,MchPrivateKey}`；配置访问 `channel.SystemInstance.Payment.Alipay` / `.WechatPay`。

- [ ] **Step 1: 添加 Go 依赖**

```bash
go get github.com/smartwalle/alipay/v3
go get github.com/wechatpay-apiv3/wechatpay-go
```

- [ ] **Step 2: 用完整字段替换占位结构体（channel/system.go）**

```go
type alipayState struct {
	Enabled         bool   `json:"enabled" mapstructure:"enabled"`
	AppID           string `json:"app_id" mapstructure:"appid"`
	PrivateKey      string `json:"private_key" mapstructure:"privatekey"`
	AlipayPublicKey string `json:"alipay_public_key" mapstructure:"alipaypublickey"`
	IsProd          bool   `json:"is_prod" mapstructure:"isprod"`
}

func (s alipayState) IsValid() bool {
	return s.Enabled && s.AppID != "" && s.PrivateKey != "" && s.AlipayPublicKey != ""
}
func (s *alipayState) Normalize() {}

type wechatPayState struct {
	Enabled         bool   `json:"enabled" mapstructure:"enabled"`
	AppID           string `json:"app_id" mapstructure:"appid"`
	MchID           string `json:"mch_id" mapstructure:"mchid"`
	APIv3Key        string `json:"api_v3_key" mapstructure:"apiv3key"`
	MchCertSerialNo string `json:"mch_cert_serial_no" mapstructure:"mchcertserialno"`
	MchPrivateKey   string `json:"mch_private_key" mapstructure:"mchprivatekey"`
}

func (s wechatPayState) IsValid() bool {
	return s.Enabled && s.AppID != "" && s.MchID != "" && s.APIv3Key != "" &&
		s.MchCertSerialNo != "" && s.MchPrivateKey != ""
}
func (s *wechatPayState) Normalize() {}
```

- [ ] **Step 3: `config.example.yaml` 加占位段**

在 `payment:` 下加：
```yaml
  alipay:
    enabled: false
    appid: ""
    privatekey: ""        # 应用私钥 (RSA2, PKCS8, 无 PEM 头)
    alipaypublickey: ""   # 支付宝公钥
    isprod: false
  wechatpay:
    enabled: false
    appid: ""
    mchid: ""
    apiv3key: ""
    mchcertserialno: ""
    mchprivatekey: ""     # 商户 API 私钥 (apiclient_key.pem 内容)
```

- [ ] **Step 4: 构建验证**

Run: `go build ./...`
Expected: 通过。

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum channel/system.go config.example.yaml
git commit -m "feat(payment): add alipay/wechatpay config structs and SDK deps"
```

---

## Phase 4 — 支付宝当面付

### Task 5: 支付宝金额/订单号纯逻辑 + 单测

**Files:**
- Create: `auth/alipay.go`
- Test: `auth/alipay_test.go`

**Interfaces:**
- Produces: `const alipayService = "alipay"`；`func alipayAmount(quota int) float32`；`func createAlipayOrderID(username string) string`（前缀 `alipay_`，长度 31）。

- [ ] **Step 1: 写失败测试 `auth/alipay_test.go`**

```go
package auth

import (
	"strings"
	"testing"
)

func TestAlipayAmount(t *testing.T) {
	if got := alipayAmount(10); got != 1.0 {
		t.Fatalf("alipayAmount(10) = %v, want 1.0", got)
	}
	if got := alipayAmount(200); got != 20.0 {
		t.Fatalf("alipayAmount(200) = %v, want 20.0", got)
	}
}

func TestCreateAlipayOrderID(t *testing.T) {
	id := createAlipayOrderID("alice")
	if !strings.HasPrefix(id, "alipay_") {
		t.Fatalf("order id %q missing alipay_ prefix", id)
	}
	if len(id) != len("alipay_")+24 {
		t.Fatalf("order id %q unexpected length %d", id, len(id))
	}
	if createAlipayOrderID("alice") == id {
		t.Fatalf("order id should be unique per call")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./auth/ -run 'TestAlipayAmount|TestCreateAlipayOrderID' -v`
Expected: FAIL（未定义 `alipayAmount`/`createAlipayOrderID`）。

- [ ] **Step 3: 写最小实现 `auth/alipay.go`**

```go
package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"chat/channel"
	"chat/globals"
	"chat/utils"

	"github.com/gin-gonic/gin"
	"github.com/smartwalle/alipay/v3"
)

func alipayAmount(quota int) float32 {
	return float32(quota) * 0.1
}

func createAlipayOrderID(username string) string {
	raw := fmt.Sprintf("%s:%d:%s", username, time.Now().UnixNano(), utils.GenerateChar(12))
	return "alipay_" + utils.Sha2Encrypt(raw)[:24]
}
```
> 同时删除 payment_core.go 里 Task 2 的 `alipayService` 常量桩与 `createAlipayOrder` 桩（移到这里）。`alipayService` 常量加到本文件：`const alipayService = "alipay"`。

- [ ] **Step 4: 运行确认通过**

Run: `go test ./auth/ -run 'TestAlipayAmount|TestCreateAlipayOrderID' -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add auth/alipay.go auth/alipay_test.go auth/payment_core.go
git commit -m "feat(alipay): amount + order id helpers with tests"
```

### Task 6: 支付宝建单（TradePreCreate）+ 客户端

**Files:**
- Modify: `auth/alipay.go`

**Interfaces:**
- Consumes: `insertPaymentOrder`, `normalizePaymentDomain`, `channel.SystemInstance.Payment.Alipay`。
- Produces: `func newAlipayClient() (*alipay.Client, error)`；`func createAlipayOrder(c *gin.Context, user *User, form CreatePaymentForm) (qrcode, orderID string, err error)`。

- [ ] **Step 1: 实现客户端构造 + 建单**

```go
func newAlipayClient() (*alipay.Client, error) {
	conf := channel.SystemInstance.Payment.Alipay
	if !conf.IsValid() {
		return nil, fmt.Errorf("支付宝支付未配置")
	}
	client, err := alipay.New(conf.AppID, conf.PrivateKey, conf.IsProd)
	if err != nil {
		return nil, fmt.Errorf("支付宝客户端初始化失败: %w", err)
	}
	if err := client.LoadAliPayPublicKey(conf.AlipayPublicKey); err != nil {
		return nil, fmt.Errorf("加载支付宝公钥失败: %w", err)
	}
	return client, nil
}

func createAlipayOrder(c *gin.Context, user *User, form CreatePaymentForm) (string, string, error) {
	client, err := newAlipayClient()
	if err != nil {
		return "", "", err
	}

	db := utils.GetDBFromContext(c)
	orderID := createAlipayOrderID(user.Username)
	if err := insertPaymentOrder(db, user, form, orderID, alipayService); err != nil {
		return "", "", err
	}

	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = fmt.Sprintf("%d quota", form.Quota)
	}
	notifyURL := normalizePaymentDomain(c, form.Domain) + "/payment/alipay/notify"

	p := alipay.TradePreCreate{}
	p.OutTradeNo = orderID
	p.Subject = name
	p.TotalAmount = fmt.Sprintf("%.2f", alipayAmount(form.Quota))
	p.NotifyURL = notifyURL

	rsp, err := client.TradePreCreate(context.Background(), p)
	if err != nil {
		return "", "", fmt.Errorf("支付宝下单失败: %w", err)
	}
	if rsp.Content.Code != alipay.CodeSuccess {
		return "", "", fmt.Errorf("支付宝下单失败: %s", rsp.Content.SubMsg)
	}
	return rsp.Content.QRCode, orderID, nil
}
```
> **验证点**：`go doc github.com/smartwalle/alipay/v3.TradePreCreate` 确认字段名（`OutTradeNo/Subject/TotalAmount/NotifyURL`）与返回结构（`rsp.Content.QRCode`、`rsp.Content.Code`、`alipay.CodeSuccess`）。版本不同字段可能为 `rsp.QrCode`，以 `go doc` 为准并相应调整。

- [ ] **Step 2: 构建**

Run: `go build ./...`
Expected: 通过。

- [ ] **Step 3: Commit**

```bash
git add auth/alipay.go auth/payment_core.go
git commit -m "feat(alipay): create precreate (QR) order"
```

### Task 7: 支付宝异步回调 `AlipayNotifyAPI` + 路由

**Files:**
- Modify: `auth/alipay.go`, `auth/router.go`

**Interfaces:**
- Consumes: `completePaymentOrder`。
- Produces: `func AlipayNotifyAPI(c *gin.Context)`；路由 `POST /payment/alipay/notify`。

- [ ] **Step 1: 实现回调**

```go
func AlipayNotifyAPI(c *gin.Context) {
	client, err := newAlipayClient()
	if err != nil {
		c.String(200, "failure")
		return
	}
	noti, err := client.DecodeNotification(c.Request) // 内部验签
	if err != nil {
		globals.Warn(fmt.Sprintf("[alipay] notify verify failed: %v", err))
		c.String(200, "failure")
		return
	}
	if noti.TradeStatus != alipay.TradeStatusSuccess && noti.TradeStatus != alipay.TradeStatusFinished {
		c.String(200, "success") // 非成功状态也要回 success 避免支付宝重试风暴
		return
	}

	db := utils.GetDBFromContext(c)
	if err := completeAlipayByOrder(db, noti.OutTradeNo); err != nil {
		globals.Warn(fmt.Sprintf("[alipay] complete order %s failed: %v", noti.OutTradeNo, err))
		c.String(200, "failure")
		return
	}
	c.String(200, "success")
}

// completeAlipayByOrder 按订单号查回 user_id/amount 后入账（幂等由 completePaymentOrder 保证）。
func completeAlipayByOrder(db *sql.DB, orderID string) error {
	var userID int64
	var amount float32
	if err := globals.QueryRowDb(db,
		`SELECT user_id, amount FROM payment WHERE order_id = ? AND service = ?`,
		orderID, alipayService).Scan(&userID, &amount); err != nil {
		return fmt.Errorf("order not found: %w", err)
	}
	return completePaymentOrder(db, orderID, userID, amount)
}
```
> **验证点**：`go doc` 确认 `client.DecodeNotification(*http.Request)` 签名、`noti.TradeStatus`/`noti.OutTradeNo` 字段、`alipay.TradeStatusSuccess`/`TradeStatusFinished` 常量名。

- [ ] **Step 2: 注册路由（auth/router.go）**

在 `/payment/check/:order` 附近加：
```go
app.POST("/payment/alipay/notify", AlipayNotifyAPI)
```

- [ ] **Step 3: 构建**

Run: `go build ./...`
Expected: 通过。

- [ ] **Step 4: Commit**

```bash
git add auth/alipay.go auth/router.go
git commit -m "feat(alipay): async notify handler + route"
```

### Task 8: 支付宝主动查单 `queryAlipayOrder`

**Files:**
- Modify: `auth/alipay.go`, `auth/payment_core.go`（删 Task 2 的 `queryAlipayOrder` 桩）

**Interfaces:**
- Produces: `func queryAlipayOrder(db *sql.DB, orderID string) (bool, error)`（已支付则入账并返回 true）。

- [ ] **Step 1: 实现查单**

```go
func queryAlipayOrder(db *sql.DB, orderID string) (bool, error) {
	client, err := newAlipayClient()
	if err != nil {
		return false, err
	}
	rsp, err := client.TradeQuery(context.Background(), alipay.TradeQuery{OutTradeNo: orderID})
	if err != nil {
		return false, fmt.Errorf("支付宝查单失败: %w", err)
	}
	if rsp.Content.Code != alipay.CodeSuccess {
		return false, nil // 订单未支付/不存在
	}
	if rsp.Content.TradeStatus != alipay.TradeStatusSuccess && rsp.Content.TradeStatus != alipay.TradeStatusFinished {
		return false, nil
	}
	if err := completeAlipayByOrder(db, orderID); err != nil {
		return false, err
	}
	return true, nil
}
```
> **验证点**：`go doc` 确认 `alipay.TradeQuery{OutTradeNo}`、`rsp.Content.TradeStatus`、`rsp.Content.Code`。删除 payment_core.go 中的 `queryAlipayOrder` 桩。

- [ ] **Step 2: 构建 + 跑已有测试**

Run: `go build ./... && go test ./auth/ -run 'Alipay' -v`
Expected: 通过。

- [ ] **Step 3: Commit**

```bash
git add auth/alipay.go auth/payment_core.go
git commit -m "feat(alipay): active order query fallback"
```

---

## Phase 5 — 微信 Native

### Task 9: 微信金额/订单号 + 客户端 + 建单

**Files:**
- Create: `auth/wechatpay.go`
- Test: `auth/wechatpay_test.go`
- Modify: `auth/payment_core.go`（删 `wechatPayService`/`createWechatOrder` 桩）

**Interfaces:**
- Produces: `const wechatPayService = "wxpay"`；`func wechatAmountFen(quota int) int64`；`func createWechatOrderID(username string) string`（前缀 `wxpay_`）；`func newWechatClient(ctx) (*core.Client, error)`；`func createWechatOrder(c, user, form) (codeURL, orderID string, err error)`。

- [ ] **Step 1: 写失败测试 `auth/wechatpay_test.go`**

```go
package auth

import (
	"strings"
	"testing"
)

func TestWechatAmountFen(t *testing.T) {
	if got := wechatAmountFen(10); got != 100 {
		t.Fatalf("wechatAmountFen(10) = %d, want 100", got)
	}
	if got := wechatAmountFen(200); got != 2000 {
		t.Fatalf("wechatAmountFen(200) = %d, want 2000", got)
	}
}

func TestCreateWechatOrderID(t *testing.T) {
	id := createWechatOrderID("bob")
	if !strings.HasPrefix(id, "wxpay_") {
		t.Fatalf("order id %q missing wxpay_ prefix", id)
	}
	if createWechatOrderID("bob") == id {
		t.Fatalf("order id should be unique")
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./auth/ -run 'Wechat' -v`
Expected: FAIL（未定义）。

- [ ] **Step 3: 实现 `auth/wechatpay.go`（含客户端 + 建单）**

```go
package auth

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"chat/channel"
	"chat/globals"
	"chat/utils"

	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	wxutils "github.com/wechatpay-apiv3/wechatpay-go/utils"
)

const wechatPayService = "wxpay"

func wechatAmountFen(quota int) int64 {
	return int64(quota) * 10 // quota*0.1 元 = quota*10 分
}

func createWechatOrderID(username string) string {
	raw := fmt.Sprintf("%s:%d:%s", username, time.Now().UnixNano(), utils.GenerateChar(12))
	return "wxpay_" + utils.Sha2Encrypt(raw)[:24]
}

func newWechatClient(ctx context.Context) (*core.Client, error) {
	conf := channel.SystemInstance.Payment.WechatPay
	if !conf.IsValid() {
		return nil, fmt.Errorf("微信支付未配置")
	}
	priv, err := wxutils.LoadPrivateKey(conf.MchPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("加载微信商户私钥失败: %w", err)
	}
	opts := []core.ClientOption{
		option.WithWechatPayAutoAuthCipher(conf.MchID, conf.MchCertSerialNo, priv, conf.APIv3Key),
	}
	return core.NewClient(ctx, opts...)
}

func createWechatOrder(c *gin.Context, user *User, form CreatePaymentForm) (string, string, error) {
	conf := channel.SystemInstance.Payment.WechatPay
	ctx := context.Background()
	client, err := newWechatClient(ctx)
	if err != nil {
		return "", "", err
	}

	db := utils.GetDBFromContext(c)
	orderID := createWechatOrderID(user.Username)
	if err := insertPaymentOrder(db, user, form, orderID, wechatPayService); err != nil {
		return "", "", err
	}

	name := strings.TrimSpace(form.Name)
	if name == "" {
		name = fmt.Sprintf("%d quota", form.Quota)
	}
	notifyURL := normalizePaymentDomain(c, form.Domain) + "/payment/wechat/notify"
	total := wechatAmountFen(form.Quota)

	svc := native.NativeApiService{Client: client}
	resp, _, err := svc.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(conf.AppID),
		Mchid:       core.String(conf.MchID),
		Description: core.String(name),
		OutTradeNo:  core.String(orderID),
		NotifyUrl:   core.String(notifyURL),
		Amount:      &native.Amount{Total: core.Int64(total), Currency: core.String("CNY")},
	})
	if err != nil {
		return "", "", fmt.Errorf("微信下单失败: %w", err)
	}
	if resp.CodeUrl == nil {
		return "", "", fmt.Errorf("微信下单未返回二维码")
	}
	return *resp.CodeUrl, orderID, nil
}
```
> **验证点**：`go doc` 确认 `option.WithWechatPayAutoAuthCipher` 参数顺序、`native.PrepayRequest` 字段、`native.Amount`、`resp.CodeUrl`。删除 payment_core.go 中 `wechatPayService` 常量桩与 `createWechatOrder` 桩。

- [ ] **Step 4: 运行测试 + 构建**

Run: `go build ./... && go test ./auth/ -run 'Wechat' -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add auth/wechatpay.go auth/wechatpay_test.go auth/payment_core.go
git commit -m "feat(wechatpay): client + native prepay (QR) with tests"
```

### Task 10: 微信异步回调 `WechatNotifyAPI` + 路由

**Files:**
- Modify: `auth/wechatpay.go`, `auth/router.go`

**Interfaces:**
- Produces: `func WechatNotifyAPI(c *gin.Context)`；`func completeWechatByOrder(db, orderID) error`；路由 `POST /payment/wechat/notify`。

- [ ] **Step 1: 实现回调**

```go
func completeWechatByOrder(db *sql.DB, orderID string) error {
	var userID int64
	var amount float32
	if err := globals.QueryRowDb(db,
		`SELECT user_id, amount FROM payment WHERE order_id = ? AND service = ?`,
		orderID, wechatPayService).Scan(&userID, &amount); err != nil {
		return fmt.Errorf("order not found: %w", err)
	}
	return completePaymentOrder(db, orderID, userID, amount)
}

func WechatNotifyAPI(c *gin.Context) {
	conf := channel.SystemInstance.Payment.WechatPay
	ctx := context.Background()
	client, err := newWechatClient(ctx)
	if err != nil {
		c.JSON(500, gin.H{"code": "FAIL", "message": "not configured"})
		return
	}

	// 用客户端的证书访问器构造通知验签 handler
	handler, err := newWechatNotifyHandler(ctx, client, conf.APIv3Key)
	if err != nil {
		c.JSON(500, gin.H{"code": "FAIL", "message": "init handler failed"})
		return
	}

	transaction := make(map[string]interface{})
	_, err = handler.ParseNotifyRequest(ctx, c.Request, &transaction)
	if err != nil {
		globals.Warn(fmt.Sprintf("[wechat] notify verify failed: %v", err))
		c.JSON(401, gin.H{"code": "FAIL", "message": "verify failed"})
		return
	}

	if state, _ := transaction["trade_state"].(string); state != "SUCCESS" {
		c.JSON(200, gin.H{"code": "SUCCESS", "message": "OK"})
		return
	}
	orderID, _ := transaction["out_trade_no"].(string)
	db := utils.GetDBFromContext(c)
	if err := completeWechatByOrder(db, orderID); err != nil {
		globals.Warn(fmt.Sprintf("[wechat] complete order %s failed: %v", orderID, err))
		c.JSON(500, gin.H{"code": "FAIL", "message": "process failed"})
		return
	}
	c.JSON(200, gin.H{"code": "SUCCESS", "message": "OK"})
}
```

加 handler 构造辅助（用自动下载的平台证书做验签）：
```go
import (
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
)

func newWechatNotifyHandler(ctx context.Context, client *core.Client, apiV3Key string) (*notify.Handler, error) {
	conf := channel.SystemInstance.Payment.WechatPay
	mgr := downloader.MgrInstance()
	if err := mgr.RegisterDownloaderWithClient(ctx, client, conf.MchID, apiV3Key); err != nil {
		return nil, err
	}
	certVisitor := mgr.GetCertificateVisitor(conf.MchID)
	return notify.NewNotifyHandler(apiV3Key, verifiers.NewSHA256WithRSAVerifier(certVisitor)), nil
}
```
> **验证点**：这是本计划最易随 SDK 版本漂移处。用 `go doc github.com/wechatpay-apiv3/wechatpay-go/core/notify` 与官方 README 校准 `ParseNotifyRequest`、`downloader.MgrInstance`、`RegisterDownloaderWithClient`、`GetCertificateVisitor`、`verifiers.NewSHA256WithRSAVerifier` 的确切签名；若 API 变化按 README 调整。回调成功必须回 HTTP 200 + `{"code":"SUCCESS"}`。

- [ ] **Step 2: 注册路由**

```go
app.POST("/payment/wechat/notify", WechatNotifyAPI)
```

- [ ] **Step 3: 构建**

Run: `go build ./...`
Expected: 通过。

- [ ] **Step 4: Commit**

```bash
git add auth/wechatpay.go auth/router.go
git commit -m "feat(wechatpay): async notify handler + route"
```

### Task 11: 微信主动查单 `queryWechatOrder`

**Files:**
- Modify: `auth/wechatpay.go`, `auth/payment_core.go`（删桩）

**Interfaces:**
- Produces: `func queryWechatOrder(db *sql.DB, orderID string) (bool, error)`。

- [ ] **Step 1: 实现查单**

```go
func queryWechatOrder(db *sql.DB, orderID string) (bool, error) {
	conf := channel.SystemInstance.Payment.WechatPay
	ctx := context.Background()
	client, err := newWechatClient(ctx)
	if err != nil {
		return false, err
	}
	svc := native.NativeApiService{Client: client}
	resp, _, err := svc.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(orderID),
		Mchid:      core.String(conf.MchID),
	})
	if err != nil {
		return false, fmt.Errorf("微信查单失败: %w", err)
	}
	if resp.TradeState == nil || *resp.TradeState != "SUCCESS" {
		return false, nil
	}
	if err := completeWechatByOrder(db, orderID); err != nil {
		return false, err
	}
	return true, nil
}
```
> **验证点**：`go doc` 确认 `QueryOrderByOutTradeNoRequest` 字段与 `resp.TradeState`。删除 payment_core.go 的 `queryWechatOrder` 桩。

- [ ] **Step 2: 构建 + 测试**

Run: `go build ./... && go test ./auth/ -v`
Expected: 通过。

- [ ] **Step 3: Commit**

```bash
git add auth/wechatpay.go auth/payment_core.go
git commit -m "feat(wechatpay): active order query fallback"
```

---

## Phase 6 — 前端二维码扫码

### Task 12: `request.ts` 增加 qrcode 字段 + i18n 文案

**Files:**
- Modify: `app/src/payment/request.ts`
- Modify: `app/src/resources/i18n/cn.json`（及其余语言可选）

**Interfaces:**
- Produces: `PaymentResponse.data.qrcode?: string`；i18n key `payment.qr-title-alipay`/`payment.qr-title-wxpay`/`payment.qr-tip`/`payment.qr-success`。

- [ ] **Step 1: 扩展类型**

把 `PaymentResponse.data` 改为：
```ts
data?: {
  url?: string;
  qrcode?: string;
  params: Record<string, string>;
};
```

- [ ] **Step 2: 加 i18n（cn.json 的 `payment` 段）**

```json
"qr-title-alipay": "支付宝扫码支付",
"qr-title-wxpay": "微信扫码支付",
"qr-tip": "请使用手机扫描二维码完成支付",
"qr-success": "支付成功，点数已到账"
```

- [ ] **Step 3: 类型检查**

Run: `cd app && npx tsc --noEmit`
Expected: 通过。

- [ ] **Step 4: Commit**

```bash
git add app/src/payment/request.ts app/src/resources/i18n/cn.json
git commit -m "feat(wallet): add qrcode field + i18n for scan-to-pay"
```

### Task 13: 二维码弹窗 + 轮询（WalletQuotaBox）

**Files:**
- Modify: `app/src/routes/wallet/WalletQuotaBox.tsx`

**Interfaces:**
- Consumes: `createPaymentOrder`、`getPaymentOrderStatus`、`PaymentResponse.data.qrcode`、`qrcode.react`。

- [ ] **Step 1: 引入依赖与状态**

文件顶部加：
```tsx
import { QRCodeSVG } from "qrcode.react";
```
组件内加状态：
```tsx
const [qrOpen, setQrOpen] = useState(false);
const [qrValue, setQrValue] = useState("");
const [qrOrder, setQrOrder] = useState("");
const [qrMethod, setQrMethod] = useState("alipay");
```

- [ ] **Step 2: 改 `doPayment` 走二维码**

```tsx
const doPayment = async (method: string) => {
  if (buyQuota <= 0) {
    toast.error(t("buy.failed"), { description: t("buy.buy-description") });
    return;
  }
  setPaying(true);
  const res = await createPaymentOrder(method, buyQuota, t("payment.order.quota", { quota: buyQuota }));
  setPaying(false);
  if (res.status && res.data?.qrcode) {
    setQrMethod(method);
    setQrValue(res.data.qrcode);
    setQrOrder(res.data.params?.order ?? "");
    setQrOpen(true);
    return;
  }
  toast.error(t("buy.failed"), { description: res.error || t("buy.failed-prompt") });
};
```

- [ ] **Step 3: 轮询 effect（弹窗打开期间每 2s 查单）**

```tsx
useEffect(() => {
  if (!qrOpen || !qrOrder) return;
  const timer = setInterval(async () => {
    const res = await getPaymentOrderStatus(qrOrder);
    if (res.status && res.order_state) {
      clearInterval(timer);
      setQrOpen(false);
      toast.success(t("payment.qr-success"));
      // 刷新点数余额：沿用本页已有的余额刷新逻辑（如 refreshQuota / dispatch）
      await refreshQuota?.();
    }
  }, 2000);
  return () => clearInterval(timer);
}, [qrOpen, qrOrder]);
```
> **验证点**：本页已有余额刷新方式（查找现有 `quota` 刷新调用，可能是 props 回调或 redux dispatch），把上面 `refreshQuota?.()` 换成该实际调用。

- [ ] **Step 4: 渲染二维码弹窗**

在组件返回的 JSX 末尾（兑换码 Dialog 旁）加：
```tsx
<Dialog open={qrOpen} onOpenChange={setQrOpen}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>{t(qrMethod === "wxpay" ? "payment.qr-title-wxpay" : "payment.qr-title-alipay")}</DialogTitle>
      <DialogDescription>{t("payment.qr-tip")}</DialogDescription>
    </DialogHeader>
    <div className="flex justify-center py-4">
      {qrValue && <QRCodeSVG value={qrValue} size={220} />}
    </div>
  </DialogContent>
</Dialog>
```

- [ ] **Step 5: 类型检查 + 构建**

Run: `cd app && npx tsc --noEmit`
Expected: 通过。

- [ ] **Step 6: Commit**

```bash
git add app/src/routes/wallet/WalletQuotaBox.tsx
git commit -m "feat(wallet): QR-code scan-to-pay dialog with polling"
```

---

## Phase 7 — 整体验证

### Task 14: 全量构建 + 回归 + 联调清单

**Files:** 无（验证）

- [ ] **Step 1: 后端全量构建 + 测试**

Run: `go build ./... && go test ./auth/ -v`
Expected: 通过；无残留 Stripe/PayPal/EPay 符号。

- [ ] **Step 2: 残留引用扫描**

Run: `grep -rn --include="*.go" "Stripe\|EPay\|PayPal\|paypalService\|stripeService" . | grep -v _test`
Expected: 仅 `auth/payment.go` 里与三家无关的内容（无 provider 残留）；理想为空。

- [ ] **Step 3: 前端构建**

Run: `cd app && npx tsc --noEmit && npx vite build`
Expected: 通过。

- [ ] **Step 4: 启动后端 + 健康检查**

Run: `go build -o chat . && (nohup ./chat > logs/chatnio.log 2>&1 &) && sleep 4 && curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8094/`
Expected: `200`。

- [ ] **Step 5: 手动联调清单（需真实/沙箱商户密钥，写入 gitignored config.yaml）**

  1. 钱包页应只显示「支付宝 / 微信」两个充值入口。
  2. 选金额 → 点支付宝 → 弹出二维码 → 手机扫码付款 → 弹窗自动关闭、toast 成功、点数增加对应数额。
  3. 微信同上。
  4. 本地无公网回调时，验证「轮询主动查单」也能在付款后入账（这是设计的兜底路径）。
  5. 重复付款/重复回调不重复加点（幂等）。

- [ ] **Step 6: 最终提交（如有验证期微调）**

```bash
git add -A
git commit -m "test(payment): full build + regression verification for alipay/wechat"
```
