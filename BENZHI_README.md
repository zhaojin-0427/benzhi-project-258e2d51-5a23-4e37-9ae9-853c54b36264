# BENZHI_README

基于 Go 实现的paperfit-release HTTP API 项目，一款后端服务，面向古籍修复机构的手工修复纸适配评估 HTTP 服务，完整实现材料登记、检测方案锁定、理化测量、模拟贴补、评审整改、结论冻结、领用凭据签发与哈希验真，并以可重放哈希链账本持久化业务事实。

## 项目说明
- 项目：benzhi-project-258e2d51-5a23-4e37-9ae9-853c54b36264
- 项目用途：面向古籍修复机构的手工修复纸适配评估 HTTP 服务，完整实现材料登记、检测方案锁定、理化测量、模拟贴补、评审整改、结论冻结、领用凭据签发与哈希验真，并以可重放哈希链账本持久化业务事实。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/paperfit -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-258e2d51-5a23-4e37-9ae9-853c54b36264-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-258e2d51-5a23-4e37-9ae9-853c54b36264-arm64 linux/arm64
docker run -it benzhi-project-258e2d51-5a23-4e37-9ae9-853c54b36264-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/paperfit -selfcheck -addr=127.0.0.1:19081`
