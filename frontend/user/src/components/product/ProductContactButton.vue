<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessageCircle } from 'lucide-vue-next'
import { contactFirstCopy } from '../../utils/contactFirstProducts'
import { useProductContact } from '../../composables/useProductContact'

const { locale } = useI18n()
const copy = computed(() => contactFirstCopy(locale.value))
const { openContact } = useProductContact()
</script>

<template>
  <button type="button" class="product-contact-trigger" aria-haspopup="dialog" @click.stop.prevent="openContact">
    <MessageCircle :size="19" aria-hidden="true" />
    <span>{{ copy.title }}</span>
  </button>
</template>

<style scoped>
.product-contact-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 44px;
  padding: 10px 18px;
  border: 1px solid var(--ui-border-strong);
  border-radius: 12px;
  background: var(--ui-bg-elevated);
  color: var(--ui-text-primary);
  font-size: 15px;
  font-weight: 650;
  cursor: pointer;
}
.product-contact-trigger:hover { background: var(--ui-bg-soft); }
.product-contact-trigger:focus-visible { outline: 2px solid var(--ui-accent); outline-offset: 3px; }
.product-contact-trigger svg { flex-shrink: 0; }
</style>
