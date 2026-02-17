import { Toaster, sileo } from 'sileo'

export const Toast = sileo

export const ToastProvider = Toaster

export function notify(message, type = 'default') {
  sileo.show({
    message,
    type,
  })
}

export function notifySuccess(message) {
  notify(message, 'success')
}

export function notifyError(message) {
  notify(message, 'error')
}

export function notifyWarning(message) {
  notify(message, 'warning')
}
