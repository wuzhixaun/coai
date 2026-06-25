# 阿里云 Docker 部署手册（ACR 自动构建方式）

把 CoAI 部署到阿里云 ECS，方式为：**打 git tag → 阿里云 ACR 自动构建镜像 → 服务器 docker compose 拉取启动**。
首版目标：**用 `http://服务器IP:端口` 跑通**（域名 + HTTPS 后续再加）。

> 为什么不用官方镜像：本项目有自定义改动（即梦集成、Photo 生图等），必须用本仓库代码构建。

## 你的资源（已就绪）

| 项 | 值 |
|---|---|
| ACR 仓库（公网） | `registry.cn-shenzhen.aliyuncs.com/wuzhixuan/coai` |
| ACR 仓库（VPC 内网） | `registry-vpc.cn-shenzhen.aliyuncs.com/wuzhixuan/coai` |
| 地域 | 华南1（深圳） |
| 登录用户名 | `吴志旋sy`（阿里云账号全名） |
| 登录密码 | ACR「访问凭证」里设置的密码 |
| 绑定代码仓库 | `github.com/wuzhixaun/coai`（注意：GitHub 名 `wuzhixaun`，与命名空间 `wuzhixuan` 不同） |
| 自动构建规则 | tag `release-v$version` → 镜像 `$version`，Dockerfile 在根目录 |

> 命名空间 `wuzhixuan` 与 GitHub 用户名 `wuzhixaun` 拼写相近但**不是同一个**，复制时别弄错。

---

## 架构

```
浏览器 ──http──> 阿里云ECS:8000 ──> [coai 容器:8094] ──┬──> [mysql 容器:3306]
                                                      └──> [redis 容器:6379]
              (coai 单容器内同时托管前端静态文件 + 后端API，serve_static=true)
```

相关文件：
- `docker-compose.prod.yaml` — 服务器部署编排
- `.env.prod.example` — 环境变量模板（复制成 `.env` 填写）
- `scripts/build-push-acr.sh` — 本地手动构建推送（**备用**，主路径用自动构建）
- `config/config.yaml` — 真实运行配置（含密钥，**不进镜像**，单独上传）

---

## 一、一次性准备

### 1. ECS + Docker
- ECS：2核4G 起，系统 Ubuntu 22.04 / Alibaba Cloud Linux 3，架构 **x86_64(amd64)**。
- 装 Docker：
  ```bash
  curl -fsSL https://get.docker.com | bash -s docker --mirror Aliyun
  systemctl enable --now docker
  docker compose version   # 确认自带 compose v2
  ```

### 2. 安全组放行
ECS → 安全组 → 入方向：放行 **8000/TCP**（对外访问）、**22/TCP**（SSH）。
**MySQL(3306)/Redis(6379) 不要对公网放行**，仅 compose 内部互通。

### 3. ⚠️ 开启「海外机器构建」（关键，否则构建大概率失败）
ACR → 镜像仓库 coai → **构建** → 构建设置：把 **「海外机器构建」打开**。

原因：Dockerfile 基础镜像（`golang`/`node`/`alpine`）在 Docker Hub，Go 模块默认走 `proxy.golang.org`，前端 pnpm 走 npmjs——这些在国内构建机上慢且易失败。开启后构建在海外机器跑，拉取顺畅；产出的镜像仍存到深圳 ACR，不影响国内服务器拉取。

> 不想开海外构建（坚持国内构建）的备选：改 Dockerfile 启用国内镜像源（`GOPROXY=https://goproxy.cn,direct`、npm/pnpm 淘宝源），并把基础镜像换成国内可达的镜像。改完需 commit 推到 GitHub 才会被自动构建使用。多数情况直接开海外构建更省事。

---

## 二、构建镜像（主路径：打 tag 自动构建）

确保代码已推到 GitHub（当前 `main` 已与远端同步）。要发版时打一个 `release-v` 开头的 tag 并推送：

```bash
# 在本地项目目录
git tag release-v1.0.0
git push origin release-v1.0.0
```

阿里云 ACR 检测到该 tag，按内置规则自动构建，产出镜像：
`registry.cn-shenzhen.aliyuncs.com/wuzhixuan/coai:1.0.0`（tag 去掉 `release-v` 前缀作为镜像版本号）。

在 ACR → coai → **构建** 页可看「构建日志」状态；→ **镜像版本** 页可看产出的 `1.0.0`。
首次构建较慢（编译 Go + 前端 build），耐心等其变「成功」。

> 想每次 push main 自动出 `:latest`：在「构建规则设置」点「添加规则」，Branch 填 `main`、镜像版本填 `latest`。生产建议仍用 tag 版本号（不可变、可回滚）。

### 备用路径：本地手动构建推送
不想等自动构建时：
```bash
docker login --username=吴志旋sy registry.cn-shenzhen.aliyuncs.com
TAG=1.0.0 bash scripts/build-push-acr.sh
```
脚本已强制 `--platform linux/amd64`（本地 Mac ARM → 阿里云 amd64）。

---

## 三、服务器部署

### 1. 部署目录与编排文件
```bash
mkdir -p /opt/coai && cd /opt/coai
```
从本地上传以下文件到 `/opt/coai/`：
```bash
# 本地执行
scp docker-compose.prod.yaml .env.prod.example root@<服务器IP>:/opt/coai/
```

### 2. 写 `.env`
```bash
cd /opt/coai
cp .env.prod.example .env
vim .env
```
关键项：
```ini
# tag 要和你构建出的镜像版本一致（打了 release-v1.0.0 就用 :1.0.0）
COAI_IMAGE=registry.cn-shenzhen.aliyuncs.com/wuzhixuan/coai:1.0.0
# 若 ECS 也在深圳，改用 VPC 内网地址更快：
# COAI_IMAGE=registry-vpc.cn-shenzhen.aliyuncs.com/wuzhixuan/coai:1.0.0
HOST_PORT=8000
MYSQL_ROOT_PASSWORD=<强密码>
MYSQL_DATABASE=chatnio
MYSQL_USER=chatnio
MYSQL_PASSWORD=<强密码>
```

### 3. 上传运行配置 `config/config.yaml`
配置含密钥、被 gitignore，**不在镜像里**，单独传到 `/opt/coai/config/`：
```bash
# 本地执行
scp config/config.yaml config/prompts.json root@<服务器IP>:/opt/coai/config/
```

**无需手改 config.yaml 的启动字段**：`docker-compose.prod.yaml` 已用环境变量覆盖（viper.AutomaticEnv，环境变量优先于配置文件）：
- `serve_static` → 强制 `true`（前端由后端托管，单容器可访问）
- `mysql.host` → `mysql`、`redis.host` → `redis`（指向 compose 内部容器）
- DB 账号密码 → 取自 `.env`

> 本地 config.yaml 里 `serve_static: false`、`host: 120.76.157.51` **不影响线上**，会被覆盖。
> 但 `secret`、渠道密钥、计费/市场等业务配置仍生效——请确认 `secret` 已是 ≥32 位随机串（否则启动告警并延迟 10 秒）。

### 4. 拉镜像并启动
```bash
cd /opt/coai
# 服务器拉私有镜像也要先登录 ACR
docker login --username=吴志旋sy registry.cn-shenzhen.aliyuncs.com

docker compose -f docker-compose.prod.yaml --env-file .env pull
docker compose -f docker-compose.prod.yaml --env-file .env up -d
```

### 5. 验证
```bash
docker compose -f docker-compose.prod.yaml ps              # 三个容器 Up
docker compose -f docker-compose.prod.yaml logs -f chatnio  # 启动日志
```
浏览器访问：`http://<服务器IP>:8000`

---

## 四、日常运维

```bash
cd /opt/coai
COMPOSE="docker compose -f docker-compose.prod.yaml --env-file .env"
$COMPOSE ps                 # 状态
$COMPOSE logs -f chatnio    # 日志
$COMPOSE restart chatnio    # 重启应用
$COMPOSE down               # 停止（保留数据卷）
$COMPOSE up -d              # 启动
```

### 发版更新（改了代码后）
```bash
# 本地：推代码 + 打新 tag（触发自动构建）
git push origin main
git tag release-v1.0.1 && git push origin release-v1.0.1
# 等 ACR 构建成功后，服务器：改 .env 的 COAI_IMAGE 版本号为 1.0.1，再：
cd /opt/coai
docker compose -f docker-compose.prod.yaml --env-file .env pull chatnio
docker compose -f docker-compose.prod.yaml --env-file .env up -d chatnio
docker image prune -f
```
> 回滚：把 `.env` 的 `COAI_IMAGE` 改回旧版本号，重新 `pull` + `up -d` 即可。

### 数据与备份
- MySQL：`/opt/coai/db`　Redis：`/opt/coai/redis`　图片/视频：`/opt/coai/storage`　配置：`/opt/coai/config`
- 备份示例：`tar czf coai-backup-$(date +%F).tgz config storage`（DB 建议用 mysqldump）

---

## 五、排错速查

| 现象 | 排查 |
|---|---|
| ACR 构建失败/卡住 | 是否开了「海外机器构建」；构建日志看是拉基础镜像失败还是 go/pnpm 下载失败 |
| 访问 IP:8000 打不开 | 安全组放行 8000；`$COMPOSE ps` 是否 Up；服务器本机 `curl localhost:8000` |
| 页面空白/404 | 确认用的是 `docker-compose.prod.yaml`（已强制 serve_static=true） |
| 连不上数据库 | mysql 容器 Up？`.env` 的 MYSQL_USER/PASSWORD 一致？首次启动 mysql 初始化要几十秒，应用会重试 |
| 拉镜像 denied | 服务器是否 `docker login` 过 ACR；镜像地址/版本号是否正确、是否已构建成功 |
| exec format error | 镜像架构不对。自动构建产出的是 amd64；本地手动构建务必走 `scripts/build-push-acr.sh`（带 --platform） |
| secret 警告/延迟启动 | config.yaml 的 `secret` 改成 ≥32 位随机串后重启 |

---

## 六、后续：域名 + HTTPS（待办）

跑通后若要正式对外：服务器加一层 Nginx 反代到 `127.0.0.1:8000`，用 Let's Encrypt/阿里云免费证书配 443。需先有域名解析到服务器 IP，届时再补这部分。
