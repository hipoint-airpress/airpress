FROM golang:1.27.1-alpine AS builder
RUN apk add --no-cache \
    git \
    ca-certificates \
    gcc \
    g++

COPY . /go/src/github.com/hipoint-airpress/airpress/
WORKDIR /go/src/github.com/hipoint-airpress/airpress

# 拉取缺省的主题
RUN git submodule update --init --remote --recursive

ARG BUILD_COMMIT
ARG BUILD_TIME
ARG AIRPRESS_VERSION

RUN CGO_ENABLED=1 GOOS=linux go build \
    -o airpress \
    -ldflags="-s -w \
    -X github.com/hipoint-airpress/airpress/consts.AirPressVersion=${AIRPRESS_VERSION} \
    -X github.com/hipoint-airpress/airpress/consts.BuildCommit=${BUILD_COMMIT} \
    -X github.com/hipoint-airpress/airpress/consts.BuildTime=${BUILD_TIME}" \
    -trimpath .

RUN mkdir -p /app \
    && cp -r airpress conf resources /app/ \
    && cp scripts/entrypoint.sh /app/ \
    && chmod +x /app/entrypoint.sh

FROM alpine:3 AS prod

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

COPY --from=builder /app/airpress /app/airpress
COPY --from=builder /app/conf /app/conf
COPY --from=builder /app/entrypoint.sh /app/entrypoint.sh
# 镜像默认 resources
COPY --from=builder /app/resources /resources

RUN mkdir -p /app/resources \
    && chmod +x /app/entrypoint.sh

EXPOSE 8080

# 使用 ENTRYPOINT 固定入口
ENTRYPOINT ["/app/entrypoint.sh"]