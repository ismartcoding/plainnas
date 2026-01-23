<template>
  <!-- Header variant: single input, filter dropdown is inside TokenSearchField -->
  <div class="header-search">
    <TokenSearchField
ref="inputRef" class="header-search-field" :text="text" :tokens="uiTokens"
      enter-submits
      :placeholder="resolvedPlaceholder" :key-options="keyOptions" :value-options="valueOptions"
      @update:text="onFreeTextChange" @update:tokens="onUiTokensChange" @enter="submitFromHeader"
      @history:select="applyHistoryQ" @history:delete="deleteHistoryItem" @history:clear="clearHistoryForPage" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useMainStore } from '@/stores/main'
import { replacePath } from '@/plugins/router'
import { buildQuery, parseQuery } from '@/lib/search'
import { decodeBase64, encodeBase64 } from '@/lib/strutil'
import { isEditableTarget } from '@/lib/dom'
import type { IBucket, IFilter, IFileFilter, ITag, IType } from '@/lib/interfaces'
import { useSearch as useMediaSearch } from '@/hooks/search'
import { useSearch as useFilesSearch } from '@/hooks/files'
import { DataType } from '@/lib/data'
import { useBucketsTags } from '@/hooks/media'
import TokenSearchField, { type Token as UiToken } from '@/components/TokenSearchField.vue'

type Kind = 'global' | 'media' | 'files'

const props = withDefaults(
  defineProps<{
    kind?: Kind
    placeholder?: string
    enableSlashFocus?: boolean
    showShortcutHint?: boolean

    // Global (header) navigation
    targetPath?: string
    syncRouteQ?: boolean

    // Media-like (SearchInput)
    filter?: IFilter
    getUrl?: (q: string) => string
    tags?: ITag[]
    buckets?: IBucket[]
    types?: IType[]
    showTrash?: boolean

    // Files-like (FileSearchInput)
    fileFilter?: IFileFilter
    getFileUrl?: (q: string) => string
    navigateToDir?: (dir: string) => void
  }>(),
  {
    kind: 'global',
    placeholder: '',
    enableSlashFocus: true,
    showShortcutHint: true,
    targetPath: '',
    syncRouteQ: true,

    filter: undefined,
    getUrl: undefined,
    tags: () => [],
    buckets: () => [],
    types: () => [],
    showTrash: false,

    fileFilter: undefined,
    getFileUrl: undefined,
    navigateToDir: () => { },
  }
)

const router = useRouter()
const mainStore = useMainStore()
const { t } = useI18n()

const inputRef = ref<{ focus: () => void } | null>(null)

const text = ref('')

const { buildQ: buildMediaQ, copyFilter: copyMediaFilter, parseQ: parseMediaQ } = useMediaSearch()
const { buildQ: buildFilesQ, parseQ: parseFilesQ } = useFilesSearch()

const mediaLocalFilter: IFilter = reactive({ tagIds: [] })
const filesLocalFilter: IFileFilter = reactive({
  showHidden: false,
  type: '',
  rootPath: '',
  relativePath: '',
  trash: false,
  text: '',
  fileSize: undefined,
})

const routeGroup = computed(() => String(router.currentRoute.value.meta?.group ?? ''))
const isFilesTrashPage = computed(() => router.currentRoute.value.path === '/files/trash')
const showMediaFilters = computed(() => {
  return routeGroup.value === 'audios' || routeGroup.value === 'videos' || routeGroup.value === 'images'
})
const showFilesFilters = computed(() => routeGroup.value === 'files')

const resolvedPlaceholder = computed(() => {
  return props.placeholder || t('search_hint')
})

const { tags: audioTags, buckets: audioBuckets, fetch: fetchAudioBucketsTags } = useBucketsTags(DataType.AUDIO)
const { tags: videoTags, buckets: videoBuckets, fetch: fetchVideoBucketsTags } = useBucketsTags(DataType.VIDEO)
const { tags: imageTags, buckets: imageBuckets, fetch: fetchImageBucketsTags } = useBucketsTags(DataType.IMAGE)

const mediaTags = computed<ITag[]>(() => {
  if (routeGroup.value === 'audios') return audioTags.value
  if (routeGroup.value === 'videos') return videoTags.value
  if (routeGroup.value === 'images') return imageTags.value
  return []
})

const mediaBuckets = computed<IBucket[]>(() => {
  if (routeGroup.value === 'audios') return audioBuckets.value
  if (routeGroup.value === 'videos') return videoBuckets.value
  if (routeGroup.value === 'images') return imageBuckets.value
  return []
})

const sortedMediaBuckets = computed<IBucket[]>(() =>
  [...(mediaBuckets.value ?? [])].sort((a, b) =>
    (a.name ?? '').localeCompare(b.name ?? '', undefined, { numeric: true, sensitivity: 'base' })
  )
)

const kind = computed<Kind>(() => props.kind ?? 'global')
const HISTORY_MAX = 10

type HistoryByPage = Record<string, string[]>

const pageKey = computed(() => {
  // Per-page history bucket: route path is stable and excludes query.
  return router.currentRoute.value.path || ''
})

const historyQ = computed<string[]>(() => {
  const all = (mainStore as any).searchHistory as HistoryByPage | undefined
  return [...((all ?? {})[pageKey.value] ?? [])]
})

function setHistoryForPage(next: string[]) {
  const all = ({ ...((mainStore as any).searchHistory ?? {}) } as HistoryByPage)
  if (next.length === 0) {
    delete all[pageKey.value]
  } else {
    all[pageKey.value] = next
  }
  ; (mainStore as any).searchHistory = all
}

function rememberHistoryDecoded(q: string) {
  const normalized = String(q ?? '').trim()
  if (!normalized) return

  const list = [...(historyQ.value ?? [])]
  const next = [normalized, ...list.filter((it) => it !== normalized)].slice(0, HISTORY_MAX)
  setHistoryForPage(next)
}

function rememberHistoryBase64(qBase64: string) {
  if (!qBase64) return
  try {
    rememberHistoryDecoded(decodeBase64(qBase64))
  } catch {
    // ignore
  }
}

function deleteHistoryItem(q: string) {
  if (!q) return
  const next = (historyQ.value ?? []).filter((it) => it !== q)
  setHistoryForPage(next)
}

function clearHistoryForPage() {
  setHistoryForPage([])
}

function formatHistoryLabel(decoded: string) {
  const parts: string[] = []
  const textParts: string[] = []

  for (const f of parseQuery(decoded)) {
    if (f.name === 'text') {
      if (f.value) textParts.push(f.value)
      continue
    }

    if (f.name === 'show_hidden') {
      // no need to show hidden items in history label
      continue
    }

    if (f.name === 'bucket_id') {
      const b = (mediaBuckets.value ?? []).find((it) => it.id === f.value)
      parts.push(`${t('folder')}: ${b?.name ?? f.value}`)
      continue
    }

    if (f.name === 'tag_id') {
      const tag = (mediaTags.value ?? []).find((it) => it.id === f.value)
      parts.push(`${t('tag')}: ${tag?.name ?? f.value}`)
      continue
    }

    if (f.name === 'trash') {
      const v = String(f.value ?? '').toLowerCase()
      if (v === 'true') parts.push(`${t('trash')}: ${t('yes')}`)
      else if (v === 'false') parts.push(`${t('trash')}: ${t('no')}`)
      else parts.push(`${t('trash')}: ${String(f.value ?? '')}`)
      continue
    }

    if (f.name === 'file_size') {
      parts.push(`${t('file_size')}: ${f.op}${f.value}`)
      continue
    }

    if (f.value) parts.push(`${f.name}:${f.value}`)
  }

  const main = [...parts, ...textParts].join(' ')
  return main.trim()
}

const historyValueOptions = computed(() => {
  return (historyQ.value ?? [])
    .map((q) => ({ value: q, label: formatHistoryLabel(q) || q }))
    .filter((it) => it.value)
})

const keyOptions = computed(() => {
  const hasHistory = (historyQ.value ?? []).length > 0

  // Trash is page-bound; do not expose it as a user-selectable token.
  if (showMediaFilters.value) return hasHistory ? ['history', 'tag', 'bucket'] : ['tag', 'bucket']
  if (showFilesFilters.value) {
    if (isFilesTrashPage.value) return hasHistory ? ['history'] : []
    return hasHistory ? ['history', 'file_size'] : ['file_size']
  }
  return hasHistory ? ['history'] : []
})

const fileSizeOptions = computed(() => {
  return [
    { value: '>1MB', label: '> 1MB', description: t('search_file_size_greater_than_1mb') },
    { value: '>10MB', label: '> 10MB', description: t('search_file_size_greater_than_10mb') },
    { value: '>100MB', label: '> 100MB', description: t('search_file_size_greater_than_100mb') },
    { value: '>1GB', label: '> 1GB', description: t('search_file_size_greater_than_1gb') },
    { value: '<1MB', label: '< 1MB', description: t('search_file_size_less_than_1mb') },
    { value: '<100KB', label: '< 100KB', description: t('search_file_size_less_than_100kb') },
  ]
})

const valueOptions = computed<Record<string, any[]>>(() => {
  const base: Record<string, any> = {}

  if ((historyQ.value ?? []).length > 0) {
    base.history = historyValueOptions.value
  }

  if (showMediaFilters.value) {
    base.tag = (mediaTags.value ?? []).map((t) => t.name)
    base.bucket = (sortedMediaBuckets.value ?? []).map((b) => b.name)
  }

  if (showFilesFilters.value) {
    base.file_size = fileSizeOptions.value
  }

  return base
})

const uiTokens = computed<UiToken[]>(() => {
  const tokens: UiToken[] = []

  if (showMediaFilters.value) {
    if (mediaLocalFilter.bucketId) {
      const b = (mediaBuckets.value ?? []).find((it) => it.id === mediaLocalFilter.bucketId)
      if (b) tokens.push({ key: 'bucket', value: b.name })
    }

    for (const id of mediaLocalFilter.tagIds ?? []) {
      const t = (mediaTags.value ?? []).find((it) => it.id === id)
      if (t) tokens.push({ key: 'tag', value: t.name })
    }

    return tokens
  }

  if (showFilesFilters.value) {
    if (filesLocalFilter.fileSize) {
      tokens.push({ key: 'file_size', value: filesLocalFilter.fileSize })
    }
    return tokens
  }
  return tokens
})

function onFreeTextChange(v: string) {
  text.value = v
}

function onUiTokensChange(tokens: UiToken[]) {
  // Translate UI tokens into internal filter state. Invalid/unrecognized values are ignored;
  // the input keeps them as plain text (TokenSearchField only tokenizes known values).

  if (showMediaFilters.value) {
    const nextTagIds: string[] = []
    for (const tok of tokens) {
      if (tok.key !== 'tag') continue
      const tag = (mediaTags.value ?? []).find((t) => t.name.toLowerCase() === tok.value.toLowerCase())
      if (tag) nextTagIds.push(tag.id)
    }

    const bucketTok = tokens.find((it) => it.key === 'bucket')
    const bucket = bucketTok
      ? (mediaBuckets.value ?? []).find((b) => b.name.toLowerCase() === bucketTok.value.toLowerCase())
      : undefined

    mediaLocalFilter.tagIds = nextTagIds
    mediaLocalFilter.bucketId = bucket?.id
    mediaLocalFilter.text = text.value
    return
  }

  if (showFilesFilters.value) {
    const fileSizeTok = tokens.find((it) => it.key === 'file_size')
    filesLocalFilter.fileSize = fileSizeTok?.value
    filesLocalFilter.text = text.value
    return
  }
}

function parseCurrentFields() {
  const qRaw = router.currentRoute.value.query.q?.toString() ?? ''
  if (!qRaw) return [] as { name: string; op: string; value: string }[]
  try {
    return parseQuery(decodeBase64(qRaw))
  } catch {
    return [] as { name: string; op: string; value: string }[]
  }
}

function decodedCurrentQuery() {
  const qRaw = router.currentRoute.value.query.q?.toString() ?? ''
  if (!qRaw) return ''
  try {
    return decodeBase64(qRaw)
  } catch {
    return ''
  }
}

function buildNextQ(nextText: string, nextShowHidden: boolean) {
  const fields = parseCurrentFields().filter((f) => f.name !== 'text' && f.name !== 'show_hidden')
  const t = nextText.trim()
  if (t) {
    fields.push({ name: 'text', op: '', value: t })
  }
  if (nextShowHidden) {
    fields.push({ name: 'show_hidden', op: '', value: 'true' })
  }
  if (fields.length === 0) return ''
  return encodeBase64(buildQuery(fields))
}

function buildNextMediaQ(next: IFilter) {
  const fields = parseCurrentFields().filter(
    (f) => f.name !== 'text' && f.name !== 'tag_id' && f.name !== 'bucket_id' && f.name !== 'type' && f.name !== 'trash'
  )

  if (next.bucketId) {
    fields.push({ name: 'bucket_id', op: '', value: next.bucketId })
  }

  if (next.trash !== undefined) {
    fields.push({ name: 'trash', op: '', value: next.trash ? 'true' : 'false' })
  }

  if (next.type) {
    fields.push({ name: 'type', op: '', value: next.type })
  }

  for (const id of next.tagIds ?? []) {
    fields.push({ name: 'tag_id', op: '', value: id })
  }

  if (next.text !== undefined) {
    const t = String(next.text).trim()
    if (t) {
      fields.push({ name: 'text', op: '', value: t })
    }
  }

  if (fields.length === 0) return ''
  return encodeBase64(buildQuery(fields))
}

function replaceCurrentRouteQ(q: string) {
  const route = router.currentRoute.value
  const targetPath = props.targetPath || (route.path === '/' ? '/files' : route.path)
  const nextQuery: Record<string, any> = { ...route.query }
  delete nextQuery.page
  delete nextQuery.q
  if (q) nextQuery.q = q

  const fullPath = router.resolve({ path: targetPath, query: nextQuery }).fullPath
  replacePath(mainStore, fullPath)

  // Only store non-empty queries.
  if (q) rememberHistoryBase64(q)
}

function applyHistoryQ(q: string) {
  if (!q) return

  // Update local UI state immediately so tokens/text reflect the selected history
  // (TokenSearchField blurs before emitting history:select).
  try {
    const fields = parseQuery(q)
    const textField = fields.find((it) => it.name === 'text')
    text.value = textField?.value ?? ''
  } catch {
    // ignore
  }
  parseMediaQ(mediaLocalFilter, q)
  parseFilesQ(filesLocalFilter, q)

  const qBase64 = encodeBase64(q)

  if (kind.value === 'global') {
    replaceCurrentRouteQ(qBase64)
    return
  }

  if (kind.value === 'media') {
    if (!props.getUrl) return
    replacePath(mainStore, props.getUrl(qBase64))
    rememberHistoryDecoded(q)
    return
  }

  if (kind.value === 'files') {
    if (!props.getFileUrl) return
    replacePath(mainStore, props.getFileUrl(qBase64))
    rememberHistoryDecoded(q)
    return
  }
}

function syncFromRoute() {
  if (!props.syncRouteQ) return

  const fields = parseCurrentFields()
  const textField = fields.find((it) => it.name === 'text')
  text.value = textField?.value ?? ''

  const decoded = decodedCurrentQuery()
  parseMediaQ(mediaLocalFilter, decoded)
  parseFilesQ(filesLocalFilter, decoded)
}

function submitGlobal(value?: string) {
  // Ensure local filters reflect the current UI before building the q.
  mediaLocalFilter.text = value ?? text.value
  filesLocalFilter.text = value ?? text.value
  const q = buildNextQ(value ?? text.value, isFilesTrashPage.value ? true : filesLocalFilter.showHidden)
  replaceCurrentRouteQ(q)
}


function isAbsolutePath(input: string) {
  return input.startsWith('/')
}

function applyPanel() {
  if (kind.value === 'global') {
    submitGlobal(text.value)
  } else if (kind.value === 'media') {
    if (!props.filter || !props.getUrl) return
    const nextFilter: IFilter = {
      ...props.filter,
      text: text.value,
      tagIds: [...(mediaLocalFilter.tagIds ?? [])],
      bucketId: mediaLocalFilter.bucketId,
      type: mediaLocalFilter.type,
      trash: mediaLocalFilter.trash,
    }

    const q = buildMediaQ(nextFilter)
    replacePath(mainStore, props.getUrl(q))
    if (q) rememberHistoryBase64(q)
  } else if (kind.value === 'files') {
    if (!props.fileFilter || !props.getFileUrl) return
    const inputText = text.value.trim()
    if (inputText && isAbsolutePath(inputText) && props.navigateToDir) {
      props.navigateToDir(inputText)
      return
    }

    const nextFilter: IFileFilter = {
      ...props.fileFilter,
      ...filesLocalFilter,
      text: text.value,
    }

    const q = buildFilesQ(nextFilter)
    replacePath(mainStore, props.getFileUrl(q))
    if (q) rememberHistoryBase64(q)
  }
}

function submitFromHeader() {
  // Enter on the header input submits without forcing the panel open.
  if (kind.value === 'global') {
    // Build q based on current route group.
    if (showMediaFilters.value) {
      const q = buildNextMediaQ({ ...mediaLocalFilter, text: text.value })
      replaceCurrentRouteQ(q)
      return
    }

    if (showFilesFilters.value) {
      const nextFilter: IFileFilter = {
        ...filesLocalFilter,
        text: text.value,
        showHidden: isFilesTrashPage.value ? true : filesLocalFilter.showHidden,
      }
      const q = buildFilesQ(nextFilter)
      replaceCurrentRouteQ(q)
      return
    }

    submitGlobal(text.value)
    return
  }
  applyPanel()
}

function onGlobalKeydown(event: KeyboardEvent) {
  if (!props.enableSlashFocus) return
  if (event.key !== '/') return
  if (event.ctrlKey || event.metaKey || event.altKey) return
  if (isEditableTarget(event.target)) return
  event.preventDefault()
  inputRef.value?.focus()
}

watch(
  () => router.currentRoute.value.fullPath,
  () => {
    syncFromRoute()
  },
  { immediate: true }
)

watch(
  () => routeGroup.value,
  (g) => {
    if (g === 'audios') {
      fetchAudioBucketsTags()
    } else if (g === 'videos') {
      fetchVideoBucketsTags()
    } else if (g === 'images') {
      fetchImageBucketsTags()
    }
  },
  { immediate: true }
)

watch(
  () => props.filter,
  (v) => {
    if (!v) return
    copyMediaFilter(v, mediaLocalFilter)
    text.value = v.text ?? ''
  },
  { immediate: true, deep: true }
)

watch(
  () => props.fileFilter,
  (v) => {
    if (!v) return
    Object.assign(filesLocalFilter, v)
    text.value = v.text ?? ''
  },
  { immediate: true, deep: true }
)

onMounted(() => {
  window.addEventListener('keydown', onGlobalKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onGlobalKeydown)
})

defineExpose({
  focus: () => inputRef.value?.focus(),
})
</script>

<style scoped lang="scss">
.header-search {
  min-width: min(520px, 46vw);
}

.header-search :deep(.header-search-field) {
  width: 100%;
}
</style>
