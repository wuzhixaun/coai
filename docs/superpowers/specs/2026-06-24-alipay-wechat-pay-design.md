# 设计：接入支付宝 / 微信官方扫码支付（点数充值）

- 日期：2026-06-24
- 分支：`feat/jimeng-official-api`
- 状态：已通过设计评审，待写实现计划

## 1. 目标与范围

为「钱包 / 点数充值」流程新增**支付宝**与**微信**两种支付方式，与现有 Stripe **完全平行**。

- **方式**：官方商户直连（非聚合/易支付），用官方/主流 Go SDK。
- **场景**：PC 扫码支付 —— 支付宝当面付（`TradePreCreate`）+ 微信 Native（`Prepay`），均返回二维码，用户手机扫码付款。
- **范围**：**仅点数充值**（10/50/100/200 及自定义 quota），与 Stripe 平行。订阅仍用点数余额扣费，本次**不改订阅购买流程**。

### 不在范围内（YAGNI）

- 不做手机 H5/WAP 跳转支付（用户主要在 PC）。
- 不做退款、对账报表、分账。
- 不改订阅（基础/标准/专业版）购买链路。
- 不动 Stripe / PayPal / EPay 现有逻辑。

## 2. 现有架构（复用点）

| 能力 | 位置 | 复用方式 |
|---|---|---|
| 充值分发入口 | `auth/paypal.go` `CreatePaymentAPI()` (L245) | 新增 `alipay`/`wxpay` 分支 |
| 写订单 | `auth/paypal.go` `insertPaymentOrder()` (L230) | 直接复用 |
| 完成订单 + 点数入账（幂等） | `auth/paypal.go` `completePaymentOrder()` (L385) | 直接复用（已含 `state=FALSE` 幂等校验 + 事务 + `IncreaseQuota`） |
| 查询订单状态 | `auth/paypal.go` `CheckPaymentAPI()` | 增强：对 pending 的 alipay/wxpay 主动查单 |
| 点数入账 | `auth/quota.go` `IncreaseQuota()` (L46) | 经由 `completePaymentOrder` 间接复用 |
| 订单表 / 点数表 | `connection/database.go` `payment` / `quota` | 无需改表结构（`service` 列已是 VARCHAR，存 `alipay`/`wxpay`） |
| 配置结构 | `channel/system.go` `paymentState`/`stripeState` (L95-121) | 新增 `alipayState`/`wechatPayState` 字段 |
| 前端发起/轮询 | `app/src/payment/request.ts` | `PaymentResponse.data` 增加可选 `qrcode` 字段 |
| 充值页 | `app/src/routes/wallet/WalletQuotaBox.tsx` `doPayment()` (L142) | 扫码方式改为弹二维码而非跳转 |
| 二维码渲染 | 依赖 `qrcode.react`（已安装） | 直接用 |
| 金额换算 | `quota * 0.1` = 人民币元（与 EPay 一致，10点=¥1） | 沿用；元→分由各 SDK 调用处转换 |

## 3. 确认支付的方式（核心设计）：双重确认

1. **异步回调（生产主路径）**：支付宝/微信付款成功后 POST 到 `notify_url` → 验签 → `completePaymentOrder()` 入账。
2. **轮询时主动查单（兜底 + 本地可测）**：前端每 2s 轮询 `/payment/check/:order`；后端发现该订单 `service ∈ {alipay,wxpay}` 且仍 `pending` 时，调用对应平台查单接口，若已支付则 `completePaymentOrder()` 入账后返回 `order_state:true`。

> 主动查单使本地开发（localhost 收不到回调）也能跑通，并在回调丢失时不漏单。`completePaymentOrder` 的幂等校验保证回调与查单两条路径不会重复入账。

## 4. 后端实现

### 4.1 配置（`channel/system.go` + gitignored `config.yaml`）

新增两个 state（沿用 `mapstructure` + Compat 别名风格）：

```go
type alipayState struct {
    Enabled         bool   // enabled
    AppID           string // appid
    PrivateKey      string // privatekey         应用私钥 RSA2
    AlipayPublicKey string // alipaypublickey     支付宝公钥（验签用）
    IsProd          bool   // isprod              true=正式网关
}

type wechatPayState struct {
    Enabled         bool   // enabled
    AppID           string // appid               公众号/小程序/APP appid（Native 需 mch 绑定 appid）
    MchID           string // mchid               商户号
    APIv3Key        string // apiv3key            APIv3 密钥
    MchCertSerialNo string // mchcertserialno     商户证书序列号
    MchPrivateKey   string // mchprivatekey       商户 API 私钥（apiclient_key.pem 内容）
}
```

`paymentState` 增加 `Alipay alipayState`、`WechatPay wechatPayState` 两字段。`notify_url` 由请求里的 `domain` 拼成 `{domain}/payment/alipay/notify`、`{domain}/payment/wechat/notify`。

### 4.2 SDK 选型

- 支付宝：`github.com/smartwalle/alipay/v3`（成熟、支持 `TradePreCreate` 与回调验签）。
- 微信：`github.com/wechatpay-apiv3/wechatpay-go`（官方 SDK，支持 Native `Prepay`、回调解密验签、证书自动轮换）。

### 4.3 新文件

**`auth/alipay.go`**
- `const alipayService = "alipay"`
- `createAlipayOrder(c, user, form) (qrcode string, orderID string, err error)`：生成订单号 → `insertPaymentOrder(db,user,form,orderID,"alipay")` → `TradePreCreate`（out_trade_no=orderID，total_amount=元，subject=form.Name，notify_url）→ 返回 `QrCode`。
- `AlipayNotifyAPI(c)`：`client.GetTradeNotification` 验签 → `trade_status ∈ {TRADE_SUCCESS,TRADE_FINISHED}` → 解析 orderID/amount → `completePaymentOrder()` → 回 `"success"`。
- `queryAlipayOrder(db, orderID) (paid bool, err error)`：`TradeQuery` → 已支付则 `completePaymentOrder()`。

**`auth/wechatpay.go`**
- `const wechatPayService = "wxpay"`
- `createWechatOrder(c, user, form) (codeURL string, orderID string, err error)`：`insertPaymentOrder(...,"wxpay")` → `native.Prepay`（out_trade_no=orderID，amount.total=分，notify_url）→ 返回 `CodeUrl`。
- `WechatNotifyAPI(c)`：`notifyHandler.ParseNotifyRequest` 验签解密 → `trade_state == "SUCCESS"` → `completePaymentOrder()`。
- `queryWechatOrder(db, orderID) (paid bool, err error)`：`QueryOrderByOutTradeNo` → 已支付则入账。

### 4.4 路由（`auth/router.go`）

```
POST /payment/alipay/notify  → AlipayNotifyAPI
POST /payment/wechat/notify  → WechatNotifyAPI
```
（`/payment/create` 与 `/payment/check/:order` 已存在，仅扩展内部逻辑。）

### 4.5 分发（`auth/paypal.go` `CreatePaymentAPI`）

在 EPay 分支后、Stripe 分支前后插入：
```go
if paymentType == alipayService { qrcode,order,err := createAlipayOrder(...); 返回 {data:{qrcode, params:{order}}} }
if paymentType == wechatPayService { codeURL,order,err := createWechatOrder(...); 返回 {data:{qrcode: codeURL, params:{order}}} }
```
统一用 `qrcode` 字段承载二维码内容（与跳转型的 `url` 区分）。

### 4.6 查单兜底（`CheckPaymentAPI`）

读取订单 `service`；若是 `alipay`/`wxpay` 且未完成，调用 `queryAlipayOrder`/`queryWechatOrder`；其余 service 行为不变。

## 5. 前端实现

### 5.1 `app/src/payment/request.ts`
`PaymentResponse.data` 增加可选 `qrcode?: string`。

### 5.2 `app/src/routes/wallet/WalletQuotaBox.tsx`
- `doPayment(method)`：
  - `method ∈ {alipay, wxpay}`：拿到 `res.data.qrcode` 后**打开二维码弹窗**（新增本地 state：`qrOpen`、`qrValue`、`qrOrder`、`qrMethod`），不跳转。
  - 其它（stripe/paypal）：维持 `window.location.href = res.data.url`。
- 新增二维码弹窗组件（可内联）：用 `qrcode.react` 的 `<QRCodeSVG value={qrValue}/>`，标题区分「支付宝扫码 / 微信扫码」，弹窗打开期间每 2s `getPaymentOrderStatus(qrOrder)`；成功 → toast 成功、关闭弹窗、刷新点数余额。
- 充值方式按钮区已有 alipay/wxpay 图标（`payment/icons.tsx`），按 `enabled` 配置显示即可。

## 6. 错误处理

- 未配置（`enabled=false` 或缺密钥）：`createXxxOrder` 返回明确错误「支付宝/微信支付未配置」，前端 toast。
- 验签失败：notify 返回非成功响应（支付宝回 `failure`，微信回 4xx + JSON），不入账。
- 金额/订单不匹配：查单/回调中校验 `out_trade_no` 与金额；不符则忽略并记日志。
- 幂等：`completePaymentOrder` 已保证重复回调/查单不重复加点。
- quota 范围沿用现有 `1 ~ 99999` 校验。

## 7. 测试策略

- **后端单测**：金额换算（元/分）、订单号生成、配置缺失时报错路径、分发分支选择。验签/SDK 调用用接口抽象 + mock 或标记为需要凭证的集成测试（默认 skip）。
- **手动联调**：配置沙箱/真实商户密钥 → 充值 → 扫码 → 确认点数到账（走查单兜底，无需公网回调）。
- **回归**：Stripe/PayPal/EPay 充值不受影响。

## 8. 安全

- 所有商户密钥仅写入 gitignored `config.yaml`，**不进 git**。
- 私钥/APIv3 key 不出现在日志、不返回前端。
- notify 接口强制验签后才入账。

## 9. 交付清单

后端：`channel/system.go`、`config.yaml`（本地）、`config.example.yaml`、`auth/alipay.go`(新)、`auth/wechatpay.go`(新)、`auth/router.go`、`auth/paypal.go`(`CreatePaymentAPI` + `CheckPaymentAPI`)、`go.mod/go.sum`。
前端：`app/src/payment/request.ts`、`app/src/routes/wallet/WalletQuotaBox.tsx`、i18n 文案（二维码弹窗标题/提示）。
