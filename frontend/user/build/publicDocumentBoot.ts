// Applied only to the server's readable document while the interactive app loads.
// Browsers without this media query or :has support keep the readable document.
// The CSS deadline is independent of JavaScript: failed/blocked scripts cannot
// leave the server content permanently hidden.
export const publicDocumentBootCss = `
@supports selector(:has(*)) {
  @media (scripting: enabled) {
    html:has(meta[property="og:site_name"][content="π星卡社"]):not([data-public-document-boot-failed]) #app:has(> #public-document) {
      min-height: 100svh;
    }
    html:has(meta[property="og:site_name"][content="π星卡社"]):not([data-public-document-boot-failed]) #app > #public-document {
      visibility: hidden;
      opacity: 0;
      max-height: 0;
      overflow: hidden;
      margin: 0 auto;
      padding: 0;
      animation: public-document-reveal 0s 10s forwards;
    }
    html:has(meta[property="og:site_name"][content="π星卡社"]):not([data-public-document-boot-failed]) #app:has(> #public-document)::before {
      content: "";
      position: fixed;
      inset: 0;
      z-index: 9998;
      background: radial-gradient(ellipse at 25% 0%, rgba(255,255,255,.78), transparent 60%), linear-gradient(180deg, #a9e8ef, #d9f5f1 76%, #eff9ef);
      pointer-events: none;
      animation: public-document-dismiss 0s 10s forwards;
    }
    html:has(meta[property="og:site_name"][content="π星卡社"]):not([data-public-document-boot-failed]) #app:has(> #public-document)::after {
      content: "π星卡社\\A 正在打开商城…";
      position: fixed;
      left: 50%;
      top: 50%;
      transform: translate(-50%, -50%);
      z-index: 9999;
      box-sizing: border-box;
      width: min(280px, calc(100vw - 48px));
      padding: 26px 24px;
      border: 1px solid rgba(255,255,255,.9);
      border-radius: 20px;
      background: rgba(242,253,252,.65);
      backdrop-filter: blur(8px);
      color: #173b36;
      font: 600 16px/2.1 system-ui,sans-serif;
      white-space: pre-line;
      text-align: center;
      box-shadow: inset 0 1px 0 rgba(255,255,255,.8), 0 12px 40px -28px rgba(13,59,62,.4);
      pointer-events: none;
      animation: public-document-dismiss 0s 10s forwards;
    }
  }
}
@keyframes public-document-reveal {
  to { visibility: visible; opacity: 1; max-height: none; overflow: visible; margin: 32px auto; padding: 24px; }
}
@keyframes public-document-dismiss {
  to { visibility: hidden; opacity: 0; }
}
`

export function publicDocumentBootScript(scripts: string[]): string {
  return `Promise.all([${scripts.map(path => `import(${JSON.stringify(path)})`).join(',')}]).catch(() => { document.documentElement.setAttribute('data-public-document-boot-failed', ''); });\n`
}
