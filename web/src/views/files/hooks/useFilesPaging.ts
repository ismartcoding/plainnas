import { computed, nextTick, ref, type Ref } from 'vue'

import { filesCountGQL, filesGQL, initLazyQuery } from '@/lib/api/query'
import { enrichFile, type IFile } from '@/lib/file'
import toast from '@/components/toaster'

import type sjcl from 'sjcl'

export type UseFilesPagingOptions = {
    q: Ref<string>
    sortBy: Ref<string>
    urlTokenKey: Ref<sjcl.BitArray | null>

    // Optional: initial page index (1-based)
    initialPage?: number
    limit?: number

    // Prevent loading more while other operations are running.
    isBlocked?: () => boolean

    // Filter out items from the response.
    shouldHideItem?: (item: any) => boolean

    // Translate/toast errors.
    t?: (key: string) => string
}

export function useFilesPaging(opts: UseFilesPagingOptions) {
    const limit = opts.limit ?? 1000

    const items = ref<IFile[]>([])
    const total = ref(0)

    const page = ref(Math.max(1, opts.initialPage ?? 1))
    const hasMore = ref(true)
    const loadingMore = ref(false)

    const firstInit = ref(true)

    const dirTotal = ref<number | null>(null)
    const displayTotal = computed(() => (dirTotal.value === null ? total.value : dirTotal.value))

    const prefetchPage = ref<number | null>(null)
    const prefetchBuffer = ref<IFile[]>([])
    const prefetching = ref(false)
    const pendingLoadMore = ref(false)

    const listWrapperRef = ref<HTMLElement | null>(null)
    let boundScrollerEl: HTMLElement | null = null

    function getScrollerEl(): HTMLElement | null {
        // VirtualList is rendered with class="scroller main-list".
        return (listWrapperRef.value?.querySelector('.main-list') as HTMLElement | null) ?? null
    }

    function isNearBottom(el: HTMLElement, thresholdPx: number): boolean {
        const remaining = el.scrollHeight - el.scrollTop - el.clientHeight
        return remaining <= thresholdPx
    }

    function onScrollFallback() {
        const el = boundScrollerEl
        if (!el) return

        // Start prefetch a bit earlier than the "load more" threshold.
        if (isNearBottom(el, 1600)) {
            ensurePrefetch()
        }
        if (isNearBottom(el, 800)) {
            loadMore()
        }
    }

    function bindScrollFallback() {
        const el = getScrollerEl()
        if (!el || el === boundScrollerEl) return

        if (boundScrollerEl) {
            boundScrollerEl.removeEventListener('scroll', onScrollFallback)
        }
        boundScrollerEl = el
        el.addEventListener('scroll', onScrollFallback, { passive: true })
    }

    function unbindScrollFallback() {
        if (!boundScrollerEl) return
        boundScrollerEl.removeEventListener('scroll', onScrollFallback)
        boundScrollerEl = null
    }

    async function maybeAutoLoadMore() {
        await nextTick()
        bindScrollFallback()
        const el = boundScrollerEl
        if (!el) return

        if (isNearBottom(el, 800)) {
            loadMore()
        }
    }

    function appendFiles(list: IFile[]) {
        if (list.length === 0) return

        const existing = new Set(items.value.map((it) => it.id))
        for (const it of list) {
            if (!existing.has(it.id)) {
                items.value.push(it)
                existing.add(it.id)
            }
        }
    }

    function normalizeFilesResponse(data: any): IFile[] {
        const list: IFile[] = []
        for (const item of data.files ?? []) {
            if (opts.shouldHideItem?.(item)) continue
            list.push(enrichFile(item, opts.urlTokenKey.value))
        }
        return list
    }

    const { loading, fetch } = initLazyQuery({
        handle: async (data: any, error: string) => {
            firstInit.value = false
            loadingMore.value = false

            if (error) {
                toast(opts.t ? opts.t(error) : error, 'error')
                return
            }

            const list = normalizeFilesResponse(data)
            hasMore.value = list.length === limit

            if (page.value <= 1) {
                items.value = list
            } else {
                appendFiles(list)
            }

            total.value = items.value.length

            // Kick off a one-page-ahead prefetch after each successful load.
            ensurePrefetch()

            // Some virtual-scroll implementations only emit "tobottom" on scroll events;
            // if we're already near the bottom after rendering/appending, trigger again.
            maybeAutoLoadMore()
        },
        document: filesGQL,
        variables: () => ({
            offset: (page.value - 1) * limit,
            limit,
            query: opts.q.value,
            sortBy: opts.sortBy.value,
        }),
        options: {
            fetchPolicy: 'cache-and-network',
        },
    })

    const { fetch: fetchCount } = initLazyQuery({
        handle: async (data: any, error: string) => {
            if (error) return
            dirTotal.value = Number(data?.filesCount ?? 0)
        },
        document: filesCountGQL,
        variables: () => ({
            query: opts.q.value,
        }),
        options: {
            fetchPolicy: 'cache-and-network',
        },
    })

    const { fetch: prefetchFetch } = initLazyQuery({
        handle: async (data: any, error: string) => {
            prefetching.value = false
            if (error) return

            prefetchBuffer.value = normalizeFilesResponse(data)

            // If the user already hit the bottom while we were prefetching, consume now.
            if (pendingLoadMore.value && prefetchPage.value === page.value + 1 && prefetchBuffer.value.length > 0) {
                pendingLoadMore.value = false
                loadingMore.value = false
                loadMore()
            }

            // If the user is already at the bottom, consume buffered page automatically.
            maybeAutoLoadMore()
        },
        document: filesGQL,
        variables: () => ({
            offset: ((prefetchPage.value ?? 1) - 1) * limit,
            limit,
            query: opts.q.value,
            sortBy: opts.sortBy.value,
        }),
        options: {
            fetchPolicy: 'cache-and-network',
        },
    })

    function resetPaging() {
        page.value = Math.max(1, opts.initialPage ?? 1)
        hasMore.value = true
        loadingMore.value = false
        prefetchPage.value = null
        prefetchBuffer.value = []
        prefetching.value = false
        pendingLoadMore.value = false

        items.value = []
        total.value = 0
        dirTotal.value = null
    }

    function ensurePrefetch() {
        if (!hasMore.value) return
        if (prefetching.value) return
        if (loading.value) return
        if (loadingMore.value) return
        if (prefetchBuffer.value.length > 0) return

        prefetchPage.value = page.value + 1
        prefetching.value = true
        prefetchFetch()
    }

    function loadMore() {
        if (!hasMore.value) return
        if (loading.value) return
        if (loadingMore.value) return
        if (opts.isBlocked?.()) return

        // If a prefetch for the next page is already in-flight, don't issue a duplicate request.
        if (prefetching.value && prefetchPage.value === page.value + 1) {
            pendingLoadMore.value = true
            loadingMore.value = true
            return
        }

        loadingMore.value = true

        // If we already prefetched the next page, use it immediately.
        if (prefetchPage.value === page.value + 1 && prefetchBuffer.value.length > 0) {
            page.value = page.value + 1
            appendFiles(prefetchBuffer.value)
            total.value = items.value.length
            hasMore.value = prefetchBuffer.value.length === limit
            prefetchBuffer.value = []
            loadingMore.value = false
            ensurePrefetch()
            maybeAutoLoadMore()
            return
        }

        page.value = page.value + 1
        fetch()
    }

    async function activate() {
        await nextTick()
        bindScrollFallback()
    }

    return {
        // data
        items,
        total,
        dirTotal,
        displayTotal,

        page,
        limit,
        hasMore,

        loading,
        firstInit,
        loadingMore,
        prefetching,

        // scroll bind target
        listWrapperRef,

        // actions
        fetch,
        fetchCount,
        resetPaging,
        ensurePrefetch,
        loadMore,
        bindScrollFallback,
        unbindScrollFallback,
        activate,
    }
}
