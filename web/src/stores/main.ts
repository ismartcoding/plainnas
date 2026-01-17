import { defineStore } from 'pinia'

// data will be stored to local storage
export type MainState = {
  fileShowHidden: boolean
  quick: string
  quickContentWidth: number
  sidebarWidth: number
  sidebar2Width: number
  miniSidebar: boolean
  lightboxInfoVisible: boolean
  videosCardView: boolean
  imagesCardView: boolean
  fileSortBy: string
  imageSortBy: string
  videoSortBy: string
  audioSortBy: string
  bucketFilterCollapsed: Record<string, boolean>
  lastRoutes: Record<string, string>
  searchHistory: Record<string, string[]>
}

export const useMainStore = defineStore('main', {
  state: () =>
    ({
      fileShowHidden: false,
      quick: '',
      quickContentWidth: 400,
      sidebarWidth: 240,
      sidebar2Width: 360,
      miniSidebar: false,
      lightboxInfoVisible: false,
      videosCardView: false,
      imagesCardView: false,
      fileSortBy: 'NAME_ASC',
      imageSortBy: 'DATE_DESC',
      videoSortBy: 'DATE_DESC',
      audioSortBy: 'DATE_DESC',
      bucketFilterCollapsed: {},
      lastRoutes: {},
      searchHistory: {},
    }) as MainState,
})
