# Pro 功能回归核对报告

核对时间：2026-06-18  
项目目录：`/Users/wuzhixuan/code/project/coai`  
当前基线：`main` / `3048a493eedcfe75de4d59afd5139847ad3195ed`

## 结论

当前现象不是单纯的“Pro 订阅失效”。更准确地说：当前本地仓库的提交基线只包含 Pro 相关页面壳和部分前端入口，很多后台功能并没有完整实现；现在页面显示已购买，是本地新增的授权判断把所有模块统一标记为 bought，但这不等于每个模块的后端能力都存在。

如果以前这些模块确实完整可用，那么以前运行的很可能不是当前 `origin/main` 这份源码，而是另一个企业版构建、旧二进制、旧分支、镜像或私有补丁包。

## 核对范围

- Git 提交基线与当前工作区差异
- 授权管理页面与 Pro 门禁实现
- 支付下单接口、PayPal、EasyPay
- 使用记录表与写入链路
- 渠道模型与价格刷新逻辑
- 当前运行态接口
- 本地 `chat.bak.*` 二进制备份中的功能字符串

## Git 基线对比

当前仓库只有 `main` / `origin/main`，没有其他本地分支或 tag 可直接对比。

`HEAD` 中存在的授权页是 `app/src/routes/admin/License.tsx`，但它是占位页面：

- `data` 被写死为 `{ domain: "", digest: "" }`
- 模块卡片全部是 `bought={false}`
- 页面只提示需要 Pro
- 不调用 `/admin/license`

也就是说，提交基线里的授权页不是一个真正的授权校验页面。

以下文件不在 `HEAD` 中，属于当前工作区新增：

- `admin/license.go`
- `admin/record.go`
- `admin/payment.go`
- `auth/paypal.go`
- `auth/epay.go`
- `manager/record.go`
- `app/src/components/admin/ProGate.tsx`
- `app/src/routes/admin/Record.tsx`
- `app/src/routes/admin/Payment.tsx`
- `app/src/routes/admin/Warmup.tsx`

这些文件承载了当前看到的大部分 Pro 功能。因此它们不是仓库原本提交过的稳定企业版实现。

## 授权逻辑对比

当前新增的 `admin/license.go` 逻辑是：

```sql
SELECT COUNT(*) FROM subscription
WHERE enterprise = TRUE AND expired_at > NOW()
```

只要本地数据库里存在任何有效企业订阅，就把这些模块全部标记为已购买：

- `coai-pro`
- `afdian`
- `paypal`
- `stripe`
- `digital`

当前新增的 `ProGate.tsx` 逻辑是：只要 `/admin/license` 返回的任意模块 `bought=true`，就允许进入 Pro 页面。

这说明当前授权判断是“全局 Pro 开关”，不是“按模块授权”。所以页面能进，不代表支付订单、使用记录、资源预热、数字人等后端功能一定完整。

## 支付模块对比

提交基线中，前端 `app/src/payment/request.ts` 已经会请求：

- `POST /payment/create`
- `GET /payment/check/:order`

但提交基线的 `auth/router.go` 没有注册这两个后端路由，也没有对应 handler。

基线里的 `auth/payment.go` 主要依赖 Deeptrain：

- `auth.use_deeptrain=true` 时走 Deeptrain 支付
- 否则购买点数会返回 `cannot find payment provider`

这解释了为什么“充值看不到”或“支付不可用”：前端入口存在，但本地后端支付 provider 没完整接上。

当前工作区已经新增：

- PayPal 下单、检查、捕获订单
- EasyPay 下单、异步通知、同步返回
- `payment` 表初始化
- 管理后台支付订单查询和重查

## 使用记录对比

提交基线中可以看到前端有用户侧 `/record/view` 请求，但没有发现聊天成功后写入 `record` 表的代码。

当前工作区新增了：

- `record` 表初始化
- `manager/record.go`
- WebSocket 聊天成功后写入使用记录
- `/v1/chat/completions` 非流式和流式成功后写入使用记录
- 后台 `/admin/record/list` 与 `/admin/record/stats`

二进制备份也能印证：只有当前 `chat` 二进制包含 `INSERT INTO record` 和 `api chat completion` 字符串，早期备份没有。因此使用记录为空主要是代码缺口，不是用户操作问题。

## 渠道与模型对比

之前出现过 `cannot find channel for model gpt-3.5-turbo`。当前运行态核对结果：

- `/v1/models` 只返回 `deepseek-v4-pro`
- 当前配置渠道模型也是 `deepseek-v4-pro`
- 所以如果聊天里仍选 `gpt-3.5-turbo`，会找不到渠道

当前工作区已修复：

- 渠道保存后立即 `Load()`，避免新建渠道后模型市场不刷新
- 价格保存后立即 `Load()`，避免计费规则不刷新

## 当前运行态核对

当前监听进程：

- 后端：`./chat`，监听 `8094`
- 前端：Vite，监听 `5173`

当前公开接口结果：

- `/info`：`payment:["epay"]`
- `/v1/models`：只有 `deepseek-v4-pro`
- `/v1/charge`：`deepseek-v4-pro` 的 token-billing 价格存在
- 未登录访问 `/admin/license`：401，说明授权接口受 admin 登录态保护

## 二进制备份对比

本地备份字符串扫描结果显示：

- `chat.bak.20260617231909` 已包含 `/admin/license`、`/admin/record/list`、`/admin/payment/view`，但不包含 `/payment/create`、`/payment/check`、EasyPay 回调，也不包含 `INSERT INTO record`
- 后续备份开始出现 PayPal/EasyPay 路由
- 只有当前 `chat` 出现 `INSERT INTO record` 和 `api chat completion`

这说明本地之前跑过的二进制也不是完整企业版，只是已经包含一部分本地新增后台接口；使用记录写入是在最近才补上的。

## 为什么会感觉“之前 Pro 是好的”

最可能的原因有三类：

1. 之前访问的是另一个部署环境或旧镜像，里面带了完整企业功能。
2. 之前本地二进制带了一部分未提交代码，但当前源码基线并没有这些实现。
3. 当前页面因为本地数据库存在企业订阅而解锁了入口，但入口背后的功能原本没有完整接入。

## 目前已确认修复的代码缺口

- PayPal / EasyPay 支付入口和回调链路
- `payment` 表创建
- `record` 表创建
- 聊天和 API 调用成功后的使用记录写入
- 后台记录统计从 `record` 表读取
- 渠道保存后刷新运行时模型列表
- 价格保存后刷新运行时计费规则
- Claude 兼容流式返回中的 `thinking_delta` 解析。当前上游 `deepseek-v4-pro` 会先返回 thinking 内容，旧适配器只解析 `delta.text`，会导致 `empty response`，从而不写使用记录。已改为输出 `<think>...</think>` 并在真正无内容时返回 `no response`。

## 本次闭环验证结果

- 当前数据库存在 1 个有效企业订阅，所以 Pro 门禁会打开。
- 修复前 `record_total=0`。
- 修复后用 `deepseek-v4-pro` 走 WebSocket 聊天，`record_delta=1`。
- 最新记录写入字段包括：`model=deepseek-v4-pro`、`channel_name=deepseek`、`input_tokens=8`、`output_tokens=15`、`quota=0.023`。
- 因此“使用记录没有出现使用”这条链路已经确认是代码/适配器问题，并已修复到可写入。

## 仍需继续核对的点

- 用真实 admin 登录态访问 `/admin/license`，确认当前数据库是否存在有效 `enterprise` 订阅。
- 用 EasyPay 发起一笔小额订单，确认 `payment` 表、异步回调、额度增加是否闭环。当前未自动创建新测试单，因为远程库已有多条 pending 支付记录，继续造单会增加噪音。
- 如果有“以前好的版本”，拿旧镜像、旧二进制、旧分支或备份源码与当前工作区做逐文件对比。
- Stripe、爱发电、数字人目前仍是授权卡片展示，不等于真实模块已完整实现。
