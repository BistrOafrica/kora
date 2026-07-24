import { create } from 'zustand'

interface CartItem {
  name?: string
  product?: string
  product_name?: string
  quantity: number
  rate: number
  amount: number
  [key: string]: any
}

interface CartState {
  items: CartItem[]
  addItem: (item: Partial<CartItem>) => void
  removeItem: (name?: string) => void
  updateQuantity: (name: string, quantity: number) => void
  clearCart: () => void
  total: () => number
}

export const useCartStore = create<CartState>((set, get) => ({
  items: [],

  addItem: (item) =>
    set((state) => {
      const existing = state.items.find(
        (i) => i.product === item.product && item.product,
      )
      if (existing) {
        return {
          items: state.items.map((i) =>
            i.product === item.product
              ? {
                  ...i,
                  quantity: i.quantity + (item.quantity || 1),
                  amount: (i.quantity + (item.quantity || 1)) * i.rate,
                }
              : i,
          ),
        }
      }
      return { items: [...state.items, { ...item, quantity: item.quantity || 1 } as CartItem] }
    }),

  removeItem: (name) =>
    set((state) => ({
      items: name
        ? state.items.filter((i) => i.name !== name && i.product !== name)
        : [],
    })),

  updateQuantity: (name, quantity) =>
    set((state) => ({
      items: state.items.map((i) =>
        i.product === name || i.name === name
          ? { ...i, quantity, amount: quantity * i.rate }
          : i,
      ),
    })),

  clearCart: () => set({ items: [] }),

  total: () => get().items.reduce((sum, i) => sum + (i.amount || i.rate * i.quantity), 0),
}))
