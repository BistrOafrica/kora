import { describe, expect, it } from 'vitest'
import {
  ACTION_SCHEMA,
  COMPONENT_SCHEMA,
  MANIFEST_SCHEMAS,
  PACKAGE_METADATA_SCHEMA,
  PAGE_MANIFEST_SCHEMA,
  RESOURCE_SCHEMA,
} from './manifest-schemas'

describe('manifest schemas', () => {
  it('define the page manifest contract surface', () => {
    expect(PAGE_MANIFEST_SCHEMA).toMatchObject({
      type: 'object',
      required: ['apiVersion', 'kind', 'metadata', 'spec'],
    })
    expect(PAGE_MANIFEST_SCHEMA).toMatchObject({
      properties: {
        metadata: PACKAGE_METADATA_SCHEMA,
        kind: { enum: ['Page'] },
        spec: {
          properties: {
            layout: {
              properties: {
                children: { type: 'array', items: COMPONENT_SCHEMA },
              },
            },
            resources: { type: 'array', items: RESOURCE_SCHEMA },
            actions: { type: 'array', items: ACTION_SCHEMA },
          },
        },
      },
    })
  })

  it('keeps package metadata and component schemas narrow', () => {
    expect(PACKAGE_METADATA_SCHEMA).toMatchObject({
      properties: {
        status: { enum: ['draft', 'preview', 'active', 'retired'] },
      },
    })
    expect(COMPONENT_SCHEMA).toMatchObject({
      required: ['id', 'component', 'version', 'region', 'position', 'props'],
      properties: {
        version: { type: 'integer', minimum: 1 },
        position: { type: 'integer', minimum: 0 },
        span: { type: 'integer', minimum: 1 },
      },
    })
  })

  it('exports a registry of canonical schemas', () => {
    expect(Object.keys(MANIFEST_SCHEMAS)).toEqual([
      'packageMetadata',
      'component',
      'resource',
      'action',
      'pageManifest',
    ])
  })
})
