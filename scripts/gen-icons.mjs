/**
 * Generates PWA PNG icons from the Twemoji 🕊️ SVG.
 *
 * Strategy:
 *   1. Fetch the Twemoji dove SVG (plain paths — no fonts needed).
 *   2. Render it to a transparent PNG with sharp.
 *   3. Composite it centred onto an indigo rounded-square background.
 *
 * Run once from the repo root:  node scripts/gen-icons.mjs
 * Requires internet on first run; re-run whenever you want to refresh icons.
 */
import https from 'node:https'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import sharp from 'sharp'

const __dirname = dirname(fileURLToPath(import.meta.url))
const publicDir = join(__dirname, '../ui/public')

// Follow up to 5 redirects and return the body as a string.
function get(url, depth = 0) {
  if (depth > 5) return Promise.reject(new Error('too many redirects'))
  return new Promise((resolve, reject) => {
    https.get(url, (res) => {
      if (res.statusCode >= 300 && res.headers.location) {
        return resolve(get(res.headers.location, depth + 1))
      }
      const chunks = []
      res.on('data', (d) => chunks.push(d))
      res.on('end', () => resolve(Buffer.concat(chunks).toString('utf8')))
      res.on('error', reject)
    }).on('error', reject)
  })
}

// Twemoji 🕊️ = U+1F54A — official MaxCDN hosted by Twitter/X.
const DOVE_URL = 'https://twemoji.maxcdn.com/v/latest/svg/1f54a.svg'

console.log('Fetching Twemoji dove SVG…')
const doveSvg = await get(DOVE_URL)
console.log(`  got ${doveSvg.length} bytes`)

// Indigo rounded-square background (same rx as before).
function bgSvg(size) {
  return Buffer.from(
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${size} ${size}">` +
    `<rect width="${size}" height="${size}" rx="${Math.round(size * 0.1875)}" fill="#4f46e5"/>` +
    `</svg>`,
  )
}

async function generate(size, dest) {
  // 1. Background as PNG.
  const bg = await sharp(bgSvg(size)).resize(size, size).png().toBuffer()

  // 2. Twemoji dove at 72 % of icon size → 14 % padding each side (maskable-safe).
  const doveSize = Math.round(size * 0.72)
  const dove = await sharp(Buffer.from(doveSvg))
    .resize(doveSize, doveSize, { fit: 'contain', background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .png()
    .toBuffer()

  // 3. Composite: dove centred on background.
  const offset = Math.round((size - doveSize) / 2)
  await sharp(bg)
    .composite([{ input: dove, top: offset, left: offset }])
    .png()
    .toFile(dest)

  console.log(`  ${dest} (${size}×${size})`)
}

// Extract the Twemoji viewBox and inner elements so we can embed them in an SVG.
const viewBox = (doveSvg.match(/viewBox="([^"]+)"/) ?? [])[1] ?? '0 0 36 36'
const inner = (doveSvg.match(/<svg[^>]*>([\s\S]*)<\/svg>/i) ?? [])[1] ?? ''

// favicon.svg — same composition as the PNGs but as a vector file.
// Browsers render nested <svg> natively so this matches the rasterised icons exactly.
// Padding: 14 % each side (same as the PNG 72 % dove size).
const faviconSvg =
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">\n` +
  `  <rect width="512" height="512" rx="96" fill="#4f46e5"/>\n` +
  `  <svg x="72" y="72" width="368" height="368" viewBox="${viewBox}">${inner}</svg>\n` +
  `</svg>\n`

import { writeFileSync } from 'node:fs'
writeFileSync(join(publicDir, 'favicon.svg'), faviconSvg)
console.log(`  ${join(publicDir, 'favicon.svg')} (vector)`)

console.log('Generating icons…')
await generate(512, join(publicDir, 'pwa-512.png'))
await generate(192, join(publicDir, 'pwa-192.png'))
await generate(180, join(publicDir, 'apple-touch-icon.png'))
console.log('Done.')
