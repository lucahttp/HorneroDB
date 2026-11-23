import { sileo } from 'sileo'

/**
 * Toast helpers.
 *
 * These live in a plain `.js` module (no components, no JSX) so the
 * `react-refresh/only-export-components` rule does not complain about
 * `Toast.jsx` mixing component and helper exports.
 */

/** Show a simple toast. `type` maps to the sileo state: info | success | error | warning */
export function notify(message, type = 'default') {
  const opts = {
    title: message,
    state: type === 'default' ? 'info' : type,
  }
  sileo.show(opts)
}

export function notifySuccess(message) {
  sileo.success({ title: message })
}

/**
 * Show an error toast.
 * @param {string} message
 * @param {{title?: string, onClick?: () => void} | null} action optional action button
 */
export function notifyError(message, action = null) {
  sileo.error({
    title: message,
    button: action // { title: 'Label', onClick: () => {} }
  })
}

export function notifyWarning(message) {
  sileo.warning({ title: message })
}
