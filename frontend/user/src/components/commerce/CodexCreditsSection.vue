<template>
  <section v-if="credits.length" class="codex-credits" :class="{ 'catalog-credits': catalog }" :aria-label="copy.heading">
    <header v-if="!catalog" class="credits-heading">
      <div><h2>{{ copy.heading }}</h2><p>{{ copy.subtitle }}</p></div>
      <span class="credits-count">{{ credits.length }} {{ copy.count }}</span>
    </header>
    <div class="credits-notice">
      <Info aria-hidden="true" :size="17" />
      <div><strong>{{ copy.noticeTitle }}</strong><p>{{ copy.notice }}<span v-if="catalog">{{ ' ' + copy.preflight }}</span></p></div>
    </div>
    <div :class="catalog ? 'catalog-grid' : 'credits-grid'">
      <template v-for="product in credits" :key="product.id">
      <slot name="card" :product="product">
      <article class="credits-card" :class="{ 'is-sold-out': isSoldOut(product) }">
        <RouterLink :to="`/products/${product.slug}`" class="credits-cover" :aria-label="getLocalizedText(product.title)">
          <div class="cover-mark">CODEX <span aria-hidden="true">&gt;_</span></div>
          <div class="cover-value">
            <span class="cover-number" :class="{ 'large-amount': codexCreditPoints(product)! >= 10000 }">{{ codexCreditPoints(product)!.toLocaleString('en-US') }}</span>
            <span class="cover-unit">CREDITS · {{ copy.points }}</span>
          </div>
          <div class="cover-bottom"><span>{{ copy.channel }}</span><span class="cover-brand">π星卡社</span></div>
        </RouterLink>
        <div class="credits-content">
          <div class="credits-channel">{{ copy.channel }}</div>
          <h3><RouterLink :to="`/products/${product.slug}`">{{ getLocalizedText(product.title) }}</RouterLink></h3>
          <div class="credits-chips">
            <span>{{ copy.auto }}</span><span>{{ getPurchaseTypeLabel(product.purchase_type) }}</span>
            <span class="credits-stock" :class="{ 'stock-empty': isSoldOut(product) }">
              {{ isSoldOut(product) || exactCreditStock(product) === undefined ? getStockStatusLabel(product) : `${copy.stock} ${exactCreditStock(product)}` }}
            </span>
          </div>
          <p class="credits-description">{{ copy.eligibility }}<br>{{ copy.preflight }}</p>
          <div class="credits-buy">
            <div class="credits-price">
              <small>{{ copy.price }} / {{ siteCurrency }}</small>
              <strong>{{ formatPrice(hasPromotionPrice(product) ? getPromotionPriceAmount(product) : product.price_amount, '') }}</strong>
              <del v-if="hasPromotionPrice(product)">{{ formatPrice(product.price_amount, '') }}</del>
            </div>
            <RouterLink class="credits-details" :to="`/products/${product.slug}`">{{ copy.details }} <ArrowUpRight :size="13" aria-hidden="true" /></RouterLink>
          </div>
        </div>
      </article>
      </slot>
      </template>
    </div>
    <footer class="credits-help">
      <div><strong>{{ copy.redeemTitle }}</strong><p>{{ copy.redeem }}</p></div>
      <div><strong>{{ copy.helpTitle }}</strong><p>{{ copy.help }}</p></div>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight, Info } from 'lucide-vue-next'
import { useLocalized, useProductLabels } from '../../composables/useProduct'
import { codexCreditPoints, exactCreditStock, splitCodexCredits } from '../../utils/codexCredits'
import { codexCreditsCopy } from '../../i18n/codexCredits'

const props = defineProps<{ products: any[]; catalog?: boolean }>()
const { locale } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const { getPurchaseTypeLabel, getStockStatusLabel, isSoldOut, hasPromotionPrice, getPromotionPriceAmount } = useProductLabels()
const copy = computed(() => codexCreditsCopy[locale.value] || codexCreditsCopy['zh-CN']!)
const credits = computed(() => splitCodexCredits(props.products).credits)
</script>

<style scoped>
.codex-credits{min-width:0;margin-bottom:32px;container-type:inline-size}
.credits-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:19px}
.credits-heading h2{font-size:28px;line-height:1.3;font-weight:700;letter-spacing:-.7px}
.credits-heading p{font-size:12px;color:var(--ui-text-muted);margin-top:6px;line-height:1.7}
.credits-count{flex-shrink:0;white-space:nowrap;font-size:11px;border:1px solid var(--ui-border);border-radius:20px;padding:6px 10px;color:var(--ui-text-muted)}
.credits-notice{display:flex;align-items:flex-start;gap:10px;border:1px solid #d9e1cc;border-left:3px solid #6a8357;background:#edf1e4;border-radius:9px;padding:15px 18px;color:#516444;font-size:12px;line-height:1.9;margin-bottom:24px}
.credits-notice svg{flex-shrink:0;margin-top:3px}.credits-notice strong{font-weight:650}
.credits-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:19px}
.credits-card{min-width:0;overflow:hidden;background:var(--ui-bg-elevated);color:var(--ui-text-primary);border:1px solid #d6e0cc;border-radius:15px;display:flex;flex-direction:column}
.credits-cover{height:178px;background:#213d2c;color:#ecf0dc;position:relative;overflow:hidden;padding:18px 22px;display:flex;flex-direction:column;justify-content:space-between;text-decoration:none}
.credits-cover:before{content:"";position:absolute;width:195px;height:195px;right:-46px;top:10px;border:1px solid #8d9b65;border-radius:50%;box-shadow:0 0 0 25px #294832,0 0 0 26px #617649,0 0 0 51px #274530,0 0 0 52px #577041;opacity:.34}
.cover-mark,.cover-value,.cover-bottom{position:relative}.cover-mark{display:flex;align-items:center;justify-content:space-between;font-size:11px;font-weight:700;letter-spacing:2px}.cover-mark>span{font:600 21px monospace;letter-spacing:-3px;opacity:.7}
.cover-number{display:block;text-align:center;font:bold 61px/1 Arial,sans-serif;color:#e2edaf;letter-spacing:-3px;white-space:nowrap}.cover-unit{display:block;text-align:center;margin-top:4px;color:#c0cfab;font-size:10px;font-weight:600;letter-spacing:3px}
.cover-bottom{display:flex;align-items:center;justify-content:space-between;font-size:9px;font-weight:600;letter-spacing:.6px;padding-top:9px;border-top:1px dashed #789063}.cover-brand{border:1px solid #617653;border-radius:20px;padding:3px 7px;font-weight:500;letter-spacing:0}
.credits-content{padding:18px 19px 17px;display:flex;flex:1;flex-direction:column}.credits-channel{font-size:10px;color:var(--ui-text-muted);letter-spacing:.5px}.credits-content h3{font-size:17px;font-weight:700;margin:8px 0 12px;letter-spacing:-.5px;line-height:1.4;overflow-wrap:anywhere}.credits-content h3 a{color:inherit;text-decoration:none}
.credits-chips{display:flex;gap:5px;flex-wrap:wrap;font-size:10px;color:#527052}.credits-chips>span{background:#f0f4ed;padding:4px 7px;border-radius:5px;white-space:nowrap}.credits-chips .credits-stock{background:transparent;padding-right:0;margin-left:auto;color:#65865e}.credits-stock:before{content:"";display:inline-block;width:5px;height:5px;background:currentColor;border-radius:50%;margin-right:4px;vertical-align:2px}.credits-chips .stock-empty{color:var(--ui-danger)}
.credits-description{font-size:11px;color:var(--ui-text-muted);line-height:1.7;margin:13px 0 16px}.credits-buy{display:flex;align-items:center;justify-content:space-between;border-top:1px solid var(--ui-border);padding-top:14px;gap:7px;margin-top:auto}.credits-price{min-width:0;overflow-wrap:anywhere}.credits-price small{display:block;font-size:9px;color:var(--ui-text-muted);margin-bottom:4px}.credits-price strong{font-size:21px;font-weight:700;line-height:1.3;color:var(--ui-text-primary)}.credits-price del{display:block;font-size:10px;color:var(--ui-text-muted)}
.credits-details{display:inline-flex;align-items:center;justify-content:center;gap:5px;border:1px solid #284c35;background:#284c35;color:white;border-radius:8px;padding:8px 10px;white-space:nowrap;font-size:10px;text-decoration:none;min-height:36px}.credits-details:hover{background:#365f42}.credits-card a:focus-visible{outline:3px solid #86a26d;outline-offset:-3px}.is-sold-out .credits-cover{filter:grayscale(.65);opacity:.8}
.credits-help{margin-top:24px;padding-top:21px;border-top:1px solid var(--ui-border);display:flex;justify-content:space-between;gap:20px;color:var(--ui-text-muted);font-size:11px;line-height:1.9}.credits-help strong{font-weight:600;color:var(--ui-text-primary)}.credits-help>div:first-child{max-width:65%}
@container(min-width:1150px){.credits-cover{height:209px}.cover-number{font-size:74px}.credits-content{padding:21px}.credits-content h3{font-size:19px}.credits-description{font-size:12px}.credits-grid{gap:22px}}
@container(max-width:780px){.credits-cover{padding:16px}.cover-number{font-size:50px}.credits-content{padding:15px}.credits-content h3{font-size:15px}.credits-chips{font-size:9px}.credits-chips>span{padding:3px 5px}.credits-chips .credits-stock{margin-left:0}.cover-brand{display:none}.credits-grid{gap:13px}}
@media(max-width:760px){.credits-heading{margin-bottom:13px}.credits-heading h2{font-size:22px}.credits-heading p{font-size:10px}.credits-count{font-size:9px;padding:5px 7px}.credits-notice{font-size:11px;padding:12px;line-height:1.8;margin-bottom:17px;gap:8px}.credits-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:11px}.credits-cover{height:143px;padding:13px 12px}.cover-mark{font-size:9px;letter-spacing:1px}.cover-mark>span{font-size:18px}.cover-number{font-size:40px;letter-spacing:-2px}.cover-number.large-amount{font-size:34px}.cover-unit{font-size:8px;letter-spacing:2px}.cover-bottom{font-size:8px}.credits-content{padding:12px}.credits-channel{font-size:8px}.credits-content h3{font-size:13px;margin:6px 0 9px;letter-spacing:-.3px;min-height:36px}.credits-chips{gap:4px;font-size:8px}.credits-chips>span{padding:3px 4px}.credits-chips .credits-stock{margin-left:0;display:block;flex:1 0 100%;font-size:9px;padding-left:0}.credits-description{font-size:10px;line-height:1.7;margin:9px 0 12px;min-height:34px}.credits-buy{align-items:flex-start;flex-direction:column;gap:10px;padding-top:10px}.credits-price strong{font-size:19px}.credits-details{width:100%;font-size:10px;padding:8px}.credits-price small{font-size:8px}.credits-help{flex-direction:column;gap:8px;font-size:10px;margin-top:19px}.credits-help>div:first-child{max-width:none}}
.catalog-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:18px}
@media(max-width:1050px){.catalog-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:767px){.catalog-grid{gap:12px}}
@media(max-width:479px){.catalog-grid{grid-template-columns:minmax(0,1fr)}}
</style>
