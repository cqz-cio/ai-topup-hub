// Reconstructed from the owned production bundle (20261115-235900).
// SHA256: a5a243964836d961a368c4590f654e22259740c4ca6289b77ed62657a587d74e
// Original Git/source file names were unavailable; see docs/wudi-source-recovery.md.
import React from "react";
import {
  CircleAlert,
  Copy,
  ArrowUpRight,
  RotateCcw,
  Ticket,
  ShieldCheck,
  Download,
} from "lucide-react";
import { api, statusLabel, extractCard, formatDate } from "./api.js";
import {
  linkDone,
  linkLabel,
  parseLinkRows,
  mergeLinkResult,
} from "./link-flow.js";

const linkDisplayLabel = (f) => linkLabel(f) || statusLabel(f);
function LinkError({ children: children }) {
  return children ? (
    <p className="error" role="alert">
      <CircleAlert size={16} />
      {children}
    </p>
  ) : null;
}
function LinkResult({
  result: result,
  mock: mock,
  onRefresh: onRefresh,
  busy: busy,
  onRetry: onRetry,
}) {
  const [copied, setCopied] = React.useState(false),
    [copyError, setCopyError] = React.useState("");
  React.useEffect(() => {
    setCopied(false);
    setCopyError("");
  }, [result == null ? void 0 : result.attempt_id]);
  async function copyCode() {
    try {
      await navigator.clipboard.writeText(result.redemption_code);
      setCopied(true);
      setCopyError("");
    } catch {
      setCopyError("复制失败，请选中下方兑换卡密手动复制。");
    }
  }
  return result ? (
    <div className="link-result" aria-live="polite">
      <div className="link-result-heading">
        <strong>{linkDisplayLabel(result.state)}</strong>
        <span>{result.code}</span>
      </div>
      <p>{result.message || "可通过原卡密继续查询兑换记录。"}</p>
      <LinkError>{result.query_error}</LinkError>
      <LinkError>{result.preflight_message || result.error_message}</LinkError>
      {result.redemption_code && result.can_open && (
        <>
          <label>{"在兑换站输入的兑换卡密"}</label>
          <code className="redemption-code">{result.redemption_code}</code>
          <p className="field-hint">
            {
              "复制这张兑换卡密，打开专属链接后粘贴，再核对充值账号。本站 PAIPLUS 卡密用于回来查询进度。"
            }
          </p>
          <div className="link-actions">
            <button type="button" className="secondary" onClick={copyCode}>
              <Copy size={15} />
              {copied ? "已复制" : "复制兑换卡密"}
            </button>
            {mock ? (
              <button type="button" className="primary" disabled={true}>
                {"模拟链接 · 不跳转"}
              </button>
            ) : (
              <a
                className="primary"
                href={result.redeem_url}
                target="_blank"
                rel="noopener noreferrer"
                referrerPolicy="no-referrer"
              >
                {"前往兑换站"}
                <ArrowUpRight size={17} />
              </a>
            )}
          </div>
          <LinkError>{copyError}</LinkError>
          {result.expires_at && (
            <p className="field-hint">
              {"链接有效期："}
              {formatDate(result.expires_at)}
              {"。创建链接不会发起充值。"}
            </p>
          )}
        </>
      )}
      {result.can_open && !result.redemption_code && (
        <p className="field-hint">
          {"如需继续兑换，请在上方输入完整 PAIPLUS 卡密，重新获取原专属链接。"}
        </p>
      )}
      <div className="link-actions">
        <button
          type="button"
          className="text-button"
          disabled={busy}
          onClick={onRefresh}
        >
          <RotateCcw size={15} />
          {busy ? "正在查询…" : "刷新结果"}
        </button>
        {result.can_retry && onRetry && (
          <button
            type="button"
            className="secondary"
            disabled={busy}
            onClick={onRetry}
          >
            {"重新生成兑换链接"}
          </button>
        )}
      </div>
      {result.can_retry && (
        <p className="field-hint">
          {"仅在链接过期，或原充值失败且卡密已恢复可用时，才可重新生成链接。"}
        </p>
      )}
    </div>
  ) : null;
}
function LinkRecharge({ config: config, onChange: onChange }) {
  const [code, setCode] = React.useState(""),
    [result, setResult] = React.useState(null),
    [busy, setBusy] = React.useState(false),
    [error, setError] = React.useState(""),
    mounted = React.useRef(true),
    generation = React.useRef(0),
    activeCode = React.useRef("");
  React.useEffect(
    () => (
      (mounted.current = true),
      () => {
        mounted.current = false;
        generation.current++;
        activeCode.current = "";
      }
    ),
    [],
  );
  async function run(D, Z) {
    const L = ++generation.current;
    setBusy(true);
    setError("");
    try {
      const le = await D();
      mounted.current && L === generation.current && Z(le);
    } catch (le) {
      mounted.current && L === generation.current && setError(le.message);
    } finally {
      mounted.current && L === generation.current && setBusy(false);
    }
  }
  async function createLink(D, Z = false) {
    D == null || D.preventDefault();
    let L;
    try {
      L = extractCard(code);
    } catch (le) {
      setError(le.message);
      return;
    }
    setResult(null);
    activeCode.current = L;
    await run(
      () =>
        api(Z ? "partner-retry" : "partner-link", {
          code: L,
          ...(Z
            ? {
                confirmed: true,
              }
            : {}),
        }),
      (le) => {
        setResult(le);
        onChange();
      },
    );
  }
  const refresh = () =>
    run(
      () =>
        api(
          activeCode.current ? "result-by-code" : "result",
          activeCode.current
            ? {
                code: activeCode.current,
              }
            : {
                attempt_id: result.attempt_id,
              },
        ),
      (D) => setResult((Z) => mergeLinkResult(Z, D)),
    );
  async function findRecord() {
    let D;
    try {
      D = extractCard(code);
    } catch (Z) {
      setError(Z.message);
      return;
    }
    activeCode.current = D;
    setResult(null);
    await run(
      () =>
        api("result-by-code", {
          code: D,
        }),
      setResult,
    );
  }
  async function openRecent(D) {
    setCode("");
    activeCode.current = "";
    setResult(null);
    await run(
      () =>
        api("result", {
          attempt_id: D.attempt_id,
        }),
      setResult,
    );
  }
  return (
    React.useEffect(() => {
      if (
        !(result != null && result.attempt_id) ||
        result.provider !== "wudi" ||
        linkDone(result.state)
      )
        return;
      const D = generation.current;
      let Z = false,
        L;
      const le = async () => {
        try {
          const ue = await api(
            activeCode.current ? "result-by-code" : "result",
            activeCode.current
              ? {
                  code: activeCode.current,
                }
              : {
                  attempt_id: result.attempt_id,
                },
          );
          !Z &&
            mounted.current &&
            generation.current === D &&
            (setResult((G) => mergeLinkResult(G, ue)), setError(""));
        } catch (ue) {
          !Z &&
            mounted.current &&
            generation.current === D &&
            setError(ue.message);
        }
        Z || (L = setTimeout(le, 15e3));
      };
      return (
        (L = setTimeout(le, 15e3)),
        () => {
          Z = true;
          clearTimeout(L);
        }
      );
    }, [
      result == null ? void 0 : result.attempt_id,
      result == null ? void 0 : result.state,
      busy,
    ]),
    (
      <>
        <ol className="stepper">
          {[
            "输入本站卡密",
            "打开兑换链接",
            "核对账号并确认",
            "返回查询结果",
          ].map((D, Z) => (
            <li className={Z === (result ? 1 : 0) ? "active" : ""} key={D}>
              <span className="step-number">{Z + 1}</span>
              <span>{D}</span>
              {Z < 3 && <span className="step-line" />}
            </li>
          ))}
        </ol>
        <section className="form-panel glass">
          <div className="kicker">
            <Ticket size={16} />
            {"专属链接兑换"}
          </div>
          <h2>{"输入你的 PAIPLUS 卡密"}</h2>
          <p className="intro">
            {
              "获取专属链接后，前往兑换站核对账号并确认充值。完成后可回到这里查看结果。"
            }
          </p>
          <form onSubmit={createLink}>
            <label htmlFor="partner-card">{"本站卡密"}</label>
            <div className="input-shell">
              <Ticket size={20} />
              <input
                id="partner-card"
                autoComplete="off"
                spellCheck={false}
                value={code}
                disabled={busy}
                placeholder="请输入完整 PAIPLUS 卡密"
                onChange={(D) => {
                  generation.current++;
                  activeCode.current = "";
                  setCode(D.target.value);
                  setResult(null);
                  setError("");
                }}
              />
            </div>
            <div className="link-actions">
              <button className="primary" disabled={busy || !code.trim()}>
                {busy ? "正在处理…" : "获取兑换链接"}
                <ArrowUpRight size={17} />
              </button>
              <button
                type="button"
                className="text-button"
                disabled={busy || !code.trim()}
                onClick={findRecord}
              >
                {"查询原兑换记录"}
              </button>
            </div>
          </form>
          <LinkError>{error}</LinkError>
          <LinkResult
            result={result}
            mock={config.mode === "mock"}
            busy={busy}
            onRefresh={refresh}
            onRetry={() => createLink(null, true)}
          />
          {config.mode === "mock" && (
            <div className="demo-picks">
              <label htmlFor="partner-sample">{"本地模拟卡"}</label>
              <select
                id="partner-sample"
                disabled={busy}
                value=""
                onChange={(D) => {
                  setCode(D.target.value);
                  setResult(null);
                  activeCode.current = "";
                  generation.current++;
                }}
              >
                <option value="">{"选择测试卡密"}</option>
                {config.examples.map((D) => (
                  <option value={D.code} key={D.code}>
                    {D.label}
                  </option>
                ))}
              </select>
            </div>
          )}
          <div className="quiet-note">
            <ShieldCheck size={15} />
            {"请在兑换站填写账号凭证；本站无需收集 Session。"}
          </div>
        </section>
        {!!config.recent.length && (
          <section className="history-panel glass">
            <h3>{"本浏览器的兑换记录"}</h3>
            {config.recent.slice(0, 6).map((D) => (
              <button
                disabled={busy}
                onClick={() => openRecent(D)}
                key={D.attempt_id}
              >
                <span>
                  <strong>{D.code}</strong>
                  <small>{formatDate(D.created_at)}</small>
                </span>
                <span className="status">{linkDisplayLabel(D.state)}</span>
              </button>
            ))}
          </section>
        )}
      </>
    )
  );
}
function LinkBatch({ config: config, onChange: onChange }) {
  const [text, setText] = React.useState(""),
    [rows, setRows] = React.useState([]),
    [busy, setBusy] = React.useState(false),
    [error, setError] = React.useState(""),
    mounted = React.useRef(true),
    stopped = React.useRef(false);
  React.useEffect(
    () => (
      (mounted.current = true),
      () => {
        mounted.current = false;
        stopped.current = true;
      }
    ),
    [],
  );
  async function importFile(q) {
    var D;
    const Y = (D = q.target.files) == null ? void 0 : D[0];
    if (((q.target.value = ""), !!Y)) {
      setError("");
      setRows([]);
      try {
        if (Y.size > 2 * 1024 * 1024)
          throw new Error("表格请控制在 2 MB 以内。");
        const Z = await import("xlsx"),
          L = Z.read(await Y.arrayBuffer(), {
            type: "array",
          }),
          le = L.Sheets[L.SheetNames[0]];
        if (Object.entries(le).some(([G, ae]) => !G.startsWith("!") && ae.f))
          throw new Error("请使用纯文本卡密，表格不能包含公式。");
        const ue = parseLinkRows(
          Z.utils.sheet_to_json(le, {
            header: 1,
            blankrows: true,
            defval: "",
            raw: false,
          }),
        );
        mounted.current &&
          (setRows(ue),
          setText(
            ue.map((G) => G.code).join(`
`),
          ));
      } catch (Z) {
        mounted.current && setError(Z.message);
      }
    }
  }
  async function downloadTemplate() {
    const q = await import("xlsx"),
      Y = q.utils.book_new();
    q.utils.book_append_sheet(Y, q.utils.aoa_to_sheet([["卡密"]]), "兑换卡密");
    q.writeFile(Y, "PAIPLUS链接兑换模板.xlsx");
  }
  async function createLinks(q) {
    q.preventDefault();
    let Y;
    try {
      Y = parseLinkRows(text.split(/\r?\n/).map((D) => [D]));
    } catch (D) {
      setError(D.message);
      return;
    }
    stopped.current = false;
    setBusy(true);
    setError("");
    setRows(Y);
    for (let D = 0; D < Y.length && !stopped.current; D++) {
      try {
        const Z = await api("partner-link", {
          code: Y[D].code,
        });
        Y = Y.map((L, le) =>
          le === D
            ? {
                ...L,
                result: Z,
                state: Z.state,
              }
            : L,
        );
      } catch (Z) {
        Y = Y.map((L, le) =>
          le === D
            ? {
                ...L,
                error: Z.message,
                state: "error",
              }
            : L,
        );
      }
      mounted.current && setRows(Y);
    }
    mounted.current && (setBusy(false), onChange());
  }
  async function refreshRow(q) {
    setBusy(true);
    setError("");
    try {
      const Y = await api("result-by-code", {
        code: rows[q].code,
      });
      mounted.current &&
        setRows((D) =>
          D.map((Z, L) =>
            L === q
              ? {
                  ...Z,
                  result: mergeLinkResult(Z.result, Y),
                  state: Y.state,
                  error: "",
                }
              : Z,
          ),
        );
    } catch (Y) {
      mounted.current && setError(Y.message);
    } finally {
      mounted.current && setBusy(false);
    }
  }
  return (
    <section className="form-panel glass">
      <div className="kicker">
        <Ticket size={16} />
        {"批量链接兑换"}
      </div>
      <h2>{"每张卡密，一个专属链接"}</h2>
      <p className="intro">
        {
          "最多 100 张，每行填写一张 PAIPLUS 卡密。获取链接后逐一打开，在兑换站确认对应账号。"
        }
      </p>
      <form onSubmit={createLinks}>
        <label htmlFor="partner-batch">{"本站卡密列表"}</label>
        <textarea
          id="partner-batch"
          value={text}
          disabled={busy}
          onChange={(q) => {
            setText(q.target.value);
            setRows([]);
          }}
          autoComplete="off"
          spellCheck={false}
          placeholder="每行一张 PAIPLUS 卡密，无需填写 Session"
        />
        <div className="link-actions">
          <button className="primary" disabled={busy || !text.trim()}>
            {"获取本批链接"}
          </button>
          <label className="secondary file-pick">
            {"导入 Excel / CSV"}
            <input
              type="file"
              accept=".xlsx,.xls,.csv"
              onChange={importFile}
              disabled={busy}
            />
          </label>
          <button
            type="button"
            className="text-button"
            disabled={busy}
            onClick={downloadTemplate}
          >
            <Download size={15} />
            {"下载模板"}
          </button>
          {busy && (
            <button
              type="button"
              className="text-button"
              onClick={() => {
                stopped.current = true;
              }}
            >
              {"暂停后续处理"}
            </button>
          )}
        </div>
      </form>
      <LinkError>{error}</LinkError>
      {rows.map((q, Y) => (
        <div className="batch-link-row" key={q.code}>
          <h3>
            {"第 "}
            {q.row}
            {" 行 · "}
            {q.code.slice(0, 7)}
            {"…"}
            {q.code.slice(-4)}
          </h3>
          <LinkError>{q.error}</LinkError>
          {q.result ? (
            <LinkResult
              result={q.result}
              mock={config.mode === "mock"}
              busy={busy}
              onRefresh={() => refreshRow(Y)}
            />
          ) : (
            <p>{linkDisplayLabel(q.state)}</p>
          )}
        </div>
      ))}
    </section>
  );
}
function LinkBilling() {
  return (
    <section className="form-panel glass">
      <div className="kicker">
        <ShieldCheck size={16} />
        {"账单查询"}
      </div>
      <h2>{"请在账号订阅页面查看账单"}</h2>
      <p className="intro">
        {
          "当前兑换服务提供兑换进度查询，尚未提供账单查询。你可以使用本站卡密查询兑换结果，或在账号的官方订阅页面查看付款与发票。"
        }
      </p>
    </section>
  );
}

export { LinkRecharge, LinkBatch, LinkBilling };
