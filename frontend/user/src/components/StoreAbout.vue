<template>
  <div class="store-about">
    <div class="about-container">
      <header class="about-hero">
        <p class="about-eyebrow">{{ copy.eyebrow }}</p>
        <h1>{{ heroTitle || copy.title }}</h1>
        <p class="about-subtitle">{{ heroSubtitle || copy.subtitle }}</p>
      </header>

      <article class="about-card">
        <section class="about-introduction" aria-labelledby="about-intro-title">
          <h2 id="about-intro-title">{{ copy.introductionTitle }}</h2>
          <p>{{ introductionText || copy.introduction }}</p>
        </section>

        <section class="about-section" aria-labelledby="about-services-title">
          <h2 id="about-services-title">{{ servicesTitle || copy.servicesTitle }}</h2>
          <div class="about-services">
            <div v-for="(service, index) in services" :key="index" class="about-service">
              <span class="about-icon" aria-hidden="true"><component :is="serviceIcons[index] || Check" /></span>
              <div>
                <h3 v-if="service.title">{{ service.title }}</h3>
                <p>{{ service.text }}</p>
              </div>
            </div>
          </div>
        </section>

        <section class="about-section" aria-labelledby="about-recharge-title">
          <h2 id="about-recharge-title">{{ copy.rechargeTitle }}</h2>
          <ol class="about-recharge">
            <li v-for="(item, index) in copy.recharge" :key="item.title">
              <span class="about-number" aria-hidden="true">{{ String(index + 1).padStart(2, '0') }}</span>
              <div><h3>{{ item.title }}</h3><p>{{ item.text }}</p></div>
            </li>
          </ol>
          <p class="about-note">{{ copy.rechargeNote }}</p>
        </section>

        <section class="about-section about-cooperation" aria-labelledby="about-cooperation-title">
          <span class="about-icon" aria-hidden="true"><Handshake /></span>
          <div>
            <h2 id="about-cooperation-title">{{ copy.cooperationTitle }}</h2>
            <p>{{ copy.cooperation }}</p>
            <p class="about-note">{{ copy.cooperationNote }}</p>
          </div>
        </section>

        <section class="about-contact" aria-labelledby="about-contact-title">
          <h2 id="about-contact-title">{{ contactTitle || copy.contactTitle }}</h2>
          <p>{{ contactText || copy.contactSubtitle }}</p>
          <div class="about-wechat">
            <div><span class="about-wechat-label">客服微信</span><strong>{{ supportWechat }}</strong></div>
            <button type="button" class="about-copy" @click="copyWechat">
              <Check v-if="copied" :size="18" aria-hidden="true" /><Copy v-else :size="18" aria-hidden="true" />
              {{ copied ? '已复制微信号' : '复制微信号' }}
            </button>
          </div>
          <p class="sr-only" role="status" aria-live="polite">{{ copied ? t('shopUi.wechatCopied') : '' }}</p>
          <p class="about-contact-note">{{ copy.contactNote }}</p>
          <p class="about-contact-note">{{ copy.contactTip }}</p>
          <div v-if="contactConfig?.telegram || contactConfig?.whatsapp" class="about-extra-contact">
            <a v-if="contactConfig?.telegram" :href="contactConfig.telegram" target="_blank" rel="noopener noreferrer">Telegram <ArrowUpRight :size="16" /></a>
            <a v-if="contactConfig?.whatsapp" :href="contactConfig.whatsapp" target="_blank" rel="noopener noreferrer">WhatsApp <ArrowUpRight :size="16" /></a>
          </div>
        </section>

        <nav class="about-links" aria-label="购买与服务说明">
          <RouterLink v-for="link in copy.links" :key="link.to" :to="link.to">{{ link.title }}<ArrowRight :size="19" aria-hidden="true" /></RouterLink>
        </nav>
      </article>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ArrowUpRight, Check, ClipboardList, Copy, Handshake, KeyRound, ShieldCheck, ShoppingCart } from 'lucide-vue-next'
import { aboutCopy as copy } from '../content/about'
import { copyText } from '../utils/clipboard'
import { toast } from '../composables/useToast'

const props = defineProps<{
  heroTitle: string
  heroSubtitle: string
  introductionText: string
  servicesTitle: string
  serviceItems: string[]
  contactTitle: string
  contactText: string
  contactConfig?: { telegram?: string; whatsapp?: string } | null
}>()
const { t } = useI18n()
const supportWechat = 'Cqz0924ttxk'
const copied = ref(false)
const serviceIcons = [ShoppingCart, ClipboardList, KeyRound, ShieldCheck]
// Existing site settings remain usable; a first line can serve as a custom card title.
const services = computed(() => props.serviceItems.length ? props.serviceItems.map(text => {
  const [first, ...rest] = text.split('\n')
  return rest.length ? { title: first, text: rest.join('\n') } : { title: '', text }
}) : copy.services)

const copyWechat = async () => {
  try {
    await copyText(supportWechat)
    copied.value = true
    toast.success(t('shopUi.wechatCopied'))
  } catch {
    copied.value = false
    toast.error(t('shopUi.wechatCopyFailed'))
  }
}
</script>

<style scoped>
.store-about { background: var(--ui-bg-page); color: var(--ui-text-primary); padding: 140px 24px 48px; }
:global(.vault-scope) .store-about { padding-top: 40px; }
.about-container { width: 100%; max-width: 940px; margin-inline: auto; }
.about-hero { margin: 0 auto 40px; padding-bottom: 28px; text-align: center; }
.about-hero::after { content: ''; display: block; height: 1px; background: var(--ui-border); max-width: 590px; margin: 28px auto -28px; }
.about-eyebrow { color: var(--ui-text-muted); font-size: 15px; margin: 0 0 8px; }
.about-hero h1 { font-size: clamp(32px, 4vw, 52px); line-height: 1.25; font-weight: 800; letter-spacing: -.035em; margin: 0 0 12px; color: var(--ui-accent); }
.about-subtitle { color: var(--ui-text-secondary); font-size: 18px; margin: 0; }
.about-card { background: var(--ui-bg-elevated); border: 1px solid var(--ui-border); border-radius: 24px; padding: 44px; box-shadow: 0 18px 50px -38px rgba(0,0,0,.16); }
.about-card h2 { margin: 0 0 16px; font-size: 28px; line-height: 1.45; letter-spacing: -.02em; font-weight: 750; }
.about-card h3 { margin: 0 0 10px; font-size: 18px; font-weight: 700; line-height: 1.5; }
.about-card p { color: var(--ui-text-secondary); font-size: 16px; line-height: 1.8; margin: 0; white-space: pre-line; overflow-wrap: anywhere; }
.about-section { border-top: 1px solid var(--ui-border); margin-top: 36px; padding-top: 36px; }
.about-services { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; margin-top: 22px; }
.about-service { background: var(--ui-bg-soft); border-radius: 14px; padding: 24px; display: flex; gap: 22px; align-items: flex-start; }
.about-service p { font-size: 15px; }
.about-icon { border-radius: 50%; display: grid; place-items: center; flex: 0 0 58px; width: 58px; height: 58px; color: var(--ui-accent); background: var(--ui-bg-elevated); border: 1px solid var(--ui-border); }
.about-icon svg { width: 29px; height: 29px; stroke-width: 1.7; }
.about-recharge { list-style: none; padding: 0; margin: 24px 0; }
.about-recharge li { display: flex; align-items: flex-start; gap: 28px; padding: 24px 0; }
.about-recharge li:first-child { padding-top: 0; }
.about-recharge li + li { border-top: 1px solid var(--ui-border); }
.about-recharge li:last-child { padding-bottom: 0; }
.about-number { display: grid; place-items: center; flex: 0 0 64px; height: 64px; border-radius: 50%; background: var(--ui-bg-soft); color: var(--ui-accent); font-size: 23px; font-weight: 650; }
.about-card p.about-note { color: var(--ui-text-muted); font-size: 14px; }
.about-cooperation { display: flex; gap: 24px; align-items: flex-start; }
.about-cooperation .about-icon { flex-basis: 76px; width: 76px; height: 76px; background: var(--ui-bg-soft); border: 0; }
.about-cooperation .about-icon svg { width: 40px; height: 40px; }
.about-cooperation h2 { margin-bottom: 8px; }
.about-contact { background: var(--ui-bg-soft); border-radius: 14px; padding: 32px; margin-top: 32px; }
.about-contact h2 { margin-bottom: 6px; }
.about-wechat { background: var(--ui-bg-elevated); border-radius: 12px; padding: 24px; margin: 22px 0 18px; display: flex; align-items: center; justify-content: space-between; gap: 24px; }
.about-wechat-label { display: block; font-size: 14px; color: var(--ui-text-secondary); }
.about-wechat strong { display: block; color: var(--ui-accent); font-size: 30px; font-weight: 750; line-height: 1.5; overflow-wrap: anywhere; }
.about-copy { cursor: pointer; display: inline-flex; align-items: center; justify-content: center; gap: 10px; flex: none; padding: 14px 24px; min-height: 48px; color: var(--ui-text-on-accent); background: var(--ui-accent); border: 1px solid transparent; border-radius: 9px; font-size: 14px; font-weight: 650; transition: background .15s; }
.about-copy:hover { background: var(--ui-accent-hover); }
.about-copy:focus-visible, .about-links a:focus-visible, .about-extra-contact a:focus-visible { outline: 3px solid var(--ui-accent); outline-offset: 4px; }
.about-card p.about-contact-note { font-size: 14px; }
.about-extra-contact { display: flex; gap: 24px; margin-top: 18px; }
.about-extra-contact a { display: inline-flex; align-items: center; gap: 6px; color: var(--ui-accent); text-decoration: underline; }
.about-links { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); border-top: 1px solid var(--ui-border); margin-top: 32px; padding-top: 22px; }
.about-links a { display: flex; justify-content: center; align-items: center; gap: 12px; padding: 8px 12px; min-height: 44px; font-size: 14px; font-weight: 600; color: var(--ui-accent); text-align: center; }
.about-links a + a { border-left: 1px solid var(--ui-border); }
.about-links a:hover { text-decoration: underline; text-underline-offset: 4px; }
.about-links svg { flex: none; }
@media (max-width: 767px) {
  .store-about { padding: 104px 16px 32px; }
  .about-hero { margin-bottom: 28px; padding-bottom: 22px; }
  .about-hero::after { margin-top: 22px; margin-bottom: -22px; }
  .about-subtitle { font-size: 15px; }
  .about-card { border-radius: 20px; padding: 24px; }
  .about-card h2 { font-size: 21px; }
  .about-card h3 { font-size: 16px; margin-bottom: 8px; }
  .about-card p { font-size: 14px; }
  .about-section { margin-top: 28px; padding-top: 28px; }
  .about-services { grid-template-columns: minmax(0, 1fr); gap: 14px; margin-top: 18px; }
  .about-service { padding: 18px; gap: 14px; }
  .about-icon { flex-basis: 42px; width: 42px; height: 42px; }
  .about-icon svg { width: 23px; height: 23px; }
  .about-recharge li { gap: 16px; padding: 20px 0; }
  .about-number { flex-basis: 42px; height: 42px; font-size: 18px; }
  .about-cooperation { gap: 16px; }
  .about-cooperation .about-icon { flex-basis: 48px; width: 48px; height: 48px; }
  .about-cooperation .about-icon svg { width: 28px; height: 28px; }
  .about-contact { padding: 20px; margin-top: 28px; }
  .about-wechat { padding: 18px; flex-direction: column; align-items: stretch; gap: 14px; }
  .about-wechat strong { font-size: 25px; }
  .about-copy { width: 100%; padding-inline: 12px; }
  .about-links { grid-template-columns: minmax(0, 1fr); padding-top: 12px; margin-top: 28px; }
  .about-links a { justify-content: space-between; padding: 12px 2px; }
  .about-links a + a { border-left: 0; border-top: 1px solid var(--ui-border); }
}
@media (max-width: 359px) {
  .store-about { padding-inline: 12px; }
  .about-card { padding: 20px 16px; }
  .about-service { padding: 16px; }
  .about-contact { padding: 18px 14px; }
}
</style>
