# BENZHI_README

基于 Go 实现的古树复壮作业放行台 Web 项目，一款后端服务，已完整实现古树复壮作业单流程 Web 放行台，覆盖建档、四类现场调查、风险形成、处置覆盖、送审锁定、退回整改复验、规范化冻结、不可变凭据签发、凭据核验和完整审计轨迹。

## 项目说明
- 项目：benzhi-project-ced5f590-321b-4879-9932-43b49d936f51
- 项目用途：已完整实现古树复壮作业单流程 Web 放行台，覆盖建档、四类现场调查、风险形成、处置覆盖、送审锁定、退回整改复验、规范化冻结、不可变凭据签发、凭据核验和完整审计轨迹。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-ced5f590-321b-4879-9932-43b49d936f51-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-ced5f590-321b-4879-9932-43b49d936f51-arm64 linux/arm64
docker run -it benzhi-project-ced5f590-321b-4879-9932-43b49d936f51-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck`
