#!/usr/bin/env bash
# ============================================================================
# 构建镜像并推送到阿里云容器镜像服务 ACR（本地手动构建 / 备用路径）
# ----------------------------------------------------------------------------
# 推荐主路径是「阿里云 ACR 自动构建」（打 git tag 触发，见 docs/DEPLOY_ALIYUN.md）。
# 本脚本用于不想等自动构建、或自动构建不可用时，在「本地开发机」手动构建推送。
# 本地若是 Apple Silicon(ARM)，强制 --platform linux/amd64，保证能在阿里云 amd64 运行。
#
# 前置：
#   1. docker buildx 可用（Docker Desktop 自带）。
#   2. 已登录 ACR（用户名是阿里云账号全名，如 吴志旋sy；密码是 ACR 访问凭证密码）：
#        docker login --username=吴志旋sy registry.cn-shenzhen.aliyuncs.com
#
# 用法：
#     TAG=1.0.0 bash scripts/build-push-acr.sh
# ============================================================================
set -euo pipefail

REGISTRY="${ACR_REGISTRY:-registry.cn-shenzhen.aliyuncs.com}"
NAMESPACE="${ACR_NAMESPACE:-wuzhixuan}"
IMAGE_NAME="${ACR_IMAGE:-coai}"
TAG="${TAG:-latest}"
PLATFORM="${PLATFORM:-linux/amd64}"

IMAGE="${REGISTRY}/${NAMESPACE}/${IMAGE_NAME}:${TAG}"

cd "$(dirname "$0")/.."

echo "==> 构建并推送镜像: ${IMAGE}  (platform=${PLATFORM})"

# 确保有一个支持多平台的 buildx builder
if ! docker buildx inspect coai-builder >/dev/null 2>&1; then
  docker buildx create --name coai-builder --use
else
  docker buildx use coai-builder
fi

docker buildx build \
  --platform "${PLATFORM}" \
  -t "${IMAGE}" \
  --push \
  .

echo "==> 完成。在服务器 .env 中设置："
echo "    COAI_IMAGE=${IMAGE}"
