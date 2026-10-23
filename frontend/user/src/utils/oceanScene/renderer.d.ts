import type { BrandPageScene } from '../brandPageScene'
export type OceanScene = BrandPageScene | 'detail'
export interface OceanSceneController { dispose(): void }
export function mountOceanScene(
  canvas: HTMLCanvasElement, bubbles: HTMLCanvasElement, scene: OceanScene,
  callbacks: { onReady(): void; onFallback(): void },
): OceanSceneController
