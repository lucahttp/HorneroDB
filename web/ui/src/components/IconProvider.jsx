import { IconoirProvider } from 'iconoir-react'

/**
 * Global Iconoir configuration wrapper.
 * Sets default icon props (color, stroke, size) for all Iconoir icons
 * rendered within the app tree.
 * 
 * Why these defaults:
 * - currentColor: inherits text color automatically, respects dark mode
 * - strokeWidth 1.5: balanced weight matching Inter's visual density
 * - 1.25em: scales proportionally with text size
 */
export function IconProvider({ children }) {
  return (
    <IconoirProvider
      iconProps={{
        color: 'currentColor',
        strokeWidth: 1.5,
        width: '1.25em',
        height: '1.25em',
      }}
    >
      {children}
    </IconoirProvider>
  )
}
