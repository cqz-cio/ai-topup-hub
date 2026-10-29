import { useEffect, useRef } from 'react';
import { mountBikiniMotion } from './scene/bikiniMotion.js';

export function SceneMotion({ onStatus }) {
  const canvas = useRef(null);
  useEffect(() => {
    onStatus('loading');
    return mountBikiniMotion(canvas.current, onStatus);
  }, [onStatus]);
  return <canvas ref={canvas} className="scene-motion" aria-hidden="true" data-ready="false" />;
}
