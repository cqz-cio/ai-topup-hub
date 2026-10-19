export interface JellyfishingScene {
  dispose(): void
}

export function mountJellyfishing(
  canvas: HTMLCanvasElement,
  callbacks: { onReady(): void; onFallback(): void },
): JellyfishingScene
