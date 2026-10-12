# USDT payment option design QA

Final result: passed

## Scope and evidence

- Approved visual: `C:/Users/admin/.codex/generated_images/01a12dfc-f389-7e01-8497-b57c04c63499/exec-7f8e66db-729f-479d-b0df-36b73188e630.png` (approved in this task on 2026-10-12).
- Implementation: checkout and the payment page's shared `PaymentChannelSelector` / `PaymentChannelOption` components. The existing vault-specific checkout is outside this change.
- Desktop screenshot: `work/codex-runtime/payment-ui-qa/desktop-plasma.jpg`, 1920 × 1080 CSS viewport.
- Mobile screenshots: `work/codex-runtime/payment-ui-qa/mobile-plasma.jpg` and `mobile-dark.jpg`, 390 × 844 CSS viewport.
- Payment selector fixture: `work/codex-runtime/payment-ui-qa/payment-selector-fees-disabled.jpg`.
- Comparison state: Plasma selected, BEP20 unselected, guest details empty, submit disabled. The original site's validation alert and complete calculation notice remain; these were simplified in the mock and are intentionally preserved.
- The generated source is an enlarged sidebar crop, not a browser viewport. Compare its card proportions against the 370.66 CSS-pixel desktop sidebar; do not compare outer image whitespace. Its payment rows normalize to roughly 65–70 CSS pixels, against the implementation's measured 70 pixels. Browser screenshots use native density; tool display may scale images to fit.
- Actual image sizes: source 1058 × 1487 px; desktop capture 1910 × 1074 px from 1920 × 1080 CSS; mobile capture 380 × 822 px from 390 × 844 CSS. The browser capture service slightly scales these outputs (approximately 0.995 and 0.974); layout measurements above use DOM CSS pixels, and visual comparison accounts for that scale.

## Visual comparison

Source, desktop implementation and focused mobile payment region were opened together in one comparison input. Both real payment cards use a single column, equal padding, the same USDT asset, left-aligned two-line labels and a right-hand radio control. Desktop rows measure 320.66 × 70 CSS pixels; mobile rows measure 294 × 70. The 10-pixel gap and 10-pixel corners follow the approved layout.

- **Typography:** existing Inter / PingFang SC / Microsoft YaHei stack; 15px semibold titles and 12px secondary network labels, no wrapping or clipping at the checked widths.
- **Spacing:** matching icon/text baselines, 36px icons, 14px internal gaps, 70px minimum row height. Rows expand when fees or limits require extra lines. Existing page layout and submit spacing retained.
- **Colors:** existing deep green accent and soft selected background; dark theme follows existing tokens. Selected state is communicated with a native checked radio as well as color.
- **Assets:** real vector USDT mark from Cryptocurrency Icons; only its circle color is adapted. No raster placeholders or recreated currency glyphs. Source and license documented beside the asset.
- **Copy:** BEP20 / BNB Smart Chain and Plasma / Plasma 网络 match the approved meaning. Plasma subtitle is translated in all three supported locales. Existing fee, amount-limit and required-field messages remain.
- **Expected native-control difference:** radio artwork follows the browser's native rendering rather than the mock's decorative circle; keyboard and assistive-technology behavior is preserved.

## Interactions and comparison history

1. Mouse selection and ArrowUp / ArrowDown switched between the two channels; exactly one radio remained checked. Empty customer details kept checkout submission disabled.
2. Initial mobile keyboard check showed duplicate focus outlines. Removed the inner native outline while retaining a visible focus outline around the entire card. Re-captured and compared the corrected mobile view.
3. Initial dark-mode check showed a light native unselected radio. Applied `color-scheme: dark` to the radio in dark theme. Re-captured dark mode; selected and unselected controls are now distinct and legible.
4. Mounted the actual payment-page selector in a temporary isolated UI fixture. Selection emitted channel ID 2, the percentage + fixed fee remained visible, and an amount-limited channel remained disabled with its reason. Fixture bootstrap initially omitted Vite's runtime environment; after using the actual project module and Vite client, it rendered successfully. The temporary source entry was removed after verification.
5. Checked browser diagnostics: no new application errors in the final checkout view; earlier HMR and fixture-bootstrap diagnostics were confined to development setup and resolved.

No actionable P0/P1/P2 findings remain. No purchases or payments were submitted. Local fixture channels were display-only and order/payment creation was blocked; no production configuration or local database was changed for visual testing.

## Validation

- Existing user frontend tests: 94 passed.
- Existing admin frontend tests: 16 passed.
- User frontend TypeScript check passed after preserving the still-used vault-specific component.
- Desktop, mobile, light/dark, mouse/keyboard, fee display and disabled-state checks completed.

Final result: passed
