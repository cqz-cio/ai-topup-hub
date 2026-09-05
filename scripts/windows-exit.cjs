// Vinext calls process.exit immediately after closing its prerender server.
// On Windows, allow pending libuv close callbacks to drain first.
if (process.platform === 'win32') {
  const exit = process.exit.bind(process);
  process.exit = (code) => { setTimeout(() => exit(code), 1000); };
}
