// deviceName makes a short, readable name of the device that plays, from
// its user agent: "iPhone · Safari", "Mac · Firefox".
export function deviceName(ua) {
  if (!ua) return 'Navegador'
  let os = ''
  if (/iPhone/.test(ua)) os = 'iPhone'
  else if (/iPad/.test(ua)) os = 'iPad'
  else if (/Android/.test(ua)) os = 'Android'
  else if (/Windows/.test(ua)) os = 'Windows'
  else if (/Macintosh|Mac OS X/.test(ua)) os = 'Mac'
  else if (/Linux|X11/.test(ua)) os = 'Linux'
  let browser = ''
  if (/Edg(e|A|iOS)?\//.test(ua)) browser = 'Edge'
  else if (/OPR\/|Opera/.test(ua)) browser = 'Opera'
  else if (/Firefox\/|FxiOS\//.test(ua)) browser = 'Firefox'
  else if (/Chrome\/|CriOS\//.test(ua)) browser = 'Chrome'
  else if (/Safari\//.test(ua)) browser = 'Safari'
  if (os && browser) return `${os} · ${browser}`
  return os || browser || 'Navegador'
}
