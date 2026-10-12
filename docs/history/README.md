# 历史资料与恢复

本目录保存旧阶段文档，“当前”“已部署”等表述只代表记录当时的状态。日常操作从根目录 [README](../../README.md) 开始。

| 资料 | 原用途 |
| --- | --- |
| [upstream-readme.md](upstream-readme.md) | 原 Dujiao-Next 通用说明 |
| [design-qa-before-cleanup.md](design-qa-before-cleanup.md) | 历次页面与支付验收记录 |
| [local-mainnet-20260930.md](local-mainnet-20260930.md) | 旧本机主网支付验收 |
| [self-service-before-cleanup.md](self-service-before-cleanup.md) | 原自助商城说明 |
| [ellaixshop-before-cleanup.md](ellaixshop-before-cleanup.md) | 供货接入与早期库存助手记录 |
| [tencent-deployments-20261012.md](tencent-deployments-20261012.md) | 新加坡多次部署流水 |

旧 React 源码保存在 Git 标签 `baseline/green-20261012` 的 `frontend/catalog` 路径，已移出主线。使用 `git show baseline/green-20261012:frontend/catalog/README.md` 可阅读原入口。

本机 Git bundle、旧 React 目录和未运行的旧程序在忽略的 `work/archive/green-cleanup-*`，供恢复使用。不要把其中的数据库、配置和凭据提交 GitHub。
