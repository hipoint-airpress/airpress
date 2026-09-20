#!/bin/sh

# 镜像内置默认 resources
IMAGE_RESOURCE_DIR="/resources"
# 宿主机实际使用的 resources
HOST_RESOURCE_DIR="/app/resources"

mkdir -p "$HOST_RESOURCE_DIR"
if [ -d "$IMAGE_RESOURCE_DIR" ]; then
    echo "Copy default resources from $IMAGE_RESOURCE_DIR to $HOST_RESOURCE_DIR..."
    # 递归复制，但不覆盖宿主机已有文件
    cp -fr "$IMAGE_RESOURCE_DIR"/* "$HOST_RESOURCE_DIR"/
fi

echo "Start AirPress..."
exec /app/airpress -config /app/conf/config.yaml
