// Content of the help page (Ayuda.svelte) and the key that opens it. Pure.

// isHelpKey tells the key that opens the help from anywhere: "?", unless the
// user is typing or holds Ctrl, ⌘ or Alt.
export function isHelpKey(event) {
  if (event.key !== '?' || event.ctrlKey || event.metaKey || event.altKey) return false
  const tag = event.target?.tagName
  return !(['INPUT', 'TEXTAREA', 'SELECT'].includes(tag) || event.target?.isContentEditable)
}

// Each section is { id, title, intro?, items }. An item is { term?, keys?,
// text, href?, blockedInListen? }: keys are shown as <kbd>, href is an
// internal link, blockedInListen marks what listen mode hides.
export const SECTIONS = [
  {
    id: 'atajos',
    title: 'Atajos y comandos',
    items: [
      { keys: ['Ctrl+K', '⌘K'], text: 'Abre la búsqueda en cualquier página. Flechas y Enter eligen un resultado; Esc la cierra.' },
      { keys: ['Espacio'], text: 'Reproduce o pausa (salvo que estés escribiendo o sobre un botón).' },
      { keys: ['/'], text: 'En Carpetas, salta al filtro del árbol.' },
      { keys: ['Esc'], text: 'Cierra los menús y paneles abiertos.' },
      { keys: ['?'], text: 'Abre esta ayuda.' },
      { term: 'Reproductor', text: 'Aleatorio, anterior, reproducir o pausar, siguiente y repetir (no, toda la cola o esta pista).' },
      { term: 'Posición y volumen', text: 'Una barra para adelantar o retroceder y otra para el volumen, con botón de silencio. Los formatos que se convierten al vuelo no se pueden adelantar.' },
      { term: 'Cola', text: 'El botón de la cola muestra lo que va a sonar; ahí podés quitar pistas y reordenarlas arrastrándolas (o con Alt + flechas).' },
      { term: 'Puntaje', text: 'Con el foco en las estrellas, las flechas cambian el puntaje.' },
    ],
  },
  {
    id: 'secciones',
    title: 'Secciones',
    items: [
      { term: 'Inicio', href: '/', text: 'Filas de álbumes para retomar: agregados recientemente, tus estaciones, seguir escuchando, tus canciones favoritas, tus álbumes preferidos y sugerencias que cambian en cada visita.' },
      { term: 'Explorar', href: '/explorar', text: 'Toda la colección por álbumes, artistas o pistas, con filtros combinables (década, país, género, formato, ♥, ★…) y orden.' },
      { term: 'Carpetas', href: '/carpetas', text: 'La colección tal como está en el disco, con un árbol de carpetas.' },
      { term: 'Listas', href: '/listas', text: 'Tus listas de reproducción y tus ♥.' },
      { term: 'Revisar', href: '/revisar', blockedInListen: true, text: 'Mantenimiento: música sin identificar, duplicados, géneros por mapear y formatos a convertir.' },
      { term: 'Ajustes', href: '/ajustes', text: 'Carpetas de música, token de Discogs, volumen, audio de esta compu, modo escucha y red local.' },
    ],
  },
  {
    id: 'conceptos',
    title: 'Conceptos',
    items: [
      { term: 'Estaciones', text: 'Una cola sin fin con música parecida a lo que elegiste (un artista, un país, un género, una carpeta, tus ♥ y ✦…). Tordo la alimenta de a diez pistas, varía los artistas y da más peso a lo que te gusta. Podés guardar una estación y la ves en Inicio.' },
      { term: '♥, ✦ y ★', text: 'El ♥ marca tus canciones favoritas (y los artistas y colecciones que te gustan); el ✦ marca álbumes preferidos, una lista corta a la que volver; la ★ puntúa pistas y álbumes de 1 a 5. Sirven para filtrar en Explorar (en Álbumes, «Preferidos»), para armar estaciones y pesan en ellas.' },
      { term: 'Duplicados', href: '/revisar/duplicados', blockedInListen: true, text: 'Tordo los muestra, nunca los borra. Exactas: el mismo audio en dos o más rutas (muestra el espacio recuperable). Formatos: la misma grabación en varios formatos dentro de un álbum, con la marca MEJOR en la de mayor calidad. Sospechas: mismo artista y título en álbumes distintos; es informativo y suele ser legítimo (un tema del disco y de un compilado).' },
      { term: 'Identificación', text: 'Después de cada escaneo, Tordo busca cada álbum en MusicBrainz y lo completa con títulos, año original y país del artista. Los géneros y estilos salen de Discogs (con un token en Ajustes; sin él, de MusicBrainz y de los tags). Lo que no puede resolver solo queda en Revisar, en Sin identificar, para elegir a mano. Sin conexión todo lo demás sigue funcionando y se reintenta solo.' },
      { term: 'Modo escucha', href: '/ajustes', text: 'Una opción de Ajustes que oculta Revisar y la curaduría y bloquea identificaciones, escaneos y cambios de géneros. Los ♥, las ★, las listas y las estaciones siguen funcionando. Es útil para dejar la música a otras personas.' },
      { term: 'Rockola', href: '/ajustes', text: 'En la compu que corre Tordo, activá «Usar el audio de esta compu» en Ajustes → Reproducción y dejá esa pestaña abierta: suena ahí. Desde un teléfono de la red, el botón 🎵 Rockola de la barra de arriba hace que el reproductor maneje la compu en lugar del teléfono. Si recargás esa pestaña, un clic en «Reactivar audio» vuelve a dar sonido.' },
      { term: 'Anfitriones e invitados', href: '/ajustes#red', text: 'La compu siempre es anfitriona; los demás dispositivos son invitados salvo que los marques como «Anfitrión de la rockola» en Ajustes → Red local. Los anfitriones controlan todo. Los invitados solo aportan: ▶ agrega a la cola (y empieza a sonar si la cola estaba vacía o terminada), pueden pausar y quitar solo lo que agregaron.' },
    ],
  },
  {
    id: 'pautas',
    title: 'Pautas de uso',
    items: [
      { term: 'Tus archivos', text: 'Tordo nunca escribe, mueve, renombra ni borra tus archivos de audio. Solo escribe en su propia carpeta: tordo.db (el catálogo), config.json (los ajustes) y cache/.' },
      { term: 'Portabilidad', text: 'El catálogo guarda rutas relativas a la carpeta de Tordo, así que podés mover la carpeta entera (por ejemplo a otro disco) y todo sigue andando.' },
      { term: 'Desde otros dispositivos', href: '/ajustes#red', text: 'Si activás la red local en Ajustes, podés entrar desde un teléfono o una tablet de la misma red. Cada dispositivo pide el PIN que elegiste; cambiarlo cierra todas las sesiones. Para que suene en los parlantes de la compu, usá la Rockola.' },
      { term: 'Ajustes', text: 'Solo se cambian desde la computadora que corre Tordo; desde otro dispositivo se pueden escuchar y consultar, pero no configurar.' },
    ],
  },
]
