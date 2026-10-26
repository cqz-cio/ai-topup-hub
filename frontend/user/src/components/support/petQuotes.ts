export type PetQuote = { id: string; text: string; label: string }
type QuoteEntry = { id: string; classic?: boolean; text: [string, string, string] }

// Two brief classic lines (Chinese translations), plus original Pi chatter.
// Sources: nick.com/info-page/8hz5p5/patrick-star and
// youtube.com/watch?v=IDjozwcjR20 (SpongeBob Official, Band Geeks).
// Original entries are not presented as dialogue from the television series.
const quotes: QuoteEntry[] = [
  { id: 'patrick', classic: true, text: ['不是，我是派大星！', '不是，我是派大星！', 'No, this is Patrick!'] },
  { id: 'mayonnaise', classic: true, text: ['蛋黄酱是一种乐器吗？', '蛋黃醬是一種樂器嗎？', 'Is mayonnaise an instrument?'] },
  { id: 'daydream', text: ['今天的安排：发呆，然后再发一会儿呆。', '今天的安排：發呆，然後再發一會兒呆。', 'Today’s plan: daydream. Then daydream a little more.'] },
  { id: 'loading', text: ['等一下，我的脑袋还在慢慢开机。', '等一下，我的腦袋還在慢慢開機。', 'One moment. My brain is still waking up.'] },
  { id: 'recharge', text: ['我不是在偷懒，我在给快乐充电。', '我不是在偷懶，我在給快樂充電。', 'I’m recharging my happiness. It takes a while.'] },
  { id: 'lunch', text: ['肚子里装的不是秘密，是午饭。', '肚子裡裝的不是秘密，是午飯。', 'My belly holds lunch. The secrets are somewhere else.'] },
  { id: 'chain', text: ['金链子可以晃，快乐不能掉。', '金鏈子可以晃，快樂不能掉。', 'The chain can swing. The good mood stays.'] },
  { id: 'stretch', text: ['先伸个懒腰，天大的事等三秒。', '先伸個懶腰，天大的事等三秒。', 'First, a stretch. The world can wait three seconds.'] },
  { id: 'company', text: ['一起发呆，也算一起做了一件大事。', '一起發呆，也算一起做了一件大事。', 'Daydreaming together counts as doing something together.'] },
  { id: 'rock', text: ['要是找不到我，我大概在石头底下补觉。', '要是找不到我，我大概在石頭底下補覺。', 'If I go missing, check under the rock. Nap time.'] },
]

export function pickPetQuote(locale: string, previousId = '', random = Math.random()): PetQuote {
  const pool = quotes.filter(quote => quote.id !== previousId)
  const value = Number.isFinite(random) ? Math.max(0, Math.min(1, random)) : 0
  const entry = pool[Math.min(pool.length - 1, Math.floor(value * pool.length))]!
  const language = locale === 'en-US' ? 2 : locale === 'zh-TW' ? 1 : 0
  return {
    id: entry.id, text: entry.text[language],
    label: entry.classic
      ? ['派大星 · 经典短句译写', '派大星 · 經典短句譯寫', 'Patrick · classic line'][language]!
      : ['派总 · 原创碎碎念', '派總 · 原創碎碎念', 'Pi · original chatter'][language]!,
  }
}

export type QuoteActivity = { hidden: boolean; focused: boolean; resting: boolean; chatting: boolean; busy: boolean; dragging: boolean; menu: boolean; preview: boolean; occupied: boolean }
export function canOfferPetQuote(state: QuoteActivity): boolean {
  return state.focused && !state.hidden && !state.resting && !state.chatting && !state.busy && !state.dragging && !state.menu && !state.preview && !state.occupied
}
export function petQuoteDelay(random = Math.random()): number {
  return 45000 + Math.max(0, Math.min(1, Number.isFinite(random) ? random : 0)) * 35000
}
