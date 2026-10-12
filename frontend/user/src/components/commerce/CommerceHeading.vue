<template>
  <header class="commerce-heading">
    <div>
      <p class="commerce-eyebrow">{{ eyebrow || t('shopUi.brandLine') }}</p>
      <h1>{{ title }}</h1>
      <p class="commerce-heading-description">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="commerce-heading-actions"><slot name="actions" /></div>
  </header>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ title: string; description: string; eyebrow?: string }>()
const { t } = useI18n()
</script>
