// Reconstructed from the owned production bundle (20261115-235900).
// SHA256: a5a243964836d961a368c4590f654e22259740c4ca6289b77ed62657a587d74e
// Original Git/source file names were unavailable; see docs/wudi-source-recovery.md.
import React from "react";
import { CircleAlert, RotateCcw, Ticket, ShieldCheck } from "lucide-react";
import { api, statusLabel, extractCard, formatDate } from "./api.js";
import { parseLinkRows } from "./link-flow.js";

const isFinished = (f) =>
    [
      "succeeded",
      "completed",
      "failed",
      "cancelled",
      "expired",
      "card_unavailable",
      "revoked",
      "invalid",
    ].includes(f),
  yesNo = (f) => (f === true ? "是" : f === false ? "否" : "未知"),
  displayValue = (f) => (f == null || f === "" ? "暂无" : String(f)),
  wudiStatusLabel = (f) =>
    ({
      link_pending: "正在验卡",
      expired: "确认授权已过期",
      ready: "预检通过",
      success: "查询完成",
    })[f] || statusLabel(f);
function WudiError({ children: children }) {
  return children ? (
    <p className="error" role="alert">
      <CircleAlert size={16} />
      {children}
    </p>
  ) : null;
}
function useRequest() {
  const mounted = React.useRef(true),
    generation = React.useRef(0),
    [busy, setBusy] = React.useState(false),
    [error, setError] = React.useState("");
  return (
    React.useEffect(
      () => (
        (mounted.current = true),
        () => {
          mounted.current = false;
          generation.current++;
        }
      ),
      [],
    ),
    {
      busy: busy,
      error: error,
      run: async (operation, accept) => {
        const version = ++generation.current;
        setBusy(true);
        setError("");
        try {
          const response = await operation();
          mounted.current && version === generation.current && accept(response);
        } catch (error) {
          mounted.current &&
            version === generation.current &&
            setError(error.message);
        } finally {
          mounted.current && version === generation.current && setBusy(false);
        }
      },
      invalidate: () => {
        generation.current++;
        setBusy(false);
        setError("");
      },
      setError: setError,
      generation: generation,
    }
  );
}
function SubscriptionDetails({ data: data }) {
  var T, d;
  return data ? (
    <section className="wudi-subscription" aria-label="订阅详情">
      <h3>{"订阅详情"}</h3>
      <p>
        {"账号邮箱："}
        <strong>{displayValue(data.email)}</strong>
      </p>
      <p>
        {"账号数量："}
        {displayValue(data.accountCount)}
        {"　查询时间："}
        {formatDate(data.queriedAt)}
      </p>
      {(T = data.accounts) == null
        ? void 0
        : T.map((r, v) => (
            <div className="wudi-account" key={v}>
              <h4>
                {r.selected ? "本次充值账号" : "账号 " + (v + 1)}
                {" · "}
                {displayValue(r.plan || r.planType)}
              </h4>
              <dl>
                {[
                  ["账号", displayValue(r.accountId)],
                  ["订阅套餐", displayValue(r.subscriptionPlan || r.plan)],
                  ["订阅来源", displayValue(r.source)],
                  ["订阅状态", displayValue(r.subscriptionStatus)],
                  ["是否有效", yesNo(r.active)],
                  ["币种", displayValue(r.currency)],
                  ["开始时间", formatDate(r.startedAt)],
                  ["到期时间", formatDate(r.expiresAt)],
                  ["自动续费", yesNo(r.willRenew)],
                  ["半价资格", yesNo(r.plusPromoEligible)],
                  ["优惠 ID", displayValue(r.campaignId)],
                  ["取消结果", displayValue(r.cancellation)],
                  ["Pro 权限", yesNo(r.pro20xEligible)],
                ].map(([A, j]) => (
                  <React.Fragment key={A}>
                    <dt>{A}</dt>
                    <dd>{j}</dd>
                  </React.Fragment>
                ))}
              </dl>
            </div>
          ))}
      {!((d = data.accounts) != null && d.length) && (
        <p>{"暂无账号订阅详情。"}</p>
      )}
    </section>
  ) : null;
}
function WudiResult({ result: result }) {
  return result ? (
    <section className="wudi-result" aria-live="polite">
      <div className="link-result-heading">
        <strong>{wudiStatusLabel(result.state)}</strong>
        <span>{result.code}</span>
      </div>
      <p>{result.message || "请继续查询原兑换记录。"}</p>
      {result.card && (
        <dl>
          <dt>{"兑换套餐"}</dt>
          <dd>{displayValue(result.card.plan_name)}</dd>
          <dt>{"地区"}</dt>
          <dd>{displayValue(result.card.region_name || result.card.region)}</dd>
          <dt>{"需要半价资格"}</dt>
          <dd>{yesNo(result.card.require_plus_promo)}</dd>
        </dl>
      )}
      {result.email && (
        <p>
          {"账号："}
          {result.email}
        </p>
      )}
      {result.expires_at && (
        <p className="field-hint">
          {"确认授权截止："}
          {formatDate(result.expires_at)}
        </p>
      )}
      <p className="field-hint">
        {"创建时间："}
        {formatDate(result.created_at)}
        {"　更新时间："}
        {formatDate(result.updated_at)}
      </p>
      <WudiError>{result.query_error}</WudiError>
      <WudiError>{result.preflight_message}</WudiError>
      {result.error_code && (
        <p className="field-hint">
          {"原因："}
          {result.error_code}
        </p>
      )}
      {result.redeem_url && result.can_open && (
        <a
          className="secondary"
          href={result.redeem_url}
          target="_blank"
          rel="noopener noreferrer"
          referrerPolicy="no-referrer"
        >
          {"继续原专属链接"}
        </a>
      )}
    </section>
  ) : null;
}
function WudiRedeem({
  config: config,
  onChange = () => {},
  promo = false,
  initialCode = "",
  onResult = () => {},
}) {
  var pe, Ee, K;
  const [code, setCode] = React.useState(initialCode),
    [result, setResult] = React.useState(null),
    [session, setSession] = React.useState(""),
    [acknowledged, setAcknowledged] = React.useState(false),
    [confirmed, setConfirmed] = React.useState(false),
    [now, setNow] = React.useState(Date.now()),
    activeCode = React.useRef(""),
    request = useRequest(),
    acceptResult = (W) => {
      setResult(W);
      onResult(W);
    };
  React.useEffect(() => {
    const W = setInterval(() => setNow(Date.now()), 1e3);
    return () => clearInterval(W);
  }, []);
  const reset = () => {
    request.invalidate();
    activeCode.current = "";
    setResult(null);
    setSession("");
    setAcknowledged(false);
    setConfirmed(false);
  };
  async function validateCard(W, oe = false) {
    W == null || W.preventDefault();
    let E;
    try {
      E = extractCard(code);
    } catch (X) {
      request.setError(X.message);
      return;
    }
    setSession("");
    setAcknowledged(false);
    setConfirmed(false);
    setResult(null);
    activeCode.current = E;
    await request.run(
      () =>
        api(oe ? "partner-retry" : "partner-link", {
          code: E,
          ...(oe
            ? {
                confirmed: true,
              }
            : {}),
        }),
      (X) => {
        acceptResult(X);
        onChange();
      },
    );
  }
  async function checkAccount(W) {
    W.preventDefault();
    const oe = {
      code: activeCode.current,
      attempt_id: result.attempt_id,
      session: session,
      acknowledged: acknowledged,
    };
    setConfirmed(false);
    await request.run(
      () => api("partner-check", oe),
      (E) => {
        setSession("");
        acceptResult(E);
      },
    );
  }
  async function confirmRedemption() {
    const W = {
      code: activeCode.current,
      attempt_id: result.attempt_id,
      confirmation: result.confirmation,
      confirmed: confirmed,
    };
    setSession("");
    setConfirmed(false);
    await request.run(
      () => api("partner-confirm", W),
      (oe) => {
        acceptResult(oe);
        onChange();
      },
    );
  }
  const refresh = () =>
      request.run(
        () =>
          api("result-by-code", {
            code: activeCode.current,
          }),
        (W) => acceptResult(W),
      ),
    checkingAccount = ["queued", "running"].includes(
      (pe = result == null ? void 0 : result.preflight) == null
        ? void 0
        : pe.status,
    );
  React.useEffect(() => {
    if (
      !(result != null && result.attempt_id) ||
      !activeCode.current ||
      isFinished(result.state) ||
      request.busy
    )
      return;
    let W = false,
      oe;
    const E = request.generation.current,
      X = activeCode.current,
      ee = result.attempt_id,
      ge = async () => {
        try {
          const be = await api(
            checkingAccount ? "partner-check-status" : "result-by-code",
            checkingAccount
              ? {
                  code: X,
                  attempt_id: ee,
                }
              : {
                  code: X,
                },
          );
          !W &&
            E === request.generation.current &&
            (setResult((h) =>
              checkingAccount
                ? be
                : {
                    ...be,
                    ...(be.can_check && h != null && h.preflight
                      ? {
                          preflight: h.preflight,
                          confirmation: h.confirmation,
                        }
                      : {}),
                  },
            ),
            request.setError(""));
        } catch (be) {
          !W &&
            E === request.generation.current &&
            request.setError(be.message);
        }
        W || (oe = setTimeout(ge, 4e3));
      };
    return (
      (oe = setTimeout(ge, 4e3)),
      () => {
        W = true;
        clearTimeout(oe);
      }
    );
  }, [
    result == null ? void 0 : result.attempt_id,
    result == null ? void 0 : result.state,
    (Ee = result == null ? void 0 : result.preflight) == null
      ? void 0
      : Ee.status,
    request.busy,
  ]);
  const canConfirm =
    ((K = result == null ? void 0 : result.preflight) == null
      ? void 0
      : K.status) === "ready" &&
    result.confirmation &&
    Date.parse(result.preflight.expiresAt) > now &&
    result.can_check;
  return (
    <section className="form-panel glass wudi-flow">
      <div className="kicker">
        <Ticket size={16} />
        {promo ? "半价兑换" : "兑换套餐"}
      </div>
      <h2>{"验证卡密，核对后兑换"}</h2>
      <p className="intro">
        {"在本站完成验卡、账号预检和确认。"}
        {promo
          ? "是否需要半价资格，以卡密实际套餐及预检结果为准。"
          : "请使用订单交付的 PAIPLUS 卡密。"}
      </p>
      <ol className="stepper">
        {["验证卡密", "账号预检", "确认兑换", "查询结果"].map((W, oe) => (
          <li
            className={
              oe ===
              (canConfirm
                ? 2
                : result != null && result.can_check
                  ? 1
                  : result
                    ? 3
                    : 0)
                ? "active"
                : ""
            }
            key={W}
          >
            <span className="step-number">{oe + 1}</span>
            <span>{W}</span>
          </li>
        ))}
      </ol>
      <form onSubmit={validateCard}>
        <label htmlFor="wudi-card">{"本站卡密"}</label>
        <input
          id="wudi-card"
          autoComplete="off"
          spellCheck={false}
          value={code}
          disabled={request.busy}
          onChange={(W) => {
            reset();
            setCode(W.target.value);
          }}
          placeholder="PAIPLUS-…"
        />
        <button className="primary" disabled={request.busy || !code.trim()}>
          {request.busy ? "正在处理…" : "验证卡密"}
        </button>
      </form>
      {(config == null ? void 0 : config.mode) === "mock" && (
        <select
          aria-label="模拟卡密"
          value=""
          disabled={request.busy}
          onChange={(W) => {
            reset();
            setCode(W.target.value);
          }}
        >
          <option value="">{"选择模拟卡密"}</option>
          {config.examples.map((W) => (
            <option value={W.code} key={W.code}>
              {W.label}
            </option>
          ))}
        </select>
      )}
      <WudiError>{request.error}</WudiError>
      <WudiResult result={result} />
      {(result == null ? void 0 : result.can_check) && (
        <form onSubmit={checkAccount} className="wudi-check">
          <label htmlFor="wudi-session">{"完整 Session JSON"}</label>
          <textarea
            id="wudi-session"
            autoComplete="off"
            spellCheck={false}
            value={session}
            disabled={request.busy || checkingAccount}
            placeholder="粘贴完整 Session JSON 对象"
            onChange={(W) => {
              request.invalidate();
              setSession(W.target.value);
              setConfirmed(false);
              setResult((oe) => ({
                ...oe,
                preflight: null,
                confirmation: null,
              }));
            }}
          />
          <p className="field-hint">
            {
              "Session 仅用于本次预检和兑换，不写入浏览器存储。预检后再次兑换须使用同一份 Session。"
            }
          </p>
          <label className="wudi-consent">
            <input
              type="checkbox"
              checked={acknowledged}
              disabled={request.busy || checkingAccount}
              onChange={(W) => setAcknowledged(W.target.checked)}
            />
            {
              "我同意进行账号预检，并知悉已有相同网页订阅时，服务方可能关闭该订阅的自动续费。"
            }
          </label>
          <button
            className="secondary"
            disabled={
              request.busy ||
              checkingAccount ||
              !session.trim() ||
              !acknowledged
            }
          >
            {checkingAccount ? "账号预检中…" : "开始账号预检"}
          </button>
        </form>
      )}
      {(result == null ? void 0 : result.preflight) && (
        <section className="wudi-preflight" aria-live="polite">
          <h3>
            {result.preflight.status === "ready"
              ? "账号预检通过"
              : result.preflight.status === "failed"
                ? "账号预检未通过"
                : "账号预检中"}
          </h3>
          <p>{result.preflight.message}</p>
          {result.preflight.code && (
            <p className="field-hint">{result.preflight.code}</p>
          )}
          <SubscriptionDetails data={result.preflight.subscription} />
          {result.preflight.status === "ready" && !canConfirm && (
            <WudiError>
              {"预检授权已失效，请重新粘贴 Session 并预检。"}
            </WudiError>
          )}
          {canConfirm && (
            <>
              <p className="field-hint">
                {"请在 "}
                {formatDate(result.preflight.expiresAt)}
                {" 前确认。"}
              </p>
              <label className="wudi-consent">
                <input
                  type="checkbox"
                  checked={confirmed}
                  disabled={request.busy}
                  onChange={(W) => setConfirmed(W.target.checked)}
                />
                {"我已核对充值账号、套餐、地区及订阅详情，确认兑换。"}
              </label>
              <button
                className="primary"
                disabled={request.busy || !confirmed}
                onClick={confirmRedemption}
              >
                {"确认兑换"}
              </button>
            </>
          )}
        </section>
      )}
      {result && (
        <div className="link-actions">
          <button
            className="text-button"
            disabled={request.busy || checkingAccount}
            onClick={refresh}
          >
            <RotateCcw size={15} />
            {"刷新原订单"}
          </button>
          {result.can_retry && (
            <button
              className="secondary"
              disabled={request.busy}
              onClick={() => validateCard(null, true)}
            >
              {"重新验证卡密"}
            </button>
          )}
        </div>
      )}
      <div className="quiet-note">
        <ShieldCheck size={15} />
        {"只有你确认后才提交充值。结果待确认时继续查询原订单。"}
      </div>
    </section>
  );
}
function WudiLookup({ orders = false }) {
  var N;
  const [code, setCode] = React.useState(""),
    [results, setResults] = React.useState([]),
    [beforeId, setBeforeId] = React.useState(null),
    request = useRequest(),
    activeCode = React.useRef("");
  React.useEffect(() => {
    var q;
    if (
      orders ||
      !((q = results[0]) != null && q.state) ||
      isFinished(results[0].state) ||
      request.busy ||
      !activeCode.current
    )
      return;
    let _ = false,
      B;
    const w = request.generation.current,
      Q = activeCode.current,
      ne = async () => {
        try {
          const Y = await api("result-by-code", {
            code: Q,
          });
          !_ &&
            w === request.generation.current &&
            (setResults([Y]), request.setError(""));
        } catch (Y) {
          !_ && w === request.generation.current && request.setError(Y.message);
        }
        _ || (B = setTimeout(ne, 4e3));
      };
    return (
      (B = setTimeout(ne, 4e3)),
      () => {
        _ = true;
        clearTimeout(B);
      }
    );
  }, [orders, (N = results[0]) == null ? void 0 : N.state, request.busy]);
  async function query(_, B = false) {
    _ == null || _.preventDefault();
    let w;
    try {
      w = B ? activeCode.current : extractCard(code);
    } catch (Q) {
      request.setError(Q.message);
      return;
    }
    activeCode.current = w;
    B || (setResults([]), setBeforeId(null));
    await request.run(
      () =>
        api(orders ? "partner-orders" : "result-by-code", {
          code: w,
          ...(B
            ? {
                before_id: beforeId,
              }
            : {}),
        }),
      (Q) => {
        setResults((ne) =>
          B ? [...ne, ...Q.orders] : orders ? Q.orders : [Q],
        );
        setBeforeId(Q.next_before_id || null);
      },
    );
  }
  return (
    <section className="form-panel glass wudi-flow">
      <h2>{orders ? "我的订单" : "查询结果"}</h2>
      <p className="intro">
        {"输入完整 PAIPLUS 卡密，可以在其他设备找回记录。查询不会提交充值。"}
      </p>
      <form onSubmit={query}>
        <label htmlFor="wudi-query-card">{"本站卡密"}</label>
        <input
          id="wudi-query-card"
          autoComplete="off"
          value={code}
          disabled={request.busy}
          onChange={(_) => {
            request.invalidate();
            activeCode.current = "";
            setCode(_.target.value);
            setResults([]);
            setBeforeId(null);
          }}
        />
        <button className="primary" disabled={request.busy || !code.trim()}>
          {request.busy ? "正在查询…" : "查询"}
        </button>
      </form>
      <WudiError>{request.error}</WudiError>
      {results.map((_, B) => (
        <WudiResult result={_} key={B} />
      ))}
      {orders &&
        activeCode.current &&
        !request.busy &&
        !results.length &&
        !request.error && <p>{"该卡密暂无本代理的新协议兑换记录。"}</p>}
      {beforeId && (
        <button
          className="secondary"
          disabled={request.busy}
          onClick={() => query(null, true)}
        >
          {"加载更多订单"}
        </button>
      )}
    </section>
  );
}
function WudiSubscription() {
  const [session, setSession] = React.useState(""),
    [query, setQuery] = React.useState(null),
    request = useRequest();
  async function submit(j) {
    j.preventDefault();
    setQuery(null);
    await request.run(
      () =>
        api("subscription-query", {
          session: session,
        }),
      (S) => {
        setSession("");
        setQuery(S);
      },
    );
  }
  return (
    React.useEffect(() => {
      if (
        !(query != null && query.query_id) ||
        ["success", "failed"].includes(query.status) ||
        request.busy
      )
        return;
      let j = false,
        S;
      const O = request.generation.current,
        p = async () => {
          try {
            const N = await api("subscription-status", {
              query_id: query.query_id,
            });
            !j &&
              O === request.generation.current &&
              (setQuery(N), request.setError(""));
          } catch (N) {
            !j &&
              O === request.generation.current &&
              (request.setError(N.message),
              N.code === "QUERY_EXPIRED" && setQuery(null));
          }
          j || (S = setTimeout(p, 4e3));
        };
      return (
        (S = setTimeout(p, 4e3)),
        () => {
          j = true;
          clearTimeout(S);
        }
      );
    }, [
      query == null ? void 0 : query.query_id,
      query == null ? void 0 : query.status,
      request.busy,
    ]),
    (
      <section className="form-panel glass wudi-flow">
        <h2>{"订阅查询"}</h2>
        <p className="intro">
          {"无需卡密，查询当前 ChatGPT 订阅和半价资格。不会提交兑换。"}
        </p>
        <form onSubmit={submit}>
          <label htmlFor="wudi-subscription-session">
            {"完整 Session JSON"}
          </label>
          <textarea
            id="wudi-subscription-session"
            value={session}
            autoComplete="off"
            spellCheck={false}
            disabled={request.busy}
            onChange={(j) => {
              request.invalidate();
              setSession(j.target.value);
              setQuery(null);
            }}
          />
          <button
            className="primary"
            disabled={request.busy || !session.trim()}
          >
            {request.busy ? "正在提交查询…" : "查询订阅"}
          </button>
        </form>
        <WudiError>{request.error}</WudiError>
        {query && <p aria-live="polite">{query.message}</p>}
        <SubscriptionDetails data={query == null ? void 0 : query.result} />
        <p className="field-hint">
          {
            "查询有效期 10 分钟；过期或服务重启后请重新查询。Session 不会保存到浏览器存储。"
          }
        </p>
      </section>
    )
  );
}
function WudiBatch(props) {
  const [text, setText] = React.useState(""),
    [rows, setRows] = React.useState([]),
    [selected, setSelected] = React.useState(null),
    [error, setError] = React.useState(""),
    readCards = (_) => {
      _ == null || _.preventDefault();
      try {
        const B = parseLinkRows(text.split(/\r?\n/).map((w) => [w]));
        setRows(B);
        setSelected(null);
        setError("");
      } catch (B) {
        setError(B.message);
      }
    };
  async function importFile(_) {
    var w;
    const B = (w = _.target.files) == null ? void 0 : w[0];
    if (((_.target.value = ""), !!B))
      try {
        if (B.size > 2 * 1024 * 1024) throw Error("表格请控制在 2 MB 以内。");
        const Q = await import("xlsx"),
          ne = Q.read(await B.arrayBuffer(), {
            type: "array",
          }),
          q = ne.Sheets[ne.SheetNames[0]];
        if (Object.entries(q).some(([D, Z]) => !D.startsWith("!") && Z.f))
          throw Error("请导入纯文本卡密，不要使用公式。");
        const Y = parseLinkRows(
          Q.utils.sheet_to_json(q, {
            header: 1,
            blankrows: true,
            defval: "",
            raw: false,
          }),
        );
        setText(
          Y.map((D) => D.code).join(`
`),
        );
        setRows(Y);
        setSelected(null);
        setError("");
      } catch (Q) {
        setError(Q.message);
      }
  }
  return (
    <>
      <section className="form-panel glass wudi-flow">
        <h2>{"批量兑换"}</h2>
        <p className="intro">
          {
            "最多导入 100 张卡密，每张分别核对账号和确认。上游每分钟允许 4 次账号预检，请按页面提示操作。"
          }
        </p>
        <form onSubmit={readCards}>
          <label htmlFor="wudi-batch">{"每行一张本站卡密"}</label>
          <textarea
            id="wudi-batch"
            value={text}
            onChange={(_) => {
              setText(_.target.value);
              setRows([]);
              setSelected(null);
            }}
          />
          <button className="secondary">{"读取卡密"}</button>
          <label className="text-button">
            {"导入单列卡密表格"}
            <input type="file" accept=".xlsx,.xls,.csv" onChange={importFile} />
          </label>
        </form>
        <WudiError>{error}</WudiError>
        <div className="wudi-batch-rows">
          {rows.map((_, B) => (
            <button
              className="secondary"
              onClick={() => setSelected(B)}
              key={_.code}
            >
              {"第 "}
              {B + 1}
              {" 张 · …"}
              {_.code.slice(-4)}
              {" · "}
              {_.state ? wudiStatusLabel(_.state) : "核对并兑换"}
            </button>
          ))}
        </div>
      </section>
      {selected !== null &&
        React.createElement(WudiRedeem, {
          ...props,
          key: rows[selected].code,
          initialCode: rows[selected].code,
          onResult: (_) =>
            setRows((B) =>
              B.map((w, Q) =>
                Q === selected
                  ? {
                      ...w,
                      state: _.state,
                    }
                  : w,
              ),
            ),
        })}
    </>
  );
}
function WudiV2Flow({ tool: tool, ...props }) {
  return tool === "lookup" ? (
    <WudiLookup />
  ) : tool === "orders" ? (
    <WudiLookup orders={true} />
  ) : tool === "billing" ? (
    <WudiSubscription />
  ) : tool === "batch" ? (
    <WudiBatch {...props} />
  ) : (
    <WudiRedeem {...props} promo={tool === "promo"} />
  );
}

export { WudiV2Flow };
