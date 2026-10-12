<template>
  <ol
    class="commerce-steps"
    :aria-label="t('checkoutSteps.label')"
  >
    <template v-for="(step, idx) in steps" :key="step.key">
      <li
        class="commerce-step"
        :class="`is-${step.status}`"
        :aria-current="step.status === 'current' ? 'step' : undefined"
      >
        <div class="commerce-step-content">
          <span
            class="commerce-step-number"
          >
            <Check v-if="step.status === 'done'" class="w-3.5 h-3.5" :stroke-width="3" />
            <span v-else>{{ idx + 1 }}</span>
          </span>
          <span
            class="commerce-step-label"
          >
            {{ step.label }}
          </span>
        </div>
      </li>
    </template>
  </ol>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check } from 'lucide-vue-next'

type StepKey = 'cart' | 'checkout' | 'payment' | 'delivery'
type StepStatus = 'done' | 'current' | 'upcoming'

const props = withDefaults(
  defineProps<{
    currentStep: StepKey
    stepKeys?: StepKey[]
    complete?: boolean
  }>(),
  {
    stepKeys: () => ['cart', 'checkout', 'payment', 'delivery'],
    complete: false,
  },
)

const { t } = useI18n()

const steps = computed(() => {
  const activeIdx = props.stepKeys.indexOf(props.currentStep)
  if (activeIdx < 0 && import.meta.env.DEV) {
    const brandWarn = (globalThis as any).console?.warn?.bind(console)
    brandWarn?.(`[CheckoutSteps] currentStep "${props.currentStep}" not found in stepKeys`, props.stepKeys)
  }
  return props.stepKeys.map((key, idx) => {
    const status: StepStatus = props.complete ? 'done' : activeIdx < 0
      ? 'upcoming'
      : idx < activeIdx
        ? 'done'
        : idx === activeIdx
          ? 'current'
          : 'upcoming'
    return { key, label: t(`shopUi.steps.${key}`), status }
  })
})
</script>
