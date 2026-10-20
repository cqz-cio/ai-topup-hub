# 商品图透明底

## 映射范围

`frontend/user/src/utils/image.ts` 只为三条已确认的原图路径返回版本化透明素材，所有其他图片保持原规则。商品记录、原图与价格不变。若后台换了图片路径，新图片不自动套用这组映射。

| 原图片 | 透明素材 | 使用商品 |
| --- | --- | --- |
| `/uploads/86gamestore/f0bb72279d34b2b14d08.jpg` | `/catalog-products/transparent-icons-20261020/gpt.png` | 4 款 GPT 代充 |
| `/uploads/86gamestore/c779fc65589ae19c7ff2.jpg` | `/catalog-products/transparent-icons-20261020/telegram.svg` | Telegram 3/6/12 个月 |
| `/catalog-products/qzweb/aa16fb0f3bb8f6933985a4c8df660946.jpg` | `/catalog-products/transparent-icons-20261020/gemini.png` | 2 款 Gemini 预售 |

GPT 与 Gemini 为含真实 alpha 通道的 PNG。GPT 仅保留黑色结形线条，内部空隙同样透明。Telegram 采用 SVG 圆角裁切，嵌入原 JPEG 字节，只隐藏原四角的黑色区域；蓝底和飞机完整保留。两次误抠除蓝底的生成候选均未加入项目或发布包。处理提示词与原始候选记录位于私有证据目录的 `image-processing.md`。

## 验收与回退

## 商品图透明底上线（2026-10-20）

- 上海商城发布 `20261020-090653`，干净源码 `6c9fc7d3ec9e394dd662014f7ac5e513e16aab44`。GPT 图标外部及内部白色空隙透明；Gemini 去掉白色底；Telegram 仅裁去黑色四角，完整保留原蓝底与白色飞机。共覆盖 9 款商品，首页、商品中心及详情共用精确路径映射。维护见 `docs/transparent-product-icons.md`。
- 用户确认日常本机效果后推送上线。157 项商城测试、类型、构建与公开资源扫描通过；6 项日常、3 项最终构建及 8 项正式浏览器检查通过，包含桌面、390/320px、深色、搜索、首页和详情。正式 35 项公开请求及 373 个构建文件哈希通过；保留 8383 个旧商城文件、1459 个后台文件及 57 个 Nginx 文件。
- 新加坡后端及上海后台保持 `20261019-160954`；后端/Redis 进程、两套库存 timer、配置、售价及支付渠道与发布前一致。两地备份均为各自 `backups/before-transparent-icons-20261020-090653`。原商品图片和数据库记录保留，未上传本机库、创建正式订单或执行真实付款采购。
- 源码已推送 main；日常主目录原分支与其他未提交草稿保留。私有证据 `work/codex-runtime/transparent-product-icons-20261020/verified-report.json`。


静态回退目标为 `/srv/zhixiang-front/releases/20261019-173847`；无需回退数据库。发布保留原图及旧脚本，历史撤下资源规则继续生效。正式三类商品截图为私有证据目录内的 `production-telegram.png`、`production-gpt.png`、`production-gemini.png`。
