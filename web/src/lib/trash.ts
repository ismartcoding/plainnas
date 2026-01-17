export function isInNasTrash(path: string | undefined | null): boolean {
  if (!path) return false

  // Backend uses disk-local `.nas-trash` and returns absolute paths using '/'.
  // Keep logic simple and Linux-centric.
  const p = path.replaceAll('\\', '/')
  return p.includes('/.nas-trash/') || p.endsWith('/.nas-trash')
}

// Trash entries are stored as f_<id>__<basename> or d_<id>__<basename>.
// This keeps the ID parseable from the physical path while still preserving the original
// name for UI. For display, we usually want to hide the prefix and show only <basename>.
export function getTrashDisplayName(name: string | undefined | null): string {
  if (!name) return ''
  if (!(name.startsWith('f_') || name.startsWith('d_'))) return name
  const idx = name.indexOf('__')
  if (idx < 0) return name
  const rest = name.substring(idx + 2)
  return rest || name
}
