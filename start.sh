#!/bin/bash
#
# 简单的 Docker Compose 启动脚本（新版 docker-compose.yml）
#

echo "正在启动 Docker Compose 服务..."

if command -v docker-compose &>/dev/null; then
    COMPOSE_CMD="docker-compose"
elif docker compose version &>/dev/null; then
    COMPOSE_CMD="docker compose"
else
    echo "错误: Docker Compose 未安装"
    exit 1
fi

echo "使用 --build 参数启动新版服务..."
$COMPOSE_CMD -f ./docker-compose.yml pull
$COMPOSE_CMD -f ./docker-compose.yml up --build -d

echo "服务已启动"
