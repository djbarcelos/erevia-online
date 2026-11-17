/**
 * Telemetria de desempenho: lê o FPS 1×/s (fora do loop de render) e, a cada 60
 * amostras, manda a média e a média dos 10% piores segundos ao servidor. Aba em
 * segundo plano (FPS ~0) não conta.
 */
const SAMPLES = 60;

export function startPerfSampler(fps: () => number, send: (avg: number, low: number, device: string, seconds: number) => void): () => void {
  const device = deviceLabel();
  let samples: number[] = [];
  const timer = window.setInterval(() => {
    if (document.hidden) return;
    const f = fps();
    if (f > 0 && Number.isFinite(f)) samples.push(f);
    if (samples.length < SAMPLES) return;
    const sorted = [...samples].sort((a, b) => a - b);
    const worst = sorted.slice(0, Math.max(1, Math.round(sorted.length * 0.1)));
    const avg = samples.reduce((s, v) => s + v, 0) / samples.length;
    const low = worst.reduce((s, v) => s + v, 0) / worst.length;
    send(avg, low, device, samples.length);
    samples = [];
  }, 1000);
  return () => window.clearInterval(timer);
}

/** "mobile android", "desktop windows"… (curto, sem identificar a pessoa). */
function deviceLabel(): string {
  const ua = navigator.userAgent.toLowerCase();
  const os = /android/.test(ua) ? "android" : /iphone|ipad|ipod/.test(ua) ? "ios" : /windows/.test(ua) ? "windows" : /mac os/.test(ua) ? "mac" : /linux|cros/.test(ua) ? "linux" : "outro";
  const touch = matchMedia("(pointer: coarse)").matches;
  return `${touch ? "mobile" : "desktop"} ${os}`;
}
