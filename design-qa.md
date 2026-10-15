# Payment page — selected option 1

final result: passed

Source visual truth: `C:/Users/CHENZIQIAO/.codex/generated_images/01a13fa7-82bc-7060-b397-39400c050a58/exec-122567e3-2d6d-43c4-92ad-5d0092a4a124.png` (the first displayed option).

Implementation: clean Git archive of `3f9c00349214520784e548ece780163a1b5a6a9e`, built with Vue type checking and Vite. Browser-rendered evidence is in the private outer-workspace directory `deployment/local/payment-redesign-release/`.

## Comparison evidence

- Reference and `final-desktop.png` were opened together in the same comparison input at original resolution. Both are 1487 × 1058 pixels; implementation viewport is 1487 × 1058 CSS pixels at DPR 1. No density scaling or device frame was used.
- State: light theme, classic storefront, pending BEP20 payment, amount `12.345678 USDT`, order total `82.10 CNY`, deliberately non-payable demonstration QR/address. Countdown and expiry are dynamic.
- The full-resolution pair makes the amount, network, address, copy controls and guide text readable; separate cropped comparison was unnecessary. The real existing header/footer are preserved rather than reproducing the concept's abbreviated navigation/footer.
- Responsive evidence: `final-classic-1920.png`, `final-classic-768.png`, `final-classic-390.png`, `final-classic-320.png` and the corresponding four `final-vault-*` images. Measurements are in `final-checks.json`.

## Findings and fixes

1. **P2, mobile first-screen QR visibility, fixed.** Initial `mobile-v1.png` placed the QR bottom at y=802 on a 390 × 844 viewport, behind the fixed bottom navigation. Reduced heading spacing and used a 192 px mobile QR. Final QR bottom is y=742; both tutorials and the complete QR are visible above the navigation at 390 px. On a 320 × 740 screen the QR requires scrolling, with no clipping or horizontal overflow.
2. **P2, desktop detail density, fixed.** Initial `desktop-v1.png` repeated fee-help and order details across additional rows. Moved the full fee explanation into the visible warning's disclosure and aligned expiry with the amount-breakdown disclosure. `desktop-v2.png` and the final clean-build comparison retain the flat two-column composition.
3. **Expected production differences.** The amount uses the existing authoritative currency formatter (`82.10 CNY` rather than a decorative yuan symbol). Real full order numbers, expiry, discounts, wallet deduction and fee breakdown remain available. These require slightly more metadata height than the concept. Vault uses its established theme tokens. No P0/P1/P2 issues remain.

## Fidelity surfaces

- **Typography:** existing Inter / PingFang SC / Microsoft YaHei stack; 52 px desktop heading, 54 px amount, 27 px QR heading, compact readable labels. Amount digits retain all precision and use tabular numerals. English, Simplified and Traditional Chinese were inspected, including 320 px English wrapping.
- **Spacing/layout:** 1224 px content region, 76 px tutorial band, approximately 490/656 px desktop columns separated by a hairline and generous gutter. Mobile reorders to tutorials → amount/network → QR → address/actions. No horizontal overflow at 1920, 1487, 768, 390 or 320 px.
- **Color/tokens:** existing warm page background, dark green accent, white QR/address controls, amber fee warning, and existing dark-theme tokens. Focus indicators and contrast remain visible in dark English capture `dark-english-320.png`.
- **Image quality:** supplied brand logo reused; QR rendered from real payment data by the existing QR renderer, never a generated picture. Lucide icons match the reference's line icons. No custom decorative imagery or placeholder assets were added to production.
- **Copy/content:** both tutorials remain prominent and open in new tabs. Exact receipt amount and separately paid withdrawal fees remain visible; expanded help preserves the full prior warning. Payment amounts, network and address are sourced from the existing composable. No financial claims or new payment logic were introduced.

## Functional evidence

- Working source passed 138 frontend tests and a typechecked production build. The clean archive passed its own typechecked production build; utility CSS and actual rendered layouts were verified.
- Actual clipboard checks returned `12.345678` exactly and the complete displayed address, including a long-address fixture. Copy feedback, keyboard focus, fee/amount disclosures, refresh, payment-method selection and reload were checked.
- Both tutorial links opened their correct articles in new tabs and retained the payment tab. Daily local `5173` payment entry and published guide article were inspected against the actual local backend.
- Classic and Vault were checked for BEP20, TRON, Plasma/USDT0 and WeChat. WeChat fixture displayed order `82.10 CNY`, fee `1.32 CNY`, payable `83.42 CNY`, without U tutorials. Two-decimal U amounts, wallet deductions, QR fallback and automatic redirect fixtures were checked.
- Expired and canceled orders do not show a payable QR. The isolated fixture server rejects all POSTs, so creation/recreation of payments was deliberately not exercised; the existing payment composable is unchanged.
- Final clean-build browser console: no errors. No live order, transfer, purchase, paid-state mutation or backend restart was used for UI validation.

## Implementation checklist

- [x] Reuse one presentation component in both payment templates.
- [x] Preserve existing payment state, polling, expiry and copy operations.
- [x] Keep tutorials above the QR and in the first screen.
- [x] Compare final clean-build screenshots with the selected source.
- [x] Verify responsive layouts, non-U payment and important alternate states.

Follow-up polish: minor font rasterization and existing shared navigation/footer differences are accepted; no blocking visual work remains.
