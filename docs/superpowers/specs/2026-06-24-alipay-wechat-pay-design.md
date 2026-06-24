# 设计：支付模块改造为支付宝 / 微信官方扫码支付（移除 Stripe / PayPal / 易支付）

- 日期：2026-06-24
- 分支：`feat/jimeng-official-api`
- 状态：已通过设计评审（含范围调整：新增支付宝/微信，删除 Stripe/PayPal/EPay），待写实现计划

## 1. 目标与范围

把「钱包 / 点数充值」的支付方式收敛为**仅支付宝 + 微信**两种官方直连扫码支付，**移除** Stripe、PayPal、易支付(EPay)。

- **新增**：官方商户直连，PC 扫码 —— 支付宝当面付（`TradePreCreate`）+ 微信 Native（`Prepay`），返回二维码，手机扫码付款。
- **移除**：Stripe、PayPal、EPay 的全部支付逻辑、路由、配置、前端入口。
- **范围**：**仅点数充值**（10/50/100/200 及自定义 quota）。订阅仍用点数余额扣费，**不改订阅购买流程**。

### 不在范围内（YAGNI）

- 不做手机 H5/WAP 跳转支付（用户主要在 PC）。
- 不做退款、对账报表、分账。
- 不改订阅（基础/标准/专业版）购买链路。
- **不动 `auth/payment.go`**（Deeptrain 余额 + `(u *User).Pay` + `BuyQuota`）—— 它与被删的三家无关，且订阅购买依赖 `user.Pay`，必须保留。
- 不动兑换码（redeem code）流程。

## 2. 现有架构与关键风险

> **关键风险**：`auth/paypal.go` 命名误导 —— 它同时包含 **PayPal 专属代码** 和 **全站共享的支付主干**。删 PayPal ≠ 删此文件。

`auth/paypal.go` 内容分类：

| 归属 | 符号 | 处置 |
|---|---|---|
| 共享主干（保留） | `CreatePaymentForm`(类型)、`normalizePaymentDomain`、`insertPaymentOrder`、`CreatePaymentAPI`、`completePaymentOrder`、`CheckPaymentAPI` | **迁出**到新文件 `auth/payment_core.go` |
| PayPal 专属（删除） | `paypalService`、`paypal*` 类型、`paypalBaseURL`、`paypalApprovalURL`、`paypalHTTP`、`getPayPalAccessToken`、`createPayPalOrder`、`capturePayPalOrder` | 随 `auth/paypal.go` 一并删除 |

复用点（迁移后不变）：

| 能力 | 位置 | 复用方式 |
|---|---|---|
| 写订单 | `insertPaymentOrder()`（迁入 payment_core.go） | 直接复用 |
| 完成订单 + 点数入账（幂等） | `completePaymentOrder()`（迁入 payment_core.go） | 直接复用（`state=FALSE` 幂等 + 事务 + `IncreaseQuota`） |
| 点数入账 | `auth/quota.go` `IncreaseQuota()` | 间接复用 |
| 订单表 / 点数表 | `connection/database.go` `payment` / `quota` | 无需改表（`service` 列存 `alipay`/`wxpay`） |
| 金额换算 | `quota * 0.1` = 人民币元（10点=¥1） | 沿用；元→分在 SDK 调用处转换 |
| 二维码渲染 | `qrcode.react`（已安装） | 直接用 |

## 3. 移除计划（Stripe / PayPal / EPay）

### 3.1 删除文件
- `auth/stripe.go`（删除）
- `auth/epay.go`（删除）
- `auth/paypal.go`（**先迁出共享主干到 `auth/payment_core.go`，再删除**）

### 3.2 路由（`auth/router.go`）删除
```
app.Any("/payment/epay/notify",  EPayNotifyAPI)   // 删
app.Any("/payment/epay/return",  EPayReturnAPI)    // 删
app.POST("/payment/stripe/webhook", StripeWebhookAPI) // 删
```

### 3.3 配置（`channel/system.go`）
- 删除类型 `payPalState`、`stripeState`、`ePayState` 及其 `IsValid()`/`Normalize()`/`GetMethods()`/`Get*Key()` 方法。
- `paymentState` 仅保留 `Alipay alipayState`、`WechatPay wechatPayState`。
- `AsInfo()`（L177-186）支付方式列表改为：`Alipay.IsValid()→"alipay"`、`WechatPay.IsValid()→"wxpay"`。
- `Normalize()`（L363-365）改为调用 Alipay/WechatPay 的 Normalize。
- `config.example.yaml`：删除 paypal/stripe/epay 段，新增 alipay/wechatpay 段。

### 3.4 前端移除（`WalletQuotaBox.tsx`、`payment/*`）
- 充值方式数组（L31-36）仅保留 `["alipay","wxpay"]`。
- 删除 `?payment=`/`provider`/`session_id` 跳转回跳处理的 useEffect（L55-92）及 stripe/paypal/epay 相关分支（扫码无跳转，不需要）。
- `payment/icons.tsx`：可保留图标定义（无害）；如清理则仅留 alipay/wxpay。
- `payment/utils.ts`：移除 epay 专用 `return_url` 逻辑（如不再被引用）。

> 不被引用的 i18n 文案（`payment.notify-stripe` 等）可保留或清理，不阻塞。

## 4. 确认支付的方式（核心设计）：双重确认

1. **异步回调（生产主路径）**：支付成功后 POST 到 `notify_url` → 验签 → `completePaymentOrder()`。
2. **轮询时主动查单（兜底 + 本地可测）**：前端每 2s 轮询 `/payment/check/:order`；后端发现订单 `service ∈ {alipay,wxpay}` 且仍 `pending` 时，调用平台查单接口，已支付则 `completePaymentOrder()` 后返回 `order_state:true`。

> 主动查单使本地开发（localhost 收不到回调）也能跑通，回调丢失也不漏单；`completePaymentOrder` 幂等保证两条路径不重复入账。

## 5. 后端实现（新增）

### 5.1 配置（`channel/system.go` + gitignored `config.yaml`）

```go
type alipayState struct {
    Enabled         bool   // enabled
    AppID           string // appid
    PrivateKey      string // privatekey         应用私钥 RSA2
    AlipayPublicKey string // alipaypublickey     支付宝公钥（验签）
    IsProd          bool   // isprod              true=正式网关
}

type wechatPayState struct {
    Enabled         bool   // enabled
    AppID           string // appid
    MchID           string // mchid               商户号
    APIv3Key        string // apiv3key            APIv3 密钥
    MchCertSerialNo string // mchcertserialno     商户证书序列号
    MchPrivateKey   string // mchprivatekey       商户 API 私钥（apiclient_key.pem 内容）
}
```
`paymentState{ Alipay, WechatPay }`。`notify_url` 由请求 `domain` 拼成 `{domain}/payment/alipay/notify`、`{domain}/payment/wechat/notify`。

### 5.2 SDK 选型
- 支付宝：`github.com/smartwalle/alipay/v3`（支持 `TradePreCreate` 与回调验签）。
- 微信：`github.com/wechatpay-apiv3/wechatpay-go`（官方 SDK，Native `Prepay` + 回调解密验签 + 证书自动轮换）。

### 5.3 新文件

**`auth/payment_core.go`**（从 paypal.go 迁入的共享主干）
- `CreatePaymentForm`、`normalizePaymentDomain`、`insertPaymentOrder`、`completePaymentOrder`、`CreatePaymentAPI`、`CheckPaymentAPI`。
- `CreatePaymentAPI` 重写为只分发 `alipay`/`wxpay`，返回 `{data:{qrcode, params:{order}}}`，其余 `unsupported payment provider`。
- `CheckPaymentAPI` 重写为对 pending 的 alipay/wxpay 主动查单，其余仅读 DB state。

**`auth/alipay.go`**
- `const alipayService = "alipay"`
- `createAlipayOrder(c, user, form) (qrcode, orderID string, err error)`：生成订单号 → `insertPaymentOrder(...,"alipay")` → `TradePreCreate`(out_trade_no=orderID, total_amount=元, subject=form.Name, notify_url) → 返回 `QrCode`。
- `AlipayNotifyAPI(c)`：`GetTradeNotification` 验签 → `trade_status ∈ {TRADE_SUCCESS,TRADE_FINISHED}` → `completePaymentOrder()` → 回 `"success"`。
- `queryAlipayOrder(db, orderID) (paid bool, err error)`：`TradeQuery` → 已支付则入账。

**`auth/wechatpay.go`**
- `const wechatPayService = "wxpay"`
- `createWechatOrder(c, user, form) (codeURL, orderID string, err error)`：`insertPaymentOrder(...,"wxpay")` → `native.Prepay`(out_trade_no=orderID, amount.total=分, notify_url) → 返回 `CodeUrl`。
- `WechatNotifyAPI(c)`：`ParseNotifyRequest` 验签解密 → `trade_state == "SUCCESS"` → `completePaymentOrder()`。
- `queryWechatOrder(db, orderID) (paid bool, err error)`：`QueryOrderByOutTradeNo` → 已支付则入账。

### 5.4 路由（`auth/router.go`）新增
```
POST /payment/alipay/notify  → AlipayNotifyAPI
POST /payment/wechat/notify  → WechatNotifyAPI
```

## 6. 前端实现（新增）

### 6.1 `app/src/payment/request.ts`
`PaymentResponse.data` 增加可选 `qrcode?: string`。

### 6.2 `app/src/routes/wallet/WalletQuotaBox.tsx`
- `doPayment("alipay"|"wxpay")`：拿到 `res.data.qrcode` 后**打开二维码弹窗**（新增 state：`qrOpen/qrValue/qrOrder/qrMethod`），不跳转。
- 二维码弹窗（可内联）：`qrcode.react` 的 `<QRCodeSVG value={qrValue}/>`，标题区分「支付宝扫码 / 微信扫码」；打开期间每 2s `getPaymentOrderStatus(qrOrder)`；成功 → toast、关闭弹窗、刷新点数余额。
- 删除原 Stripe/PayPal 跳转回跳处理逻辑（见 3.4）。

## 7. 错误处理

- 未配置（`enabled=false` 或缺密钥）：`createXxxOrder` 返回「支付宝/微信支付未配置」，前端 toast。
- 验签失败：notify 返回非成功响应（支付宝回 `failure`，微信回 4xx + JSON），不入账。
- 金额/订单不匹配：查单/回调校验 `out_trade_no` 与金额；不符忽略并记日志。
- 幂等：`completePaymentOrder` 保证重复回调/查单不重复加点。
- quota 范围沿用现有 `1 ~ 99999` 校验。

## 8. 测试策略

- **编译/回归**：删除三家后 `go build ./...` 与前端 `tsc` 必须通过（重点查残留引用）。
- **后端单测**：金额换算（元/分）、订单号生成、配置缺失报错、分发分支选择。验签/SDK 调用用接口抽象 + mock 或集成测试（默认 skip）。
- **手动联调**：配置沙箱/真实商户密钥 → 充值 → 扫码 → 确认点数到账（走查单兜底，无需公网回调）。

## 9. 安全

- 商户密钥仅写 gitignored `config.yaml`，不进 git。
- 私钥/APIv3 key 不出现在日志、不返回前端。
- notify 接口强制验签后才入账。

## 10. 交付清单

**新增**：`auth/payment_core.go`、`auth/alipay.go`、`auth/wechatpay.go`、`channel/system.go`（alipayState/wechatPayState）、`config.yaml`(本地)、`config.example.yaml`、`go.mod/go.sum`、前端 `request.ts`、`WalletQuotaBox.tsx`、i18n（二维码弹窗文案）。

**删除/改造**：`auth/paypal.go`(删)、`auth/stripe.go`(删)、`auth/epay.go`(删)、`auth/router.go`(增删路由)、`channel/system.go`(删三家 state + 改 AsInfo/Normalize)、`WalletQuotaBox.tsx`(删三家入口与回跳逻辑)、`payment/icons.tsx`+`payment/utils.ts`(清理，可选)。
