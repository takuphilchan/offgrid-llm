export function formatBytes(bytes: number, language: string) {
  if (!Number.isFinite(bytes) || bytes <= 0) return `${new Intl.NumberFormat(language).format(0)} B`;
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const digits=index>1?1:0;
  return `${new Intl.NumberFormat(language,{minimumFractionDigits:digits,maximumFractionDigits:digits}).format(bytes / 1024 ** index)} ${units[index]}`;
}
