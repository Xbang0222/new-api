#!/bin/bash

# 自定义 Docker 部署脚本
# 使用方法: ./deploy.sh [your-dockerhub-username]

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 获取 Docker Hub 用户名
DOCKERHUB_USERNAME="${1:-your-dockerhub-username}"
IMAGE_NAME="${DOCKERHUB_USERNAME}/new-api:latest"
CONTAINER_NAME="new-api"
DATA_DIR="./data"

echo -e "${GREEN}=== New API 自定义版本部署脚本 ===${NC}"
echo ""

# 检查 Docker 是否安装
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装${NC}"
    exit 1
fi

echo -e "${YELLOW}1. 拉取最新镜像...${NC}"
docker pull "$IMAGE_NAME"

echo -e "${YELLOW}2. 停止旧容器...${NC}"
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    docker stop "$CONTAINER_NAME" || true
    docker rm "$CONTAINER_NAME" || true
    echo -e "${GREEN}   旧容器已停止并删除${NC}"
else
    echo -e "${GREEN}   没有找到旧容器${NC}"
fi

echo -e "${YELLOW}3. 创建数据目录...${NC}"
mkdir -p "$DATA_DIR"

echo -e "${YELLOW}4. 启动新容器...${NC}"
docker run --name "$CONTAINER_NAME" -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v "$DATA_DIR:/data" \
  "$IMAGE_NAME"

echo ""
echo -e "${GREEN}=== 部署完成！ ===${NC}"
echo ""
echo "容器名称: $CONTAINER_NAME"
echo "镜像: $IMAGE_NAME"
echo "访问地址: http://localhost:3000"
echo ""
echo "查看日志: docker logs -f $CONTAINER_NAME"
echo "停止容器: docker stop $CONTAINER_NAME"
echo ""
