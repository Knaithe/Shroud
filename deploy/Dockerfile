FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
ARG GOPROXY=https://goproxy.cn,direct
RUN GOPROXY=${GOPROXY} go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/admin ./admin
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agent ./agent

FROM alpine:3.21
RUN apk add --no-cache bash curl procps netcat-openbsd
COPY --from=builder /out/admin /opt/shroud/admin
COPY --from=builder /out/agent /opt/shroud/agent
COPY deploy/test/ /opt/shroud/test/
RUN chmod +x /opt/shroud/test/*.sh
WORKDIR /opt/shroud
