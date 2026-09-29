import { marked } from 'marked'
import DOMPurify from 'dompurify'

// release notes 是 GitHub 风格 markdown，换行需要按原样保留
marked.setOptions({ gfm: true, breaks: true })

// Release body 是远端可变内容，且 markdown 允许内联原始 HTML —— marked 自 v5 起移除了
// sanitize 选项，会把 <img onerror> / <a href="javascript:"> 原样透传。这段 HTML 最终进
// v-html，管理员只要打开更新对话框就会执行，足以读走 localStorage 里的 admin_token。
// 因此渲染结果必须再过一次 DOMPurify，白名单只保留 release notes 真正会用到的排版标签。
//
// 这个模块单独抽出来是为了能被单测覆盖 —— 见 releaseNotes.test.ts，
// 那里固化了一批攻击向量，防止以后有人「顺手」放开白名单或换回裸 marked 输出。
export const RELEASE_NOTES_SANITIZE_CONFIG = {
  ALLOWED_TAGS: [
    'p', 'br', 'hr', 'strong', 'em', 'del', 's', 'code', 'pre', 'blockquote',
    'ul', 'ol', 'li', 'a', 'img', 'span',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'table', 'thead', 'tbody', 'tr', 'th', 'td',
  ],
  ALLOWED_ATTR: ['href', 'title', 'src', 'alt'],
  FORBID_ATTR: ['style', 'class', 'id'],
  ALLOW_DATA_ATTR: false,
  // 只放行 http(s)、mailto 与页内锚点，挡掉 javascript:/data: 等可执行协议
  ALLOWED_URI_REGEXP: /^(?:https?:|mailto:|#)/i,
}

/**
 * 把 GitHub Release 的 markdown 正文渲染成可安全交给 v-html 的 HTML。
 *
 * 净化后再统一给外链补 target/rel：交给 DOMPurify 输出 DOM 片段而不是字符串，
 * 避免用正则改 HTML 时反而引入新的注入面。
 */
export function renderReleaseNotes(raw: string): string {
  if (!raw) return ''

  const fragment = DOMPurify.sanitize(marked.parse(raw, { async: false }) as string, {
    ...RELEASE_NOTES_SANITIZE_CONFIG,
    RETURN_DOM_FRAGMENT: true,
  })
  fragment.querySelectorAll('a[href]').forEach((anchor) => {
    anchor.setAttribute('target', '_blank')
    anchor.setAttribute('rel', 'noopener noreferrer')
  })
  const holder = document.createElement('div')
  holder.appendChild(fragment)
  return holder.innerHTML
}
