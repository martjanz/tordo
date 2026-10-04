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
      { term: 'Modo escucha', href: '/ajustes', text: 'Una opción de Ajustes que oculta Revisar y la curaduría y bloquea identificaciones, escaneos y cambios de géneros. Los ♥, las ★, las listas y las estaciones siguen funcionando. Es útil para dejar la música a otras personas.' },
      { term: 'Rockola', href: '/ajustes', text: 'En la compu que corre Tordo, activá «Usar el audio de esta compu» en Ajustes → Reproducción y dejá esa pestaña abierta: suena ahí. Desde un teléfono de la red, el botón 🎵 Rockola de la barra de arriba hace que el reproductor maneje la compu en lugar del teléfono. Si recargás esa pestaña, un clic en «Reactivar audio» vuelve a dar sonido.' },
      { term: 'Anfitriones e invitados', href: '/ajustes#red', text: 'La compu siempre es anfitriona; los demás dispositivos son invitados salvo que los marques como «Anfitrión de la rockola» en Ajustes → Red local. Los anfitriones controlan todo. Los invitados solo aportan: ▶ agrega a la cola (y empieza a sonar si la cola estaba vacía o terminada), pueden pausar y quitar solo lo que agregaron.' },
    ],
  },
  {
    id: 'como-funciona',
    title: 'Cómo funciona: escaneo e identificación',
    items: [
      { term: 'Cuándo escanea', text: 'Al abrir Tordo y cada vez que cambiás las carpetas de música o las carpetas a ignorar en Ajustes. El escaneo es incremental: solo lee los archivos nuevos o modificados (por tamaño y fecha), así que los siguientes son mucho más rápidos que el primero. Mientras tanto la barra de arriba muestra «Escaneando» y la música aparece a medida que se encuentra.' },
      { term: '1. Recorrido', text: 'Tordo recorre las carpetas buscando audio (MP3, FLAC, M4A, Ogg, Opus, WMA, WAV, AIFF, APE, WavPack), imágenes de tapa, letras y listas M3U. Se saltea las carpetas a ignorar y las de otras aplicaciones (Ableton, djay, Apple Music, descargas a medias de Soulseek). Si un disco no está conectado, lo omite y conserva lo que ya conocía; un archivo que ya no está se marca como ausente, no se olvida.' },
      { term: '2. Lectura', text: 'De cada archivo nuevo lee los tags (título, artista, álbum, número de pista, año, géneros, tapa incluida), los datos técnicos (formato, duración, calidad) y calcula una huella del audio. La huella ignora los tags: un archivo movido, renombrado o re-etiquetado sigue siendo el mismo, y dos copias idénticas se reconocen como duplicados. Si ffprobe está instalado, lee también los formatos que Tordo no lee por su cuenta. Un archivo ilegible se reintenta en el próximo escaneo.' },
      { term: '3. Álbumes', text: 'Arma los álbumes con los tags y, cuando faltan, con los nombres de carpetas y archivos («Artista - Álbum (1985)», «CD1»). Los discos de un mismo álbum en subcarpetas se juntan; unas pocas pistas con un tag de álbum distinto se suman al álbum de su carpeta; una carpeta de temas sueltos de distintos álbumes queda como una selección con el nombre de la carpeta. Elige la tapa (cover, folder o front, si no la más grande de la carpeta) y detecta las colecciones: carpetas con varios álbumes de distintos artistas.' },
      { term: '4. Identificación', text: 'Al terminar cada escaneo, Tordo busca cada álbum en MusicBrainz, de a una consulta por segundo como pide MusicBrainz (con miles de álbumes la primera vez lleva horas; si se corta, sigue donde quedó). La barra de arriba muestra «Identificando» y cuántos lleva. Si los tags ya traen el identificador de MusicBrainz, lo usa. Si no, busca por título y artista (también como los nombra la carpeta y sin notas entre paréntesis) y puntúa cada candidato: título 30 %, duraciones de las pistas 25 %, artista 20 %, cantidad de pistas 15 % y año 10 %. Con 80 % o más lo identifica solo; si no, queda en Revisar → Sin identificar con hasta cinco candidatos para elegir.' },
      { term: '5. Datos que agrega', text: 'Un álbum identificado muestra los títulos de MusicBrainz (también de cada pista), el año original, el país del artista y, si la carpeta no tiene tapa, la del Cover Art Archive (se guarda en cache/). Mientras busca estos datos la barra de arriba muestra «Completando datos»; cuando termina, «Al día». Los géneros y estilos salen de Discogs: por los enlaces que tiene MusicBrainz y, con un token de Discogs en Ajustes, también buscando. Sin Discogs, de MusicBrainz; sin identificación, de los tags. Los géneros de los tags que Tordo no reconoce quedan en Revisar → Géneros por mapear.' },
      { term: 'Tus correcciones', text: 'Lo que elegís a mano (en Sin identificar o con «Corregir identificación» en la página del álbum) se guarda por la huella de cada pista: sobrevive si movés, renombrás o re-etiquetás los archivos, y la identificación automática nunca lo pisa. Nada de esto toca tus archivos: los tags no se reescriben.' },
      { term: 'Sin conexión', text: 'El escaneo no usa la red. Si no hay conexión, la identificación se pausa (la barra muestra «Sin conexión con MusicBrainz») y se reintenta sola, cada vez más espaciado; todo lo demás funciona igual. Lo ya identificado no se vuelve a buscar.' },
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
