# paperfit-release

`paperfit-release` 是面向古籍修复机构的手工修复纸适配评估服务。它以一个古籍修复部位与候选纸批次为档案边界，贯通材料登记、检测方案锁定、理化结果采集、模拟贴补试验、技术评审、整改复测、结论冻结和领用放行凭据验真。

所有写接口都要求 `expectedVersion`、`Idempotency-Key`、`X-Actor` 和 `X-Role`，以实现乐观并发、稳定重试和角色授权。数据保存在 `-data` 指定目录中的 `events.jsonl` 哈希链账本与 `projection.json` 原子投影；启动时会验证账本并重放恢复。

## 构建

```text
go build ./cmd/paperfit
```

## 运行

服务默认只监听高位回环地址 `127.0.0.1:19081`：

```text
go run ./cmd/paperfit -addr=127.0.0.1:19081 -data=./data
```

也可设置 `PORT` 为端口号，使未显式传入 `-addr` 时监听 `127.0.0.1:<PORT>`。程序拒绝非回环监听地址。健康检查为 `GET /healthz`，业务 API 前缀为 `/api/v1`。

## 自检与测试

自检会建立真实回环监听器，通过 HTTP 依次完成建档、方案预演与锁定、六项检测、模拟贴补及复测修订、两轮评审、证据闭环、冻结、签发与验真，随后主动关闭并清理临时数据：

```text
go run ./cmd/paperfit -selfcheck -addr=127.0.0.1:19081
```

运行全部回归测试：

```text
go test ./...
```

## 主要角色与请求头

- `restorer`：创建档案、记录模拟贴补、提交评审和登记整改。
- `tester`：锁定检测方案并提交理化测量。
- `reviewer`：审核、冻结并签发领用凭据。

每个写请求使用唯一 `Idempotency-Key`。相同键与相同请求会返回首次结果；把同一个键用于不同内容会返回 `idempotency_conflict`。版本冲突响应包含当前档案版本，调用方读取最新档案后再决定是否重试。

## 扩展业务入口

- `GET /api/v1/suitability-cases/{caseID}/test-plan/preview?sampleCount=2`：生成无副作用的检测方案预演；锁定时把返回的 `previewDigest` 连同 `expectedVersion` 提交到 `test-plan`。
- `POST /api/v1/suitability-cases/{caseID}/measurements/batch`：用一个版本原子提交 1 至 60 条测量，并返回六项检测进度。
- `GET /api/v1/suitability-cases/{caseID}/trial-assessments`：查询模拟贴补修订链；复测提交使用 `supersedesAssessmentID`、`revisionReason` 和 `riskDispositions`。
- `GET /api/v1/suitability-cases/{caseID}/review-rounds`：查询全部送审快照，也可在路径末尾追加轮次号；审核请求必须绑定当前 `reviewRound` 与 `submissionDigest`。
- `GET /api/v1/suitability-cases/{caseID}/closure-matrix`：查询整改问题、同档案退回后证据及未满足条件；登记整改使用结构化 `evidenceReferences`。
- `GET /api/v1/suitability-cases/{caseID}/credentials/precheck`：签发前查看冻结范围与限制条款。验真接口可附加 `artifactRef`、`repairArea`、`paperLotID`，分别返回凭据状态与范围状态。
