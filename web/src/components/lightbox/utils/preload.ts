export interface CreateAroundPreloaderOptions<T> {
    getItems: () => T[]
    getUrl: (item: T) => string | null
    loop: () => boolean
    ahead?: number
    behind?: number
    maxUrls?: number
}

// A tiny in-memory URL preloader intended for lightbox navigation.
// It prefetches a small number of neighbor URLs and keeps a bounded cache
// to avoid unbounded memory growth.
export function createAroundPreloader<T>(opts: CreateAroundPreloaderOptions<T>) {
    const ahead = opts.ahead ?? 3
    const behind = opts.behind ?? 1
    const maxUrls = opts.maxUrls ?? 60

    const preloadedUrls = new Set<string>()
    const preloadQueue: string[] = []
    const inflight = new Map<string, HTMLImageElement>()

    function touch(url: string) {
        if (!preloadedUrls.has(url)) {
            preloadedUrls.add(url)
            preloadQueue.push(url)
        }

        while (preloadQueue.length > maxUrls) {
            const old = preloadQueue.shift()
            if (old) {
                preloadedUrls.delete(old)
            }
        }
    }

    function preloadUrl(url: string) {
        if (!url) return
        if (preloadedUrls.has(url)) return
        if (inflight.has(url)) return

        const img = new Image()
            ; (img as any).decoding = 'async'
        inflight.set(url, img)

        img.onload = () => {
            inflight.delete(url)
            touch(url)
        }
        img.onerror = () => {
            inflight.delete(url)
        }

        img.src = url
    }

    function clear() {
        for (const img of inflight.values()) {
            img.src = ''
        }
        inflight.clear()
        preloadedUrls.clear()
        preloadQueue.length = 0
    }

    function preloadAround(index: number) {
        const items = opts.getItems() ?? []
        const n = items.length
        if (n <= 1) return

        const loop = opts.loop()

        const preloadIndex = (i: number) => {
            const item = items[i]
            if (!item) return
            const url = opts.getUrl(item)
            if (url) preloadUrl(url)
        }

        for (let step = 1; step <= ahead; step++) {
            const i = loop ? (index + step) % n : index + step
            if (!loop && i >= n) break
            preloadIndex(i)
        }

        for (let step = 1; step <= behind; step++) {
            const i = loop ? (index - step + n) % n : index - step
            if (!loop && i < 0) break
            preloadIndex(i)
        }
    }

    return {
        clear,
        preloadAround,
        preloadUrl,
    }
}
