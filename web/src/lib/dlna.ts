import { openModal } from '@/components/modal'
import { DataType } from '@/lib/data'
import DlnaCastModal from '@/components/dlna/DlnaCastModal.vue'

export interface DlnaCastPayload {
  url: string
  title: string
  mime: string
  type: DataType
}

export function guessDlnaMimeByName(name: string, type: DataType): string {
  const lower = (name || '').toLowerCase()
  const ext = lower.includes('.') ? lower.substring(lower.lastIndexOf('.') + 1) : ''

  if (type === DataType.VIDEO) {
    if (ext === 'mp4' || ext === 'm4v') return 'video/mp4'
    if (ext === 'webm') return 'video/webm'
    if (ext === 'mkv') return 'video/x-matroska'
    if (ext === 'mov') return 'video/quicktime'
    if (ext === 'avi') return 'video/x-msvideo'
    if (ext === '3gp' || ext === '3gpp') return 'video/3gpp'
    return 'video/mp4'
  }

  if (type === DataType.AUDIO) {
    if (ext === 'mp3') return 'audio/mpeg'
    if (ext === 'm4a') return 'audio/mp4'
    if (ext === 'aac') return 'audio/aac'
    if (ext === 'flac') return 'audio/flac'
    if (ext === 'wav') return 'audio/wav'
    if (ext === 'ogg') return 'audio/ogg'
    if (ext === 'opus') return 'audio/opus'
    return 'audio/mpeg'
  }

  if (type === DataType.IMAGE) {
    if (ext === 'jpg' || ext === 'jpeg') return 'image/jpeg'
    if (ext === 'png') return 'image/png'
    if (ext === 'webp') return 'image/webp'
    if (ext === 'gif') return 'image/gif'
    if (ext === 'bmp') return 'image/bmp'
    if (ext === 'svg') return 'image/svg+xml'
    if (ext === 'tif' || ext === 'tiff') return 'image/tiff'
    return 'image/jpeg'
  }

  return 'application/octet-stream'
}

export function openDlnaCastModal(payload: DlnaCastPayload) {
  if (!payload.url) return
  openModal(DlnaCastModal, payload, { backgroundClose: true })
}
