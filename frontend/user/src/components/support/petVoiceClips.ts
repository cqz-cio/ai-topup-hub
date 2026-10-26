// Short Mandarin animation excerpts. Provenance and trim points: audio/SOURCES.md.
export const petVoiceClips = [
  { id: 'telephone', text: ['不是，我是派大星！', '不是，我是派大星！', 'No, this is Patrick!'],
    src: new URL('./audio/patrick-telephone-zh.mp3', import.meta.url).href,
    source: 'https://www.bilibili.com/video/BV1wp4y1v7H4/' },
  { id: 'where', text: ['这是什么地方啊！', '這是什麼地方啊！', 'What is this place?!'],
    src: new URL('./audio/patrick-where-zh.mp3', import.meta.url).href,
    source: 'https://www.bilibili.com/video/BV1t4411S7fo/' },
] as const
