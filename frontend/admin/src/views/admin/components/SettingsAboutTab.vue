<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { notifyError, notifySuccess } from '@/utils/notify'
import { aboutPageEditor } from '@/i18n/aboutPageEditor'
import {
  createAboutPageConfig, isSafeAboutLink, isValidSupportWechat, localizedAboutText, newAboutItem,
  type AboutLanguage, type AboutTextField,
} from '../../../../../shared/aboutPage'

const props = defineProps<{ currentLang: AboutLanguage }>()
const { t, locale } = useI18n()
const labels = computed(() => aboutPageEditor[locale.value as AboutLanguage] || aboutPageEditor['zh-CN'])
const submitting = ref(false)
const loaded = ref(false)
const failed = ref(false)
const form = reactive(createAboutPageConfig())
const groups = computed((): Array<{
  key: string; title: string; fields: Array<{ key: AboutTextField; label: string; multiline?: boolean }>
  list?: 'services' | 'recharge'
}> => [
  { key: 'basic', title: labels.value.basic, fields: [
    { key: 'eyebrow', label: labels.value.eyebrow }, { key: 'title', label: labels.value.pageTitle },
    { key: 'subtitle', label: labels.value.subtitle }, { key: 'introductionTitle', label: labels.value.introTitle },
    { key: 'introduction', label: labels.value.introduction, multiline: true },
  ] },
  { key: 'services', title: labels.value.services, fields: [{ key: 'servicesTitle', label: labels.value.sectionTitle }], list: 'services' },
  { key: 'recharge', title: labels.value.recharge, fields: [{ key: 'rechargeTitle', label: labels.value.sectionTitle }], list: 'recharge' },
  { key: 'cooperation', title: labels.value.cooperation, fields: [
    { key: 'cooperationTitle', label: labels.value.sectionTitle }, { key: 'cooperation', label: labels.value.text, multiline: true },
    { key: 'cooperationNote', label: labels.value.note, multiline: true },
  ] },
  { key: 'contact', title: labels.value.contact, fields: [
    { key: 'contactTitle', label: labels.value.sectionTitle }, { key: 'contactSubtitle', label: labels.value.contactSubtitle },
    { key: 'contactNote', label: labels.value.note, multiline: true }, { key: 'contactTip', label: labels.value.tip, multiline: true },
    { key: 'wechatLabel', label: labels.value.wechatLabel }, { key: 'copyLabel', label: labels.value.copyLabel },
    { key: 'copiedLabel', label: labels.value.copiedLabel }, { key: 'telegramLabel', label: labels.value.telegramLabel },
    { key: 'whatsappLabel', label: labels.value.whatsappLabel },
  ] },
])
const fetchContent = async () => {
  loaded.value = false
  failed.value = false
  try {
    const [pageRes, siteRes] = await Promise.all([
      adminAPI.getSettings({ key: 'about_page_config' }), adminAPI.getSettings({ key: 'site_config' }),
    ])
    const site = siteRes.data?.data as Record<string, unknown> | undefined
    Object.assign(form, createAboutPageConfig(pageRes.data?.data, site?.about, site?.contact))
    loaded.value = true
  } catch {
    failed.value = true
  }
}
const save = async () => {
  if (submitting.value) return
  if (!loaded.value) { notifyError(labels.value.failed); return }
  if (!isValidSupportWechat(form.supportWechat)) { notifyError(labels.value.invalidWechat); return }
  if (form.links.some(link => !Object.values(link.title).some(text => text.trim()) || !isSafeAboutLink(link.to))) {
    notifyError(labels.value.invalidLink); return
  }
  submitting.value = true
  try {
    const res = await adminAPI.updateSettings({ key: 'about_page_config', value: form })
    Object.assign(form, createAboutPageConfig(res.data?.data))
    notifySuccess(t('admin.settings.alerts.saveSuccess'))
  } catch {
    notifyError(t('admin.settings.alerts.saveFailed'))
  } finally { submitting.value = false }
}
onMounted(fetchContent)
defineExpose({ save, submitting })
</script>

<template>
  <div class="rounded-xl border border-border bg-card" data-about-editor>
    <div class="flex flex-col gap-3 border-b border-border bg-muted/40 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold">{{ labels.title }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ labels.description }}</p>
      </div>
      <span class="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">{{ currentLang }}</span>
    </div>
    <p v-if="failed" class="p-6 text-sm text-destructive" role="alert">{{ labels.failed }}</p>
    <p v-else-if="!loaded" class="p-6 text-sm text-muted-foreground">{{ t('admin.common.loading') }}</p>
    <div v-else class="space-y-6 p-4 sm:p-6">
      <section v-for="group in groups" :key="group.key" class="rounded-xl border border-border" :data-about-section="group.key">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-muted/30 px-4 py-3">
          <h3 class="text-sm font-semibold">{{ group.title }}</h3>
          <Button v-if="group.list" type="button" size="sm" variant="outline" :disabled="form[group.list].length >= 12" @click="form[group.list].push(newAboutItem())">{{ labels.add }}</Button>
        </div>
        <div class="space-y-4 p-4">
          <div v-for="field in group.fields" :key="field.key" class="space-y-2">
            <Label :for="'about-' + field.key" class="text-xs font-medium text-muted-foreground">{{ field.label }}</Label>
            <Textarea v-if="field.multiline" :id="'about-' + field.key" v-model="form[field.key][props.currentLang]" :rows="field.key === 'introduction' ? 4 : 3" maxlength="8000" />
            <Input v-else :id="'about-' + field.key" v-model="form[field.key][props.currentLang]" maxlength="8000" />
          </div>
          <template v-if="group.list">
            <div v-for="(item, index) in form[group.list]" :key="index" class="space-y-3 rounded-lg border border-border bg-muted/10 p-3">
              <div class="flex items-center justify-between gap-3">
                <span class="text-xs text-muted-foreground">{{ index + 1 }}</span>
                <Button type="button" size="sm" variant="destructive" :aria-label="labels.remove + ' ' + (index + 1)" @click="form[group.list].splice(index, 1)">{{ labels.remove }}</Button>
              </div>
              <div class="space-y-2">
                <Label :for="'about-' + group.list + '-' + index + '-title'" class="text-xs">{{ labels.itemTitle }}</Label>
                <Input :id="'about-' + group.list + '-' + index + '-title'" v-model="item.title[props.currentLang]" maxlength="8000" />
              </div>
              <div class="space-y-2">
                <Label :for="'about-' + group.list + '-' + index + '-text'" class="text-xs">{{ labels.text }}</Label>
                <Textarea :id="'about-' + group.list + '-' + index + '-text'" v-model="item.text[props.currentLang]" :rows="3" maxlength="8000" />
              </div>
            </div>
          </template>
          <div v-if="group.key === 'recharge'" class="space-y-2">
            <Label for="about-rechargeNote" class="text-xs">{{ labels.note }}</Label>
            <Textarea id="about-rechargeNote" v-model="form.rechargeNote[props.currentLang]" :rows="3" maxlength="8000" />
          </div>
          <div v-if="group.key === 'contact'" class="space-y-2">
            <Label for="about-supportWechat" class="text-xs">{{ labels.wechat }}</Label>
            <Input id="about-supportWechat" v-model="form.supportWechat" maxlength="32" />
            <p class="text-xs text-muted-foreground">{{ labels.wechatHint }}</p>
          </div>
        </div>
      </section>
      <section class="rounded-xl border border-border" data-about-section="links">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-muted/30 px-4 py-3">
          <h3 class="text-sm font-semibold">{{ labels.links }}</h3>
          <Button type="button" size="sm" variant="outline" :disabled="form.links.length >= 12" @click="form.links.push({ title: localizedAboutText(), to: '' })">{{ labels.add }}</Button>
        </div>
        <div class="space-y-4 p-4">
          <div class="space-y-2">
            <Label for="about-linksLabel" class="text-xs">{{ labels.linksLabel }}</Label>
            <Input id="about-linksLabel" v-model="form.linksLabel[props.currentLang]" maxlength="8000" />
          </div>
          <p class="text-xs text-muted-foreground">{{ labels.linkHint }}</p>
          <div v-for="(link, index) in form.links" :key="index" class="space-y-3 rounded-lg border border-border bg-muted/10 p-3">
            <div class="flex items-center justify-between gap-3">
              <span class="text-xs text-muted-foreground">{{ index + 1 }}</span>
              <Button type="button" size="sm" variant="destructive" :aria-label="labels.remove + ' ' + (index + 1)" @click="form.links.splice(index, 1)">{{ labels.remove }}</Button>
            </div>
            <div class="space-y-2">
              <Label :for="'about-link-' + index + '-title'" class="text-xs">{{ labels.linkTitle }}</Label>
              <Input :id="'about-link-' + index + '-title'" v-model="link.title[props.currentLang]" maxlength="8000" />
            </div>
            <div class="space-y-2">
              <Label :for="'about-link-' + index + '-to'" class="text-xs">{{ labels.linkUrl }}</Label>
              <Input :id="'about-link-' + index + '-to'" v-model="link.to" maxlength="2000" />
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
