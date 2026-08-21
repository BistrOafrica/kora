import { describe, expect, it } from 'vitest'
import { validateAttachmentFile } from './attachment-utils'

describe('validateAttachmentFile', () => {
  it('accepts image files for image attachments', () => {
    const file = new File(['x'], 'photo.png', { type: 'image/png' })
    expect(validateAttachmentFile(file, 'Attach Image')).toBeNull()
  })

  it('rejects non-image files for image attachments', () => {
    const file = new File(['x'], 'document.pdf', { type: 'application/pdf' })
    expect(validateAttachmentFile(file, 'Attach Image')).toBe('Please choose an image file.')
  })

  it('accepts audio files for audio attachments', () => {
    const file = new File(['x'], 'voice.m4a', { type: 'audio/mp4' })
    expect(validateAttachmentFile(file, 'Attach Audio')).toBeNull()
  })

  it('rejects non-audio files for audio attachments', () => {
    const file = new File(['x'], 'photo.jpg', { type: 'image/jpeg' })
    expect(validateAttachmentFile(file, 'Attach Audio')).toBe('Please choose an audio file.')
  })

  it('honours a custom accept list (extensions)', () => {
    const pdf = new File(['x'], 'invoice.pdf', { type: 'application/pdf' })
    expect(validateAttachmentFile(pdf, 'Attach', '.pdf,.docx')).toBeNull()

    const csv = new File(['x'], 'data.csv', { type: 'text/csv' })
    expect(validateAttachmentFile(csv, 'Attach', '.pdf,.docx')).toBe(
      'Please choose a file in one of these formats: .pdf, .docx.',
    )
  })

  it('honours MIME-type accept entries', () => {
    const png = new File(['x'], 'photo.png', { type: 'image/png' })
    expect(validateAttachmentFile(png, 'Attach', 'image/*')).toBeNull()

    const mp3 = new File(['x'], 'song.mp3', { type: 'audio/mpeg' })
    expect(validateAttachmentFile(mp3, 'Attach', 'image/*')).toContain('Please choose a file')
  })
})
