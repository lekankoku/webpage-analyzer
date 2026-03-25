import type { Metadata } from 'next'
import { IBM_Plex_Mono, DM_Sans } from 'next/font/google'
import './globals.css'

const mono = IBM_Plex_Mono({
  subsets: ['latin'],
  weight: ['400', '600'],
  variable: '--font-mono',
  display: 'swap',
})

const sans = DM_Sans({
  subsets: ['latin'],
  variable: '--font-sans',
  display: 'swap',
})

export const metadata: Metadata = {
  title: 'Page Insight',
  description: 'Analyse any web page — headings, links, HTML version, login form detection.',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${mono.variable} ${sans.variable}`}>
      <body className="bg-[var(--bg-base)] text-[var(--text-primary)] font-sans antialiased">
        {children}
      </body>
    </html>
  )
}
