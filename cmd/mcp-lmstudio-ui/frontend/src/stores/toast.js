import { writable } from 'svelte/store'

export const toasts = writable([])

let nextId = 0

export function addToast(message, type = 'info', duration = 3000) {
  const id = nextId++
  toasts.update((t) => [...t, { id, message, type }])
  if (duration > 0) {
    setTimeout(() => removeToast(id), duration)
  }
}

export function removeToast(id) {
  toasts.update((t) => t.filter((toast) => toast.id !== id))
}
