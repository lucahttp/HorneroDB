# HorneroDB — Manual de Marca

> Guía de diseño y sistema visual para el desarrollo de la aplicación HorneroDB.

---

## 1. Identidad de Marca

### Nombre
**HorneroDB**

### Posicionamiento
CRM de datos con foco en seguridad, inspirado en Airtable, NocoDB, PocketBase y Dataverse. Combina la flexibilidad de una hoja de cálculo con el poder de una base de datos, protegido por SSO (OIDC) y permisos granulares a nivel de tabla, columna y fila.

### Logo
- Ícono: emoji 🐝 dentro de un cuadrado redondeado (`border-radius: 10px`) con fondo `--primary` y borde de 2px negro.
- Texto: **HorneroDB** en Inter 800, color blanco sobre fondos oscuros.

---

## 2. ADN de Diseño

El sistema visual de HorneroDB combina elementos de cuatro referencias:

| Referencia | Qué tomamos |
|---|---|
| **[giffgaff](https://www.giffgaff.com/)** | Botones pill (border-radius alto), estética amigable y playful, colores vibrantes |
| **[Gumroad](https://gumroad.com/)** | Bordes gruesos negros, sombras offset tipo neo-brutalist (`4px 4px 0px`), tipografía bold y limpia |
| **[PocketBase](https://pocketbase.io/)** | Layout de dashboard admin con sidebar, tablas de datos limpias, UX minimalista |
| **[FunctionGemma](https://huggingface.co/spaces/webml-community/FunctionGemma-Physics-Playground)** | Estética developer/técnica, uso de tipografía monospace, header oscuro funcional |

### Principios Visuales

1. **Bold pero limpio** — Bordes gruesos y sombras offset dan carácter, pero el espacio y la jerarquía mantienen la claridad.
2. **Neo-brutalist funcional** — Las sombras offset y los bordes pesados comunican solidez y confianza.
3. **Color intencionado** — Cada color tiene un propósito semántico (acción, éxito, peligro, advertencia).
4. **Dark mode como ciudadano de primera clase** — No es un agregado: los tokens se intercambian completamente.

---

## 3. Sistema de Color

### Modo Claro (default)

| Token | Hex | Uso |
|---|---|---|
| `--primary` | `#36C9FF` | Acciones principales, links activos, foco |
| `--primary-hover` | `#1EB8E6` | Hover de primary |
| `--primary-light` | `#E0F4FF` | Fondos tintados, badges, tabs activos |
| `--accent` | `#FF6B36` | Acentos, CTAs secundarios |
| `--accent-hover` | `#E65A2A` | Hover de accent |
| `--accent-light` | `#FFE0D4` | Fondos tintados accent |
| `--bg` | `#FFFFFF` | Fondo principal |
| `--bg-surface` | `#F7F7F5` | Superficies elevadas, fondos de header |
| `--bg-elevated` | `#FFFFFF` | Cards, modales |
| `--text` | `#1A1A1A` | Texto principal |
| `--text-secondary` | `#6B6B6B` | Texto secundario, labels |
| `--text-muted` | `#999999` | Texto terciario, placeholders |
| `--border-color` | `#222222` | Bordes principales (gruesos) |
| `--border-light` | `#E8E8E8` | Bordes sutiles, separadores |
| `--danger` | `#FF4444` | Errores, acciones destructivas |
| `--danger-hover` | `#E63333` | Hover de danger |
| `--success` | `#00C853` | Confirmaciones, estados exitosos |
| `--warning` | `#FF9100` | Advertencias |

### Modo Oscuro (`.dark`)

| Token | Hex | Nota |
|---|---|---|
| `--primary` | `#FF6B36` | ⚠️ Se intercambia con accent del modo claro |
| `--primary-hover` | `#FF8A5C` | |
| `--primary-light` | `#3D1A0A` | |
| `--accent` | `#36C9FF` | ⚠️ Se intercambia con primary del modo claro |
| `--accent-hover` | `#5DD6FF` | |
| `--accent-light` | `#0A2A3D` | |
| `--bg` | `#0D0D0D` | |
| `--bg-surface` | `#1A1A1A` | |
| `--bg-elevated` | `#232323` | |
| `--text` | `#F5F5F5` | |
| `--text-secondary` | `#A0A0A0` | |
| `--text-muted` | `#666666` | |
| `--border-color` | `#444444` | |
| `--border-light` | `#2A2A2A` | |
| `--danger` | `#FF5555` | |
| `--success` | `#00E676` | |
| `--warning` | `#FFAB40` | |

> [!IMPORTANT]
> En dark mode, primary y accent intercambian valores. Esto asegura contraste adecuado: el naranja (`#FF6B36`) tiene mejor visibilidad sobre fondos oscuros como color primario.

---

## 4. Tipografía

### Familias

| Rol | Fuente | Variable CSS | Fallbacks |
|---|---|---|---|
| **Sans (UI)** | [Inter](https://fonts.google.com/specimen/Inter) | `--font-sans` | `system-ui, -apple-system, sans-serif` |
| **Mono (código)** | [JetBrains Mono](https://fonts.google.com/specimen/JetBrains+Mono) | `--font-mono` | `monospace` |

### Pesos disponibles

| Fuente | Pesos |
|---|---|
| Inter | 300 (Light), 400 (Regular), 500 (Medium), 600 (SemiBold), 700 (Bold), 800 (ExtraBold), 900 (Black) |
| JetBrains Mono | 400 (Regular), 500 (Medium), 600 (SemiBold) |

### Escala de Tamaños

| Nombre | Valor | Uso |
|---|---|---|
| `xs` | `0.75rem` (12px) | Labels uppercase, badges |
| `sm` | `0.8125rem` (13px) | Hints, metadata, botones pequeños |
| `base` | `0.875rem` (14px) | Tabs, sidebar links |
| `md` | `0.9rem` (14.4px) | Texto de tabla, inputs, botones |
| `lg` | `1rem` (16px) | Texto de cuerpo, botones grandes |
| `xl` | `1.125rem` (18px) | Títulos de card, modal |
| `2xl` | `1.25rem` (20px) | Logo texto, nombres de sección |
| `stat` | `2rem` (32px) | Números de estadísticas |

### Convenciones

- **Labels y encabezados de tabla**: `uppercase`, `letter-spacing: 0.04–0.06em`, `font-weight: 700`
- **Logo**: `font-weight: 800`, `letter-spacing: -0.02em`
- **Card titles**: `font-weight: 700`, `1.125rem`
- **Code blocks**: Siempre `--font-mono`

---

## 5. Espaciado y Layout

### Layout Principal

```
┌─────────────────────────────────────────────────┐
│ Sidebar (260px)  │  Main Content                │
│ fixed, 100vh     │  margin-left: var(--sidebar)  │
│ bg: #1A1A1A      │  bg: var(--bg)               │
│                  │                              │
│ ┌─ Header ─────┐│  ┌─ Sticky Header ──────────┐│
│ │ Logo         ││  │ Breadcrumb  │  Actions    ││
│ └──────────────┘│  └────────────────────────────┘│
│ ┌─ Nav ────────┐│  ┌─ Body ───────────────────┐│
│ │ Links        ││  │ max-width: 1400px        ││
│ │              ││  │ padding: 2rem            ││
│ └──────────────┘│  └────────────────────────────┘│
│ ┌─ Footer ─────┐│                               │
│ │ User info    ││                               │
│ └──────────────┘│                               │
└─────────────────────────────────────────────────┘
```

### Variables de Layout

| Variable | Valor |
|---|---|
| `--sidebar-width` | `260px` |

### Escala de Padding/Gap

| Valor | rem | px |
|---|---|---|
| `0.25rem` | 4px | Micro spacing |
| `0.375rem` | 6px | Button padding sm |
| `0.5rem` | 8px | gap-2, inline spacing |
| `0.625rem` | 10px | Input padding |
| `0.75rem` | 12px | gap-3, card internal |
| `1rem` | 16px | gap-4, section spacing |
| `1.25rem` | 20px | Card padding, modal header |
| `1.5rem` | 24px | Section padding, modal body |
| `2rem` | 32px | Page body padding |
| `3rem` | 48px | Login panel padding |
| `4rem` | 64px | Empty state padding |

---

## 6. Bordes y Elevación

### Sistema de Bordes

El borde grueso es el rasgo visual más distintivo de HorneroDB (inspirado en Gumroad).

| Token | Valor | Uso |
|---|---|---|
| `--border-thick` | `2.5px` | Bordes principales de cards, modales, tablas, inputs |
| `--border-color` | `#222222` / `#444444` (dark) | Color del borde grueso |
| `--border-light` | `#E8E8E8` / `#2A2A2A` (dark) | Separadores internos, divisores |

### Escala de Border Radius

| Token | Valor | Uso |
|---|---|---|
| `--radius-sm` | `8px` | Inputs, sidebar links |
| `--radius-md` | `12px` | Cards, tablas, stat-cards |
| `--radius-lg` | `16px` | Modales |
| `--radius-pill` | `999px` | Botones, badges |

### Sistema de Sombras

| Token | Valor Light | Valor Dark |
|---|---|---|
| `--shadow-sm` | `0 1px 2px rgba(0,0,0,0.04)` | `0 1px 2px rgba(0,0,0,0.2)` |
| `--shadow-md` | `0 4px 12px rgba(0,0,0,0.06)` | `0 4px 12px rgba(0,0,0,0.3)` |
| `--shadow-lg` | `0 8px 30px rgba(0,0,0,0.08)` | `0 8px 30px rgba(0,0,0,0.4)` |

### Sombras Offset (Neo-Brutalist)

Usadas en hover de cards y botones para dar sensación de profundidad física:

```css
/* Card hover */
box-shadow: 4px 4px 0px var(--border-color);

/* Button hover */
box-shadow: 3px 3px 0px var(--border-color);
transform: translate(-1px, -1px);

/* Modal */
box-shadow: 8px 8px 0px var(--border-color);
```

---

## 7. Iconografía — Iconoir

### Librería

**[Iconoir](https://iconoir.com/)** — 1500+ íconos SVG open-source, diseñados en grilla de 24×24px, licencia MIT.

### Instalación

```bash
npm i iconoir-react
```

### Uso Básico

```jsx
import { Home, Settings, Plus } from 'iconoir-react'

function Example() {
  return (
    <div>
      <Home />
      <Settings color="var(--text-secondary)" />
      <Plus width={20} height={20} />
    </div>
  )
}
```

### Configuración Global — `IconoirProvider`

Usar `IconoirProvider` en el root de la app para definir props default de todos los íconos:

```jsx
import { IconoirProvider } from 'iconoir-react'

function App() {
  return (
    <IconoirProvider
      iconProps={{
        color: 'currentColor',
        strokeWidth: 1.5,
        width: '1.25em',
        height: '1.25em',
      }}
    >
      {/* ... app content ... */}
    </IconoirProvider>
  )
}
```

### Convenciones de Uso

| Contexto | Tamaño | strokeWidth | Ejemplo |
|---|---|---|---|
| **Sidebar links** | `1.25em` (20px) | 1.5 | `<Home />` |
| **Botones con ícono** | `1em` (16px) | 2 | `<Plus strokeWidth={2} />` |
| **Badges / inline** | `0.875em` (14px) | 1.5 | `<Check />` |
| **Empty states** | `3rem` (48px) | 1 | `<DatabaseScript width={48} height={48} />` |
| **Stat cards** | `1.5rem` (24px) | 1.5 | `<Group />` |

### Íconos Recomendados por Contexto

| Contexto | Ícono Iconoir | Import |
|---|---|---|
| Inicio / Dashboard | `Home` | `import { Home } from 'iconoir-react'` |
| Tablas / Data | `Table2Columns` | `import { Table2Columns } from 'iconoir-react'` |
| Agregar / Nuevo | `Plus` | `import { Plus } from 'iconoir-react'` |
| Editar | `EditPencil` | `import { EditPencil } from 'iconoir-react'` |
| Eliminar | `Trash` | `import { Trash } from 'iconoir-react'` |
| Configuración | `Settings` | `import { Settings } from 'iconoir-react'` |
| Usuarios / Permisos | `Group` | `import { Group } from 'iconoir-react'` |
| Seguridad / Auth | `Lock` | `import { Lock } from 'iconoir-react'` |
| Buscar | `Search` | `import { Search } from 'iconoir-react'` |
| Filtrar | `FilterList` | `import { FilterList } from 'iconoir-react'` |
| Ordenar | `Sort` | `import { Sort } from 'iconoir-react'` |
| Cerrar | `Xmark` | `import { Xmark } from 'iconoir-react'` |
| Menú / Más opciones | `MoreVert` | `import { MoreVert } from 'iconoir-react'` |
| Copiar | `Copy` | `import { Copy } from 'iconoir-react'` |
| Éxito / Check | `Check` | `import { Check } from 'iconoir-react'` |
| Error / Warning | `WarningTriangle` | `import { WarningTriangle } from 'iconoir-react'` |
| Info | `InfoCircle` | `import { InfoCircle } from 'iconoir-react'` |
| Base de datos | `DatabaseScript` | `import { DatabaseScript } from 'iconoir-react'` |
| API / Llaves | `Key` | `import { Key } from 'iconoir-react'` |
| Logout | `LogOut` | `import { LogOut } from 'iconoir-react'` |
| Dark mode toggle | `SunLight` / `HalfMoon` | `import { SunLight, HalfMoon } from 'iconoir-react'` |
| Descargar / Export | `Download` | `import { Download } from 'iconoir-react'` |
| Subir / Import | `Upload` | `import { Upload } from 'iconoir-react'` |
| Workspace | `Folder` | `import { Folder } from 'iconoir-react'` |
| Columna | `ViewColumns2` | `import { ViewColumns2 } from 'iconoir-react'` |
| Fila | `List` | `import { List } from 'iconoir-react'` |
| Refresh | `Refresh` | `import { Refresh } from 'iconoir-react'` |

> [!TIP]
> Explorar el catálogo completo en [iconoir.com](https://iconoir.com/). Todos los íconos siguen nombres PascalCase en React.

---

## 8. Componentes

### Botones

Estilo pill inspirado en giffgaff, con bordes gruesos Gumroad.

| Clase | Uso | Estilo |
|---|---|---|
| `.btn-primary` | Acción principal | Fondo `--primary`, texto blanco, borde `--primary-hover` |
| `.btn-secondary` | Acción secundaria | Fondo `--bg-elevated`, texto `--text`, borde `--border-color` |
| `.btn-ghost` | Acción terciaria | Transparente, sin borde visible |
| `.btn-danger` | Acción destructiva | Fondo `--danger`, texto blanco |

**Tamaños:** `.btn-sm`, `.btn-md`, `.btn-lg`

**Interacciones:**
- **Hover**: sombra offset `3px 3px 0px` + `translate(-1px, -1px)`
- **Active**: `scale(0.97)`

```jsx
<button className="btn btn-primary btn-md">
  <Plus strokeWidth={2} /> Crear tabla
</button>
```

### Cards

Estilo Gumroad con bordes gruesos y sombra offset en hover.

```css
.card {
  border: var(--border-thick) solid var(--border-color);
  border-radius: var(--radius-md);     /* 12px */
  padding: 1.5rem;
}
.card:hover {
  transform: translateY(-2px);
  box-shadow: 4px 4px 0px var(--border-color);
}
```

**Variantes:** `.card.border-dashed` para cards de "agregar nuevo".

### Modales

```css
.modal-overlay { z-index: 50; backdrop-filter: blur(8px); }
.modal { 
  max-width: 32rem;
  border-radius: var(--radius-lg);     /* 16px */
  box-shadow: 8px 8px 0px var(--border-color);
}
```

**Estructura:** `.modal-header` → `.modal-body` → `.modal-footer`

### Tablas de Datos

Estilo PocketBase: limpias, contenidas en un container con bordes.

```css
.table-container {
  border: var(--border-thick) solid var(--border-color);
  border-radius: var(--radius-md);
}
.table th {
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 700;
  font-size: 0.75rem;
}
.table tr:hover td {
  background: var(--primary-light);
}
```

### Badges

Píldoras con borde de color semántico.

| Clase | Color | Uso |
|---|---|---|
| `.badge-primary` | `--primary` | Estado principal |
| `.badge-success` | `--success` | Activo, completado |
| `.badge-warning` | `--warning` | Pendiente, atención |
| `.badge-error` | `--danger` | Error, bloqueado |
| `.badge-gray` | `--border-light` | Inactivo, neutral |

### Formularios

```css
.form-input {
  border: var(--border-thick) solid var(--border-color);
  border-radius: var(--radius-sm);     /* 8px */
}
.form-input:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px var(--primary-light);
}
```

**Labels**: `.form-label` — uppercase, 0.8125rem, semibold

### Tabs

```css
.tab.active {
  border-bottom: 3px solid var(--primary);
  background: var(--primary-light);
}
```

---

## 9. Dark Mode

### Activación

Agregar la clase `.dark` al elemento `<html>` o `<body>`.

### Reglas

1. **Tokens se intercambian automáticamente** — Todo componente que use variables CSS se adapta.
2. **Primary ↔ Accent swap** — En dark mode, los roles se invierten para optimizar contraste.
3. **Sombras más profundas** — Los valores de opacidad en sombras aumentan significativamente.
4. **Sidebar** — Pasa de `#1A1A1A` a `#0A0A0A`, bordes de `#333` a `#222`.
5. **No usar colores hardcoded** — Siempre usar tokens `var(--xxx)`.

### Excepciones Permitidas

- Fondos de código: `#1e1e1e` (siempre oscuro, como VS Code)
- Avatar border: siempre `#000`

---

## 10. Movimiento y Animación

### Animaciones Definidas

| Nombre | Duración | Easing | Uso |
|---|---|---|---|
| `fadeIn` | `0.2s` | `ease-out` | Modal overlay, aparición de elementos |
| `scaleIn` | `0.2s` | `ease-out` | Modal, cards de aparición |
| `slideUp` | — | — | Elementos que entran desde abajo |
| `spin` | `0.6s` | `linear` | Loading spinner (infinito) |

### Transiciones

| Propiedad | Duración | Uso |
|---|---|---|
| `all` | `0.15s ease` | Botones, links, inputs |
| `all` | `0.2s ease` | Cards |

### Principios

1. **Rápido y sutil** — Máximo 300ms para transiciones interactivas.
2. **No decorativo** — Toda animación comunica un cambio de estado.
3. **Active = feedback** — `scale(0.97)` en botones para feedback táctil.
4. **Hover = invitación** — Sombra offset + translate comunica que es interactivo.

---

## 11. Scrollbar

Slim y moderna, no intrusiva:

```css
::-webkit-scrollbar { width: 6px; height: 6px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { 
  background: var(--border-light); 
  border-radius: 3px; 
}
::-webkit-scrollbar-thumb:hover { 
  background: var(--text-muted); 
}
```

---

## 12. Voz y Tono

### En la UI

- **Conciso**: Texto corto, directo. "Crear tabla" no "Haga click aquí para crear una nueva tabla".
- **Técnico pero humano**: El usuario es developer o admin, pero no queremos frialdad.
- **Labels en español** (configurable): Toda la UI se escribe en español como default.
- **Errores con contexto**: No solo "Error", sino "Error 400: workspace_id inválido" con link a detalles.
- **Confirmaciones claras**: "¿Eliminar tabla 'Clientes'? Esta acción no se puede deshacer."

### En la Documentación

- Documentar el **por qué**, no el **qué**.
- Código explica el qué, los comentarios explican la intención.
