/**
 * 将 Blob 作为文件下载触发到浏览器，用后即释。
 * 抽取自 Notes / Editor 中重复的 createObjectURL + a.click + revoke 样板。
 */
export function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
