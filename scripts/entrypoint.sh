#!/bin/sh

set -eu

# 镜像内置的默认 resources
DEFAULT_ROOT="/resources"
# 宿主机实际使用的 resources
TARGET_ROOT="/app/resources"

# 递归复制：
# - 目标不存在：复制
# - 目标目录存在：继续检查里面的内容
# - 目标文件存在：不覆盖
copy_missing() {
    SOURCE="$1"
    TARGET="$2"

    # 目标不存在，直接复制
    if [ ! -e "$TARGET" ]; then
        mkdir -p "$(dirname "$TARGET")"
        echo "Initialize: $SOURCE -> $TARGET"
        cp -a "$SOURCE" "$TARGET"
        return
    fi

    # 两边都是目录，继续递归
    if [ -d "$SOURCE" ] && [ -d "$TARGET" ]; then
        for ITEM in "$SOURCE"/*; do
            [ -e "$ITEM" ] || continue

            NAME=$(basename "$ITEM")

            copy_missing \
                "$ITEM" \
                "$TARGET/$NAME"
        done
        return
    fi

    # 文件已经存在，不覆盖
    echo "Already exists, skip: $TARGET"
}

echo "Initialize AirPress resources..."
mkdir -p "$TARGET_ROOT"

for ITEM in "$DEFAULT_ROOT"/*; do
    [ -e "$ITEM" ] || continue
    NAME=$(basename "$ITEM")
    copy_missing "$ITEM" "$TARGET_ROOT/$NAME"
done

echo "AirPress resources initializing done"
echo "Start AirPress..."

exec /app/airpress -config /app/conf/config.yaml