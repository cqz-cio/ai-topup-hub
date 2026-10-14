# 上海前端与新加坡后端

2026-10-14 已清理含旧个人联系方式的 6 个历史商城脚本：文件移入上海私有备份，原链接由 [撤下清单](frontend-gateway/nginx-retired-assets.conf) 返回 `410 Gone` 与 `Cache-Control: no-store`，根域跳转后同样生效。规则源码 `b4dd8445e6cf67c9efdb3fc82fa35500ac312966`；后续发布、保留旧资源或回滚均须保留该清单，不能把隔离文件重新放回公开目录。

当前 448 个发布清单文件均未引用被撤下资源；清理后上海公开目录 4995 个文件、20 个公开接口响应及 234 项当前入口与资源均未检出旧号码。6 个旧链接经 WWW 与根域共 12 项带条件请求验证，均返回 410/no-store；真实浏览器确认旧链接失效、关于页及商品页客服微信正常，无页面脚本错误。本机另有 2 个未使用的嵌入式构建残留已移至私有隔离目录，新加坡当前程序和公开上传文件未检出该号码。

此为资源撤下与 Nginx 平滑重载，商城及后台程序仍为 `20261014-162234`，其余公开文件哈希保持一致；后端、Redis 进程、配置、商品和 SKU 价格/成本、支付渠道、站点及邮件设置与备份核对一致，库存 timer 正常。两地备份为各自 `backups/before-phone-cleanup-20261014-221615`；本机私有报告 `deployment/local/privacy-cleanup-20261014/verified-report.json`（项目外层目录）。未上传本机库或执行付款采购。服务器无法撤回第三方已下载或已缓存的历史副本。

2026-10-14 已将关于页面全部内容接入后台。进入 **系统设置 → 站点设置 → 关于我们 (About)**，修改后点击「保存更改」，刷新商城即可生效。42 个现有编辑字段包含服务指引、充值续费、代理合作、客服文字及相关入口，列表支持增删，三种语言独立维护。当前内容与样式保持一致，原有商品说明不变；独立设置键为 `about_page_config`，不连带保存注册、订单和其他站点设置。详见 [关于页面](../../docs/about-page.md)。

新加坡后端及上海商城、后台均为 `20261014-162234`，干净构建源码 `506c523bcec9a1cd9c656c0fe62b6d33e47afbcb`。商城 114 项、后台 60 项、相关 Go 回归 847 项及两套类型构建通过；日常本机重建后通过 18 项浏览器检查和 9 组数据比对，正式站 14 项浏览器及 6 项公网 HTTP 检查通过。覆盖 1920、390、320px；商城无横向溢出，后台 320px 原有顶部控件的 5px 溢出保留，关于页表单可正常使用。

切换时 35 组业务数据一致；保存内容后，商品描述与售价、SKU 售价与成本、支付渠道、个人及会员价、其他设置六组比对一致，仅新增关于页配置。448 个发布文件、4711 个保留旧资源及 54 个 Nginx 文件核验通过。Redis 未重启，storefront、采购队列和库存 timer 保留，未上传本机库或执行付款采购。

备份 `/srv/zhixiang/backups/before-about-settings-20261014-162234`、`/srv/zhixiang-front/backups/before-about-settings-20261014-162234`；私有证据 `work/codex-runtime/about-settings-20261014/verified-report.json`。新加坡链路低速时使用既有 BSDIFF40 差分和非流水线 SSH 传输，本机和服务器还原后的完整程序 SHA256 均与干净构建一致，主机指纹验证保持启用。

## 此前后端发布：上游成交金额对账
2026-10-14 已上线 [上游成交金额对账](../../docs/amount-reconciliation.md)。新加坡后端 `20261014-155607`，干净源码 `6331d68873009d366994b5a1f1012368eee939c2`；上海静态保留并行开票任务发布的 `20261014-160123`，商城来源 `14e28d402b205ef9fb94a94ba7c1f30a0a76a147`，后台来源 `08cc45c4ab9b14561d48dcc350428cebc338c5d7`。先通过商户签名验证原订单归属，再从同站订单状态接口读取成交金额；金额、全量对账均已对 3 笔原有采购单完成核验，均为 3 笔一致、0 差异。查询失败、编号/状态/币种不符或金额非法仍不计为匹配成功，发卡查询不依赖金额接口。

903 项相关 Go 回归（含子测试）及两套类型构建通过，日常 8080 后端已重建，8 项本机接口检查与实际后台页面核验通过。原有本地财务声明未确认，未代签；完整流程在临时 SQLite 库和模拟上游验证。上线切换时 35 组业务数据与全部配置一致，Redis 未重启；四条支付渠道、售价、采购队列及库存 timer 保留。正式新增 2 条对账审计任务，未创建订单、采购、付款或上传本地库。核验保留 3552 个旧商城资源、969 个后台文件及 54 个 Nginx 文件；商城首页和新增资源与并行开票任务的 328 个发布文件清单一致，本次未发布上海资源。

备份 `/srv/zhixiang/backups/before-amount-reconciliation-20261014-155607`；私有证据 `work/codex-runtime/amount-reconciliation/verified-report.json`。直连 SSH 中断期间使用已授权上海 SSH 跳转、原凭据和固定指纹维护，未改变安全策略。因链路低速使用 BSDIFF40 增量传输，本机及服务器还原后的完整程序 SHA256 均与干净构建一致。退款净额和人工余额调整仍需上游账单，本次核验的是原订单成交总额。

## 此前商城发布：开票提示

2026-10-14 已上线[付款后联系客服开票](../../docs/invoicing.md)。上海静态发布 `20261014-160123`，商城干净构建源码 `14e28d402b205ef9fb94a94ba7c1f30a0a76a147`；后台保留 `08cc45c4ab9b14561d48dcc350428cebc338c5d7`，新加坡程序仍为 `20261014-150947`。商品购买按钮下显示支持开票；已付款的会员及游客订单提供客服微信和订单号复制。购买须知文章与首页公告已于后续内容更新补齐开票及 TRON 网络说明，当前公告版本 `f2340cb5`。

108 项商城测试及类型构建通过。日常本机商品页与正式商品页通过 1920、390、320px 检查；订单的六种已付款状态、四种未付款或关闭状态、会员/游客、经典/Vault、三种语言及复制按钮使用本地模拟订单验证。正式站 5 项浏览器检查、8 项服务器 HTTPS 资源检查通过，328 个商城文件和 969 个保留后台文件核对一致。未创建正式测试订单或进行真实付款、采购、开票验收。

两地备份为各自 `backups/before-invoice-notices-20261014-160123`；私有报告、截图和构建清单在 `work/codex-runtime/invoice-preview-20261014/`。后端及 Redis 的程序和进程、配置、站点设置、支付渠道、商品价格、个人单价和会员等级与备份一致；Nginx 配置未变，旧散列资源及商品图保留。本次新加坡 SSH 直连在握手前关闭，改经上海临时 SSH 转发，并继续核验既有新加坡主机指纹；未变更凭据或访问控制。

## 当前内容更新：支付网络说明

2026-10-14 已补齐公告正文、公告摘要、首页弹窗和服务条款共 4 处客户可见文案，统一列出 TRON（TRC20）、BEP20（BNB Smart Chain）和 Plasma（USDT0），以结算页可选渠道为准；保留开票、客服微信及兑换指引。内容源码 `d7d1f94e36617e87cfff133a97190455f3f8a5ea`，公告版本 `f2340cb5`。同步修订钱包充值、付款金额、到账提示、购买流程、自建兑换边界和微信手续费说明中的当前操作文案；历史发布验证范围保持原记录。

本机与正式站各 12 项浏览器检查通过，覆盖 1920、390、320px、公告列表、正文、弹窗关闭刷新及条款刷新；无脚本错误或横向溢出。既有支付提示与钱包解析的 8 项测试通过。66 组数据库表核对通过，除目标文案外业务数据和配置一致（供货连接仅排除自动心跳字段）；支付渠道、售价、程序、私有配置与凭据摘要、后端和 Redis 进程及库存 timer 保持原状态。仅更新内容，后端仍为 `20261014-162234`。备份 `/srv/zhixiang/backups/before-tron-network-copy-20261014-165251`，私有证据 `work/codex-runtime/tron-content-audit-20261014/`。未上传本机库、提交正式验收订单或执行转账采购。

## 上次完整发布：TRON USDT

2026-10-14 已上线 TRON（TRC20）USDT，正式渠道 ID 4，支持游客/会员商品付款及钱包充值。收款地址 `TQmtYPYUySY1ABLTdtcGMFEyTtzFjAFUpn`，仅识别官方 USDT 合约 `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`，精度 6，汇率 `6.649820455`、确认数 15。十款已支持 USDT 且显式限制渠道的商品在原列表追加 TRON，原微信选择范围、售价、成本及其他商品字段保留。

新加坡后端及上海两套前端发布均为 `20261014-150947`，干净构建源码 `08cc45c4ab9b14561d48dcc350428cebc338c5d7`。1659 项相关 Go 回归（含子测试，1 项跳过）、商城 108 项、后台 60 项及两套类型构建通过；日常本机验证 9 笔未付款订单/充值单、二维码、金额复制、刷新与扫描游标，临时记录已清理。完整 Go 测试仍有 Windows 文件锁、CGO SQLite 与 Linux 自更新测试环境限制，不声称全库全部通过。

正式 445 个发布文件、4233 个保留文件、54 个 Nginx 文件、15 项服务器 HTTPS 检查及 1920/390/320px 结算页面通过，无脚本错误或横向溢出。两个 TRON 只读监控接口验证成功；尚无正式 TRON 支付单，正式游标未初始化。Redis 未重启，storefront、采购队列、库存 timer 和原三条渠道保留。并行的手续费提醒及兑换返回修复已包含。本地财务声明未代签，未上传本机库、提交正式验收订单、转账或有效采购；真实资金验收仍未完成。

后端备份 `/srv/zhixiang/backups/before-tron-usdt-20261014-150947-retry2`（首次启用前备份无 `-retry2` 后缀），前端备份 `/srv/zhixiang-front/backups/before-tron-usdt-20261014-150947`，商品渠道范围备份 `/srv/zhixiang/backups/before-tron-product-channels-20261014-1525`。私有报告与截图 `work/codex-runtime/tron-usdt/verified-report.json` 及同目录文件。首次发布因权限快照和公开配置旧缓存触发回退；最终按权限语义比对并只刷新主站公开配置缓存后通过。

详见 [TRON 支付接入](../../docs/tron-usdt-payment.md)。以下保留此前各功能发布记录。

此前上海静态由并行的兑换返回修复更新为 `20261014-151124`（源码 `d2cbc34162744f6fb061524041464c4ec6718903`），完整保留 U 提醒；相关源码、445 个发布文件哈希及两项正式页面复查已通过。本次提醒的独立发布与完整验证记录如下。

2026-10-14 已上线[功能状态与任务修复](../../docs/feature-availability.md)。新加坡后端发布 `20261014-145155`，后台及程序干净源码 `b6dcc7d1a99b3ed91b1cfeb9b468642984bcd042`；上海静态最终发布 `20261014-151124`，商城源码 `d2cbc34162744f6fb061524041464c4ec6718903`，后台资源保持上述来源。分销到期结算、推广佣金确认和对账由 storefront 消费；未开放的返利、兑换、群发、系统二级域名与定时巡检按实际状态展示，库存继续由独立 300 秒 timer 管理。供货商未返回金额时，对账明确标记未完整核验，可单独核对状态。兑换关闭页使用完整页面跳转返回商城。

1992 项 Go 回归通过（含子测试，3 项外部测试跳过），合并主线后商城 107 项、后台 60 项及两套类型构建通过。日常本地后端重建，9 项接口、14 项页面检查通过；现有本地库没有用户及分销商，财务声明未确认，相关场景使用临时数据库回归，不代签声明或新增管理员。正式站 22 项浏览器、10 项公网、9 项后端检查通过，覆盖 1920、390、320px；原定价下拉框导航也已复查。

切换时 35 组业务数据和全部私有配置与备份一致，Redis 未重启，支付渠道、售价、采购队列和库存 timer 保留。核验 445 个前端文件、4045 个保留文件及 54 个 Nginx 文件。备份 `/srv/zhixiang/backups/before-feature-availability-20261014-145155`、`/srv/zhixiang-front/backups/before-feature-availability-return-20261014-151124`，首次前端备份 `/srv/zhixiang-front/backups/before-feature-availability-20261014-145155`；私有报告 `work/codex-runtime/feature-availability/verified-report.json`。未上传本机库、提交正式验收订单、付款采购、提现或发送通知。

2026-10-14 已上线[U 支付到账与手续费提醒](../../docs/usdt-transfer-notice.md)。上海商城发布 `20261014-150333`，源码 `e10040f8ae2835ab1487502ad8d5edb5831dffea`；保留并行任务已部署的新加坡后端 `20261014-145155` 与后台源码 `b6dcc7d1a99b3ed91b1cfeb9b468642984bcd042`。所有 U 商品付款和钱包充值提醒客户另付手续费，并保证钱包实际到账与支付单完整数量一致。

107 项商城测试、类型构建通过；日常本机、实际构建和正式站各 37 项浏览器检查通过。325 个商城文件、875 个后台文件、54 个 Nginx 文件及 28 组业务数据/配置核对通过。后端和 Redis 未重启，未提交正式验收订单或进行真实资金测试。备份为两地各自 `backups/before-usdt-fee-notice-20261014-150333`；私有证据 `work/codex-runtime/usdt-fee-notice/verified-report.json`。

2026-10-14 已上线[关于页面与充值说明](../../docs/about-page.md)。该次上海静态发布 `20261014-143305`，商城干净构建源码 `0fa08784dde3e748e14114e9a6119778532ebf8b`；后台资源和新加坡后端保留 `1be05e228bc34336cfc86e7c7c5558ac3dac5609`，后端仍为 `20261014-105629`。主站 `/about` 展示已确认的第二版内容和布局，原有商品说明与例外规则保留。

先完成日常本机页面验证、104 项商城测试和类型构建，再推送部署。上线后核验 1920、390、320px、复制微信反馈、须知/法律入口、返回与刷新；16 项公网、7 项服务器页面检查通过。324 个商城文件、781 个保留后台文件及 54 个 Nginx 文件一致，28 组业务数据及配置与备份一致，后端和 Redis 进程未重启。

两地备份为各自 `backups/before-about-page-20261014-143305`，私有证据 `work/codex-runtime/about-page-release/verified-report.json`。本机缺失的既有公开公告和法律文案从核实后的快照补回；正式内容数据未写入，未上传本地库、创建订单或执行付款采购。源码中的后续发布记录提交不改变实际构建来源。

2026-10-14 已上线[分销定价下拉框导航修复](../../docs/dropdown-scroll.md)。该次上海静态发布 `20261014-112316`，商城干净源码 `cad9c4020d7a70240ffa3c9ec853349c28c27971`；后台和新加坡程序保留 `1be05e228bc34336cfc86e7c7c5558ac3dac5609`，后端发布仍为 `20261014-105629`。打开下拉框时，正文滚动位置、sticky 顶栏和左侧导航不再跳动或消失。

先通过日常本机 11 项浏览器检查、104 项商城测试和类型构建，再推送并部署。正式 8 项浏览器、14 项公网、6 项服务器页面检查通过，覆盖 1920、390、320px，无脚本错误；323 个商城文件、781 个保留后台文件及 54 个 Nginx 文件核对一致。28 组业务快照和配置与备份一致，后端、Redis 进程与库存 timer 保留，分销申请继续开启。

两地备份为各自 `backups/before-dropdown-scroll-20261014-112316`，私有记录 `work/codex-runtime/dropdown-scroll/verified-report.json`。仅替换商城静态资源，保留旧散列资源、商品图片和后台资源；未保存验收定价规则、上传本机库或执行付款采购。

2026-10-14 已统一并开启[分销申请](../../docs/reseller-applications.md)。该次新加坡后端与上海商城、后台均为 `20261014-105629`，同一干净主线源码 `1be05e228bc34336cfc86e7c7c5558ac3dac5609`，后端 SHA256 `a898764f5250a2c711f9a673ff40589bb38f57ebd9f9c19ad42e68165f90b645`。后端配置新增 `reseller`，启用功能和用户申请；商城按 `can_apply` 展示申请表，后台显示真实状态。审核规则保留，未配置自动二级域名。

先通过日常本机的 25 项接口及浏览器检查，再推送、构建和部署。相关 Go 回归 1853 项通过（含子测试，3 项外部测试跳过），商城 104 项、后台 58 项及两套类型构建通过。正式站 10 项公网、9 项后端入口与鉴权、15 项服务器页面及资源检查通过；442 个新资源、2905 个保留旧文件和 54 个 Nginx 文件核验通过。实际已登录账号验证申请可用，并只读观察到运营者随后提交、审核通过的新记录；1920、390、320px 展示正常，无脚本错误。

切换时 28 组业务数据与一致性备份相同，只增加分销运行配置；之后运营者创建的申请与审批记录正常保留。两地备份分别为 `/srv/zhixiang/backups/before-reseller-applications-20261014-105629`、`/srv/zhixiang-front/backups/before-reseller-applications-20261014-105629`，私有报告为 `work/codex-runtime/reseller-application-release/verified-report.json`。Redis 未重启，storefront、支付采购队列、库存 timer、三条收款渠道及微信手续费保留；未上传本地库或代客户提交、审批、付款采购。

2026-10-14 此前已上线[应用层防刷与资源保护](../../docs/abuse-protection.md)。该次新加坡后端为 `20261014-085600`，干净主线构建源码 `993fe972efc8b359ca82e1c21f8272303baea751`，程序 SHA256 `1d624247f1e1080b525a06111395ec4ccf35978c6eb3d3914ca61800f1fa463c`。该次上海商城/后台保持 `20261013-174612`，未替换静态资源或 Nginx。

1951 项 Go 回归（含子测试，3 项外部测试跳过）、商城 103 项、后台 58 项测试及两套类型构建通过；日常后端重建后，14 项接口与实际页面核验通过。生产 11 项隔离防护和 8 项公网检查通过，真实浏览器确认注册图片验证码、登录表单、后台会话、三条收款渠道及微信 1.60% 手续费正常。

备份 `/srv/zhixiang/backups/before-abuse-protection-20261014-085600`；服务器报告 `shared/reports/abuse-protection-release-20261014-085600.json`，本机证据 `work/codex-runtime/security-release/verified-report.json`。切换前后 19 组业务数据及配置一致，仅供货连接自动心跳时间不参与比较；2910 个上海前端文件与 54 个 Nginx 文件保持一致。Redis 未重启，storefront、支付/采购队列及库存 timer 保留，自动充值关闭。未上传本机库、真实发信、创建验收账号或执行付款采购测试；应用层验证不代表带宽型 DDoS 或极限 QPS 验收。

2026-10-13 已上线 [付款页返回商品](../../docs/payment-navigation.md)：浏览器返回时，单商品订单跳到对应商品详情，多商品跳到列表。上海静态发布 `20261013-174612`，商城构建源码 `e249905d2e75d0d6e87c576367bc80ae62e25fe8`，后台沿用 `1f4584c19123f4c187403be7ce22db8cf3ed167c`；新加坡后端仍为 `20261013-172211`（`9020d261d49e3b7a706125708beafa3c2e4823af`）。

103 项商城测试、类型构建、本机与正式站各 30 项浏览器检查通过，覆盖 1920、390、320px、游客/会员、购物车、快速购买、经典/Vault 模板，以及刷新、前进返回和失败重试。正式站 323 个本次商城文件与 687 个保留后台文件哈希一致，Nginx、后端与 Redis 进程保持原状态。18 组业务数据和配置与切换前备份一致；供货连接只排除自动更新的 `last_ping_at`、`updated_at`，其余配置逐列比对。实际接口仅用于读取商品、配置及预估金额，订单创建和支付在测试浏览器内模拟。

两地备份分别为 `/srv/zhixiang/backups/before-payment-return-20261013-174612` 和 `/srv/zhixiang-front/backups/before-payment-return-20261013-174612`，私有报告与截图在 `work/codex-runtime/payment-return/`。本次只替换商城静态资源及发布清单，保留旧散列资源、商品图片、三条付款渠道、订单和余额，未上传本机库或执行真实付款采购。后续文档提交不代表新的程序构建。

2026-10-13 已上线 [XorPay 微信支付](../../docs/xorpay-wechat.md)：渠道 ID 3 支持商品付款和钱包充值，客户手续费 1.6% 向上取分。该次新加坡发布 `20261013-172211`，干净构建源码 `9020d261d49e3b7a706125708beafa3c2e4823af`，二进制 SHA256 `8640e3d7829b7885c875c18a37459f2f8b39b4cbd92f5901d504cd0bd7c1feda`。该次上海最终静态发布 `20261013-173020`，前端构建来源 `1f4584c19123f4c187403be7ce22db8cf3ed167c`，其前端树与后端提交一致。

实际 XorPay 出码、98 项商城及 58 项后台测试、两套类型构建、支付/钱包/订单/架构/容器/模拟采购回归通过。日常本地已重建重启；用户明确要求不确认本地声明而直接配置正式版，本地未写微信渠道，不能称本地支付页面全流程通过。正式已有财务声明确认，本次未重新签署。正式后台启用状态及结算页 38.00 元加收 0.61 元验证通过，切换 USDT 不加收，未提交正式订单或付款采购。

首次上海静态包后台 HTML 的 `__DJ_ADMIN_BASE__` 占位符造成页面空白，已在 `20261013-173020` 渲染为 `<base href="/">`；441 个文件与实际入口引用的 JS/CSS 响应核验通过。将 fullstack 资源用于独立静态后台时，必须渲染此占位符，并验证浏览器加载；不能只核对 HTML 的 200 状态。旧散列资源、商品图及 Nginx 配置保留。

上线后 18 组业务记录/价格/配置与切换前一致，包括订单、钱包、支付、采购、专属价、供货、公开售价与成本及原两条 USDT 渠道；仅新增微信渠道。Redis 未重启，stock timer 与 storefront 模式保留，自动充值关闭。两地备份均为各自 `backups/before-xorpay-20261013-172211`，上海补充 `backups/before-xorpay-admin-base-20261013-173020`。私有证据 `work/codex-runtime/xorpay/`，未上传本机库；真实付款、到账回调和采购发卡资金验收仍未完成。

以下按功能保留此前发布记录；当前后端与两套前端以本页最新记录及 `production-baseline.json` 为准。

2026-10-13 已上线 [USDT 两位付款数量规则](../../docs/usdt-payment-amounts.md)：BEP20 和 Plasma 的新支付单按人民币金额除以锁定汇率，向上取到两位小数；金额已占用时，以 0.01 USDT 为步长选择最小可用金额，最多增加 0.99 USDT。历史金额永久占用，旧六位支付单继续按原金额识别。该次新加坡后端发布 `20261013-171054`，构建源码 `414a1fe84ba56adea03c00e3af97bb03d96cd23e`，程序 SHA256 为 `14ef8b1747418b70734ed2c6ab2d80249278146ddd46352120e5310cd4f13b55`。

支付、钱包、模拟采购和架构回归通过；本机日常入口重建后验证两条网络共 6 笔未付款订单/充值单、最小金额分配、浏览器展示与复制及刷新复用，临时记录已清理。两套类型构建和干净源码 Linux fullstack 构建通过。线上历史支付单、订单、钱包、专属价、公开售价与成本、私有配置保持切换前状态；收款地址为 `0xA265E6d58B56757aEC3EB3a16b9Edf41De79A26F`，汇率为 `6.649820455`。Redis 未重启，库存 timer 保留。上海 `20261013-165016` 的 2437 个文件及 Nginx 摘要核对一致，正式商品页只读检查通过。

备份 `/srv/zhixiang/backups/before-usdt-cent-20261013-171054`；服务器报告 `/srv/zhixiang/shared/reports/usdt-cent-release-20261013-171054.json`，本机私有记录 `work/codex-runtime/usdt-cent/`。未上传本机库、创建正式测试订单或执行真实转账和采购。

上海静态发布 `20261013-165016`，商城源码 `e6c56be1a964d6c3ef98220c25e60775d7b997a6`；仅更新注册发送验证码成功后的提醒弹窗，提示查收邮箱及垃圾邮件/广告邮件文件夹，关闭后聚焦验证码输入框。97 项商城测试、类型构建、本机与正式站各 16 项浏览器检查通过，覆盖 1920、390、320px、发送成功/失败、重发、键盘操作及三种语言；对照已批准预览核验。322 个商城文件和 588 个保留后台文件哈希核验通过。 见[注册邮箱验证码发送提醒](../../docs/registration-email-reminder.md)。

两地备份均为各自 `backups/before-email-code-popup-20261013-165016`；证据在忽略目录 `work/codex-runtime/email-code-popup-20261013/`。该次前端发布时，新加坡后端为 `20261013-162754`，后端及 Redis 进程、站点设置、私有配置和 Nginx 配置保持一致。邮箱发送成功与失败在测试浏览器内模拟，未实际发送验收邮件、创建账号或执行付款采购。

上一版新加坡后端为 `20261013-162754`，程序源码 `6770a90ad543240e5c208f60f4916a9be8d7631b`，已上线[验证码邮件网址与有效期](../../docs/verification-email.md)：主站邮件使用 `https://www.qzweb.cloud`，三种语言均显示与实际配置一致的有效期（当前 10 分钟）。程序 SHA256 为 `f63c2aac81dc9eae36324cd51f61fbf86a9afe2e1bc85a8886e3a75639034249`。该次后端发布保留当时上海 `20261013-160811`，2245 个商城和后台文件哈希一致。备份 `/srv/zhixiang/backups/before-verification-email-20261013-162754`；17 组业务数据、私有配置和支付设置保持原值，Redis 未重启。91 项 Go 测试及本地三语模拟邮件、正式站只读页面检查通过；未发送真实验收邮件或进行付款采购测试。

2026-10-13 已上线[成交与上游余额邮件](../../docs/sale-email.md)。新加坡后端 `20261013-162050` 从干净提交 `2bbb4e0b802114dd3bda394b28cddcf83639228a` 构建，二进制 SHA256 为 `98b2be670d483fff557f04b2d9b321e2e95bc32183873d42f925f5f874339185`。上海 `20261013-160811` 的 2247 个文件及 Nginx 配置未变。现有 Gmail SMTP 已接受一封含真实只读余额与示例订单的测试邮件，发送日志 ID 1 成功；用户指定的收件地址仅保存在环境通知设置中。

发布前备份 `/srv/zhixiang/backups/before-sale-mail-20261013-162050-retry`，配置仅新增通知中心经营者邮件、订单支付场景和合并模板，客户订单邮件仍关闭。订单、采购、钱包、价格与成本、支付渠道、个人价、供货密钥和其他设置均与本次切换前一致；Redis 没有重启，库存 timer 正常。未进行真实付款采购测试。第一次切换因旧的 1160.00 价格断言自动回退；实测升级前和当前 `qzweb-55` 基础标价均为 1200.00，重试按实际数据快照核验且未调价。证据在忽略目录 `work/codex-runtime/sale-mail/`，服务器报告 `shared/reports/sale-mail-final-20261013-162050.json`。


π星卡社的[商城](https://www.qzweb.cloud/)和[后台](https://124.220.2.69:18888/login)前端运行于上海 `124.220.2.69`，目录 `/srv/zhixiang-front`。新加坡 `43.134.141.10` 的 `/srv/zhixiang` 保留 Go 后端、生产数据库、持久化 Redis、支付监听、采购交付和库存定时任务。

2026-10-12 16:30 完成 `www A 124.220.2.69` 的腾讯云身份验证及解析保存，根域 `@` 解析未修改。16:53 核验正常域名访问为新版 π星卡社，HTTPS 链与域名验证通过；管理端使用独立 IP 端口，`www` 下的 `/zxa-management/` 返回 404。旧新加坡 HTTP 入口仍可用于兼容，但日常登录和访问使用上海 HTTPS 入口。

## 上海入口维护

2026-10-13 已上线[注册图形验证码](../../docs/registration-captcha.md)：商城发布 `20261013-160811`，源码 `c0d5e48c1407261e5d800969c228f03a644732d7`，注册页明确区分图形验证码与邮箱验证码。通过既有后台 API 仅启用图片提供方和注册发送邮件场景；图形校验通过后才发送邮件，创建账号仍需邮箱验证码。其他验证码场景保持关闭。

97 项商城测试、类型构建及验证码/注册相关 Go 回归通过，日常本机 13 项和正式站 11 项浏览器检查覆盖 1920、390、320px 及三语。核验 322 个新商城文件、588 个保留后台文件；旧散列资源、商品图、Nginx 配置和后端、Redis 进程保留。售价、成本、支付渠道、个人专属价及非验证码配置一致。两地备份为 `/srv/zhixiang/backups/before-registration-captcha-20261013-160811`、`/srv/zhixiang/backups/before-registration-captcha-settings-20261013-160811`、`/srv/zhixiang-front/backups/before-registration-captcha-20261013-160811`，核验记录在忽略目录 `work/codex-runtime/register-captcha-preview-20261013/`。未上传本机库、创建正式测试账号、发送真实验收邮件或进行付款采购测试；成功发送后的刷新状态以浏览器模拟响应验证。

2026-10-13 15:38 已按批准的 B 方案上线[后台商品缩略图修复](../../docs/admin-product-thumbnails.md)：100×60 完整横图、加载失败占位，并恢复后台对商城 `/catalog-products/` 及 `/products/` 图片文件的访问。上海发布 `20261013-153730`，源码 `471ded02e1b265520a53485288bdcc4f47dbece9`；只替换后台资源及后台 Nginx 配置并平滑重载。新加坡程序 `20261013-114011` 与商城资源来源保持原基准。

本机 58 项后台测试、类型构建、50 项 HTTP 和 7 项实际浏览器检查通过；正式 49 项服务器检查、9 项浏览器检查通过，40 个商品图片地址及 44 条列表记录均正常，119 个后台文件和 1468 个保留商城文件核验通过。320px 且有系统滚动条时，原顶部控件有 5px 溢出，已记录；本次未改顶栏布局。两地备份分别为 `/srv/zhixiang/backups/before-product-thumbnails-20261013-153730`、`/srv/zhixiang-front/backups/before-product-thumbnails-20261013-153730`，报告为上海 `shared/work/product-thumbnails-20261013-153730.json`，本机证据位于 `work/codex-runtime/product-thumbnails/`。售价、成本、商品图地址、支付与站点配置、个人专属价及后端和 Redis 进程与切换前备份一致，未上传本机库、修改业务数据或进行付款采购测试。

2026-10-13 14:43 已修复商品列表直接访问的 403 和服务器错误页残留旧图标。`/products` 与商城静态图片目录同名，原 `try_files $uri $uri/ /index.html` 优先命中目录并触发禁止目录列表；现改为 `try_files $uri /index.html`，真实图片仍直接读取，商品列表及带尾斜杠、查询参数的地址交给 Vue 路由。根域继续跳转到 WWW。

2026-10-13 14:52 已补齐后台错误页标签图标的响应类型修复，配置源码 `7a669ccbf6d6165a26daf82e5a4aa3cb06e52ff9`：缺失 `.js`、`.css`、`.png` 等资源时，命名错误位置清空扩展名 MIME 映射并明确返回 HTML，浏览器才能解析图标声明。错误状态码、版本化派大星图标、API 错误透传保留。本机 37 项 HTTP 和 22 项浏览器检查、正式站 16 项浏览器和 12 项服务器 HTTP 检查通过；本机覆盖六类错误状态，正式站只读验证 403/404、登录页、浏览器内模拟接口失败和失效登录，未制造正式服务中断。两地备份均为各自 `backups/before-admin-error-icon-20261013-145235`；报告 `/srv/zhixiang-front/shared/work/admin-error-icon-20261013-145235.json`，本机证据 `work/codex-runtime/admin-error-icon/`。仅更新共享错误片段并平滑重载 Nginx，现有 1960 个前端文件及程序发布、后端与 Redis 进程、价格和收款配置保持一致。

商城和后台共同加载 [错误页与图标配置](frontend-gateway/nginx-errors.conf)，安装位置 `/etc/nginx/snippets/qzweb-errors.conf`。默认 `/favicon.ico` 直接返回现有派大星 PNG，明确图片类型并要求重新校验缓存；403、404、500、502、503、504 错误页在 HTML 中指定带版本的图标地址，保留原状态码和错误正文结构，不依赖 Vue、API 或旧图标缓存。后端 API 错误响应不拦截。以后安装或更新 [商城配置](frontend-gateway/nginx-www.conf)、[后台配置](frontend-gateway/nginx-admin-ip.conf) 时，须先安装这一共享片段，再执行 `nginx -t` 和平滑重载；若更换默认 Logo，同时更新错误页图标 URL 的版本值。

配置源码 `9fd00866eb655b0da9e95c8fdaca20152c73911e` 先通过本机 Nginx 1.24 的 35 项检查及日常 `5173/5174/8080` 入口核验，再推送部署。正式服务器 15 项、公网 16 项 HTTP 检查通过；实际浏览器复查商品列表、刷新、403 与 404 页面的图标引用。新加坡一致性备份 `/srv/zhixiang/backups/before-favicon-route-20261013-144241`，上海配置与证书备份 `/srv/zhixiang-front/backups/before-favicon-route-20261013-144241`，服务器报告 `/srv/zhixiang-front/shared/work/favicon-route-20261013-144241.json`，本机报告和截图在忽略目录 `work/codex-runtime/favicon-repair/`。仅安装三份 Nginx 配置并平滑重载：上海发布仍为 `20261013-143008`，现有 1960 个前端文件哈希不变；新加坡程序仍为 `20261013-114011`，后端、Redis、库存定时器及 storefront 模式正常。公开售价与成本、专属价格、两条支付渠道、站点设置与私有配置均与部署前备份一致，未修改业务数据或执行付款采购测试。

2026-10-13 已修复不带 `www` 的根域进入家具页面的问题。新增 [根域入口配置](frontend-gateway/nginx-apex.conf)，实际安装于上海 `/etc/nginx/conf.d/qzweb-apex.conf`：`http://qzweb.cloud` 与 `https://qzweb.cloud` 均以 308 跳转到 `https://www.qzweb.cloud`，保留原路径和查询参数。根域使用独立的有效证书，续期配置位于 `/srv/zhixiang-front/shared/letsencrypt/renewal/qzweb.cloud.conf`，由现有 `qzweb-certificates.timer` 管理；HTTP 的 ACME 验证目录仍为 `/var/lib/letsencrypt`。DNS 没有修改。

原因是当前 Nginx 只加载 `/etc/nginx/conf.d/*.conf`，此前已加载的商城配置只声明 `www.qzweb.cloud`，根域请求落入 80 端口的默认家具站 `/etc/nginx/conf.d/oakved.conf`，其页面目录为 `/opt/oakved/frontend/furniture`、标题为 `RH Layout Extraction Preview`。该家具配置仅声明服务器 IP，没有显式绑定 `qzweb.cloud`。旧 `/etc/nginx/sites-enabled/qzweb.cloud` 指向另一套旧站配置，当前没有被加载，不应据此判断正在运行的入口。

追溯证据均按北京时间记录：腾讯云控制台已有记录表显示根域 `@ A 124.220.2.69` 的最后操作时间为 **2026-06-02 10:50:12**，`www` 为 **2026-10-12 16:30:35**；检查时控制台要求重新登录，以上取自已显示记录表，并非新查询的操作审计。服务器家具首页文件创建于 **2026-07-22 09:03:15**，当前 `oakved.conf` 的创建及修改时间均为 **2026-09-21 09:46:32**。这些证据分别说明解析记录和文件时间，不能认定有人在其中某一时刻单独把根域绑定到家具站，也不能由此确定操作者。

配置源码 `2774d53935c098d9a1c86b90a425ca826e91bdc3` 先通过本机 Nginx 1.24 的 6 项 HTTP/HTTPS 跳转、1 项 ACME 入口及日常 `5173/5174/8080` 检查，再推送上线。正式公网 9 项检查覆盖根域两种协议、路径和参数、原 `www` 入口及管理端，实际浏览器也从根域进入 π星卡社。新加坡一致性数据库备份为 `/srv/zhixiang/backups/before-apex-domain-20261013-135130`；上海 Nginx 与证书备份为 `/srv/zhixiang-front/backups/before-apex-domain-20261013-135130`。服务器报告为 `/srv/zhixiang-front/shared/work/apex-domain-20261013-135130.json`，本机证据在忽略目录 `work/codex-runtime/domain-diagnostic/` 和 `work/codex-logs/`。仅增加根域配置及证书并平滑重载 Nginx；家具站与原商城配置哈希保持一致，程序仍为 `20261013-114011`，未替换应用、修改业务数据或进行付款采购测试。

2026-10-13 00:34 按用户要求补齐注册页链接对应的[隐私政策](https://www.qzweb.cloud/privacy)和[服务条款](https://www.qzweb.cloud/terms)。文案快照为 [site-legal.json](../../docs/catalog/site-legal.json)，配置来源 `9b260f889420d8c386c6c2c20d76c27e1630a94d`；通过既有后台 API 仅更新 `site_config` 中的 `legal.privacy.zh-CN`、`legal.terms.zh-CN`。后台 **系统设置 → 站点设置 → 法律条款** 可继续编辑。正文覆盖邮箱注册、订单查询、USDT 核验、卡密交付、具体商品质保与退款、信息收集与存储、个人信息权利及客服微信 `Cqz0924ttxk`；兑换入口遵循订单说明。

先在日常本地入口保存正文、核对实际页面，再推送文案并更新正式配置；本机与正式站各通过 1920、390、320px 共 9 项只读页面检查，两页各 9 节完整显示，注册页两个链接实际点击、刷新保留、未自动勾选协议、无横向溢出和脚本错误均通过。没有注入测试配置。线上备份 `/srv/zhixiang/backups/before-site-legal-20261013-003435`，报告 `/srv/zhixiang/shared/reports/site-legal-20261013-003435.json`；本机报告与截图位于忽略目录 `work/codex-runtime/site-legal-qa/`，配置更新报告为工作区私有文件 `deployment/local/site-legal-deployment.json`。前后端发布、其他站点设置、商品/SKU 售价、两条支付渠道和 storefront 服务状态保持一致；未替换程序、重启服务、上传本机库或提交付款及采购测试。

当前上海静态发布 `/srv/zhixiang-front/releases/20261013-160811`，商城源码为 `c0d5e48c1407261e5d800969c228f03a644732d7`；后台资源沿用 `20261013-153730`，后台源码为 `471ded02e1b265520a53485288bdcc4f47dbece9`。新加坡程序为 `/srv/zhixiang/releases/20261013-162050`，程序源码为 `2bbb4e0b802114dd3bda394b28cddcf83639228a`。上海上一轮仅替换商城资源并启用注册图形验证码设置；本次成交邮件只升级新加坡后端。两地组件来源分别记录，不以之后的文档提交替代实际构建版本。

2026-10-13 已完成 [后台品牌与运营入口统一](../../docs/admin-branding.md)：按批准预览采用 π星卡社与派大星 Logo，清理原项目名称、旧域名示例、推广位和上游升级入口。后台的商城入口固定使用 `https://www.qzweb.cloud/`，避免正式站已有的本机预览配置覆盖链接；本次未改写该旧配置值。55 项后台测试、类型构建，以及本机、正式站各 19 项真实浏览器只读检查通过，覆盖 1920、390、320px、三种语言与实际导航；核验 119 个后台资源和 1468 个保留商城文件。

两地备份分别为 `/srv/zhixiang/backups/before-admin-brand-20261013-143008`、`/srv/zhixiang-front/backups/before-admin-brand-20261013-143008`；报告为新加坡 `shared/reports/admin-brand-backup-20261013-143008.json` 与上海 `shared/work/admin-brand-release-20261013-143008.json`，本机证据在 `work/codex-runtime/admin-brand-qa/`、`work/codex-runtime/admin-brand-release/`。准备阶段已有业务记录更新，本次保留了这些更新，并以切换前数据库一致性备份核对发布区间。程序、服务进程、支付与私有配置、业务数据、公开售价和成本、Nginx 配置核对一致；未上传本机库、提交真实付款或采购测试。

此前会员价发布 `20261013-114011` 将统一折扣改为商品/规格固定优惠金额，保留会员等级和升级规则、个人专属价、钱包充值以及现有支付采购流程。

会员价发布说明见 [会员商品优惠价](../../docs/member-pricing.md)。日常本地入口验证了价格保存、清空、跨等级隔离及实际商品接口与订单预览金额，临时会员已清理。正式站 437 个前端文件哈希、商城与后台登录页、服务和业务数据对比核验通过；未在生产创建验收会员或改价。新备份为 `/srv/zhixiang/backups/before-member-prices-20261013-114011`、`/srv/zhixiang-front/backups/before-member-prices-20261013-114011`，核验资料在 `work/codex-runtime/member-prices-release/` 和 `work/codex-runtime/member-prices-qa/`。历史钱包发布记录如下。

2026-10-13 11:13 已为现有 BEP20、Plasma 渠道启用钱包充值，补齐充值支付期限、确认入账和收银页链上信息；后台 **用户详情 → 钱包明细** 正常提示超级管理员本人完成声明，确认后可主动加减余额。未代签正式声明。使用方法见 [钱包充值](../../docs/wallet-recharge.md)。日常本地入口实际验证加减余额和两条充值渠道，后台 52 项、商城 97 项测试及相关 Go 回归、类型构建通过；上线后 5 项只读浏览器检查、437 个静态文件哈希核验通过。

本次备份为 `/srv/zhixiang/backups/before-wallet-20261013-110949-retry`、`/srv/zhixiang-front/backups/before-wallet-20261013-110949`，部署报告分别为 `/srv/zhixiang/shared/reports/wallet-release-20261013-110949.json`、`/srv/zhixiang-front/shared/work/wallet-release-20261013-110949.json`。仅调整两条渠道的付款类型为 `order`、`wallet`；真实用户、余额和流水、订单、专属价、公开售价与成本、其他渠道字段及私有配置保持一致。Redis 未重启，自动充值仍关闭；未上传本机库、提交真实付款或采购验收。私有核验资料在 `work/codex-runtime/wallet-release/`、`work/codex-runtime/wallet-repair-qa/`。

2026-10-13 已上线 **用户管理 → 用户列表 → 用户详情 → 设置商品单价**。填写实际金额，清空恢复现有定价；只对对应登录用户生效，商品展示、购物车与下单预览一致。使用方法见 [用户商品单价](../../docs/user-product-prices.md)。本地 13 项、正式站 9 项浏览器检查及 437 项静态文件核验通过，后台 48 项、商城 95 项测试与相关 Go 回归通过。两地发布前备份为 `backups/before-user-prices-20261013-012037`；历史记录、公开售价、采购成本、支付渠道与私有配置摘要一致，未上传本机库或进行真实付款采购测试。

2026-10-13 00:27 已将公告、11 款 ChatGPT 商品购买说明及新订单发货指引的兑换入口更新为 **https://cdk.bar/**，配置来源 `0448e64db4058e52f6b16fe5ac1a23c70975578a`，公告版本 `bf3a1445`。商品同步工具同时迁移已有指引、iOS 原有链接及其他语言中的旧地址；原站资料快照保留，导入后应用当前兑换配置。9 项临时数据库测试和本机、正式站各三项实际桌面手机检查通过。历史订单、交付记录、售价及其他商品说明摘要一致；前后端程序版本、微信号 `Cqz0924ttxk` 和支付配置保持原状态。线上商品更新前备份 `/srv/zhixiang/backups/before-redemption-url-20261013-002736`，公告更新前备份 `/srv/zhixiang/backups/before-redemption-announcement-20261013-002738`，报告 `/srv/zhixiang/shared/reports/redemption-url-20261013-002736.json`。本机核验记录在忽略目录 `work/codex-runtime/redemption-url-qa/`。

2026-10-12 23:52 按用户要求将客服微信统一更换为 **Cqz0924ttxk**，不展示电话号码。已更新首页公告、首页客服区、全站页脚及对应复制按钮；公告版本 `e97dfd7a`。本地和正式站各通过 1920、390、320px 三项实际页面检查，覆盖公告与客服展示、剪贴板复制、商品详情及公告列表页脚，无旧号码、横向溢出或脚本错误；94 项前台测试、类型检查及生产构建通过。源码 `02c1a082c3b989adee398f16073e7ef8d89a4806`，仅部署上海商城静态资源并更新后台公告。新加坡配置、storefront 模式、支付与库存服务保持一致；修改公告前备份 `/srv/zhixiang/backups/before-wechat-contact-20261012-235139`，静态发布前备份 `/srv/zhixiang/backups/before-frontend-wechat-contact-20261012-235241`。发布报告 `shared/work/wechat-contact-release-20261012-235241.json`，浏览器记录在忽略目录 `work/codex-runtime/wechat-unified-qa/`。

2026-10-12 23:17 发布首页公告弹窗接入。后台 **系统设置 → 站点设置 → 首页公告** 支持标题、图文内容、开关及起止时间；首页复用既有弹窗与版本关闭记录。关闭后本次会话不重复弹出，支持今日不再提示和不再提示；标题或内容更新后重新提示。正式公告尚未填写，配置保持关闭，等待管理员自行设置。本地通过真实设置 API 临时保存公告并完成 12 项检查，随后恢复设置；正式商城通过 12 项只读浏览器检查，其中弹窗使用仅测试浏览器内生效的配置，未修改线上公告。94 项前台测试、类型检查及生产构建通过，六种宽度无溢出或脚本错误。发布报告 `shared/work/announcement-release-20261012-231734.json`，截图与报告在忽略目录 `work/codex-runtime/announcement-qa/`；新加坡一致性备份 `/srv/zhixiang/backups/before-frontend-announcement-20261012-231734`。未进行真实付款或采购测试。使用方法见 [首页公告](../../docs/home-announcement.md)。

2026-10-12 23:35 已按用户提供的参考图片整理并启用正式弹窗 **“购买须知与代理合作”**：包含购买与发货说明、ChatGPT 兑换入口、诚招代理与代理价咨询，以及本店客服微信 **Cqz0924ttxk**。兑换使用 `https://cdkai.kdns.fr/`，代理价及拿货方式联系客服；未沿用参考图片中的第三方联系方式。此次仅通过后台 API 更新 `home_announcement`，文案配置提交 `6c088c7a20725b5ebcfec10286e6346b2c8870d2`，公开版本 `c13a6e2c`；前后端程序发布保持上文版本。实际本机和正式商城各通过 1920、390、320px 三项浏览器检查，未注入测试内容，正文、联系方式、兑换链接及关闭后刷新均通过。线上修改前备份 `/srv/zhixiang/backups/before-announcement-content-20261012-233551`，报告 `/srv/zhixiang/shared/reports/announcement-content-20261012-233551.json`；本机截图在忽略目录 `work/codex-runtime/announcement-content-qa/`。服务、售价和收款渠道保持一致，未重启程序或进行真实付款采购测试。

2026-10-12 20:51 发布商品详情封面修复：主图采用 5:3 比例及 `object-contain`，缩略图也完整显示，避免 `object-cover` 裁掉 Go、Plus、Pro 图中文字。94 项现有前台测试、类型检查和生产构建通过；本机和公网各 30 项页面检查覆盖 15 款商品封面，以及 1920、1280、1024、768、390、320px 六种宽度，无横向溢出或页面脚本错误。DEMO 的既有图标不计入商品封面检查。后台文件、Nginx 配置、16 款售价和两条 USDT 渠道核对正常；后端配置、storefront 模式和库存定时器保持原状态。发布报告为 `shared/work/detail-image-release-20261012-205131.json`，浏览器截图及记录在本机忽略目录 `work/codex-runtime/product-detail-image-qa/`。新加坡一致性备份位于 `/srv/zhixiang/backups/before-frontend-detail-20261012-205131`；没有执行真实付款或采购测试。

`zhixiang-api-tunnel.service` 使用上海本机私钥，将 `127.0.0.1:18880` 转发到新加坡 `127.0.0.1:8080`。新加坡账号 `zhixiang-api-proxy` 只接受上海来源和这一转发目标，禁止终端、其他转发、代理转发与数据库访问；上海私钥不复制至开发机或提交 Git。Nginx 配置和服务模板见本目录 `frontend-gateway/`。后端反代禁止自动重试付款请求；管理 API 未登录返回业务状态码 401。

根域、WWW 与管理 IP 的三张证书由独立 Certbot 5.8.0 环境管理，配置目录为 `/srv/zhixiang-front/shared/letsencrypt`。`qzweb-certificates.timer` 每日 00、06、12、18 时 15 分触发，续期成功后校验并重载 Nginx；管理端使用短期 IP 证书，必须保持该定时器运行。原有 WWW 和管理 IP 两张证书的实际续期、Webroot 配置、定时服务和 HTTPS 访问均已核验；根域证书于 2026-10-13 签发并核验 HTTPS，续期配置已纳入同一定时器。根域及 WWW Webroot 为 `/var/lib/letsencrypt`，管理 IP Webroot 为 `/var/lib/enterprise-exam-acme`，不能互相覆盖；未改动服务器其他站点的证书环境。

上海证据保存在 `shared/work/frontend-final-report.json` 与 `shared/work/renew-final-20261012-1644/status.json`；新加坡最终只读核验报告为 `/srv/zhixiang/shared/reports/shanghai-backend-final-read.json`。本机截图与记录在忽略的 `work/codex-runtime/alipay-domain-qa/`、`work/codex-logs/`。腾讯云上海实例为 `lhins-kb6tfs2w`；尚未另行提供上海 SSH 凭据，维护可使用账号本人已登录的腾讯云自动化助手，不能借用其他账号或项目凭据。

入口 `index.html` 禁用 ETag 和 If-Modified-Since 条件缓存，刷新返回当前发布内容；带哈希的静态资源保持长期缓存。17:13 验证旧条件请求仍返回最新 HTML，并在浏览器确认新导航已加载。证据为 `shared/work/entry-cache-report.json`，修改前两份 Nginx 配置备份在 `backups/before-entry-cache-20261012-1710`。

## 已核验的运行基准

2026-10-12 16:11（北京时间）已完成发布 **`20261012-153531`**，应用已确认的第 2 版导航 Logo：派大星图标使用浅绿圆角方块，名称与副标题放在图标外，增加间距和上下留白。品牌图片、商品内容和购买行为保持一致。

| 项目 | 核验结果 |
| --- | --- |
| 当前发布目录 | `/srv/zhixiang/current` → `/srv/zhixiang/releases/20261012-153531` |
| 正在运行的程序 | 上述目录的 `zhixiang-shop`，90366114 字节 |
| 程序 SHA-256 | `5c3707bff99f2e16a66b4f9924150d2778ca7bcdc19b89e35dd7e2e737e6452f` |
| 发布清单 | `manifest.json` 的程序哈希与运行中程序一致 |
| 编译信息 | Go 1.26.5，Linux amd64，CGO=0，`release,fullstack` |
| 运行模式 | `storefront`，`UPSTREAM_PURCHASE_ENABLED=true` |
| 服务状态 | 商城、专用 Redis、库存定时器、Nginx 均 active |
| 构建源码 | `main` 的 `948c37f2d33420631eb77a75d3c1debe40d967b6`，`vcs.modified=false` |
| 上一个实际发布 | `20261012-141843`，源码 `06620f0e61a32fa17760b6f8b94312a29c03fb4e`，旧程序和升级前备份均保留 |
| 收款渠道 | BEP20 与 Plasma 均启用，公网 IP 支付配置已显示两项 |

完整证据见 [production-baseline.json](production-baseline.json)。发布清单、Go 编译信息及实际运行程序哈希相互匹配。公网 IP 的首页、后台、健康、配置和 20 项资源共 24 项核验通过；桌面 1920px、手机 390px 与 320px 的实际页面符合本地确认方案，Logo 与按钮没有重叠或横向溢出，浏览器无错误。后续文档提交不改变线上程序构建来源，库存仍为动态业务数据。

本次验证包括前台 94 项、后台 28 项现有测试、两套前端 TypeScript 检查及完整发布构建。部署前备份线上数据库、配置、私有凭据、旧发布、Nginx 配置和 Redis 快照；部署保留现场配置、商品说明、价格、付款渠道、权限与声明，Redis 没有重启。自动采购保持既有授权设置，自动充值仍关闭，未提交真实付款、验收订单或采购。截图和核验记录保存在本机忽略目录 `work/codex-runtime/logo-spacing-preview-20261012/`。

## ChatGPT 发货兑换指引

2026-10-12 13:57（北京时间）已应用商品说明配置，14:01 完成应用接口复核。11 款 ChatGPT 卡密（含两款 iOS 渠道）发货使用 **https://cdkai.kdns.fr/**，交付使用说明附完整链接和自助充值步骤；新订单保存该说明，交付页及已启用的发货邮件读取订单快照。历史订单不回写。

配置来源为 `main` 的 `2ef4790a64f351a85e91d59bebe593beb0f31dce`，映射和文案见 [qzweb-redemption-guide.json](../../docs/catalog/qzweb-redemption-guide.json)。该次更新仅修改商品数据库，没有替换程序；当时运行发布为 `20261012-134141`，构建源码为 `b478c600dc6cda365289d4bbb76501987d16a377`。当前程序发布见上表，本次 Logo 发布已对照核验商品交付说明内容保持一致。

数据库、私有配置和当前发布已在服务器备份。原始更新前备份为 `/srv/zhixiang/backups/before-redemption-guide-20261012T055249Z`，最终配置更新前补充备份为 `/srv/zhixiang/backups/before-redemption-guide-20261012T055749Z`；一致性检查均通过。报告为 `/srv/zhixiang/shared/reports/redemption-guide-20261012T055749Z.json`。

已核验 11 款后台商品详情实际返回兑换链接和步骤，两款 iOS 公开详情显示统一兑换入口并保留 30 天权益、剩余时长不叠加的规则。SQLite JSON 按现有 Go 存储契约写为 BLOB。8 项临时数据库测试覆盖备份、回滚、重复应用和存储兼容性；售价、支付渠道及配置核对通过，商城、Redis 和库存定时器保持 active。未创建真实验收订单或主动采购。

## 持久化数据与服务

| 用途 | 服务器位置 |
| --- | --- |
| 配置 | `/srv/zhixiang/shared/config.yml` |
| 生产数据库 | `/srv/zhixiang/shared/db/shop.db` |
| 上传与日志 | `/srv/zhixiang/shared/uploads`、`logs` |
| 持久化 Redis | `shared/redis/`，回环地址端口 16379 |
| 库存助手私有凭据 | `shared/stock-sync.credentials.json` |
| 库存报告 | `shared/reports/merchant1039-stock-sync.json` |
| 当前版本升级前备份 | `/srv/zhixiang/backups/before-20261012-153531`，数据库一致性检查通过，包含配置、凭据、旧程序、Nginx 与 Redis 快照 |
| 本次发布核验报告 | `shared/reports/logo-release-20261012-153531.json` |

`zhixiang-shop.service` 以 `zhixiang` 用户运行；`40-storefront.conf` 指定当前模式。支付监听、采购/交付 worker 与恢复任务由应用管理。`zhixiang-redis.service` 保存订单队列。`zhixiang-stock-sync.timer` 在每次任务完成后等待 300 秒，仅查询和刷新库存；不要另起重复助手或完整 worker 来同步库存。服务单元模板保存在本目录。

商户 1039 使用 `acg-faka`，已有 12 款供货映射，`auto_sync_price=false`。全部 16 款商品售价独立维护，`qzweb-55` 保持 1160.00 元。自动充值关闭；余额和库存需实时查询，不把历史报告的数值当作当前数值。客户确认付款后的采购已经授权，但没有做真实扣款验收，不主动提交有效采购或修改测试订单为已付款。

客户付到 BEP20 / Plasma 地址的 USDT 与供货商采购余额相互独立；自动采购需要供货商人民币余额充足、映射有效且对应 SKU 有库存。余额不足的采购记录进入 `rejected`，客户订单保留已付款状态，补足余额后需对这些失败记录手动重试；扣款结果未知时进入 `manual_review`，核对上游后再处理，不能直接重新采购。两款本地卡密商品和两款人工交付商品不走供货商采购。充值后无需重启，新采购每次都会读取最新余额。2026-10-12 只读核验了实际商户报价、库存、单件购买数量限制与本地采购成本上限，12 款供货商品中 11 款满足余额充足后的采购条件，`qzweb-59` 当时缺货；商品状态与价格会变化，正式采购前仍会重新检查。

链路回归测试位于 `internal/bootstrap/bscusdt/procurement_flow_test.go`：BEP20、Plasma 分别覆盖即时发卡、延迟取卡、队列恢复、余额不足和采购结果未知、充值后手动重试，共 12 个场景，同时核验最终客户订单响应包含交付内容以及重复任务不重复采购。2026-10-12 已通过；测试采用临时数据库和模拟链、队列传输及供应商，不构成真实资金验收。本机私有检查记录保存在忽略的 `work/codex-runtime/finance-release/verification-summary.json`。

## 状态检查

### 本机固定登录入口

用户已授权后续会话复用已提供的服务器信息，并于 2026-10-12 确认上海与新加坡均使用已提供的 `ubuntu` 登录信息。当前 C 盘工作区私有目录 `C:\Codex program\智享A社\deployment\local\` 已保存新加坡 `tencent-cloud-server.credentials.json`、`tencent-cloud-known-hosts`，以及上海 `shanghai-front-server.credentials.json`、`shanghai-front-known-hosts`。上海登录已验证，密码文件仅当前用户和 SYSTEM 可访问，全部由 Git 忽略。字段为 `host`、`port`、`username`、`password`；连接时核对目标 IP，加载对应已保存指纹并拒绝未知或变化的主机密钥，不输出凭据。D 盘工作区若存在其原 `work/codex-runtime/tencent-cloud-server.credentials.json` 与 `tencent-known-hosts`，仍可用于同一新加坡目标；不要因旧路径缺失而忽略当前已保存的私有文件或重复询问。

新会话先复用这一入口，不再要求用户重复提供服务器信息。只有文件缺失、认证失败或目标不匹配时才询问。凭据值不写入本文、Git、日志或发布包；备份仍按生产发布流程保存在私有位置。

### 只读命令

以下命令只查询状态，不重启或部署：

```sh
sudo systemctl status zhixiang-shop.service zhixiang-redis.service zhixiang-stock-sync.timer --no-pager
sudo systemctl list-timers zhixiang-stock-sync.timer --no-pager
sudo readlink -f /srv/zhixiang/current
sudo sha256sum /srv/zhixiang/current/zhixiang-shop
```

日志读取使用有界参数，例如 `journalctl -n 100 --no-pager`，先在服务器本地检查是否含客户或凭据数据。不要输出整个配置、环境变量、商户密钥、JWT、密码或卡密。

## 后续发布

### Plasma 收款配置

线上已启用独立的 `plasma_usdt` 渠道；已有 BEP-20 继续使用 `bsc_usdt`。两张截图与线上配置的收款地址相同：`0x66a8bab15067eab1efc948ac03684f0397c8416f`，两条链的监听、金额标识和交易去重分别保存。新环境的 `config.yml.example` 默认关闭 Plasma，不能据此判断现有生产状态。

Plasma 主网编号为 `9745`，USDT0 合约为 `0xb8ce59fc3717ada4c02eadf9682a9e934f625ebb`，精度为 6；BSC USDT 精度仍为 18。合约依据：[USDT0 官方部署表](https://docs.usdt0.to/technical-documentation/deployments)。2026-10-12 已对默认 Plasma RPC 只读核验链编号、代币精度、`finalized` 和日志接口；模拟链付款及交付测试不等于真实资金验收。

当前生产配置如下，汇率沿用 BEP20 的 6.66。生产 `config.yml` 实际使用 JSON 格式（兼容 YAML），后续修改需保持文件格式并核对其余配置，不能直接在 JSON 文档末尾追加 YAML 段。以下仅以 YAML 展示相同字段：

```yaml
plasma_usdt:
  enabled: true
  recipient: "0x66a8bab15067eab1efc948ac03684f0397c8416f"
  cny_per_usdt: "6.66"
  confirmations: 15
```

已从生产服务器核验 RPC 可达、chain ID 为 9745、合约 `decimals()` 为 6、最终确认区块、日志及交易收据接口可用。默认节点是 `https://rpc.plasma.to`；自定义节点只写服务端 `PLASMA_RPC_URL`，需要分开查询收据时用 `PLASMA_RECEIPT_RPC_URL`。无需钱包私钥或助记词。

首次启用自动创建 `USDT · Plasma` 订单支付渠道；重启不会重新启用已禁用渠道或覆盖商户名称。沿用 storefront 模式即可启动独立 Plasma 监听。后台只读查询入口为 `/api/v1/admin/chain-payments/plasma-usdt/status` 和 `/transfers`，使用已有后台鉴权。核验支付页显示 Plasma、USDT0、正确地址及完整 6 位小数金额；两条链都要求完整金额和最终确认，异常金额或过期付款进入人工核查。

2026-10-12 链路检查发现，维护账号访问两条链的后台监控接口均返回 `compliance_required`，需超级管理员在后台自行确认现有声明，检查脚本不代为确认；此限制不阻断客户支付或链上监听。Plasma 的财务权限遗漏已在发布 `20261012-114319` 修复：内置 `finance` 角色拥有两条链 `status` 和 `transfers` 的 GET 权限。启动时自动补齐已有角色，已从运行中后台权限接口核验；仅增加上述两项策略，未代为确认声明，也未放宽其他角色或写入权限。

### 正式升级规则

GitHub `main` 为当前绿色商城源码主线，推送不会自动更新此服务器。升级遵循 [发布规则](../../docs/release-process.md)：先记录源码提交及构建哈希，备份线上库、配置和当前发布，再执行已授权的程序升级；保留 `shared/`、storefront 模式、支付采购队列和持久化 Redis。不得上传本地预览库覆盖生产订单。

历次部署细节已移入 [历史部署记录](../../docs/history/tencent-deployments-20261012.md)，其中旧 `api`/`checkout` 命令和本机私有包路径仅为历史资料。不要将通用上游安装器或旧初始化脚本用于覆盖正式环境。

### 公告列表（2026-10-13）

已补发“购买须知与代理合作”公告文章，ID 1、slug `purchase-and-agent-guide`，列表 `/notice`、详情 `/blog/purchase-and-agent-guide`。正文来源 `docs/catalog/purchase-notice.json`（内容提交 `1f07520e00941bf78442c86cefb89c92a7db07c8`），与首页弹窗一致。两处内容独立维护；后台文章入口为 **内容管理 → 公告列表**，弹窗入口为 **系统设置 → 站点设置 → 首页公告**。

先通过本机实际页面核验，再推送内容并发布。正式站通过 1920、390、320px 的导航、列表、详情、刷新和返回检查，未注入浏览器内容。程序仍为 `20261013-012037`，弹窗版本 `bf3a1445`。备份位于 `/srv/zhixiang/backups/before-notice-post-20261013-104352`，含一致性 SQLite 快照；服务器报告 `shared/reports/notice-post-20261013-104352.json`。没有替换程序、重启服务、上传本机数据库或执行付款采购测试，售价、成本、支付渠道、个人固定价格和站点设置均未修改。

当时两份已保存的后台口令被登录 API 拒绝，SSH 维护连接有效；通过该通道精确新增公告，保留账号设置。维护脚本先在本机临时库核对与普通 API 写入的字段和 JSON BLOB 格式、防重复行为，再在服务器备份副本演练，最后核验实际公开接口。后续后台操作须使用当前有效的登录凭据，不自动重置账号或反复提交旧口令。
