FROM golang:1.27.1-alpine AS builder
RUN apk --no-cache add git ca-certificates gcc g++

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
    -X github.com/hipoint-airpress/airpress/consts.AirPressVersion=${AIRPRESS_VERSION}  \
    -X github.com/hipoint-airpress/airpress/consts.BuildCommit=${BUILD_COMMIT}  \
    -X github.com/hipoint-airpress/airpress/consts.BuildTime=${BUILD_TIME}" \
    -trimpath .

RUN mkdir -p /app/conf \
    && mkdir /app/resources \
    && cp -r /go/src/github.com/hipoint-airpress/airpress/airpress /app/ \
    && cp -r /go/src/github.com/hipoint-airpress/airpress/conf /app/ \
    && cp -r /go/src/github.com/hipoint-airpress/airpress/resources /app/ \
    && cp /go/src/github.com/hipoint-airpress/airpress/scripts/docker_init.sh /app/

FROM alpine:latest AS prod

COPY --from=builder /app/ /app/

RUN apk add --no-cache tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

RUN chmod +x /app/docker_init.sh

VOLUME /app
EXPOSE 8080

WORKDIR /app
CMD /app/docker_init.sh && /app/airpress -config /app/conf/config.yaml