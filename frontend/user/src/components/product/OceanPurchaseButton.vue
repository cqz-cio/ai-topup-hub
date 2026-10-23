<script setup lang="ts">
import { ShoppingCart } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

defineProps<{ action: 'buy' | 'cart'; compact?: boolean }>()
</script>

<template>
  <Button
    :variant="action === 'cart' ? (compact ? 'secondary' : 'outline') : 'default'"
    class="ocean-purchase-button font-bold"
    :class="[compact ? 'h-10' : 'h-12', `ocean-purchase-button--${action}`, { 'ocean-purchase-button--compact': compact }]"
  >
    <span class="ocean-purchase-button__label">
      <ShoppingCart v-if="action === 'cart'" class="ocean-purchase-button__cart" aria-hidden="true" />
      <slot />
    </span>
  </Button>
</template>

<style>
.ocean-purchase-button__label { display: inline-flex; align-items: center; justify-content: center; gap: .5rem; }
.ocean-purchase-button__cart { display: none; }

.brand-product-scene .ocean-purchase-button {
  position: relative;
  isolation: isolate;
  min-width: 0;
  height: 3.5rem;
  padding: .5rem 2rem;
  border: 0;
  border-radius: .85rem;
  background: #d6f0f4;
  color: #103f42;
  font-size: 1rem;
  box-shadow: 0 3px 7px rgb(22 93 94 / 14%);
  transition: filter 150ms ease, box-shadow 150ms ease;
}

.brand-product-scene .ocean-purchase-button::before {
  content: '';
  position: absolute;
  z-index: -1;
  inset: 0;
  pointer-events: none;
  background: var(--ocean-button-art) var(--ocean-art-position, center) / var(--ocean-art-size, 100% 100%) no-repeat;
}

.brand-product-scene .ocean-purchase-button--buy {
  --ocean-button-art: url('/brand/ocean-buttons/buy-now-20261022.png');
  --ocean-art-size: 101.02% 157.39%;
  --ocean-art-position: 45.45% 53.03%;
}
.brand-product-scene .ocean-purchase-button--cart {
  --ocean-button-art: url('/brand/ocean-buttons/add-cart-20261022.png');
  /* Frame the button body within the generated asset's transparent canvas. */
  --ocean-art-size: 101.02% 157.39%;
  --ocean-art-position: 45.45% 53.03%;
}
.brand-product-scene .ocean-purchase-button__label { position: relative; z-index: 1; }
.brand-product-scene .ocean-purchase-button__cart { display: block; width: 1.15rem; height: 1.15rem; }
.brand-product-scene .ocean-purchase-button:hover:not(:disabled) {
  filter: brightness(1.035);
  box-shadow: 0 4px 10px rgb(22 93 94 / 23%);
}
.brand-product-scene .ocean-purchase-button:focus-visible { outline: 2px solid #156579; outline-offset: 3px; }
.brand-product-scene .ocean-purchase-button:active:not(:disabled) { filter: brightness(.96); }
.brand-product-scene .ocean-purchase-button:disabled { opacity: .5; filter: saturate(.4); }
.brand-product-scene .ocean-purchase-button--compact { height: 2.75rem; padding: .5rem .75rem; font-size: .8125rem; }
.brand-product-scene .ocean-purchase-button--compact .ocean-purchase-button__cart { display: none; }
@media (prefers-reduced-motion: reduce) {
  .brand-product-scene .ocean-purchase-button { transition: none; }
}
</style>
