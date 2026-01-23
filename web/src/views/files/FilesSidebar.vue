<template>
  <left-sidebar class="files-sidebar">
    <template #body>
      <ul class="nav">
        <li
v-for="item in quickLinks" :key="item.fullPath" :class="{ active: item.isChecked }"
          @click.prevent="openLink(item)">
          <span class="icon" aria-hidden="true">
            <i-lucide:history v-if="item.type === 'RECENTS'" />
            <i-lucide:trash v-else />
          </span>
          <span class="title">{{ item.title }}</span>
          <v-icon-button v-if="item.type === 'TRASH'" v-tooltip="$t('trash_tips')" class="btn-help sm">
            <i-material-symbols:help-outline-rounded />
          </v-icon-button>
          <span v-if="item.type === 'RECENTS' && recentCount >= 0" class="count">{{ recentCount.toLocaleString()
          }}</span>
          <span v-else-if="item.type === 'TRASH' && trashCount >= 0" class="count">{{ trashCount.toLocaleString()
          }}</span>
        </li>
      </ul>

      <div class="section-title">
        {{ $t('volumes') }}
        <v-icon-button v-tooltip="$t('disk_manager')" class="sm" @click.stop="openDiskManager">
          <i-material-symbols:settings-outline-rounded />
        </v-icon-button>
      </div>
      <div class="volumes">
        <VolumeCard
v-for="item in volumeLinks" :key="item.fullPath" :title="item.title" :drive-type="item.driveType"
          :used-percent="item.usedPercent || 0" :count="item.count || ''" :data="item"
          :percent-class="percentClass(item.usedPercent)" :active="item.isChecked" @click="openLink(item)">
          <template #actions>
            <v-icon-button
:id="'volume-' + item.id" v-tooltip="$t('actions')" class="sm"
              @click.prevent.stop="showVolumeMenu(item)">
              <i-material-symbols:more-vert />
            </v-icon-button>
          </template>
        </VolumeCard>
      </div>

      <template v-if="favoriteLinks.length">
        <div class="section-title">{{ $t('favorites') }}</div>
        <ul class="nav">
          <li
v-for="item in favoriteLinks" :key="item.fullPath" :class="{ active: item.isChecked }"
            @click.prevent="openLink(item)">
            <span class="title">{{ item.title }}</span>
            <v-icon-button
:id="'favorite-' + item.fullPath" v-tooltip="$t('actions')" class="sm"
              @click.prevent.stop="showFavoriteMenu(item)">
              <i-material-symbols:more-vert />
            </v-icon-button>
          </li>
        </ul>
      </template>

      <v-dropdown-menu v-model="favoriteMenuVisible" :anchor="'favorite-' + selectedFavorite?.fullPath">
        <div class="dropdown-item" @click="openSetFavoriteAlias(); favoriteMenuVisible = false">{{ $t('rename') }}</div>
        <div class="dropdown-item" @click="removeFavoriteFolder(selectedFavorite!); favoriteMenuVisible = false">
          {{ $t('remove_from_favorites') }}
        </div>
      </v-dropdown-menu>

      <v-dropdown-menu v-model="volumeMenuVisible" :anchor="'volume-' + selectedVolume?.id">
        <div class="dropdown-item" @click="openSetAlias(); volumeMenuVisible = false">{{ $t('rename') }}</div>
      </v-dropdown-menu>
    </template>
  </left-sidebar>
</template>

<script setup lang="ts">
import router, { replacePath } from '@/plugins/router'
import { useMainStore } from '@/stores/main'
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { buildQuery } from '@/lib/search'
import type { IFileFilter, IFavoriteFolder } from '@/lib/interfaces'
import { useSearch } from '@/hooks/files'
import { decodeBase64, encodeBase64 } from '@/lib/strutil'
import { useI18n } from 'vue-i18n'
import { initMutation, removeFavoriteFolderGQL, setFavoriteFolderAliasGQL, setMountAliasGQL } from '@/lib/api/mutation'
import { favoriteFoldersGQL, filesSidebarCountsGQL, initLazyQuery, initQuery } from '@/lib/api/query'
import toast from '@/components/toaster'
import emitter from '@/plugins/eventbus'
import { useVolumes } from '@/hooks/files'
import { getFavoriteDisplayTitle, getFavoriteFolderFullPath } from '@/lib/favorites'
import { formatUsedTotalBytes } from '@/lib/format'
import { openModal } from '@/components/modal'
import EditValueModal from '@/components/EditValueModal.vue'
import VolumeCard from '@/components/storage/VolumeCard.vue'
import DiskManagerModal from '@/components/storage/DiskManagerModal.vue'
import { getStorageVolumeTitle } from '@/lib/volumes'

const mainStore = useMainStore()
const { t } = useI18n()

const favoriteFolders = ref<IFavoriteFolder[]>([])
const { refetch: refetchFavoriteFolders } = initQuery({
  handle: (data: any, error: string) => {
    if (error) return
    favoriteFolders.value = (data?.favoriteFolders || []) as IFavoriteFolder[]
  },
  document: favoriteFoldersGQL,
})

const { parseQ } = useSearch()
const filter = reactive<IFileFilter>({
  showHidden: false,
  type: '',
  rootPath: '',
  text: '',
  relativePath: '',
})

const parent = ref('')
const recent = ref(false)
const trash = ref(false)

const recentCount = ref(-1)
const trashCount = ref(-1)

const { fetch: fetchCounts } = initLazyQuery({
  handle: (data: any, _error: string) => {
    if (data) {
      recentCount.value = typeof data.recentFilesCount === 'number' ? data.recentFilesCount : -1
      trashCount.value = typeof data.trashCount === 'number' ? data.trashCount : -1
    }
  },
  document: filesSidebarCountsGQL,
  variables: () => ({}),
})

const favoriteMenuVisible = ref(false)
const selectedFavorite = ref<LinkItem | null>(null)
const volumeMenuVisible = ref(false)
const selectedVolume = ref<LinkItem | null>(null)
const aliasInput = ref('')

function openDiskManager() {
  openModal(DiskManagerModal)
}

function openRecent() {
  replacePath(mainStore, '/files/recent')
}

function openTrash() {
  replacePath(mainStore, '/files/trash')
}

function showFavoriteMenu(item: LinkItem) {
  selectedFavorite.value = item
  // Close other dropdowns before opening this one
  const anchorElement = document.getElementById('favorite-' + item.fullPath)
  document.dispatchEvent(new CustomEvent('dropdown-toggle', { detail: { exclude: anchorElement } }))
  favoriteMenuVisible.value = true
}

function showVolumeMenu(item: LinkItem) {
  selectedVolume.value = item
  const anchorElement = document.getElementById('volume-' + item.fullPath)
  document.dispatchEvent(new CustomEvent('dropdown-toggle', { detail: { exclude: anchorElement } }))
  aliasInput.value = item.title || ''
  volumeMenuVisible.value = true
}

const { mutate: removeFavoriteFolderMutation } = initMutation({
  document: removeFavoriteFolderGQL,
  options: {
    update: () => {
      emitter.emit('refetch_favorite_folders')
    },
  },
})

function removeFavoriteFolder(item: LinkItem) {
  removeFavoriteFolderMutation({
    rootPath: item.rootPath,
    relativePath: item.relativePath,
  }).then(() => {
    toast(t('removed'))
  }).catch((error) => {
    console.error('Error removing favorite folder:', error)
    toast(t('error'), 'error')
  })
}

interface LinkItem {
  id?: string
  rootPath: string
  relativePath: string
  fullPath: string
  type: string
  title: string
  isChecked: boolean
  isFavoriteFolder: boolean
  count?: string
  driveType?: string
  usedPercent?: number
  remote?: boolean
}

const { volumes, refetch: refetchVolumes } = useVolumes()

const links = computed(() => {
  // Helper to unify favorite display title
  const generateFavoriteDisplayTitle = (favoriteFolder: IFavoriteFolder): string => {
    return getFavoriteDisplayTitle(favoriteFolder, volumes.value, t)
  }

  // Compute current full path and the best-matching favorite folder path
  const currentRoot = (parent.value || '').replace(/\/+$/, '')
  const currentRel = (filter.relativePath || '').replace(/^\/+/, '')
  const currentFull = currentRel ? `${currentRoot}/${currentRel}` : currentRoot

  let bestFavoriteMatchFullPath = ''
  if (favoriteFolders.value && favoriteFolders.value.length > 0 && currentFull) {
    for (const f of favoriteFolders.value) {
      const favFull = getFavoriteFolderFullPath(f)
      if (favFull && (currentFull === favFull || currentFull.startsWith(favFull + '/'))) {
        if (favFull.length > bestFavoriteMatchFullPath.length) {
          bestFavoriteMatchFullPath = favFull
        }
      }
    }
  }

  const links: LinkItem[] = []

  links.push({
    rootPath: '',
    relativePath: '',
    fullPath: '',
    type: 'RECENTS',
    title: t('recents'),
    isChecked: recent.value,
    isFavoriteFolder: false
  })

  links.push({
    rootPath: '',
    relativePath: '',
    fullPath: '',
    type: 'TRASH',
    title: t('trash'),
    isChecked: trash.value,
    isFavoriteFolder: false,
  })

  volumes.value.forEach((v) => {
    const mp = String(v.mountPoint || '').trim()
    if (!mp) return
    const title = getStorageVolumeTitle(v, t)
    const totalBytes = v.totalBytes ?? 0
    const usedBytes = v.usedBytes ?? 0
    const count = totalBytes > 0 ? formatUsedTotalBytes(usedBytes, totalBytes) : ''
    links.push({
      id: v.id,
      rootPath: mp,
      relativePath: '',
      fullPath: mp,
      type: 'VOLUME',
      title,
      driveType: v.driveType,
      usedPercent: totalBytes > 0 ? (usedBytes / totalBytes) * 100 : 0,
      remote: !!v.remote,
      count,
      // Highlight volume only if no favorite folder matches the current location
      isChecked: !recent.value && !bestFavoriteMatchFullPath && mp === parent.value,
      isFavoriteFolder: false,
    })
  })

  // Favorite folders
  if (favoriteFolders.value && favoriteFolders.value.length > 0) {
    favoriteFolders.value.forEach((folder: IFavoriteFolder, index: number) => {
      const displayTitle = generateFavoriteDisplayTitle(folder)
      const full = getFavoriteFolderFullPath(folder)

      links.push({
        rootPath: folder.rootPath,
        relativePath: folder.relativePath,
        fullPath: full,
        type: 'FAVORITE',
        title: displayTitle,
        // Highlight the deepest matching favorite folder
        isChecked: !recent.value && bestFavoriteMatchFullPath === full,
        isFavoriteFolder: true,
      })
    })
  }

  return links
})

const quickLinks = computed(() => links.value.filter((it) => it.type === 'RECENTS' || it.type === 'TRASH'))
const volumeLinks = computed(() => links.value.filter((it) => it.type === 'VOLUME'))
const favoriteLinks = computed(() => links.value.filter((it) => it.type === 'FAVORITE'))

function percentClass(p?: number) {
  const v = Math.round(p || 0)
  if (v >= 85) return 'warn'
  return ''
}

function openLink(link: LinkItem) {
  if (link.type === 'RECENTS') {
    openRecent()
    return
  }

  if (link.type === 'TRASH') {
    openTrash()
    return
  }

  const fields: { name: string; op: string; value: string }[] = []
  // carry volume mount point in query
  fields.push({ name: 'root_path', op: '', value: link.rootPath })
  if (link.relativePath) {
    fields.push({ name: 'relative_path', op: '', value: link.relativePath })
  }
  if (mainStore.fileShowHidden) {
    fields.push({ name: 'show_hidden', op: '', value: 'true' })
  }
  const q = buildQuery(fields)
  replacePath(mainStore, `/files?q=${encodeBase64(q)}`)
}

function updateActive() {
  const route = router.currentRoute.value
  fetchCounts()
  if (route.path === '/files/recent') {
    recent.value = true
    trash.value = false
    return
  }

  if (route.path === '/files/trash') {
    recent.value = false
    trash.value = true
    parent.value = ''
    return
  }

  recent.value = false
  trash.value = false
  const q = decodeBase64(route.query.q?.toString() ?? '')
  parseQ(filter, q)
  parent.value = filter.rootPath
}

updateActive()

watch(
  () => router.currentRoute.value.fullPath,
  () => {
    updateActive()
  }
)

const refreshCounts = () => fetchCounts()
const refreshFavorites = () => refetchFavoriteFolders()

onMounted(() => {
  emitter.on('media_items_actioned', refreshCounts)
  emitter.on('file_deleted', refreshCounts)
  emitter.on('file_trashed', refreshCounts)
  emitter.on('file_restored', refreshCounts)
  emitter.on('file_renamed', refreshCounts)
  emitter.on('upload_task_done', refreshCounts)
  emitter.on('refetch_favorite_folders', refreshFavorites)
})

onUnmounted(() => {
  emitter.off('media_items_actioned', refreshCounts)
  emitter.off('file_deleted', refreshCounts)
  emitter.off('file_trashed', refreshCounts)
  emitter.off('file_restored', refreshCounts)
  emitter.off('file_renamed', refreshCounts)
  emitter.off('upload_task_done', refreshCounts)
  emitter.off('refetch_favorite_folders', refreshFavorites)
})

function openSetFavoriteAlias() {
  const item = selectedFavorite.value
  if (!item) return

  const current = favoriteFolders.value.find(
    (f) => f.rootPath === item.rootPath && f.relativePath === item.relativePath
  )
  const currentAlias = (current?.alias || '').trim()

  const mutationFactory = () =>
    initMutation({
      document: setFavoriteFolderAliasGQL,
      options: {
        update: () => {
          refetchFavoriteFolders()
        },
      },
    })

  openModal(EditValueModal, {
    title: t('name'),
    placeholder: item.title || '',
    value: currentAlias || '',
    mutation: mutationFactory,
    getVariables: (value: string) => ({
      rootPath: item.rootPath,
      relativePath: item.relativePath,
      alias: (value || '').trim(),
    }),
    done: () => {
      toast(t('saved'))
    },
  })
}

function openSetAlias() {
  const item = selectedVolume.value
  if (!item || !item.id) return
  const mutationFactory = () =>
    initMutation({
      document: setMountAliasGQL,
      options: {
        update: () => {
          refetchVolumes()
        },
      },
    })

  openModal(EditValueModal, {
    title: t('name'),
    placeholder: item.title || '',
    value: item.title || '',
    mutation: mutationFactory,
    getVariables: (value: string) => ({ id: item.id!, alias: (value || '').trim() }),
    done: () => {
      toast(t('saved'))
    },
  })
}
</script>
