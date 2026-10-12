# π星卡社开发约束

- 客服微信统一使用 `Cqz0924ttxk`，不能用电话号码作为微信号展示或回退值。

- `main` 是唯一正式源码主线，维护当前绿色 Go + Vue 商城，包含已确认的 π星卡社名称、派大星 Logo 和水母背景。用户端为 `frontend/user`，后台为 `frontend/admin`；这些品牌改动属于同一项目主版本。
- 支付、订单、数据库和采购后端部署在腾讯云新加坡 `43.134.141.10`，账号 `ubuntu`，目录 `/srv/zhixiang`。两套前端部署在上海 `124.220.2.69` 的 `/srv/zhixiang-front`：商城 `https://www.qzweb.cloud/`，管理端 `https://124.220.2.69:18888/login`。上海通过受限 SSH 通道连接新加坡回环 API；维护说明见 `ops/tencent/README.md`。
- 当前生产运行基准是新加坡发布 `20261012-153531`，构建源码为 `948c37f2d33420631eb77a75d3c1debe40d967b6`，使用 `-mode storefront`、`UPSTREAM_PURCHASE_ENABLED=true`。BEP20 与 Plasma USDT 收款均已启用；精确核验范围以 `ops/tencent/production-baseline.json` 为准，不把后续文档提交自动当作线上程序的 Git 版本。
- 本地使用 `scripts/local/start.ps1`、`status.ps1`、`stop.ps1`，运行库为 `work/codex-runtime/live/shop.db`，本地采购关闭。旧说明位于 `docs/history/`，不能按历史文档覆盖当前环境。
- 用户要求固定执行“先本地验证，再推送和上线”：先在日常本地入口验证实际运行程序、配置、相关数据和页面，保存验证记录；通过后才推送已验证提交并部署所需改动。后端改动须重建并重启本地后端，支付渠道及商品说明不能只在临时测试入口核验。上线后对照核验本地与生产；环境私有配置、订单和实时库存独立维护，不互相覆盖。
- 用户已授权真实客户确认付款后的自动采购发卡，仍禁止主动提交有效采购、转账，或将验收订单改为已付款进行扣款测试。测试使用模拟供应商和临时数据库。
- 未完成真实资金验收，不得把模拟测试通过描述成真实收款或采购验收。
- 库存由独立的 `zhixiang-stock-sync.timer` 每 300 秒只查询和同步。不要重复启动助手或为同步启动完整 worker。自动充值保持关闭。
- 后续升级保留 storefront 模式、支付及采购队列、持久化 Redis。先备份线上数据，不用本机数据库覆盖线上订单，也不套用旧 api 或 checkout 初始化脚本。
- 用户已授权跨会话读取、保存并复用服务器信息，2026-10-12 再次确认上海和新加坡使用此前提供的同一 `ubuntu` 登录信息。当前 C 盘工作区 `C:\Codex program\智享A社\deployment\local\` 中，新加坡凭据和指纹为 `tencent-cloud-server.credentials.json`、`tencent-cloud-known-hosts`，上海凭据和指纹为 `shanghai-front-server.credentials.json`、`shanghai-front-known-hosts`；登录均已验证，文件被 Git 忽略，上海密码文件仅当前用户和 SYSTEM 可访问。D 盘工作区原 `work/codex-runtime/tencent-cloud-server.credentials.json` 与 `tencent-known-hosts` 若存在仍可用于新加坡。新会话先检查对应目标的已保存文件，不因旧路径缺失而重复索要信息；仅在所有已保存入口缺失、凭据失效或目标不一致时询问。禁止输出密码。
- 当前上海前端发布为 `20261012-235241`，商城源码 `02c1a082c3b989adee398f16073e7ef8d89a4806`；后台沿用 `948c37f`，新加坡后端仍为 `20261012-153531`。首页弹窗已启用，公告版本 `bf3a1445`，文案快照 `docs/catalog/home-announcement.json`。客服微信统一为 `Cqz0924ttxk`，公告、首页客服区及全站页脚均不展示电话号码，复制内容已核验。只更新商城静态资源和公告配置，没有上传本机库或重启后端，见 `ops/tencent/production-baseline.json`。
- 凭据只保存在忽略的 `work/` 或 `deployment/local/`，限制本机文件访问权限。不得在源码、提交、对话、日志或构建产物中写入或回显密码、商户密钥、JWT、私钥或助记词；项目文档只记录私有文件路径和连接方式。使用已保存的 SSH 主机指纹核验服务器，指纹变化时停止连接并核查。
- 商户 1039 使用 `acg-faka`；密钥仅用于本地签名，不随请求发送。余额采购须持久化原子提交状态，未知结果进入人工核查，不能自动重新采购。
- 商品目录为 6 个分类、16 款商品，供货映射 12 款。`qzweb-55` 售价固定为 1160.00 元；库存及商户进价同步不能覆盖本站售价。
- 11 款 ChatGPT 卡密（渠道一、渠道二及 iOS）统一使用 `https://cdk.bar/` 自助充值，发货须附兑换链接和操作指引；配置见 `docs/catalog/qzweb-redemption-guide.json`。Gemini、DEMO 不套用此地址；iOS 原有覆盖充值与不叠加剩余时长的规则保留。
- 原始目录和用户价格覆盖保存在 `docs/catalog/`；同步脚本优先遵循价格覆盖，不能拿参考站库存替代商户库存。
- 表格和 Word 默认白底黑字、首行仅加粗；不使用彩色底色。其他交付物按任务需求选择样式，用户明确指定时优先遵循用户要求。

## 新会话入口与工作区边界

- 本机唯一日常主目录是 `D:\卡网`，默认主线为 `main`。新会话先读本文件和根目录 `README.md`，再核对分支、HEAD 与 `git status --short`；不能凭旧会话的目录名、旧截图或历史构建推断当前版本。
- 当前操作依据仅为根目录说明、`docs/local-development.md`、`docs/release-process.md` 和 `ops/tencent/`。`docs/history/`、`work/archive/` 与历史标签仅供明确请求的追溯或恢复，常规开发不要展开、搜索或作为启动和部署依据。
- 并行设计或功能草稿在独立 `codex/*` 分支及工作区中维护。未经用户要求合入，不把草稿文件复制到主目录，不将草稿称为主版本或线上版本；不得擅自清理其他会话正在使用的工作区。
- 不在主目录中新增第二套完整仓库、重复前台或名称相近的“最新版”目录。本机数据库、日志、构建与恢复资料只放在已有的忽略目录，不能提交 Git。
- 交付时分别说明主目录是否有未提交改动、是否与远端同步、是否实际部署。只有核验 `git status --short` 为空时才能称工作区干净；保留的运行数据和恢复备份不算另一套活动项目。
