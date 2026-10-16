<script setup lang="ts">
import { computed } from 'vue'
import { processHtmlForDisplay } from '../../utils/content'

const props = defineProps<{ html: string }>()
const sections = computed(() => {
  const doc = new DOMParser().parseFromString(processHtmlForDisplay(props.html), 'text/html')
  const groups: string[][] = []
  for (const node of Array.from(doc.body.childNodes)) {
    if (node.nodeType === Node.COMMENT_NODE) continue
    if (!node.textContent?.trim() && node.nodeType === Node.TEXT_NODE) continue
    const el = node as HTMLElement
    const heading = /^H[1-6]$/.test(el.tagName || '')
    const paragraphHeading = el.tagName === 'P' && /^【/.test(el.textContent?.trim() || '')
    if (!groups.length || heading || paragraphHeading) groups.push([])
    groups[groups.length - 1]!.push(el.outerHTML ?? (node.textContent || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'))
  }
  return groups.map(group => group.join(''))
})
</script>

<template>
  <div class="product-content-sections" :class="{ 'three-sections': sections.length === 3 }">
    <section v-for="(section, index) in sections" :key="index" class="product-content-section prose dark:prose-invert max-w-none" v-html="section"></section>
  </div>
</template>

<style scoped>
.product-content-sections { display: grid; gap: 20px; align-items: stretch; }
.product-content-section { min-width: 0; padding: 24px; border: 1px solid var(--ui-border); border-radius: 16px; background: var(--ui-bg-elevated); font-size: 15px; line-height: 1.85; overflow-wrap: anywhere; }
.product-content-section :deep(h2), .product-content-section :deep(h3) { margin: 0 0 16px; padding-bottom: 12px; border-bottom: 1px solid var(--ui-border); font-size: 18px; line-height: 1.5; }
.product-content-section :deep(br + br) { display: none; }
.product-content-section :deep(p) { margin: 12px 0; }
.product-content-section :deep(ol), .product-content-section :deep(ul) { margin: 12px 0; padding-left: 1.5em; }
.product-content-section :deep(li) { padding-left: 4px; margin: 8px 0; }
.product-content-section :deep(> :first-child) { margin-top: 0; }
.product-content-section :deep(> :last-child) { margin-bottom: 0; }
@media (min-width: 1024px) {
  .product-content-sections { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; }
  .three-sections > :nth-child(2) { grid-column: 1; }
  .three-sections > :last-child { grid-column: 2; grid-row: 1 / span 2; }
}
@media (max-width: 479px) { .product-content-section { padding: 18px; } }
</style>
