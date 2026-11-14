<script setup lang="ts">
import { ref } from 'vue'
import { adminAPI } from '@/api/admin'
import { getImageUrl } from '@/utils/image'
import { notifyError } from '@/utils/notify'
const props = defineProps<{ modelValue?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const pending = ref(false)
const upload = async (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  pending.value = true
  try {
    const data = new FormData()
    data.append('file', file)
    const response = await adminAPI.upload(data, 'seo-share')
    const url = response.data.data?.url
    if (url) emit('update:modelValue', url)
  } catch (err) { notifyError((err as Error)?.message || '分享图上传失败') }
  finally { pending.value = false; input.value = '' }
}
</script>
<template>
  <div class="space-y-2">
    <label class="block text-sm font-medium">分享卡片图
      <input type="file" accept="image/png,image/jpeg,image/webp,image/gif" :disabled="pending" class="mt-2 block w-full text-sm" @change="upload" />
    </label>
    <p class="text-xs text-muted-foreground">{{ pending ? '正在生成分享图…' : '上传后自动留白适配为 1200 × 630，保留完整画面。留空时使用商品图片或全站默认图。' }}</p>
    <div v-if="props.modelValue" class="flex items-start gap-3">
      <img :src="getImageUrl(props.modelValue)" alt="分享卡片预览" class="w-60 max-w-full rounded border" />
      <button type="button" class="text-sm underline" :disabled="pending" @click="emit('update:modelValue', '')">清除</button>
    </div>
  </div>
</template>
