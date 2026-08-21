export interface OfflineQueueItem<T = Record<string, unknown>> {
  id: string
  createdAt: string
  updatedAt?: string
  type: string
  payload: T
  status: 'queued' | 'syncing' | 'accepted' | 'rejected' | 'conflict' | 'discarded'
  site?: string
  branchId?: string
  deviceId?: string
  operationId?: string
  baseVersion?: number
  correlationId?: string
  error?: { code: string; message: string }
}

const DB_NAME = 'kora_offline_runtime'
const DB_VERSION = 1
const STORE_NAME = 'operation_queue'

const memoryQueue = new Map<string, OfflineQueueItem>()

export async function loadOfflineQueue(): Promise<OfflineQueueItem[]> {
  const db = await openOfflineDb()
  if (!db) return Array.from(memoryQueue.values()).sort(sortByCreatedAt)

  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readonly')
    const request = tx.objectStore(STORE_NAME).getAll()
    request.onsuccess = () => resolve((request.result as OfflineQueueItem[]).sort(sortByCreatedAt))
    request.onerror = () => reject(request.error)
  })
}

export async function enqueueOfflineItem<T>(
  item: Omit<OfflineQueueItem<T>, 'status' | 'createdAt' | 'updatedAt'>,
): Promise<OfflineQueueItem<T>> {
  const queued: OfflineQueueItem<T> = {
    ...item,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    operationId: item.operationId || item.id,
    status: 'queued',
  }
  await putOfflineItem(queued as OfflineQueueItem)
  return queued
}

export async function markOfflineItemStatus(
  id: string,
  status: OfflineQueueItem['status'],
): Promise<OfflineQueueItem[]> {
  const queue = await loadOfflineQueue()
  const item = queue.find((entry) => entry.id === id)
  if (!item) return queue
  await putOfflineItem({ ...item, status, updatedAt: new Date().toISOString() })
  return loadOfflineQueue()
}

export async function recordOfflineConflict(
  id: string,
  error: { code: string; message: string },
): Promise<OfflineQueueItem[]> {
  const queue = await loadOfflineQueue()
  const item = queue.find((entry) => entry.id === id)
  if (!item) return queue
  await putOfflineItem({ ...item, status: 'conflict', error, updatedAt: new Date().toISOString() })
  return loadOfflineQueue()
}

export async function clearOfflineQueue(): Promise<void> {
  const db = await openOfflineDb()
  memoryQueue.clear()
  if (!db) return
  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const request = tx.objectStore(STORE_NAME).clear()
    request.onsuccess = () => resolve()
    request.onerror = () => reject(request.error)
  })
}

async function putOfflineItem(item: OfflineQueueItem): Promise<void> {
  const db = await openOfflineDb()
  if (!db) {
    memoryQueue.set(item.id, item)
    return
  }

  await new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const request = tx.objectStore(STORE_NAME).put(item)
    request.onsuccess = () => resolve()
    request.onerror = () => reject(request.error)
  })
}

function openOfflineDb(): Promise<IDBDatabase | null> {
  if (typeof indexedDB === 'undefined') return Promise.resolve(null)

  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        const store = db.createObjectStore(STORE_NAME, { keyPath: 'id' })
        store.createIndex('type', 'type', { unique: false })
        store.createIndex('status', 'status', { unique: false })
        store.createIndex('operationId', 'operationId', { unique: false })
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
    request.onblocked = () => resolve(null)
  })
}

function sortByCreatedAt(a: OfflineQueueItem, b: OfflineQueueItem): number {
  return a.createdAt.localeCompare(b.createdAt)
}
