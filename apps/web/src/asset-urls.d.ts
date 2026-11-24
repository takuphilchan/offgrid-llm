// Vite emits this worker as a local file; AudioWorklet requires its URL.
declare module '*?url&no-inline' {
  const url: string;
  export default url;
}
