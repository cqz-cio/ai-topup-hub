<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import { useHead } from '@unhead/vue'
import { Check, ArrowRight, ArrowLeft, Info, Sparkles, LoaderCircle, CircleAlert, CircleCheck, X } from 'lucide-vue-next'
import { useRedemption } from '../composables/useRedemption'
import { needsAccountCheck, planLabel, shouldPoll, statusPresentation, subscriptionLabel } from '../utils/redemption'

const { codeInput, activeCode, sessionInput, authorized, confirmed, record, proof, step, busy, error, notice,
  uncertain, proofValid, canConfirm, canRefresh, maskedCode, lookup, checkAccount, editAccount, confirmRecharge, refresh, reset } = useRedemption()
const help = ref<HTMLDialogElement | null>(null)
const heading = ref<HTMLElement | null>(null)
const steps = ['验证卡密', '核验账号', '确认充值', '查看结果']
const product = computed(() => record.value ? planLabel(record.value.plan) : '等待验证卡密')
const status = computed(() => uncertain.value
  ? { title: '正在核实提交状态', detail: '连接中断不代表提交失败。正在查询原兑换记录，请勿再次提交充值。', tone: 'warning' }
  : statusPresentation(record.value?.state || ''))
const redemptionLabel = computed(() => {
  if (step.value === 1) return '待验证'
  if (step.value === 2) return '待核验账号'
  if (step.value === 3) return proofValid.value ? '待确认' : '核验已过期'
  return record.value?.state === 'completed' ? '已兑换' : status.value.title
})
const canRecheck = computed(() => step.value === 4 && !uncertain.value && needsAccountCheck(record.value?.state || ''))
useHead({ title: '自助兑换中心 · AI Topup Hub', meta: [{ name: 'robots', content: 'noindex, nofollow' }, { name: 'referrer', content: 'no-referrer' }] })
watch(step, async () => { await nextTick(); heading.value?.focus({ preventScroll: true }) })
</script>

<template>
  <div class="redeem-page">
    <header class="redeem-header">
      <router-link to="/" class="brand" aria-label="AI Topup Hub 返回商城"><i>A</i><strong>AI Topup Hub</strong><small>自助兑换中心</small></router-link>
      <nav aria-label="兑换导航"><router-link to="/">返回商城</router-link><button type="button" @click="help?.showModal()">兑换帮助</button></nav>
    </header>
    <main class="redeem-main">
      <div class="intro"><div class="eyebrow">SELF-SERVICE REDEMPTION</div><h1>兑换你的 GPT 订阅</h1><p>核验卡密，确认账号，随时查看充值进度。</p></div>
      <ol class="steps" aria-label="兑换步骤">
        <li v-for="(label, index) in steps" :key="label" :class="{ done: step > index + 1, now: step === index + 1 }" :aria-current="step === index + 1 ? 'step' : undefined">
          <span class="circle"><Check v-if="step > index + 1" :size="13" aria-hidden="true" /><template v-else>{{ index + 1 }}</template></span><span>{{ label }}</span>
        </li>
      </ol>
      <div class="redeem-grid">
        <section class="redeem-card main-card" :aria-busy="!!busy">
          <template v-if="step === 1">
            <div class="cardhead"><div><h2 ref="heading" tabindex="-1">验证你的兑换卡密</h2><p class="note">输入购卡后收到的卡密，开始兑换或查询进度。</p></div></div>
            <form autocomplete="off" @submit.prevent="lookup">
              <label for="redemption-code" class="field-label">兑换卡密</label>
              <input id="redemption-code" v-model="codeInput" name="redemption-code" class="text-input code-input" type="text" maxlength="64" autocomplete="off" autocapitalize="characters" :spellcheck="false" placeholder="XXXXX-XXXXX-XXXXX-XXXXX-XXXXX" :disabled="!!busy" aria-describedby="code-hint" :aria-invalid="!!error" />
              <p id="code-hint" class="input-hint">共 5 组，每组 5 位；小写或不带横线也可验证。</p>
              <div class="entry-note"><Info :size="18" aria-hidden="true"/><p>已提交过充值？输入原卡密即可查询进度，无需再次购买。</p></div>
              <p v-if="error" class="error" role="alert">{{ error }}</p>
              <button class="primary" type="submit" :disabled="!!busy || !codeInput.trim()"><LoaderCircle v-if="busy" class="spin" :size="17"/><template v-else>验证卡密</template><span v-if="busy">正在验证</span><ArrowRight v-else :size="16"/></button>
              <p class="button-note">验证卡密不会开始充值。</p>
            </form>
          </template>

          <template v-else-if="step === 2">
            <div class="cardhead"><div><h2 ref="heading" tabindex="-1">核验你的 GPT 账号</h2><p class="note">此卡密可兑换 {{ product }}，套餐已由购买订单锁定。请确认与所购套餐一致，再核验账号当前订阅；如不一致，请先联系商城客服。</p></div><span class="tag"><Check :size="12"/>卡密有效</span></div>
            <form autocomplete="off" @submit.prevent="checkAccount">
              <label for="redemption-session" class="field-label">账号 Session</label>
              <textarea id="redemption-session" v-model="sessionInput" class="text-input session-input" rows="6" maxlength="65536" autocomplete="off" autocapitalize="off" :spellcheck="false" :disabled="!!busy" placeholder="粘贴完整的 Session JSON" aria-describedby="session-hint" :aria-invalid="!!error" />
              <p id="session-hint" class="input-hint">需包含 accessToken 和 sessionToken。核验后将清空输入框。</p>
              <button class="text-button small" type="button" @click="help?.showModal()">了解账号核验与授权</button>
              <label class="consent"><input v-model="authorized" type="checkbox" :disabled="!!busy"/><span>我有权使用此账号，并授权核验其订阅信息及在后续确认后进行充值。</span></label>
              <p v-if="error" class="error" role="alert">{{ error }}</p>
              <button class="primary" type="submit" :disabled="!!busy || !authorized || !sessionInput.trim()"><LoaderCircle v-if="busy" class="spin" :size="17"/>{{ busy ? '正在核验账号' : '核验账号信息' }}<ArrowRight v-if="!busy" :size="16"/></button>
              <p class="button-note">此步骤仅核验账号，确认充值后才开始处理。</p>
            </form>
          </template>

          <template v-else-if="step === 3">
            <div class="cardhead"><div><h2 ref="heading" tabindex="-1">确认充值账号与套餐</h2><p class="note">请同时核对目标账号和本次充值套餐，确认后将开始处理。</p></div><span class="tag" :class="{ expired: !proofValid }">{{ proofValid ? '核验通过' : '核验已过期' }}</span></div>
            <div class="account"><span class="label">已核验的账号标识</span><div class="mono account-id">{{ proof?.account?.account_id }}</div><div class="fields"><div><span class="label">检测到的当前订阅</span><b>{{ subscriptionLabel(proof?.account?.current_plan || '') }} · 符合新开通条件</b></div><div><span class="label">本次充值套餐（卡密绑定）</span><b>{{ product }}</b></div></div></div>
            <p class="note" :class="{ 'expired-note': !proofValid }" aria-live="polite">{{ proofValid ? '核验结果在 10 分钟内有效。请确认这是你要充值的账号。' : '核验结果已过期，请重新核验账号后再确认。' }}</p>
            <button class="text-button change" type="button" :disabled="!!busy" @click="editAccount">{{ proofValid ? '账号不对？返回重新核验' : '重新核验账号' }} <ArrowRight :size="14"/></button>
            <div class="divider"></div><div class="row"><span>卡密状态</span><strong class="valid">有效，尚未兑换</strong></div><div class="row"><span>兑换方式</span><span>充值至上方已核验账号</span></div>
            <form @submit.prevent="confirmRecharge"><label class="consent"><input v-model="confirmed" type="checkbox" :disabled="!!busy || !proofValid"/><span>我已核对目标账号，确认 {{ product }} 与所购套餐一致，并使用此卡密为该账号开通该套餐。</span></label>
              <p v-if="error" class="error" role="alert">{{ error }}</p>
              <button class="primary" type="submit" :disabled="!canConfirm"><LoaderCircle v-if="busy" class="spin" :size="17"/>{{ busy ? '正在提交，请勿重复操作' : '确认账号与套餐并充值' }}<ArrowRight v-if="!busy" :size="16"/></button><p class="button-note">确认后请勿重复提交，可随时回来查询进度。</p>
            </form>
          </template>

          <template v-else>
            <div class="cardhead"><div><h2 ref="heading" tabindex="-1">你的充值进度</h2><p class="note">当前显示原卡密对应的兑换记录。</p></div><span v-if="record?.state === 'completed'" class="tag">已完成</span></div>
            <div class="result" :class="status.tone" aria-live="polite"><div class="result-icon"><CircleCheck v-if="status.tone === 'success'" :size="36"/><CircleAlert v-else-if="status.tone === 'warning'" :size="36"/><LoaderCircle v-else :size="36" class="spin"/></div><h3>{{ status.title }}</h3><p>{{ status.detail }}</p></div>
            <div class="divider"></div><div class="row"><span>兑换套餐</span><strong>{{ product }}</strong></div><div class="row"><span>卡密状态</span><span>{{ record?.state === 'completed' ? '已核销' : '保留原兑换记录' }}</span></div>
            <p v-if="notice" class="error" role="status">{{ notice }}</p>
            <button v-if="canRecheck" type="button" class="primary" :disabled="!!busy" @click="editAccount">重新核验账号 <ArrowRight :size="16"/></button>
            <button v-else type="button" class="primary" :disabled="!canRefresh" @click="refresh()"><LoaderCircle v-if="busy" :size="17" class="spin"/>{{ busy ? '正在查询' : record?.state === 'completed' ? '刷新兑换结果' : '刷新充值进度' }}</button>
            <p class="button-note">{{ shouldPoll(record?.state || '') || uncertain ? '页面打开时会自动更新；离开后可凭卡密继续查询。' : '有疑问请联系商城客服，提供原订单信息。' }}</p>
          </template>
        </section>

        <aside class="redeem-card side-card"><div class="subheading">你的兑换凭证</div><div class="product"><span class="producticon"><Sparkles :size="22"/></span><div><b>{{ product }}</b><p>{{ record ? '订阅新开通' : '套餐信息将在验证后显示' }}</p></div></div><div class="divider"></div><span class="label">{{ activeCode ? '已验证的卡密' : '兑换卡密格式' }}</span><div class="code">{{ maskedCode || 'XXXXX-XXXXX-XXXXX-XXXXX-XXXXX' }}</div><div class="row"><span>兑换状态</span><span>{{ redemptionLabel }}</span></div><div class="aside-help">请保管好兑换卡密。充值后可以凭卡密查询结果，无需再次购买。</div><button v-if="activeCode" class="text-button switch-code" type="button" :disabled="!!busy" @click="reset"><ArrowLeft :size="14"/>查询其他卡密</button></aside>
      </div>
      <div class="help-strip"><Info :size="19" aria-hidden="true"/><div><b>充值期间可以离开页面</b><p>我们会保存处理进度。若需要等待，请查询原兑换记录，遇到问题可联系商城客服。</p></div></div>
      <footer class="redeem-footer"><span>AI Topup Hub · 自助兑换中心</span><div><button type="button" @click="help?.showModal()">兑换帮助</button><span>/</span><router-link to="/">返回商城联系客服</router-link></div></footer>
    </main>
    <dialog ref="help" class="help-dialog" aria-labelledby="help-title"><div class="dialog-head"><h2 id="help-title">兑换帮助</h2><button type="button" class="icon-button" aria-label="关闭兑换帮助" @click="help?.close()"><X :size="20"/></button></div><ol><li><b>输入卡密</b><p>从已付款订单的交付内容中复制卡密，验证后可查看对应套餐。</p></li><li><b>核验账号</b><p>按商城提供的操作指南获取你有权使用的完整 Session JSON。此处只接收账号授权资料，不需要银行卡或账号密码。</p></li><li><b>确认后充值</b><p>核对服务方返回的账号标识及订阅状态，勾选确认后才会开始处理。已有订阅或核验不完整时，请联系商城客服。</p></li><li><b>查询结果</b><p>刷新页面后重新输入原卡密即可查询，不需要重新购买。结果不明或需要协助时，请联系购卡商城核实原记录。</p></li></ol><button type="button" class="primary" @click="help?.close()">知道了</button></dialog>
  </div>
</template>

<style scoped>
.redeem-page{--blue:#0071e3;--ink:#1d1d1f;--muted:#6e6e73;--line:#e5e7ec;--green:#24886b;min-height:100vh;background:#f5f5f7;color:var(--ink);font:15px/1.65 "Microsoft YaHei",system-ui,sans-serif;color-scheme:light}
.redeem-page *{box-sizing:border-box}.redeem-page button,.redeem-page input,.redeem-page textarea{font:inherit}.redeem-page a{color:inherit;text-decoration:none}.redeem-page button{cursor:pointer}.redeem-page button:disabled{cursor:not-allowed}.redeem-page :focus-visible{outline:3px solid #0071e355;outline-offset:4px}.redeem-page h2[tabindex]{outline:none}.redeem-header{height:80px;background:#fff;border-bottom:1px solid var(--line);display:flex;align-items:center;justify-content:space-between;padding:0 6.5%}.brand{display:flex;align-items:center;gap:12px;font-size:19px}.brand strong{font-weight:700}.brand i{font-style:normal;display:grid;place-items:center;width:36px;height:36px;border-radius:11px;color:#fff;background:var(--blue);font-size:17px;font-weight:700}.brand small{margin-left:10px;padding-left:20px;border-left:1px solid var(--line);font-size:14px;font-weight:400;color:var(--muted)}nav{display:flex;align-items:center;gap:28px;font-size:14px;color:var(--muted)}nav button{border:0;background:none;color:inherit;padding:8px 0}.redeem-main{max-width:1120px;margin:40px auto 0;padding-bottom:22px}.eyebrow{color:var(--blue);font-size:12px;font-weight:700;letter-spacing:2px}.intro h1{font-size:32px;font-weight:700;letter-spacing:-1px;margin:7px 0 9px;line-height:1.4}.redeem-page p{margin:0;color:var(--muted)}.intro{margin-bottom:30px}.steps{display:flex;align-items:center;margin:25px 0 32px;padding:0;list-style:none}.steps li{display:flex;align-items:center;gap:10px;color:#777d87;font-size:14px;flex:1;white-space:nowrap}.steps li:last-child{flex:0}.steps li:not(:last-child):after{content:"";height:1px;background:#dce0e7;flex:1;margin:0 22px 0 12px}.circle{width:28px;height:28px;flex-shrink:0;border:1px solid #d9dce3;border-radius:50%;display:grid;place-items:center;font-size:12px;background:#fff}.steps .done{color:var(--green)}.done .circle{background:#e5f4ee;border-color:#e5f4ee}.steps .now{font-weight:700;color:var(--blue)}.now .circle{color:#fff;background:var(--blue);border-color:var(--blue)}.redeem-grid{display:grid;grid-template-columns:minmax(0,1fr) 322px;gap:24px}.redeem-card{background:#fff;border:1px solid var(--line);border-radius:18px;padding:30px;min-width:0}.main-card h2{font-size:19px;font-weight:700;margin:0 0 5px;line-height:1.65}.note{font-size:13px!important;color:var(--muted)}.tag{display:inline-flex;align-items:center;gap:6px;white-space:nowrap;border-radius:20px;padding:3px 10px;background:#e8f5ef;color:var(--green);font-size:12px}.tag:before{content:"";background:var(--green);width:5px;height:5px;border-radius:50%}.cardhead{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:25px}.account{border:1px solid #d7e7f8;background:#f5f9ff;border-radius:12px;padding:21px 22px;margin:20px 0}.label{color:var(--muted);font-size:12px;display:block;margin-bottom:5px}.mono{font-family:Consolas,monospace;font-size:17px;letter-spacing:.35px}.account-id{overflow-wrap:anywhere}.fields{display:grid;grid-template-columns:1fr 1fr;gap:20px;margin:22px 0 4px}.fields b{font-size:14px;font-weight:500}.row{display:flex;align-items:center;justify-content:space-between;gap:20px;margin:15px 0;font-size:14px}.row span:first-child{color:var(--muted);flex-shrink:0}.row> :last-child{text-align:right;overflow-wrap:anywhere}.row strong{font-size:13px}.valid{color:var(--green)}.divider{height:1px;background:var(--line);margin:23px 0}.text-button{display:inline-flex;align-items:center;gap:5px;background:none;border:0;color:var(--blue);padding:0;font-size:13px!important}.text-button:disabled{color:var(--muted)}.change{margin-top:12px}.small{margin-top:10px}.consent{display:flex;align-items:flex-start;gap:10px;font-size:13px;margin:25px 0 18px;color:#47494e;cursor:pointer}.consent input{accent-color:var(--blue);border-radius:4px;width:17px;height:17px;flex-shrink:0;margin:3px 0 0}.primary{background:var(--blue);color:#fff;border:0;width:100%;min-height:49px;border-radius:10px;font-weight:600!important;display:flex;align-items:center;justify-content:center;gap:8px;padding:12px 18px}.primary:hover:not(:disabled){background:#0067d2}.primary:disabled{background:#dce8f5;color:#576b84}.button-note{text-align:center;font-size:12px;margin:10px 0 0!important;color:var(--muted)}.subheading{font-size:13px;font-weight:700;margin-bottom:10px}.product{display:flex;gap:12px;align-items:center;margin:20px 0}.producticon{display:grid;place-items:center;flex-shrink:0;width:45px;height:45px;border-radius:12px;background:#eaf3ff;color:var(--blue)}.product b{display:block;font-size:18px;font-weight:700}.product p{font-size:12px}.code{background:#f5f5f7;border-radius:9px;padding:12px;font:13px/1.8 Consolas,monospace;color:#515763;overflow-wrap:anywhere}.aside-help{padding-top:20px;color:var(--muted);font-size:12px}.switch-code{margin-top:24px}.help-strip{display:flex;gap:14px;align-items:flex-start;margin:24px 0 0;padding:20px 24px;border:1px solid var(--line);border-radius:14px;background:#fafafb}.help-strip>svg{color:var(--blue);flex-shrink:0;margin-top:2px}.help-strip b{font-size:13px}.help-strip p{font-size:12px;margin-top:4px}.redeem-footer{display:flex;justify-content:center;flex-wrap:wrap;gap:24px;color:#777d87;font-size:12px;margin:35px 0 22px}.redeem-footer div{display:flex;gap:8px}.redeem-footer button{background:none;border:0;color:inherit;padding:0}.field-label{display:block;font-size:14px;font-weight:600;margin:20px 0 10px}.text-input{display:block;border:1px solid #d7dce5;background:#fff;border-radius:10px;width:100%;padding:14px 16px;color:var(--ink);min-height:49px;resize:vertical}.text-input::placeholder{color:#9198a4}.text-input:disabled{background:#f5f5f7}.code-input{font-family:Consolas,monospace!important;font-size:16px!important;letter-spacing:.5px}.session-input{font-family:Consolas,monospace!important;font-size:13px!important;min-height:150px;max-height:350px}.input-hint{font-size:12px;margin-top:9px!important}.entry-note{display:flex;align-items:flex-start;gap:10px;padding:18px 0;margin:14px 0 20px}.entry-note svg{color:var(--blue);flex-shrink:0;margin-top:2px}.entry-note p{font-size:13px}.error{background:#fff3ed;border:1px solid #f2d6c5;border-radius:9px;padding:13px 15px;color:#934b25!important;font-size:13px;margin:16px 0!important;overflow-wrap:anywhere}.expired{background:#fff3dd;color:#976415}.expired:before{background:#976415}.expired-note{color:#976415!important}.result{text-align:center;padding:22px 18px 26px}.result-icon{display:grid;place-items:center;width:74px;height:74px;background:#eaf3ff;color:var(--blue);margin:0 auto 20px;border-radius:50%}.result.success .result-icon{background:#e8f5ef;color:var(--green)}.result.warning .result-icon{background:#fff3dd;color:#a16d1c}.result h3{font-size:24px;font-weight:700;margin:0 0 12px}.result p{font-size:14px;line-height:1.9;max-width:460px;margin:0 auto}.spin{animation:redeem-spin 1.5s linear infinite}@keyframes redeem-spin{to{transform:rotate(360deg)}}.help-dialog{margin:auto;border:1px solid var(--line);background:#fff;color:var(--ink);border-radius:18px;padding:28px;max-width:min(560px,calc(100vw - 32px));max-height:85vh;overflow:auto}.help-dialog::backdrop{background:#15233866}.dialog-head{display:flex;align-items:center;justify-content:space-between;gap:20px}.dialog-head h2{font-size:20px;font-weight:700;margin:0}.icon-button{border:0;background:#f5f5f7;border-radius:8px;padding:7px;display:grid;place-items:center}.help-dialog ol{padding-left:22px;list-style:decimal;margin:22px 0}.help-dialog li{margin:16px 0;font-size:14px}.help-dialog p{font-size:13px;margin-top:5px}
@media(max-width:1180px){.redeem-main{margin:32px 24px 0}.redeem-header{padding:0 24px}}
@media(max-width:800px){.redeem-header{height:70px;padding:0 18px}.brand{gap:9px;font-size:16px}.brand small{display:none}nav{gap:15px;font-size:12px}.redeem-main{margin:28px 18px 0}.intro h1{font-size:27px}.intro p{font-size:13px}.redeem-grid{grid-template-columns:1fr}.redeem-card{padding:24px}.steps li{font-size:12px;gap:6px}.steps li:not(:last-child):after{margin:0 9px 0 3px}.circle{width:24px;height:24px}.side-card{padding:22px 24px}.side-card .product{margin:12px 0}.side-card .divider{margin:17px 0}.aside-help{padding-top:5px}.switch-code{margin-top:16px}.redeem-footer{gap:10px;flex-direction:column;align-items:center}.help-strip{padding:18px}.fields{gap:14px}.cardhead{align-items:flex-start}.tag{margin-top:4px}.main-card h2{font-size:18px}.account{padding:18px}.help-strip p{line-height:1.9}.code-input{font-size:13px!important;letter-spacing:0}}
@media(max-width:430px){.redeem-header nav>a{display:none}.steps li{flex-direction:column;position:relative;gap:6px;text-align:center;flex:1}.steps li:last-child{flex:1}.steps li:not(:last-child):after{position:absolute;top:12px;left:calc(50% + 20px);right:calc(-50% + 20px);margin:0}.redeem-card{padding:22px 18px}.cardhead{gap:10px;flex-wrap:wrap}.cardhead .note{font-size:12px!important}.fields{grid-template-columns:1fr;gap:15px;margin-top:20px}.account-id{font-size:15px}.row{font-size:13px;gap:12px}.intro h1{font-size:26px}.result{padding:18px 0}.result h3{font-size:22px}}
@media(prefers-reduced-motion:reduce){.spin{animation:none}}
</style>
