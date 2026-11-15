<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import {RouterLink} from 'vue-router'
import {api} from '../../api/client'
import {useAppStore} from '../../stores/app'
import {useLocalized} from '../../composables/useProduct'
import {isGrokProduct, otherGrokProducts} from '../../utils/grokRelatedProducts'
const props=defineProps<{product: any}>()
const app=useAppStore()
const {getLocalizedText,formatPrice,siteCurrency}=useLocalized()
const rows=ref<any[]>([])
const headings: Record<string,string>={'en-US':'Other Grok plans','zh-TW':'其他 Grok 檔位'}
const heading=computed(()=>headings[app.locale] || '其他 Grok 档位')
watch(()=>[props.product?.id,props.product?.category?.id,app.isResellerTenant],async(_,__,onCleanup)=>{
  let stale=false;onCleanup(()=>{stale=true});rows.value=[]
  if(app.isResellerTenant || !isGrokProduct(props.product))return
  try{
    const response=await api.get('/public/products',{params:{category_id:props.product.category.id,page:1,page_size:100}})
    if(!stale)rows.value=otherGrokProducts(props.product,response.data.data || [])
  }catch{ /* The product remains usable; no invented or stale recommendation links. */ }
},{immediate:true})
</script>

<template>
  <section v-if="rows.length" class="my-8 rounded-2xl border p-5" :aria-label="heading">
    <h2 class="mb-3 text-lg font-semibold">{{heading}}</h2>
    <ul class="grid gap-3 sm:grid-cols-2">
      <li v-for="item in rows" :key="item.id">
        <RouterLink class="underline underline-offset-4" :to="`/products/${encodeURIComponent(item.slug)}`">{{getLocalizedText(item.title)}}</RouterLink>
        <span class="ml-2">{{formatPrice(item.effective_price_amount ?? item.price_amount,siteCurrency)}}</span>
      </li>
    </ul>
  </section>
</template>
