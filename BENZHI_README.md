# task033-bizcal

这是一个可配置工作日历服务，支持工作日判定、按工作日增减日期、半开区间统计、工作日列表，以及将 RFC3339 时刻换算为日历时区的本地日期。服务只依赖 Go 标准库和仓库内的日历配置，不需要数据库或外部服务。

## 标准命令

在 `env/` 目录执行：

```bash
go build ./...
go test ./...
go vet ./...
go run . --smoke-test
go run . server --addr :8080 --calendar calendar.json
```

`--smoke-test` 会启动进程内 HTTP 服务并完成自检后退出；服务器模式默认监听 `:8080`。

## Benzhi 容器

`build_benzhi_docker.sh` 使用 `benzhi.Dockerfile` 构建自检镜像，参数依次是镜像名和平台，默认值为 `my-project` 与 `linux/amd64`。例如：

```bash
bash build_benzhi_docker.sh bizcal-benzhi linux/amd64
docker run --rm -it bizcal-benzhi:latest
```

容器启动后进入 shell；构建阶段会执行 `go build ./...`，不访问外部业务服务。
