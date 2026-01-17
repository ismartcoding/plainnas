export function noDataKey(loading: boolean): string {
  if (loading) {
    return 'loading'
  }

  return 'no_data'
}
