#!/bin/sh

set -eu

# 镜像内置的默认 resources
IMAGE_RESOURCE="/resources"

# 宿主机实际使用的 resources
HOST_RESOURCE="/app/resources"

echo "Initialize AirPress resources..."

# 确保目标 resources 存在
mkdir -p "$HOST_RESOURCE"

# 复制镜像中的默认只复制不存在的文件
cp -an "$IMAGE_RESOURCE"/. "$HOST_RESOURCE"/

echo "AirPress resources initialization done."

echo "Start AirPress..."

exec /app/airpress -config /app/conf/config.yaml
