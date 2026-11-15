<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {adminAPI} from '@/api/admin'
import {nearDuplicateProducts} from '@/utils/productContentSimilarity'
const props=defineProps<{productId:number|null;categoryId:number|null;content:string;language:string}>()
const peers=ref<any[]>([])
const state=ref('idle')
const more=ref(false)
watch(()=>props.categoryId,async(category,_,onCleanup)=>{
  let stale=false;onCleanup(()=>{stale=true});peers.value=[];more.value=false
  if(!category){state.value='idle';return}
  state.value='loading'
  try{
    const response=await adminAPI.getProducts({category_id:category,is_active:true,page:1,page_size:100})
    if(stale)return
    peers.value=(response.data.data || []).filter((p:any)=>p.is_active && p.category_id===category)
    more.value=Number(response.data.pagination?.total || 0)>100
    state.value='ready'
  }catch{if(!stale)state.value='error'}
},{immediate:true})
const matches=computed(()=>nearDuplicateProducts(props.content,peers.value.map(p=>({id:p.id,content:p.content?.[props.language] || ''})),props.productId))
const label=(id:number)=>{const p=peers.value.find(p=>p.id===id);return p?.title?.[props.language] || p?.title?.['zh-CN'] || `#${id}`}
</script>
<template>
  <aside class="mt-3 rounded-md border p-3 text-xs leading-relaxed" aria-label="商品正文检查">
    <p>请写明本商品的档位、周期、适用条件、相邻商品差异和常见问题。摘要与 SEO 描述也应独立；只填写已确认的权益和售后条件。</p>
    <p v-if="state==='loading'" class="mt-2">正在对照同分类在售商品正文…</p>
    <p v-else-if="state==='error'" class="mt-2">暂时无法读取同分类正文，本次未完成近重复检查。</p>
    <div v-else-if="matches.length" role="status" class="mt-2 font-medium">
      <p>正文近重复提醒：去掉档位、周期、数字与价格后，下列商品相似度超过 85%。请补充已确认的商品差异；提示不阻止保存。</p>
      <ul class="list-disc pl-4"><li v-for="match in matches" :key="match.id">{{label(match.id)}}：{{(match.similarity*100).toFixed(1)}}%</li></ul>
    </div>
    <p v-if="more" class="mt-2">本次只核对前 100 件同分类在售商品，未覆盖全部商品。</p>
  </aside>
</template>
