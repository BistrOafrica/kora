export type JsonSchema = Record<string, unknown>

const stringEnum = (values: readonly string[]): JsonSchema => ({
  type: 'string',
  enum: [...values],
})

const objectSchema = (properties: Record<string, JsonSchema>, required: string[] = []): JsonSchema => ({
  type: 'object',
  additionalProperties: false,
  properties,
  required,
})

const arraySchema = (items: JsonSchema): JsonSchema => ({
  type: 'array',
  items,
})

export const PACKAGE_METADATA_SCHEMA: JsonSchema = objectSchema(
  {
    name: { type: 'string', minLength: 1 },
    version: { type: 'string', minLength: 1 },
    package: { type: 'string', minLength: 1 },
    hash: { type: 'string' },
    digest: { type: 'string' },
    signature: { type: 'string' },
    engine_range: { type: 'string' },
    frontend_range: { type: 'string' },
    status: stringEnum(['draft', 'preview', 'active', 'retired']),
  },
  ['name', 'version', 'package', 'status'],
)

export const COMPONENT_SCHEMA: JsonSchema = objectSchema(
  {
    id: { type: 'string', minLength: 1 },
    component: { type: 'string', minLength: 1 },
    version: { type: 'integer', minimum: 1 },
    region: { type: 'string' },
    position: { type: 'integer', minimum: 0 },
    span: { type: 'integer', minimum: 1 },
    props: { type: 'object' },
    data: { type: 'string' },
    actions: arraySchema({ type: 'string' }),
    required_capabilities: arraySchema({ type: 'string' }),
    permissions: arraySchema({ type: 'string' }),
    offline: stringEnum(['unsupported', 'read_only', 'queue_writes', 'full_slice']),
    children: arraySchema({ type: 'object' }),
  },
  ['id', 'component', 'version', 'region', 'position', 'props'],
)

export const RESOURCE_SCHEMA: JsonSchema = objectSchema(
  {
    id: { type: 'string', minLength: 1 },
    query: { type: 'string', minLength: 1 },
    params: { type: 'object' },
    depends_on: arraySchema({ type: 'string' }),
  },
  ['id', 'query', 'params'],
)

export const ACTION_SCHEMA: JsonSchema = objectSchema(
  {
    id: { type: 'string', minLength: 1 },
    command: { type: 'string', minLength: 1 },
    input: { type: 'object' },
    invalidate: arraySchema({ type: 'string' }),
    confirmation: { type: 'boolean' },
    offline: stringEnum(['unsupported', 'queue_writes']),
  },
  ['id', 'command', 'input', 'invalidate'],
)

export const PAGE_MANIFEST_SCHEMA: JsonSchema = objectSchema(
  {
    apiVersion: { type: 'string', minLength: 1 },
    kind: stringEnum(['Page']),
    metadata: PACKAGE_METADATA_SCHEMA,
    spec: objectSchema(
      {
        route: { type: 'string', minLength: 1 },
        runtime: { type: 'string', minLength: 1 },
        permissions: arraySchema({ type: 'string' }),
        capabilities: arraySchema({ type: 'string' }),
        offline: stringEnum(['unsupported', 'read_only', 'queue_writes', 'full_slice']),
        resources: arraySchema(RESOURCE_SCHEMA),
        actions: arraySchema(ACTION_SCHEMA),
        layout: objectSchema(
          {
            type: stringEnum(['single', 'two_panel', 'three_panel', 'grid']),
            columns: { type: 'integer', minimum: 1 },
            children: arraySchema(COMPONENT_SCHEMA),
          },
          ['type', 'children'],
        ),
      },
      ['route', 'runtime', 'permissions', 'capabilities', 'offline', 'resources', 'actions', 'layout'],
    ),
  },
  ['apiVersion', 'kind', 'metadata', 'spec'],
)

export const MANIFEST_SCHEMAS = {
  packageMetadata: PACKAGE_METADATA_SCHEMA,
  component: COMPONENT_SCHEMA,
  resource: RESOURCE_SCHEMA,
  action: ACTION_SCHEMA,
  pageManifest: PAGE_MANIFEST_SCHEMA,
} as const
