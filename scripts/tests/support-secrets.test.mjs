import { test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'
import { buildConfig, writePrivateConfig } from '../local/support-secrets.mjs'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const dummy = 'sk-test-only-do-not-use-12345678'

test('invalid key errors do not include input', () => {
  const value = 'sensitive malformed value'
  assert.throws(() => buildConfig({}, value), error => !error.message.includes(value))
})

test('initializer keeps nonsecret limits and writes only an ignored private fixture', () => {
  const testRoot = path.join(root, 'work/codex-runtime/support-secret-tests')
  fs.mkdirSync(testRoot, { recursive: true })
  const directory = fs.mkdtempSync(path.join(testRoot, 'fixture-'))
  const file = path.join(directory, 'private/config.json')
  const template = JSON.parse(fs.readFileSync(path.join(root, 'docs/examples/support-agent.config.example.json'), 'utf8'))
  const config = buildConfig(template, dummy)
  writePrivateConfig(file, config)
  const stored = JSON.parse(fs.readFileSync(file, 'utf8'))
  assert.equal(stored.api_key, dummy)
  assert.equal(stored.daily_budget_nanousd, 250000000)
  assert.equal(template.api_key, '')
  if (process.platform !== 'win32') assert.equal(fs.statSync(file).mode & 0o777, 0o600)
  else {
    const encoded = Buffer.from(file, 'utf8').toString('base64')
    const script = `$ErrorActionPreference='Stop';$file=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('${encoded}'));$acl=Get-Acl -LiteralPath $file;$allowed=@([Security.Principal.WindowsIdentity]::GetCurrent().User.Value,'S-1-5-18');if(@($acl.Access).Count -ne 2){exit 1};foreach($rule in $acl.Access){if($allowed -notcontains $rule.IdentityReference.Translate([Security.Principal.SecurityIdentifier]).Value){exit 1}}`
    const environment = { ...process.env }
    for (const name of Object.keys(environment)) if (name.toLowerCase() === 'psmodulepath') delete environment[name]
    execFileSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', script], { env: environment, timeout: 10000, windowsHide: true, stdio: 'pipe' })
  }
  const before = fs.readFileSync(file)
  assert.throws(() => writePrivateConfig(file, { api_key: 'different' }), /not overwritten/)
  assert.deepEqual(fs.readFileSync(file), before)
})

test('tracked inventory and example contain references, not key values', () => {
  const manifest = JSON.parse(fs.readFileSync(path.join(root, 'docs/development-secrets.json'), 'utf8'))
  const entry = manifest.secrets[0]
  assert.equal(entry.github.name, 'SUPPORT_DEEPSEEK_API_KEY')
  assert.equal(entry.github.value_readable, false)
  assert.equal(entry.committed_to_git, false)
  const template = JSON.parse(fs.readFileSync(path.join(root, entry.template), 'utf8'))
  assert.equal(template.api_key, '')
  assert.equal(entry.local_file, 'work/codex-runtime/support-agent/private/config.json')
})
