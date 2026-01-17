export async function copyToClipboard(text: string): Promise<boolean> {
  const v = String(text ?? '')
  if (!v) return false

  // Try modern clipboard API first
  if (navigator.clipboard && navigator.clipboard.writeText) {
    try {
      await navigator.clipboard.writeText(v)
      return true
    } catch {
      // fall through to legacy
    }
  }

  // Fallback for older browsers or non-HTTPS environments
  try {
    const textArea = document.createElement('textarea')
    textArea.value = v
    textArea.style.position = 'fixed'
    textArea.style.left = '-999999px'
    textArea.style.top = '-999999px'
    document.body.appendChild(textArea)
    textArea.focus()
    textArea.select()

    const ok = document.execCommand('copy')
    document.body.removeChild(textArea)
    return ok
  } catch {
    return false
  }
}
