import './session-guide.css';

export default function SessionGuide({ product }) {
  if (product !== 'gpt') return null;

  return <section className="session-guide" aria-label="获取账号 Session 的操作步骤">
    <div className="session-guide-heading">按下面 3 步，获取并填写 Session</div>
    <ol>
      <li>
        <span className="session-guide-number" aria-hidden="true">1</span>
        <div>
          <h3>先登录并确认目标充值账号</h3>
          <p>打开 ChatGPT，确认已经登录；核对个人资料中的邮箱，确保是本次要充值的账号。账号不对时，请先切换账号。</p>
          <a className="session-guide-link" href="https://chatgpt.com/" target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">
            <strong>点击这里打开 ChatGPT，完成第 1 步 ↗</strong>
            <span>https://chatgpt.com/</span>
          </a>
        </div>
      </li>
      <li>
        <span className="session-guide-number" aria-hidden="true">2</span>
        <div>
          <h3>打开 Session 页面，复制完整内容</h3>
          <p>完成第 1 步后，在同一浏览器打开下面的链接。全选并复制页面显示的完整 JSON，不要只复制其中一段。</p>
          <a className="session-guide-link" href="https://chatgpt.com/api/auth/session" target="_blank" rel="noopener noreferrer" referrerPolicy="no-referrer">
            <strong>点击这里获取 Session，继续第 2 步 ↗</strong>
            <span>https://chatgpt.com/api/auth/session</span>
          </a>
        </div>
      </li>
      <li>
        <span className="session-guide-number" aria-hidden="true">3</span>
        <div>
          <h3>回到本页粘贴，再核验账号</h3>
          <p>将复制的完整内容粘贴到下方输入框，点击“核验账号”。核验成功后，再核对邮箱与套餐，确认无误才提交充值。</p>
        </div>
      </li>
    </ol>
    <p className="session-guide-note">若页面为空、显示 {'{}'} 或提示未登录，请回到第 1 步确认登录后重试。Session 属于账号登录凭证，请勿发给他人或公开分享。</p>
  </section>;
}
