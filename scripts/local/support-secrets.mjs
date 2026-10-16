import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
class SetupError extends Error {}

export function buildConfig(template, key) {
  if (typeof key !== 'string' || !/^sk-[A-Za-z0-9_-]{20,128}$/.test(key)) throw new SetupError('Missing or invalid API key. No key value was printed or saved.')
  return { ...template, api_key: key }
}

function protectDirectory(directory) {
  fs.mkdirSync(directory, { recursive: true, mode: 0o700 })
  if (process.platform !== 'win32') { fs.chmodSync(directory, 0o700); return }
  // Only the nonsecret target path crosses this child-process boundary.
  const encodedPath = Buffer.from(directory, 'utf8').toString('base64')
  const script = `$ErrorActionPreference='Stop'
$target=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('${encodedPath}'))
$acl=[System.Security.AccessControl.DirectorySecurity]::new()
$acl.SetAccessRuleProtection($true,$false)
$sid=[System.Security.Principal.WindowsIdentity]::GetCurrent().User
foreach($identity in @($sid,[System.Security.Principal.SecurityIdentifier]::new('S-1-5-18'))){
 $acl.AddAccessRule([System.Security.AccessControl.FileSystemAccessRule]::new($identity,'FullControl','ContainerInherit,ObjectInherit','None','Allow'))
}
Set-Acl -LiteralPath $target -AclObject $acl
$verified=Get-Acl -LiteralPath $target
$allowed=@($sid.Value,'S-1-5-18')
if(-not $verified.AreAccessRulesProtected -or @($verified.Access).Count -ne 2){throw 'Private ACL verification failed'}
foreach($rule in $verified.Access){
 if($rule.AccessControlType -ne 'Allow' -or $allowed -notcontains $rule.IdentityReference.Translate([System.Security.Principal.SecurityIdentifier]).Value){throw 'Unexpected ACL principal'}
}`
  try {
    // Windows PowerShell must resolve its own modules, even when Node was
    // launched from PowerShell 7 with an incompatible inherited PSModulePath.
    const environment = { ...process.env }
    for (const name of Object.keys(environment)) if (name.toLowerCase() === 'psmodulepath') delete environment[name]
    execFileSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], {
      env: environment, windowsHide: true, timeout: 10000, stdio: ['ignore', 'pipe', 'pipe'],
    })
  } catch { throw new SetupError('Unable to restrict private directory permissions. No key was written.') }
}

export function writePrivateConfig(file, config) {
  if (fs.existsSync(file)) throw new SetupError('Private configuration already exists; it was not overwritten. Use check to inspect its status.')
  protectDirectory(path.dirname(file))
  // Exclusive creation also protects against a second initializer winning the race.
  let descriptor
  try {
    descriptor = fs.openSync(file, 'wx', 0o600)
    fs.writeFileSync(descriptor, JSON.stringify(config, null, 2) + '\n')
    fs.fsyncSync(descriptor)
  } catch { throw new SetupError('Unable to create private configuration exclusively. Existing files were not overwritten.') }
  finally { if (descriptor !== undefined) fs.closeSync(descriptor) }
}

async function readHiddenKey() {
  const supplied = process.env.SUPPORT_DEEPSEEK_API_KEY
  delete process.env.SUPPORT_DEEPSEEK_API_KEY
  if (supplied) return supplied.trim()
  if (!process.stdin.isTTY || !process.stdin.setRawMode) throw new SetupError('No key provided. Run init in an interactive terminal, or securely inject SUPPORT_DEEPSEEK_API_KEY into this process environment.')
  process.stdout.write('DeepSeek API key (hidden, 120-second timeout): ')
  return new Promise((resolve, reject) => {
    let value = ''
    const input = process.stdin
    const wasRaw = input.isRaw
    function finish(error) {
      clearTimeout(timer); input.off('data', onData); input.setRawMode(Boolean(wasRaw)); input.pause(); process.stdout.write('\n')
      if (error) reject(error); else resolve(value.trim())
      value = ''
    }
    function onData(data) {
      const chunk = data.toString('utf8').replace(/\x1b\[(?:200|201)~/g, '')
      for (const character of chunk) {
        if (character === '\u0003') { finish(new SetupError('Cancelled; no key was saved.')); return }
        if (character === '\r' || character === '\n') { finish(); return }
        if (character === '\u007f' || character === '\b') value = value.slice(0, -1)
        else if (value.length < 132 && /^[A-Za-z0-9_-]$/.test(character)) value += character
        else { finish(new SetupError('Invalid key input; no key was saved.')); return }
      }
    }
    const timer = setTimeout(() => finish(new SetupError('Input timed out; no key was saved.')), 120000)
    input.setRawMode(true); input.resume(); input.on('data', onData)
  })
}

async function main() {
  const action = process.argv[2] || 'check'
  if (!['check', 'init'].includes(action) || process.argv.length > 3) throw new SetupError('Usage: node scripts/local/support-secrets.mjs [check|init]. Never pass a key as an argument.')
  const manifest = JSON.parse(fs.readFileSync(path.join(root, 'docs/development-secrets.json'), 'utf8'))
  const entry = manifest.secrets.find(secret => secret.id === 'support_deepseek')
  const file = path.resolve(root, entry.local_file)
  if (!file.startsWith(root + path.sep)) throw new SetupError('Private file must stay inside this checkout.')
  try {
    execFileSync('git', ['-c', 'safe.directory=' + root, '-C', root, 'check-ignore', '--quiet', '--', entry.local_file], { timeout: 5000, windowsHide: true, stdio: 'pipe' })
  } catch { throw new SetupError('Private file is not confirmed Git-ignored. No key was written.') }
  if (action === 'check') {
    let ready = false
    if (fs.existsSync(file)) {
      try { const config = JSON.parse(fs.readFileSync(file, 'utf8')); buildConfig(config, config.api_key); ready = true } catch {}
    }
    console.log(JSON.stringify({ configured: ready, localFile: entry.local_file, githubSecret: entry.github.name, repository: entry.github.repository, githubValueReadable: false, nextStep: ready ? 'Start the local support service.' : 'Run init and securely enter/import the key; git pull cannot download secret values.' }, null, 2))
    if (!ready) process.exitCode = 2
    return
  }
  if (fs.existsSync(file)) throw new SetupError('Private configuration already exists; it was not overwritten. Use check to inspect its status.')
  const template = JSON.parse(fs.readFileSync(path.join(root, entry.template), 'utf8'))
  const key = await readHiddenKey()
  const config = buildConfig(template, key)
  writePrivateConfig(file, config)
  console.log('Private support configuration created with restricted permissions. No network request was made; usage.db was not changed.')
}

if (process.argv[1] && pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url) {
  main().catch(error => {
    console.error(error instanceof SetupError ? error.message : 'Secret setup failed; raw details were suppressed to protect credentials.')
    process.exitCode = 1
  })
}
