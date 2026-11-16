import en from './en.js'
import es from './es.js'
import ptBR from './pt-BR.js'

export const LOCALES = ['es', 'en', 'pt-BR']
export const LOCALE_NAMES = { es: 'Español', en: 'English', 'pt-BR': 'Português (Brasil)' }

const NUMBER_LOCALES = { es: 'es-AR', en: 'en', 'pt-BR': 'pt-BR' }

let messages = { es, en, 'pt-BR': ptBR }

// _setMessages swaps the message tables (tests only) and returns a function
// that restores the previous ones.
export function _setMessages(m) {
  const prev = messages
  messages = m
  return () => {
    messages = prev
  }
}

// canonical maps a language tag (any case, any region) to a supported locale,
// or '' when none matches. Portuguese of any region maps to pt-BR.
function canonical(tag) {
  if (typeof tag !== 'string') return ''
  const lower = tag.trim().toLowerCase()
  for (const l of LOCALES) if (l.toLowerCase() === lower) return l
  const base = lower.split(/[-_]/)[0]
  if (base === 'es') return 'es'
  if (base === 'en') return 'en'
  if (base === 'pt') return 'pt-BR'
  return ''
}

// resolveLocale turns the installation setting ('' or unknown = automatic)
// and the browser languages into one of the supported locales.
export function resolveLocale(setting, languages) {
  const chosen = LOCALES.includes(setting) ? setting : ''
  if (chosen) return chosen
  if (Array.isArray(languages)) {
    for (const tag of languages) {
      const l = canonical(tag)
      if (l) return l
    }
  }
  return 'en'
}

function browserLanguages() {
  try {
    return typeof navigator !== 'undefined' ? navigator.languages : undefined
  } catch {
    return undefined
  }
}

let current = $state(resolveLocale('', browserLanguages()))

export function setLocale(l) {
  if (LOCALES.includes(l)) current = l
}

let applied // the last installation setting applied (undefined = none yet)

// applyLanguage makes the installation setting ('' = automatic) the locale of
// the page. The same setting twice does nothing, so status polls are cheap.
export function applyLanguage(setting) {
  if (setting === applied) return
  applied = setting
  previewLanguage(setting)
}

// previewLanguage shows a setting that is not saved yet (the assistant). It
// leaves the last applied setting alone, and applyLanguage skips a setting
// equal to it, so a later poll with the unchanged saved setting does not
// revert the preview: it lasts until a different setting is applied.
export function previewLanguage(setting) {
  setLocale(resolveLocale(setting, browserLanguages()))
  try {
    if (typeof document !== 'undefined') document.documentElement.lang = locale()
  } catch {
    // no document to update
  }
}

export function locale() {
  return current
}

export function numberLocale() {
  return NUMBER_LOCALES[current]
}

export function formatNumber(n, options) {
  return new Intl.NumberFormat(numberLocale(), options).format(n)
}

const pluralRules = new Map()

function pluralFor(l, n) {
  if (!pluralRules.has(l)) pluralRules.set(l, new Intl.PluralRules(l))
  return pluralRules.get(l).select(n)
}

const isPlural = (v) => v !== null && typeof v === 'object' && 'other' in v

function lookup(table, key) {
  let node = table
  for (const part of key.split('.')) {
    if (node === null || typeof node !== 'object' || !(part in node)) return undefined
    node = node[part]
  }
  return typeof node === 'string' || isPlural(node) ? node : undefined
}

function find(key) {
  return lookup(messages[current] ?? {}, key) ?? lookup(messages.en ?? {}, key)
}

export function has(key) {
  return find(key) !== undefined
}

export function t(key, params) {
  let value = find(key)
  if (value === undefined) return key
  if (typeof value !== 'string') {
    let form = 'other'
    if (params && typeof params === 'object' && typeof params.n === 'number') {
      form = pluralFor(current, params.n)
    }
    value = typeof value[form] === 'string' ? value[form] : value.other
  }
  if (params === null || typeof params !== 'object') return value
  return value.replace(/\{(\w+)\}/g, (whole, name) => {
    if (!Object.hasOwn(params, name)) return whole
    const v = params[name]
    return name === 'n' && typeof v === 'number' ? formatNumber(v) : String(v)
  })
}

export function msg(key, params) {
  return { key, params }
}

// text renders a plain string as is and a msg() object in the current locale.
export function text(m) {
  return typeof m === 'string' ? m : t(m.key, m.params)
}
