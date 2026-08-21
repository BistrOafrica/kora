import { describe, expect, it } from 'vitest'
import {
  buildPageManifestETag,
  computePageManifestDigest,
  verifyPageManifestSignature,
  serializePageManifest,
  WORKSPACE_DASHBOARD_MANIFEST,
} from './page-manifests'

describe('page manifest integrity', () => {
  it('produces stable digests and etags', async () => {
    const digest = await computePageManifestDigest(WORKSPACE_DASHBOARD_MANIFEST)
    const etag = await buildPageManifestETag(WORKSPACE_DASHBOARD_MANIFEST)

    expect(digest).toMatch(/^sha256:/)
    expect(etag).toBe(`"${digest}"`)
  })

  it('verifies signatures when a public key is provided', async () => {
    const keyPair = await crypto.subtle.generateKey(
      { name: 'Ed25519' },
      true,
      ['sign', 'verify'],
    )
    const publicKey = await crypto.subtle.exportKey('jwk', keyPair.publicKey)
    const payload = new TextEncoder().encode(serializePageManifest(WORKSPACE_DASHBOARD_MANIFEST))
    const signature = await crypto.subtle.sign('Ed25519', keyPair.privateKey, payload)
    const encoded = bytesToBase64Url(new Uint8Array(signature))

    await expect(verifyPageManifestSignature(
      WORKSPACE_DASHBOARD_MANIFEST,
      encoded,
      publicKey,
    )).resolves.toBe(true)
  })

  it('fails closed without a signature', async () => {
    await expect(verifyPageManifestSignature(
      WORKSPACE_DASHBOARD_MANIFEST,
      undefined,
      {
        kty: 'OKP',
        crv: 'Ed25519',
        x: 'AA',
      },
    )).resolves.toBe(false)
  })
})

function bytesToBase64Url(bytes: Uint8Array): string {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}
