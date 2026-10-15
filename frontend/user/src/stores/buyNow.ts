import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { CartItem } from './cart'
import { readBuyNowSession, saveBuyNowSession, clearBuyNowSession } from '../utils/buyNowSession.ts'

export const useBuyNowStore = defineStore('buyNow', () => {
    const item = ref<CartItem | null>(readBuyNowSession())

    const hasItem = computed(() => item.value !== null)

    const setItem = (newItem: CartItem) => {
        item.value = { ...newItem }
        saveBuyNowSession(newItem)
    }

    const clear = () => {
        item.value = null
        clearBuyNowSession()
    }

    return {
        item,
        hasItem,
        setItem,
        clear,
    }
})
