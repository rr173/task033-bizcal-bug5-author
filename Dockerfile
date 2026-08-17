# syntax=docker/dockerfile:1

# builder: identical Go toolchain to the local machine
FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS builder
WORKDIR /src
COPY . .
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOPROXY=https://goproxy.cn,direct \
    GOSUMDB=sum.golang.google.cn
RUN go build -o /out/task033-bizcal .

# runtime: minimal image; calendar.json is bundled so server mode works too.
FROM docker.m.daocloud.io/library/alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/task033-bizcal /task033-bizcal
COPY --from=builder /src/calendar.json /app/calendar.json
ENTRYPOINT ["/task033-bizcal"]
