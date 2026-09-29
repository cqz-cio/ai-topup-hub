<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  AlertCircle,
  CheckCircle2,
  Copy,
  Download,
  ExternalLink,
  Info,
  RefreshCw,
  RotateCcw,
} from 'lucide-vue-next'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { adminAPI } from '@/api/admin'
import { renderReleaseNotes } from '@/utils/releaseNotes'
import { confirmAction } from '@/utils/confirm'
import { notifyError, notifySuccess } from '@/utils/notify'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const { t } = useI18n()

interface UpdateCheckResult {
  current_version: string
  latest_version: string
  has_update: boolean
  release_url?: string
  release_name?: string
  release_notes?: string
  published_at?: string | null
  source?: string
}

type BlockReason =
  | ''
  | 'container'
  | 'unsupported_os'
  | 'source_build'
  | 'dir_not_writable'
  | 'exec_not_found'

interface BackupInfo {
  previous_version: string
  target_version: string
  swapped_at: string
  // 迁移开始后即视为不安全；即使启动失败，schema 也可能已经部分改变
  migration_started: boolean
  migration_started_at?: string
  // 完整启动状态用于展示与诊断，安全判定仍以后端 rollback_unsafe 为准
  new_version_started: boolean
  new_version_started_at?: string
}

interface UpdateCapability {
  can_update: boolean
  can_restart: boolean
  block_reason?: BlockReason
  deployment: 'binary' | 'container'
  supervisor: 'systemd' | 'none'
  // systemd unit 的 Restart= 取值，决定 can_restart
  restart_policy?: string
  build_type: string
  exec_path?: string
  platform: string
  asset_name?: string
  has_backup: boolean
  backup?: BackupInfo
  // 回滚是否需要确认风险后强制执行。迁移已开始、或元数据不可信时都为 true。
  // 由后端统一判定，前端不要自己从 backup 推导 —— 元数据缺失时 backup 本身就是空的。
  rollback_unsafe: boolean
}

type UpdateStatus = 'idle' | 'running' | 'succeeded' | 'failed'
type UpdateStage = 'idle' | 'downloading' | 'verifying' | 'extracting' | 'swapping' | 'done'

interface UpdateState {
  status: UpdateStatus
  stage: UpdateStage
  percent: number
  target_version?: string
  error?: string
  restart_required: boolean
  started_at?: string | null
  finished_at?: string | null
}

const checkLoading = ref(false)
const checkResult = ref<UpdateCheckResult | null>(null)
const checkError = ref('')

const capability = ref<UpdateCapability | null>(null)
const updateState = ref<UpdateState | null>(null)
const starting = ref(false)
const restarting = ref(false)
const rollingBack = ref(false)
const copied = ref(false)

// 升级任务在后端异步执行，前端靠轮询拿进度；重启期间同一个定时器用来探活。
let pollTimer: ReturnType<typeof setInterval> | null = null
const POLL_INTERVAL = 1500
// 重启探活上限：systemd 拉起 + 数据库迁移在慢机器上可能要几十秒
const RESTART_TIMEOUT_MS = 120_000

const dockerUpgradeCommand = 'docker compose pull && docker compose up -d'

const renderedReleaseNotes = computed(() => {
  const raw = checkResult.value?.release_notes
  if (!raw) return ''
  return renderReleaseNotes(raw)
})

const isUpdating = computed(() => updateState.value?.status === 'running')

// 二进制已替换但进程仍是旧版本 —— 无论本次会话内升级的还是上次遗留的，都需要重启
const needsRestart = computed(() => updateState.value?.restart_required === true)

const canOneClickUpgrade = computed(
  () => checkResult.value?.has_update === true && capability.value?.can_update === true,
)

// 有新版本但环境不允许自动升级时，展示对应的手动升级指引
const blockedWithUpdate = computed(
  () => checkResult.value?.has_update === true && capability.value?.can_update === false,
)

const stageLabel = computed(() => {
  const stage = updateState.value?.stage
  if (!stage || stage === 'idle') return ''
  return t(`admin.systemUpdate.stage.${stage}`)
})

const progressPercent = computed(() => {
  const state = updateState.value
  if (!state) return 0
  if (state.status === 'succeeded') return 100
  // 只有下载阶段有真实百分比，后续三个阶段都很快，给固定推进值避免进度条卡住
  switch (state.stage) {
    case 'downloading':
      return Math.min(state.percent, 90)
    case 'verifying':
      return 93
    case 'extracting':
      return 96
    case 'swapping':
      return 98
    case 'done':
      return 100
    default:
      return 0
  }
})

function formatPublishedAt(raw: string | null | undefined) {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

function extractErrorMessage(err: any): string {
  return err?.response?.data?.msg || err?.message || ''
}

async function loadCheck() {
  checkLoading.value = true
  checkError.value = ''
  checkResult.value = null
  try {
    const res = await adminAPI.checkSystemUpdate()
    const payload = res.data?.data as UpdateCheckResult | undefined
    if (payload) {
      checkResult.value = payload
    } else {
      checkError.value = t('admin.updateCheck.failedGeneric')
    }
  } catch (err: any) {
    const status = err?.response?.status
    const code = err?.response?.data?.status_code
    const msg = extractErrorMessage(err)
    if (status === 429 || code === 429) {
      checkError.value = t('admin.updateCheck.rateLimited')
    } else if (msg) {
      checkError.value = t('admin.updateCheck.failed', { message: msg })
    } else {
      checkError.value = t('admin.updateCheck.failedGeneric')
    }
  } finally {
    checkLoading.value = false
  }
}

async function loadCapability() {
  try {
    const res = await adminAPI.getUpdateCapability()
    const payload = res.data?.data as { capability: UpdateCapability; state: UpdateState } | undefined
    if (payload) {
      capability.value = payload.capability
      updateState.value = payload.state
      // 打开面板时后端可能仍在跑上一次触发的升级，直接接上进度
      if (payload.state?.status === 'running') startPolling()
    }
  } catch {
    // 能力探测失败不阻断版本检测结果的展示，只是不显示升级按钮
    capability.value = null
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => {
    try {
      const res = await adminAPI.getUpdateStatus()
      const state = res.data?.data as UpdateState | undefined
      if (!state) return
      updateState.value = state
      if (state.status !== 'running') {
        stopPolling()
        if (state.status === 'succeeded') {
          notifySuccess(t('admin.systemUpdate.downloadSucceeded'))
          // 升级完成后备份已生成，刷新能力探测以显示回滚入口
          void loadCapability()
        } else if (state.status === 'failed') {
          notifyError(state.error || t('admin.systemUpdate.failed'))
        }
      }
    } catch {
      // 轮询失败通常是网络抖动，保持定时器继续尝试
    }
  }, POLL_INTERVAL)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function handleStartUpdate() {
  const ok = await confirmAction({
    title: t('admin.systemUpdate.confirmTitle'),
    description: t('admin.systemUpdate.confirmDescription', {
      version: checkResult.value?.latest_version || '',
    }),
    confirmText: t('admin.systemUpdate.upgradeNow'),
  })
  if (!ok) return

  starting.value = true
  try {
    const res = await adminAPI.startSystemUpdate()
    updateState.value = (res.data?.data as UpdateState) || null
    startPolling()
  } catch (err: any) {
    notifyError(extractErrorMessage(err) || t('admin.systemUpdate.failed'))
  } finally {
    starting.value = false
  }
}

async function handleRestart() {
  const ok = await confirmAction({
    title: t('admin.systemUpdate.restartConfirmTitle'),
    description: t('admin.systemUpdate.restartConfirmDescription'),
    confirmText: t('admin.systemUpdate.restartNow'),
    variant: 'destructive',
  })
  if (!ok) return

  restarting.value = true
  try {
    await adminAPI.restartSystemService()
    waitForBackend()
  } catch (err: any) {
    restarting.value = false
    notifyError(extractErrorMessage(err) || t('admin.systemUpdate.restartFailed'))
  }
}

// 重启期间后端会短暂不可达，轮询到恢复后刷新整页，
// 确保内嵌的前端资源也换成新版本二进制里的那一份。
function waitForBackend() {
  const deadline = Date.now() + RESTART_TIMEOUT_MS
  stopPolling()
  pollTimer = setInterval(async () => {
    if (Date.now() > deadline) {
      stopPolling()
      restarting.value = false
      notifyError(t('admin.systemUpdate.restartTimeout'))
      return
    }
    try {
      await adminAPI.getUpdateStatus()
      stopPolling()
      window.location.reload()
    } catch {
      // 后端还没起来，继续等
    }
  }, 2000)
}

// 迁移已经开始（即使服务后来没启动成功），或者元数据不可信、压根判断不了 ——
// 两种情况回滚风险都远高于「刚替换完还没重启」，确认文案要换成更重的版本并带 force。
const rollbackIsRisky = computed(() => capability.value?.rollback_unsafe === true)
// 元数据不可信时连「会退到哪个版本」都不知道，提示要说清楚这一点而不是编一个版本号
const rollbackVersionUnknown = computed(() => !capability.value?.backup?.previous_version)

async function handleRollback() {
  const risky = rollbackIsRisky.value
  const backup = capability.value?.backup
  const ok = await confirmAction({
    title: t('admin.systemUpdate.rollbackConfirmTitle'),
    description: !risky
      ? t('admin.systemUpdate.rollbackConfirmDescription')
      : rollbackVersionUnknown.value
        ? t('admin.systemUpdate.rollbackUnknownDescription')
        : t('admin.systemUpdate.rollbackRiskyDescription', {
            previous: backup?.previous_version || t('admin.updateCheck.unknownVersion'),
            current: backup?.target_version || t('admin.updateCheck.unknownVersion'),
          }),
    confirmText: t('admin.systemUpdate.rollback'),
    variant: 'destructive',
  })
  if (!ok) return

  rollingBack.value = true
  try {
    const res = await adminAPI.rollbackSystemUpdate({ force: risky })
    updateState.value = (res.data?.data as UpdateState) || null
    notifySuccess(t('admin.systemUpdate.rollbackSucceeded'))
    void loadCapability()
  } catch (err: any) {
    notifyError(extractErrorMessage(err) || t('admin.systemUpdate.rollbackFailed'))
  } finally {
    rollingBack.value = false
  }
}

async function copyDockerCommand() {
  try {
    await navigator.clipboard.writeText(dockerUpgradeCommand)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    notifyError(t('admin.systemUpdate.copyFailed'))
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      copied.value = false
      void loadCheck()
      void loadCapability()
    } else {
      // 关闭面板时停止轮询；升级任务本身在后端继续跑，重新打开会接上进度
      stopPolling()
    }
  },
)

onBeforeUnmount(stopPolling)
</script>

<template>
  <Dialog :open="props.open" @update:open="emit('update:open', $event)">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ t('admin.updateCheck.title') }}</DialogTitle>
      </DialogHeader>

      <div class="space-y-3 text-sm">
        <div v-if="checkLoading" class="flex items-center gap-2 py-4 text-muted-foreground">
          <RefreshCw class="h-4 w-4 animate-spin" />
          <span>{{ t('admin.updateCheck.checking') }}</span>
        </div>

        <div v-else-if="checkError" class="flex items-start gap-2 text-destructive">
          <AlertCircle class="mt-0.5 h-4 w-4 shrink-0" />
          <span>{{ checkError }}</span>
        </div>

        <template v-else-if="checkResult">
          <div
            class="flex items-start gap-2 rounded-md border px-3 py-2"
            :class="checkResult.has_update ? 'border-amber-500/40 bg-amber-500/5 text-amber-600 dark:text-amber-400' : 'border-emerald-500/40 bg-emerald-500/5 text-emerald-600 dark:text-emerald-400'"
          >
            <component :is="checkResult.has_update ? AlertCircle : CheckCircle2" class="mt-0.5 h-4 w-4 shrink-0" />
            <span v-if="checkResult.has_update">
              {{ t('admin.updateCheck.hasUpdate', { version: checkResult.latest_version || t('admin.updateCheck.unknownVersion') }) }}
            </span>
            <span v-else>
              {{ t('admin.updateCheck.latest', { version: checkResult.current_version || t('admin.updateCheck.unknownVersion') }) }}
            </span>
          </div>

          <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-muted-foreground">
            <dt>{{ t('admin.updateCheck.currentLabel') }}</dt>
            <dd class="text-foreground">{{ checkResult.current_version || t('admin.updateCheck.unknownVersion') }}</dd>
            <dt>{{ t('admin.updateCheck.latestLabel') }}</dt>
            <dd class="text-foreground">{{ checkResult.latest_version || t('admin.updateCheck.unknownVersion') }}</dd>
            <template v-if="checkResult.published_at">
              <dt>{{ t('admin.updateCheck.publishedLabel') }}</dt>
              <dd class="text-foreground">{{ formatPublishedAt(checkResult.published_at) }}</dd>
            </template>
          </dl>

          <div
            v-if="checkResult.release_notes"
            class="release-notes max-h-40 overflow-y-auto rounded-md border bg-muted/40 px-3 py-2 text-xs text-muted-foreground"
            v-html="renderedReleaseNotes"
          ></div>

          <!-- 升级进度 -->
          <div v-if="isUpdating" class="space-y-2 rounded-md border border-primary/40 bg-primary/5 px-3 py-2.5">
            <div class="flex items-center justify-between gap-2 text-xs">
              <span class="flex items-center gap-1.5 font-medium text-foreground">
                <RefreshCw class="h-3.5 w-3.5 animate-spin" />
                {{ stageLabel }}
              </span>
              <span class="tabular-nums text-muted-foreground">{{ progressPercent }}%</span>
            </div>
            <div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
              <div
                class="h-full rounded-full bg-primary transition-all duration-300"
                :style="{ width: `${progressPercent}%` }"
              ></div>
            </div>
            <p class="text-xs text-muted-foreground">{{ t('admin.systemUpdate.keepOpenHint') }}</p>
          </div>

          <!-- 已替换二进制，等待重启生效 -->
          <div
            v-else-if="needsRestart"
            class="space-y-2 rounded-md border border-emerald-500/40 bg-emerald-500/5 px-3 py-2.5"
          >
            <div class="flex items-start gap-2 text-emerald-600 dark:text-emerald-400">
              <CheckCircle2 class="mt-0.5 h-4 w-4 shrink-0" />
              <span class="text-xs">
                {{
                  capability?.can_restart
                    ? t('admin.systemUpdate.restartRequired')
                    : t('admin.systemUpdate.manualRestartRequired')
                }}
              </span>
            </div>
            <p v-if="!capability?.can_restart" class="text-xs text-muted-foreground">
              {{ t('admin.systemUpdate.manualRestartHint') }}
            </p>
          </div>

          <!-- 升级失败 -->
          <div
            v-else-if="updateState?.status === 'failed'"
            class="space-y-1.5 rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2.5"
          >
            <div class="flex items-start gap-2 text-destructive">
              <AlertCircle class="mt-0.5 h-4 w-4 shrink-0" />
              <span class="text-xs font-medium">{{ t('admin.systemUpdate.failed') }}</span>
            </div>
            <p class="break-all text-xs text-muted-foreground">{{ updateState.error }}</p>
          </div>

          <!-- 环境不支持一键升级：给出对应的手动升级方式 -->
          <div
            v-else-if="blockedWithUpdate && capability"
            class="space-y-2 rounded-md border bg-muted/40 px-3 py-2.5"
          >
            <div class="flex items-start gap-2">
              <Info class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
              <span class="text-xs text-foreground">
                {{ t(`admin.systemUpdate.blocked.${capability.block_reason || 'unsupported_os'}`) }}
              </span>
            </div>

            <template v-if="capability.block_reason === 'container'">
              <p class="text-xs text-muted-foreground">{{ t('admin.systemUpdate.dockerHint') }}</p>
              <div class="flex items-center gap-2">
                <code class="flex-1 overflow-x-auto whitespace-nowrap rounded bg-background px-2 py-1.5 text-xs">
                  {{ dockerUpgradeCommand }}
                </code>
                <Button size="icon-sm" variant="outline" :title="t('admin.systemUpdate.copy')" @click="copyDockerCommand">
                  <CheckCircle2 v-if="copied" class="h-3.5 w-3.5 text-emerald-500" />
                  <Copy v-else class="h-3.5 w-3.5" />
                </Button>
              </div>
            </template>
          </div>
        </template>
      </div>

      <DialogFooter class="gap-2 sm:gap-2">
        <Button
          v-if="capability?.has_backup && !isUpdating"
          variant="outline"
          size="sm"
          class="gap-1.5"
          :disabled="rollingBack"
          @click="handleRollback"
        >
          <RotateCcw class="h-4 w-4" :class="rollingBack ? 'animate-spin' : ''" />
          {{ t('admin.systemUpdate.rollback') }}
        </Button>

        <a
          v-if="checkResult?.release_url"
          :href="checkResult.release_url"
          target="_blank"
          rel="noopener noreferrer"
          class="inline-flex h-9 items-center justify-center gap-1.5 rounded-md border px-3 text-sm font-medium hover:bg-accent"
        >
          <ExternalLink class="h-4 w-4" />
          {{ t('admin.updateCheck.viewRelease') }}
        </a>

        <Button
          v-if="needsRestart && capability?.can_restart"
          size="sm"
          class="gap-1.5"
          :disabled="restarting"
          @click="handleRestart"
        >
          <RefreshCw class="h-4 w-4" :class="restarting ? 'animate-spin' : ''" />
          {{ restarting ? t('admin.systemUpdate.restarting') : t('admin.systemUpdate.restartNow') }}
        </Button>

        <Button
          v-else-if="canOneClickUpgrade && !needsRestart"
          size="sm"
          class="gap-1.5"
          :disabled="starting || isUpdating"
          @click="handleStartUpdate"
        >
          <Download class="h-4 w-4" />
          {{ isUpdating ? t('admin.systemUpdate.upgrading') : t('admin.systemUpdate.upgradeNow') }}
        </Button>

        <Button variant="outline" size="sm" @click="emit('update:open', false)">
          {{ t('admin.updateCheck.close') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.release-notes :deep(h1),
.release-notes :deep(h2),
.release-notes :deep(h3),
.release-notes :deep(h4) {
  color: hsl(var(--foreground));
  font-weight: 600;
  margin: 0.6em 0 0.3em;
  line-height: 1.3;
}
.release-notes :deep(h1) { font-size: 1rem; }
.release-notes :deep(h2) { font-size: 0.95rem; }
.release-notes :deep(h3) { font-size: 0.85rem; }
.release-notes :deep(h4) { font-size: 0.8rem; }
.release-notes :deep(p) { margin: 0.4em 0; }
.release-notes :deep(ul),
.release-notes :deep(ol) {
  padding-left: 1.25rem;
  margin: 0.4em 0;
}
.release-notes :deep(ul) { list-style: disc; }
.release-notes :deep(ol) { list-style: decimal; }
.release-notes :deep(li) { margin: 0.15em 0; }
.release-notes :deep(strong) { color: hsl(var(--foreground)); font-weight: 600; }
.release-notes :deep(em) { font-style: italic; }
.release-notes :deep(a) {
  color: hsl(var(--primary));
  text-decoration: underline;
  text-underline-offset: 2px;
}
.release-notes :deep(code) {
  background: hsl(var(--muted));
  padding: 0.1em 0.35em;
  border-radius: 0.25rem;
  font-size: 0.9em;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.release-notes :deep(pre) {
  background: hsl(var(--muted));
  padding: 0.6em 0.8em;
  border-radius: 0.375rem;
  overflow-x: auto;
  margin: 0.5em 0;
}
.release-notes :deep(pre code) {
  background: transparent;
  padding: 0;
}
.release-notes :deep(blockquote) {
  border-left: 3px solid hsl(var(--border));
  padding-left: 0.75rem;
  margin: 0.5em 0;
  color: hsl(var(--muted-foreground));
}
.release-notes :deep(hr) {
  border: 0;
  border-top: 1px solid hsl(var(--border));
  margin: 0.8em 0;
}
.release-notes :deep(img) { max-width: 100%; height: auto; }
.release-notes :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5em 0;
}
.release-notes :deep(th),
.release-notes :deep(td) {
  border: 1px solid hsl(var(--border));
  padding: 0.3em 0.5em;
  text-align: left;
}
</style>
