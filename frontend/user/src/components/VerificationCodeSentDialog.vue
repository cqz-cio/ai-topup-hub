<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MailCheck, X } from 'lucide-vue-next'
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import { Button } from '@/components/ui/button'

defineProps<{ open: boolean }>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  closed: []
}>()
const { t } = useI18n()
const confirmButton = ref<InstanceType<typeof Button> | null>(null)

const focusConfirm = (event: Event) => {
  event.preventDefault()
  confirmButton.value?.$el.focus()
}
const restoreFocus = (event: Event) => {
  event.preventDefault()
  emit('closed')
}
</script>

<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[150] bg-black/40 backdrop-blur-sm" />
      <DialogContent
        aria-modal="true"
        class="fixed left-1/2 top-1/2 z-[150] max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-sm -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-2xl border bg-card p-6 text-card-foreground shadow-2xl outline-none"
        @open-auto-focus="focusConfirm"
        @close-auto-focus="restoreFocus"
      >
        <DialogClose as-child>
          <button
            type="button"
            class="absolute right-4 top-4 flex size-8 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            :aria-label="t('common.close')"
          >
            <X class="size-4" aria-hidden="true" />
          </button>
        </DialogClose>
        <div
          class="mb-4 flex size-11 items-center justify-center rounded-xl text-primary"
          style="background: color-mix(in srgb, var(--ui-accent) 10%, var(--ui-bg-elevated))"
          aria-hidden="true"
        >
          <MailCheck class="size-6" :stroke-width="1.8" />
        </div>
        <DialogTitle class="text-lg font-bold leading-7 text-foreground">
          {{ t('auth.register.codeSent.title') }}
        </DialogTitle>
        <DialogDescription as-child>
          <div class="text-sm leading-[23px] text-muted-foreground">
            <p class="mt-2">{{ t('auth.register.codeSent.description') }}</p>
            <div
              class="mt-4 rounded-[10px] px-3.5 py-3"
              style="background: color-mix(in srgb, var(--ui-accent) 5%, var(--ui-bg-elevated))"
            >
              <i18n-t keypath="auth.register.codeSent.spamHint" tag="p" scope="global">
                <template #spam><strong class="font-semibold text-foreground">{{ t('auth.register.codeSent.spamFolder') }}</strong></template>
                <template #promotions><strong class="font-semibold text-foreground">{{ t('auth.register.codeSent.promotionsFolder') }}</strong></template>
              </i18n-t>
            </div>
          </div>
        </DialogDescription>
        <DialogClose as-child>
          <Button ref="confirmButton" type="button" class="mt-6 h-11 w-full font-semibold">
            {{ t('auth.register.codeSent.confirm') }}
          </Button>
        </DialogClose>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
