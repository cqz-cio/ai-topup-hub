<template>
  <div class="grid gap-3">
    <h3 v-if="title" class="text-[15px] font-bold">{{ title }}</h3>
    <div class="flex justify-between gap-3 text-[13px]"><span class="text-muted-foreground">{{ t('orderDetail.fulfillmentType') }}</span><span>{{ fulfillmentTypeLabelText(fulfillment.type) }}</span></div>
    <div class="flex justify-between gap-3 text-[13px]"><span class="text-muted-foreground">{{ t('orderDetail.fulfillmentStatus') }}</span><span>{{ fulfillmentStatusLabelText(fulfillment.status) }}</span></div>
    <DeliveryContent v-if="fulfillment.status === 'delivered'" :order-no="orderNo" :downloading="downloading" :load-delivery="loadDelivery" :session-key="sessionKey" @download="$emit('download', $event)" />
  </div>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DeliveryContent from '@/components/commerce/DeliveryContent.vue'
import { useOrderDisplayHelpers } from '../../../composables/useOrderDisplayHelpers'
defineProps<{ title?: string; fulfillment: any; items?: any[]; orderNo: string; downloading: boolean; loadDelivery: (orderNo: string) => Promise<any>; sessionKey: string | number }>()
defineEmits<{ download: [orderNo: string] }>()
const { t } = useI18n()
const { fulfillmentTypeLabelText, fulfillmentStatusLabelText } = useOrderDisplayHelpers(ref(null))
</script>
