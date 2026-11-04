import { randomBytes } from 'node:crypto'
import { readFileSync, writeFileSync } from 'node:fs'

// CI uses its own fresh .env and volumes. Never replace a developer's settings.
let template = readFileSync(new URL('../.env.example', import.meta.url), 'utf8')
template = template.replace(/^([A-Z_]*PASSWORD)=.*$/gm, (_, key) => `${key}=${randomBytes(24).toString('hex')}`)
template = template.replace(/^USERS_COUNT=.*$/m, 'USERS_COUNT=10000')
writeFileSync(new URL('../.env', import.meta.url), template, { flag: 'wx', mode: 0o600 })
console.log('Created isolated CI configuration with random credentials and 10000 users.')
