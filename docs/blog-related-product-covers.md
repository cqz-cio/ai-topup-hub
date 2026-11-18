# 博客相关文章页的商品封面

2026-11-18 商城静态版本 `20261118-131948` 已上线。博客详情页的相关商品在 740px 及以下使用单列，较宽视口使用两列。5:3 图片框使用 contain 完整展示原图，悬停时图片不放大，商品名称可换行。维护样式在 [blogRelatedProducts.css](../frontend/user/src/styles/blogRelatedProducts.css)，由 BlogDetail.vue 导入。

本次发布从现行静态目录 `20261117-162000` 复制生成。新增 `assets/blog-related-products-48fcf9563a06b7.css`，并在 index.html、storefront-entry.js、storefront-assets.json 中登记该样式。其余 15,730 个文件与发布前共用原始文件。切换前已以 www-data 检查新增和替换文件可读。发布过程只切换商城静态版本链接。

正式验收覆盖 1440、741、740、390、320px。逐卡检查图像加载、contain、5:3 图框、悬停无缩放、列数和标题布局，对照批准预览核验尺寸。4 个发布文件公网哈希通过，博客详情与列表、商品列表、购物车入口均返回 200。候选阶段首次测量发生在卡片动画期间，改为动画结束后比对，候选和正式复验均通过。

回退备份位于 `/srv/zhixiang-front/backups/before-related-covers-20261118-131948`。确认现行版本仍为本次版本时，可将 `/srv/zhixiang-front/current` 原子切回回执中的上一静态目录。回退只涉及商城静态目录链接。
