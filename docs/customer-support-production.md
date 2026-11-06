# 派大星规则客服

2026-11-06，正式商城挂载现有派总互动组件，独立服务 `zhixiang-support.service` 仅监听 `127.0.0.1:8085`，Nginx 将主站 `/api/v1/support/` 转发到它。商城业务后端保持 `20261105-173644`，客服不打开业务数据库，不修改订单、支付、采购或退款。

## 运行边界

- 正式启动只有 `-public-origin https://www.qzweb.cloud`，不传 `-allow-model`。离线启动不读取模型配置、不创建 Provider、不打开用量账本；公开模式和付费模型同时启用会拒绝启动。
- systemd 使用独立动态用户、只读文件系统、禁止访问业务目录，并限制网络为本机。没有上传本地模型密钥或业务库。
- 只接受回环来源及固定 Host，POST 必须具有精确正式 Origin；跨站请求被拒绝。会话 cookie 为 Secure、HttpOnly、SameSite=Strict，路径限于客服 API。
- 请求正文最多 4 KiB、消息最多 500 字符；会话每分钟 6 次、全局每分钟 20 次，最多 200 会话、2 个并发。网关另有每 IP 30 次/分钟、突发 12 次及并发 4 条限制。限流保持现有保守值，没有真实压力测试。
- 退款仅按虚构或用户提供的材料试算，实际售后由人工核实。联系人工按钮展示联系方式，不自动创建工单或发送消息。

## 商品资料

`scripts/sync_support_catalog.py` 只读取固定正式公开目录，生成内嵌 `internal/modules/supportagent/catalog-snapshot.json`。本次同步北京时间 2026-11-06 11:00，当前在售 26 款；价格和库存明确标为快照，下单以结算页为准。

旧 35 款快照保存在 `internal/modules/supportagent/testdata/catalog-20261026.json`，仅作为历史回归夹具，不进入当前推荐。下架 Claude、X、Grok 半价等不能被旧记录重新推荐。保留原有规则适用边界：成品号、新 Claude、Grok 和 X 不借用普通 GPT 的按时退款公式。

目录或政策更新后，执行同步、审阅生成差异、运行同步脚本及 Go 测试、重建客服程序，再通过独立服务发布。结构不完整、重复条目或无法识别的规则会停止同步；不要以商品标题推断质保，也不要恢复旧库存或售价。

## 发布与回退

商城及 Agent 发布 `20261106-110800`；Agent 位于 `/srv/zhixiang-support/releases/20261106-110800`。原业务后端和后台程序保持。备份 `/srv/zhixiang/backups/before-support-agent-20261106-110800` 保存原 Nginx 配置及基线；此前商城为 `/srv/zhixiang-front/releases/20261105-173644`。

回退时协调恢复商城 current 链接、备份中的商城 Nginx 配置，撤销本次 `cardnet-support-limits.conf`，执行 `nginx -t` 并平滑重载，然后停止并禁用 `zhixiang-support.service`。不回灌业务库、不回退 6.72 汇率。

发布必须保留旧散列资源及后台目录，并以新加坡实际 Nginx 用户 `www-data` 检查可读性。`systemctl reload nginx` 返回时新 worker 可能尚未接管；健康探测需要有上限的重试。本次第一次即时探测收到旧路由 404 后自动回退，补充生效等待后成功。

发布时基于当前工作区及 Agent 分支未提交成果集成，随后按用户授权将实际发布代码和记录归档到 main；本次归档不重新部署、不启用模型。不能将基础提交号当作完整发布源码版本。

可公开的发布清单与验收摘要见 [派大星发布记录](history/support-agent-release-20261106.json)，记录实际构建源码与产物的 SHA256、测试结果和回退目标。完整原始证据保留在本机忽略目录 `work/codex-runtime/support-release-20261106/`，其中 `manifest.json` 与 `source-evidence.tar.gz` 对应当时实际构建输入；不上传私有配置、账本、日志或数据库。Git 文本换行规范化单独记录，不将其误认为功能变化。
