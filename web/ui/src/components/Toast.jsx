import { Toaster, sileo } from 'sileo'

export const Toast = sileo

export const ToastProvider = Toaster

export function notify(message, type = 'default') {
  // Sileo uses 'title' for the main text. 
  // We map 'message' to 'title' for simple toasts.
  const opts = {
    title: message,
    state: type === 'default' ? 'info' : type,
  }
  sileo.show(opts)
}

export function notifySuccess(message) {
  sileo.success({ title: message })
}

export function notifyError(message, action = null) {
  sileo.error({ 
    title: message,
    button: action // { title: 'Label', onClick: () => {} }
  })
}

export function notifyWarning(message) {
  sileo.warning({ title: message })
}
