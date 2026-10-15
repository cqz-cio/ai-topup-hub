// Values come from useCheckout's existing integer-cent preview and fee calculation.
// Wallet payments are part of the total; only the unpaid online portion has a fee.
export function checkoutPaymentSummary(subtotal: number, wallet: number, onlineBeforeFee: number, fee: number) {
  return { subtotal, fee, total: subtotal + fee, wallet, online: onlineBeforeFee + fee }
}
