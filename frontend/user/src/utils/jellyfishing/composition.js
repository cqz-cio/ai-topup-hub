/** Approved portrait composition. Artwork, animation and backdrop keep their original rendering. */
export function actorCamera(width, height) {
    if (width >= 768 || width > height) {
        const scale = Math.max(width / 1672, height / 941);
        return { scale, ox: (width - 1672 * scale) * (width < 768 ? .75 : .5), oy: (height - 941 * scale) / 2 };
    }
    const scale = (width - 28) / 800;
    return { scale, ox: (width - 800 * scale) / 2 - 800 * scale, oy: height * .75 - 850 * scale };
}
