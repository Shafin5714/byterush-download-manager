import type { SVGProps } from 'react'

export type IconName =
  | 'activity'
  | 'alert'
  | 'check'
  | 'chevronDown'
  | 'clipboard'
  | 'download'
  | 'folder'
  | 'grid'
  | 'link'
  | 'pause'
  | 'play'
  | 'plus'
  | 'search'
  | 'settings'
  | 'trash'
  | 'x'

interface Props extends Omit<SVGProps<SVGSVGElement>, 'name'> {
  name: IconName
  size?: number
}

export default function Icon({ name, size = 18, ...props }: Props) {
  const common = {
    width: size,
    height: size,
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.8,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    'aria-hidden': true,
  }

  return (
    <svg {...common} {...props}>
      {name === 'activity' && <><path d="M4 12h3l2.2-6 4.2 12 2.1-6H20" /><path d="M5 4.8A9 9 0 1 0 20.2 16" opacity=".45" /></>}
      {name === 'alert' && <><circle cx="12" cy="12" r="9" /><path d="M12 8v4.5" /><path d="M12 16h.01" /></>}
      {name === 'check' && <><circle cx="12" cy="12" r="9" /><path d="m8.2 12.2 2.5 2.5 5.2-5.4" /></>}
      {name === 'chevronDown' && <path d="m7 10 5 5 5-5" />}
      {name === 'clipboard' && <><rect x="6" y="5" width="12" height="15" rx="2" /><path d="M9 5.5V4.2A1.2 1.2 0 0 1 10.2 3h3.6A1.2 1.2 0 0 1 15 4.2v1.3" /></>}
      {name === 'download' && <><path d="M12 3v11" /><path d="m7.5 10 4.5 4.5 4.5-4.5" /><path d="M5 20h14" /></>}
      {name === 'folder' && <path d="M3.5 7.5v9.7A2.3 2.3 0 0 0 5.8 19.5h12.4a2.3 2.3 0 0 0 2.3-2.3V8.8a2.3 2.3 0 0 0-2.3-2.3h-6.1l-2-2H5.8a2.3 2.3 0 0 0-2.3 2.3v.7Z" />}
      {name === 'grid' && <><rect x="4" y="4" width="6" height="6" rx="1.4" /><rect x="14" y="4" width="6" height="6" rx="1.4" /><rect x="4" y="14" width="6" height="6" rx="1.4" /><rect x="14" y="14" width="6" height="6" rx="1.4" /></>}
      {name === 'link' && <><path d="m9.5 14.5 5-5" /><path d="m7.2 16.8-1.1 1.1a3.5 3.5 0 0 1-5-5l3.1-3.1a3.5 3.5 0 0 1 4.9 0" transform="translate(3 0)" /><path d="m16.8 7.2 1.1-1.1a3.5 3.5 0 0 0-5-5L9.8 4.2a3.5 3.5 0 0 0 0 4.9" transform="translate(-3 0)" /></>}
      {name === 'pause' && <><path d="M9 7v10" /><path d="M15 7v10" /></>}
      {name === 'play' && <path d="m9 7 8 5-8 5V7Z" />}
      {name === 'plus' && <><path d="M12 5v14" /><path d="M5 12h14" /></>}
      {name === 'search' && <><circle cx="10.8" cy="10.8" r="6.8" /><path d="m16 16 4 4" /></>}
      {name === 'settings' && <><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-1.6v-.2h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z" /></>}
      {name === 'trash' && <><path d="M5 7h14" /><path d="M9 7V4h6v3" /><path d="m7 7 1 13h8l1-13" /><path d="M10 11v5M14 11v5" /></>}
      {name === 'x' && <><path d="m7 7 10 10" /><path d="M17 7 7 17" /></>}
    </svg>
  )
}
