// The key that opens the help page (Ayuda.svelte) and its content per language. Pure.

import { SECTIONS as es } from './i18n/help/es.js'
import { SECTIONS as en } from './i18n/help/en.js'
import { SECTIONS as ptBR } from './i18n/help/pt-BR.js'

// isHelpKey tells the key that opens the help from anywhere: "?", unless the
// user is typing or holds Ctrl, ⌘ or Alt.
export function isHelpKey(event) {
  if (event.key !== '?' || event.ctrlKey || event.metaKey || event.altKey) return false
  const tag = event.target?.tagName
  return !(['INPUT', 'TEXTAREA', 'SELECT'].includes(tag) || event.target?.isContentEditable)
}

const BY_LOCALE = { es, en, 'pt-BR': ptBR }

// helpSections returns the help sections of a locale: each section is
// { id, title, intro?, items } and an item is { term?, keys?, text, href?,
// blockedInListen? }. keys are the same tokens in every language, href is an
// internal link, blockedInListen marks what listen mode hides. An unknown
// locale gets English.
export function helpSections(locale) {
  return BY_LOCALE[locale] ?? en
}
