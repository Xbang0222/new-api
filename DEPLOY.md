# 自定义部署指南

本文档说明如何使用 GitHub Actions 自动构建并部署自定义版本的 new-api。

## 📋 前置准备

1. ✅ GitHub 账号（已有）
2. ✅ Docker Hub 账号（已创建）
3. ✅ 服务器（已有 Docker 环境）

## 🔧 一次性配置（只需做一次）

### 步骤 1：获取 Docker Hub Token

1. 访问 https://hub.docker.com/settings/security
2. 点击 "New Access Token"
3. Token 名称填写：`github-actions`
4. 权限选择：`Read, Write, Delete`
5. 点击 "Generate"
6. **复制生成的 token**（只显示一次！）

### 步骤 2：配置 GitHub Secrets

1. 访问你的 GitHub 仓库：https://github.com/Xbang0222/new-api
2. 点击 `Settings` → `Secrets and variables` → `Actions`
3. 点击 `New repository secret`，添加两个 secret：

   **Secret 1:**
   - Name: `DOCKERHUB_USERNAME`
   - Value: `你的 Docker Hub 用户名`

   **Secret 2:**
   - Name: `DOCKERHUB_TOKEN`
   - Value: `刚才复制的 token`

### 步骤 3：推送代码触发构建

```bash
# 提交新创建的文件
git add .github/workflows/docker-custom-build.yml
git add deploy.sh
git add DEPLOY.md
git commit -m "ci: add custom Docker build workflow"

# 推送到 GitHub（会自动触发构建）
git push origin main
```

## 🚀 使用方法

### 自动构建

每次你推送代码到 `main` 分支，GitHub Actions 会自动：
1. 构建前端（使用 Bun）
2. 构建后端（Go）
3. 打包成 Docker 镜像
4. 推送到你的 Docker Hub

查看构建状态：https://github.com/Xbang0222/new-api/actions

### 手动触发构建

1. 访问 https://github.com/Xbang0222/new-api/actions
2. 点击左侧 "Build Custom Docker Image"
3. 点击右侧 "Run workflow"
4. 点击绿色的 "Run workflow" 按钮

### 在服务器上部署

```bash
# 方法 1：使用部署脚本（推荐）
wget https://raw.githubusercontent.com/Xbang0222/new-api/main/deploy.sh
chmod +x deploy.sh
./deploy.sh 你的dockerhub用户名

# 方法 2：手动部署
docker pull 你的dockerhub用户名/new-api:latest
docker stop new-api && docker rm new-api
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  你的dockerhub用户名/new-api:latest
```

## 🔄 更新流程

### 跟随上游更新

```bash
# 1. 拉取上游更新
git fetch upstream
git merge upstream/main

# 2. 解决冲突（如果有）
# 手动编辑冲突文件，然后：
git add .
git commit -m "merge: sync with upstream"

# 3. 推送（会自动触发构建）
git push origin main

# 4. 等待构建完成（约 5-10 分钟）
# 访问 https://github.com/Xbang0222/new-api/actions 查看进度

# 5. 在服务器上更新
./deploy.sh 你的dockerhub用户名
```

### 修改前端后更新

```bash
# 1. 修改代码
# 编辑 web/ 目录下的文件

# 2. 提交并推送
git add .
git commit -m "feat: 你的修改说明"
git push origin main

# 3. 等待自动构建完成

# 4. 在服务器上更新
./deploy.sh 你的dockerhub用户名
```

## 📊 镜像版本说明

- `你的dockerhub用户名/new-api:latest` - 最新版本（推荐使用）
- `你的dockerhub用户名/new-api:custom-20260306-abc1234` - 带时间戳和 commit 的版本

## 🔍 故障排查

### 构建失败

1. 检查 GitHub Actions 日志：https://github.com/Xbang0222/new-api/actions
2. 确认 Docker Hub secrets 配置正确
3. 检查 Docker Hub 存储空间是否充足

### 部署失败

```bash
# 查看容器日志
docker logs new-api

# 查看容器状态
docker ps -a | grep new-api

# 重启容器
docker restart new-api
```

## 💡 提示

- 构建时间约 5-10 分钟，请耐心等待
- 每次推送都会触发构建，建议本地测试后再推送
- 数据保存在 `./data` 目录，升级不会丢失数据
- 可以在 GitHub Actions 页面查看构建历史和日志
