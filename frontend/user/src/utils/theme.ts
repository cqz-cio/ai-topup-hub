// Keep the existing daytime palette regardless of system or saved preferences.
export function initializeLightTheme(): void {
    document.documentElement.classList.remove('dark')
    document.documentElement.style.colorScheme = 'light'
    try {
        localStorage.setItem('dujiao_theme', 'light')
    } catch {
        // Storage can be blocked without preventing the light theme.
    }
}
