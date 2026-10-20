/** Visible animation artwork must not time out just because the connection is slow. */
export function loadSceneArtwork(src, pendingLoads) {
    return new Promise((resolve, reject) => {
        const img = new Image();
        const finish = (error) => {
            pendingLoads.delete(cancel);
            img.onload = null;
            img.onerror = null;
            if (error) {
                img.src = '';
                reject(error);
            } else resolve(img);
        };
        const cancel = () => finish(new Error('Scene disposed'));
        pendingLoads.add(cancel);
        img.decoding = 'async';
        img.fetchPriority = 'high';
        img.onload = () => finish();
        img.onerror = () => finish(new Error('Scene artwork unavailable'));
        img.src = src;
    });
}
