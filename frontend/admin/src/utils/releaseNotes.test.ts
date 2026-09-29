import { describe, expect, it } from 'vitest'
import { renderReleaseNotes } from './releaseNotes'

// GitHub Release 的正文是远端可变内容，渲染结果直接进 v-html。发布账号、Actions
// 或 Release 内容任一环节失守，未净化的 markdown 就能在管理员打开更新对话框时执行，
// 读走 localStorage 里的 admin_token。
//
// 这批用例的作用是防回归：以后有人放宽白名单、换回裸 marked 输出，或升级 marked/DOMPurify
// 引入行为变化时，这里会立刻红。
describe('renderReleaseNotes', () => {
  const attacks: Array<[name: string, payload: string]> = [
    ['原始 img onerror', '<img src=x onerror=alert(localStorage.admin_token)>'],
    ['原始 a javascript:', '<a href=javascript:alert(1)>click</a>'],
    ['markdown 链接 javascript:', '[click](javascript:alert(1))'],
    ['markdown 链接大小写混淆', '[click](JaVaScRiPt:alert(1))'],
    ['svg onload', '<svg onload=alert(1)></svg>'],
    ['svg 嵌套 script', '<svg><script>alert(1)</script></svg>'],
    ['iframe srcdoc', '<iframe srcdoc="<script>alert(1)</script>"></iframe>'],
    ['script 标签', '<script>alert(1)</script>'],
    ['body onload', '<body onload=alert(1)>'],
    ['form + formaction', '<form action=x><button formaction=javascript:alert(1)>x</button></form>'],
    ['data: URI 链接', '[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)'],
    ['img data: URI', '<img src="data:text/html,<script>alert(1)</script>">'],
    ['style 表达式', '<div style="background:url(javascript:alert(1))">x</div>'],
    ['大小写混淆事件属性', '<IMG SRC=x OnErRoR=alert(1)>'],
    ['带前导空格的 javascript:', '<a href=" javascript:alert(1)">y</a>'],
    ['object/embed', '<object data="javascript:alert(1)"></object><embed src="javascript:alert(1)">'],
    ['meta refresh', '<meta http-equiv="refresh" content="0;url=javascript:alert(1)">'],
    ['math mtext 绕过', '<math><mtext><script>alert(1)</script></mtext></math>'],
    ['details ontoggle', '<details open ontoggle=alert(1)>x</details>'],
    ['base 标签劫持相对路径', '<base href="https://evil.example/">'],
  ]

  // 任何一条命中都说明净化被绕过
  const forbidden = [
    /\son\w+\s*=/i, // 事件处理器属性
    /javascript:/i,
    /<script/i,
    /<iframe/i,
    /srcdoc/i,
    /formaction/i,
    /data:text\/html/i,
    /<object/i,
    /<embed/i,
    /<meta/i,
    /<base[\s>]/i,
    /\sstyle\s*=/i,
  ]

  it.each(attacks)('中和 %s', (_name, payload) => {
    const html = renderReleaseNotes(payload)
    for (const pattern of forbidden) {
      expect(html, `payload="${payload}" 输出="${html}"`).not.toMatch(pattern)
    }
  })

  it('保留正常的 markdown 排版', () => {
    const html = renderReleaseNotes(
      [
        '## 更新内容',
        '',
        '- 修复 **支付回调** 问题',
        '- 见 [文档](https://dujiao-next.com/deploy/)',
        '',
        '| 项 | 值 |',
        '| --- | --- |',
        '| a | b |',
        '',
        '`code`',
      ].join('\n'),
    )

    expect(html).toContain('<h2>')
    expect(html).toContain('<strong>')
    expect(html).toContain('<table>')
    expect(html).toContain('<code>')
    expect(html).toContain('<li>')
  })

  it('给外链补上 target 与 rel，避免 window.opener 反向控制', () => {
    const html = renderReleaseNotes('[docs](https://dujiao-next.com/deploy/)')
    expect(html).toContain('href="https://dujiao-next.com/deploy/"')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('rel="noopener noreferrer"')
  })

  it('保留 https 图片但剥掉事件属性', () => {
    const html = renderReleaseNotes('<img src="https://example.com/a.png" alt="x" onerror="alert(1)">')
    expect(html).toContain('src="https://example.com/a.png"')
    expect(html).toContain('alt="x"')
    expect(html).not.toMatch(/onerror/i)
  })

  it('空输入返回空串', () => {
    expect(renderReleaseNotes('')).toBe('')
  })
})
