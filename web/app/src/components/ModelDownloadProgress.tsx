import type { DownloadProgress } from '../api/client';
import { useI18n } from '../i18n';

export function isActiveDownload(download?: DownloadProgress) {
  return download?.status === 'downloading' || download?.status === 'finalizing';
}

export function ModelDownloadProgress({ download }: { download: DownloadProgress }) {
  const { messages: text } = useI18n();
  const percent = Number.isFinite(download.percent) ? Math.max(0, Math.min(100, download.percent)) : 0;
  const status = text.modelDownload[download.status] ?? text.common.loading;
  const speed = download.status === 'downloading' ? Math.max(0, download.speed) / (1024 * 1024) : 0;
  const retained = (download.status === 'failed' || download.status === 'cancelled') && download.bytes_done > 0;
  return <div className={`download-state ${download.status}`}>
    <div role="progressbar" aria-label={status} aria-valuenow={percent} aria-valuemin={0} aria-valuemax={100}><span style={{ width: `${percent}%` }} /></div>
    <small role="status">{status} · {percent.toFixed(1)}%{speed > 0 && ` · ${speed.toFixed(1)} MB/s`}</small>
    {download.error && <p>{download.error}</p>}
    {retained && <p>{text.modelDownload.retained}</p>}
  </div>;
}
