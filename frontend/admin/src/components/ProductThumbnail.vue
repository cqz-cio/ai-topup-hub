<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ImageOff } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { getFirstImageUrl } from '@/utils/image'

const props = defineProps<{ images: unknown; alt: string }>()
const { t } = useI18n()
const imageUrl = computed(() => getFirstImageUrl(props.images))
const failed = ref(false)

watch(imageUrl, () => { failed.value = false })
</script>

<template>
  <div class="flex h-[60px] w-[100px] shrink-0 items-center justify-center overflow-hidden rounded-lg border border-border bg-muted/40 text-muted-foreground">
    <img
      v-if="imageUrl && !failed"
      :key="imageUrl"
      :src="imageUrl"
      :alt="alt"
      class="h-full w-full object-contain"
      decoding="async"
      @error="failed = true"
    />
    <span v-else class="flex flex-col items-center gap-1 text-[10px]" role="img" :aria-label="t('admin.common.noImage')">
      <ImageOff class="h-4 w-4" aria-hidden="true" />
      {{ t('admin.common.noImage') }}
    </span>
  </div>
</template>
