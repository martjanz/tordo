# Tordo

[English](README.md) · Español · [Português](README.pt-BR.md)

Un catálogo y reproductor local y portable para una colección de música repartida en unas
pocas carpetas de un mismo disco. **Estado: M6 (modo Rockola: usar la computadora como parlante)** — escaneo,
lectura nativa de tags, huellas de audio, álbumes, carpetas y colecciones, identificación
automática con MusicBrainz y Discogs, una API JSON en localhost y la interfaz web
con reproductor, me gusta, puntajes, listas de reproducción, conteo de reproducciones, estaciones, Inicio,
estadísticas de escucha, normalización de volumen, detección de duplicados, acceso desde la red con PIN y modo Rockola
para compartir la reproducción entre dispositivos, en tres idiomas (español, inglés y
portugués de Brasil); ver `docs/superpowers/specs/2026-10-01-tordo-design.md` (en español).

- **Solo lectura** sobre tu música: nunca mueve, renombra, retaguea ni borra archivos.
- **Portable:** un ejecutable por sistema operativo; el catálogo (`tordo.db`) y
  `config.json` viven al lado, con rutas relativas a él.
- **Local o LAN:** por defecto corre en `127.0.0.1`; opcionalmente escucha en la red de tu casa (`0.0.0.0`) con autenticación por PIN.

## Capturas de pantalla

Las capturas muestran la interfaz en español.

![Explorar: la grilla de álbumes con filtros por faceta (década, artista, país, género, colección, puntaje, escucha)](docs/screenshots/explorar.webp)

<table>
  <tr>
    <td width="50%"><img src="docs/screenshots/inicio.png" alt="Inicio: álbumes agregados recientemente y tus estaciones"><br><sub><b>Inicio</b>: álbumes agregados recientemente y tus estaciones</sub></td>
    <td width="50%"><img src="docs/screenshots/album.png" alt="Página de un álbum con pistas, formato, puntaje e identificación"><br><sub><b>Álbum</b>: pistas, formato, puntaje, reproducir, aleatorio, estación, cola y lista</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/screenshots/now-playing.png" alt="Sonando: el reproductor a pantalla completa"><br><sub><b>Sonando</b>: el reproductor a pantalla completa</sub></td>
    <td width="50%"><img src="docs/screenshots/carpetas.png" alt="Carpetas: recorré las carpetas de música con cantidad de pistas, duración y tamaño"><br><sub><b>Carpetas</b>: recorré las carpetas con cantidad de pistas, duración, formato y tamaño</sub></td>
  </tr>
  <tr>
    <td width="50%"><img src="docs/screenshots/scan-status.png" alt="Panel de estado del escaneo con pasos, progreso y tiempo restante"><br><sub><b>Estado del escaneo</b>: cada paso, el progreso y el tiempo restante; los álbumes aparecen mientras se lee</sub></td>
    <td width="50%"></td>
  </tr>
</table>

## Uso

Poné la carpeta `tordo/` en el disco, al lado de tus carpetas de música, y ejecutá el
binario de tu sistema. La primera vez abre Bienvenida (el asistente) en el
navegador, que te pide las carpetas de música (con `-no-browser` no se abre; la dirección
está en el log). Elegí las carpetas, un token de Discogs opcional y la normalización,
y guardá para crear `config.json` y escanear la biblioteca. Si ya existe un `config.json`,
el asistente no aparece. El modo consulta (carpetas de solo lectura o un binario `tordo`
sin permiso de escritura) muestra en cambio la página de ajustes.

```json
{
  "roots": ["../musica", "../musica-ordenar"],
  "ignore": ["Podcasts", "Soulseek/incomplete"],
  "discogsToken": "",
  "normalize": true,
  "listenMode": false,
  "language": "",
  "rescanMinutes": 60,
  "lan": {
    "enabled": false,
    "port": 4747,
    "pinHash": ""
  }
}
```

`listenMode` (por defecto `false`) es el modo escucha: el servidor responde 403 en las rutas
de curaduría (escaneo, identificación, colecciones, alias de géneros); se activa
en Ajustes, solo desde la computadora que corre Tordo.

`language` elige el idioma de la interfaz: `""` (por defecto) es automático y sigue el
idioma del navegador, o uno de `"es"`, `"en"` y `"pt-BR"`. Si el idioma del navegador no es
ninguno de esos, la interfaz queda en inglés. El idioma elegido en Ajustes (solo desde la
computadora que corre Tordo) vale para todos los navegadores que usan esta instalación.

`rescanMinutes` (por defecto `60`) es cada cuánto se revisan las carpetas en busca de cambios; `0`
lo desactiva. El máximo es 10080 (una semana).

`discogsToken` es opcional; sin él, los géneros salen de MusicBrainz, de sus
enlaces a Discogs y de los tags; conseguí uno en https://www.discogs.com/settings/developers.

`normalize` (verdadero si falta) activa la normalización de volumen; los cambios se aplican
enseguida en Ajustes, sin reiniciar.

`ignore` suma carpetas a saltear: un nombre coincide en cualquier lugar, una ruta con `/`
coincide desde la raíz. Las carpetas de otras aplicaciones (Ableton, djay, Audio Music Apps,
`downloading` de Soulseek, bibliotecas de Apple Music) siempre se saltean.
`ffprobe`, si está en el `PATH`, lee los formatos que los lectores incorporados no
leen (APE, WavPack, AAC crudo…).

### Ajustes

La página de ajustes permite administrar las carpetas de música, los patrones a ignorar, cada cuánto buscar
cambios, el token de Discogs, la normalización de volumen, el idioma de la interfaz y el acceso desde la red local. Cada ajuste muestra qué hace
al cambiarlo: escanear la biblioteca (cambiaron las carpetas o los patrones), actualizar
los géneros (token de Discogs nuevo) o aplicarse enseguida (normalización).

El campo **token de Discogs** verifica el token contra el endpoint `/oauth/identity` de Discogs
cuando hacés clic en "Verificar"; el servidor lo acepta igual si la verificación
falla (sin red). Cambiar el token dispara una actualización de la identificación.

La **normalización de volumen** se puede activar o desactivar en vivo, sin reiniciar Tordo; el
reproductor y los filtros de Explorar se actualizan enseguida.

Los ajustes solo se pueden cambiar desde la computadora que corre Tordo. Desde otros dispositivos
de la red local aparece el mensaje "Los ajustes solo se cambian en la computadora
que corre Tordo", y solo podés cerrar tu sesión.

### Explorar y escuchar

Explorar lista álbumes, artistas o pistas de todo el catálogo, filtrados por
década y año, artista, país, género y estilo, colección, carpeta,
calidad, formato y estado (sin identificar, no reproducible, con duplicados); también
filtra por Me gusta (♥; se llama Preferidos en la pestaña Álbumes, donde filtra por el marcador),
Puntaje (★ N o más) y Escucha (nunca escuchado, o sin escuchar hace un año), y ordena por
Más escuchado. Los filtros viven en la URL. Las páginas de artista muestran sus álbumes por año y sus
pistas en compilados. La búsqueda (Ctrl+K / ⌘K) encuentra artistas, álbumes, pistas y
carpetas mientras escribís, sin importar mayúsculas ni acentos. Carpetas es un explorador de archivos:
un árbol de carpetas a la izquierda (las carpetas que contienen una sola carpeta se unen en una
fila, `Soulseek › complete`; filtralo por nombre, o apretá `/`, y movete con las
flechas y Enter) y, a la derecha, las subcarpetas de la carpeta con su
tamaño relativo y los archivos que contiene directamente, ordenables por columna. En un celular
se convierte en una lista por carpeta. A una carpeta detectada por error como colección se le puede
decir "No es una colección". Abrí un álbum
y reproducilo, mezclá una carpeta, agregá pistas a la cola. La cola
y el volumen se guardan en el navegador, por dispositivo. MP3, FLAC, AAC, Opus,
Vorbis y WAV se reproducen tal cual, con búsqueda dentro de la pista; WMA, ALAC, AIFF, APE y
WavPack necesitan `ffmpeg` en el `PATH` y se convierten al vuelo (los lossless a
FLAC, los lossy a MP3), sin poder adelantar. Las carátulas salen de la imagen de la carpeta
del álbum, de las imágenes incrustadas en los archivos o del Cover Art Archive; las miniaturas se
guardan en `cache/thumbs/`.

#### Rockola

Usá la computadora que corre Tordo como parlante desde tu celular u otro navegador: abrí
Ajustes → Reproducción en la computadora y activá "Usar el audio de esta compu para Tordo"
(esta opción aparece solo en la computadora). La barra superior muestra "🔊 Audio de esta compu"
mientras esa pestaña tiene el audio. Mantené esa pestaña abierta; cuando la recargás, hacé clic en "Reactivar audio"
una vez (los navegadores bloquean el sonido hasta que se hace clic en la página). El ajuste de audio se recuerda
por pestaña: cuando esa pestaña se recarga, se reconecta y pide un clic; si otra pestaña
toma el control, la primera muestra "Otro navegador tomó el audio de esta compu."

Desde un celular u otro navegador con acceso a la LAN, la barra superior muestra un botón "🎵 Rockola"
(visible solo mientras el audio de la computadora está activo). Tocalo para pasar el reproductor a la
computadora: la barra muestra "Rockola · en la compu" con la pista, la cola y los
controles de la computadora; la reproducción de tu celular se pausa y su cola se conserva para después. No se recuerda
al recargar.

**Anfitriones e invitados:** La computadora que corre Tordo siempre es
anfitriona. Los demás dispositivos de la LAN son invitados por defecto, pero se pueden marcar como
anfitriones en Ajustes → Red local → Dispositivos. Los anfitriones tienen control total: ▶
reemplaza la cola, saltar, adelantar, volumen, aleatorio/repetir, reordenar, vaciar y estaciones.
Los invitados solo pueden agregar: ▶ agrega al final de la cola y empieza a reproducir solo cuando
la cola está vacía o terminó; al tocar una sola canción se agrega esa canción; ⤮ agrega las pistas mezcladas;
"A la cola" y "Reproducir a continuación" funcionan como siempre. Los invitados pueden pausar/reanudar y quitar solo las pistas
que agregaron. El servidor hace cumplir estos límites.

#### Me gusta y puntajes

Marcá pistas, artistas y colecciones con ♥ (Me gusta; las pistas con me gusta son las
*Canciones favoritas*). Marcá álbumes con el marcador (*Preferido*): los álbumes son una lista corta para
volver a ellos, no favoritos. Los álbumes preferidos se guardan como marcas de álbum (`kind: album`
en la API). Puntuá pistas y
álbumes con ★ (1–5 estrellas). Estas marcas se muestran en las filas de pistas (junto a un menú
con puntaje, "Reproducir a continuación", "A la cola" y "Agregar a una lista…"), en los encabezados
de álbum, las páginas de artista, la barra del reproductor, las tarjetas de colección y las páginas de carpeta.
Se guardan por huella de audio, así que acompañan a los archivos que se mueven, se renombran o
se retaguean.

#### Conteo de reproducciones

Una reproducción se cuenta una vez por pasada cuando escuchás al menos 4 minutos o la mitad de la
pista, lo que sea menos. Adelantar no suma tiempo de escucha. Las
reproducciones se registran con el nombre del dispositivo; recorrelas en Explorar.

#### Listas de reproducción

Creá una lista desde la página Listas o con "+ Lista" en cualquier álbum, carpeta,
artista, colección, o desde el menú ⋯ de una pista. Reordená las entradas arrastrando
el asa (con mouse o tacto) o con Alt+↑/↓. Renombrá, eliminá la lista o quitá entradas
sueltas; las pistas cuyos archivos desaparecieron siguen en la lista como "no disponible" y se
saltean al reproducir. Máximo 5000 entradas por lista.

#### Página Listas

Mirá tus listas de reproducción y tu curaduría: Listas de reproducción, Canciones favoritas (pistas con ♥, una
lista automática), Álbumes preferidos (álbumes con marcador, primero los marcados más recientemente),
tus Me gusta (Artistas · Colecciones) y tus Colecciones.

#### Estaciones

Una estación reproduce una secuencia de álbumes o pistas sacados de una fuente, sin repetir
lo que acaba de poner en la cola, con reglas de variedad para evitar lo escuchado hace poco. Su
fuente es una de estas:
- Una **consulta de Explorar** con dos vistas: `tracks` (las pistas que coinciden) o `albums`
  (todas las pistas de los álbumes que coinciden; para artistas corresponde a la pestaña Artistas de Explorar).
- Una **carpeta** (recursiva; las colecciones también son carpetas).
- Una **lista de reproducción** (las entradas que todavía se pueden reproducir).
- **Me gusta** (♥): todas las pistas con me gusta, todas las pistas de los álbumes con me gusta, todas las pistas de
  los artistas con me gusta y todas las pistas de las colecciones con me gusta (un álbum Preferido con marcador cuenta como un
  álbum con me gusta).

Los **modos** son `tracks` (pista por pista) o `albums` (álbumes enteros de a uno; una sola
pista cuenta como un álbum de uno).

**Reglas** (con sus valores por defecto):
- `artistGap` (0–20, por defecto 3): ningún artista de nuevo dentro de las últimas N pistas.
- `avoidDays` (0–365, por defecto 7): saltear las pistas reproducidas en los últimos N días (un álbum:
  cualquiera de sus pistas).
- La estación nunca repite una pista que ya puso en la cola (el navegador envía hasta las
  últimas 100 reproducidas).

Cuando ninguna pista cumple todas las reglas, estas se **relajan de a una** en este orden: separación entre artistas,
días a evitar, ya puesta en la cola (pero nunca la que acaba de sonar, si hay otra).
El lote queda vacío (y la estación se detiene) solo cuando la fuente no tiene nada reproducible;
de lo contrario, al relajarse las reglas las pistas pueden repetirse.

El navegador trae hasta 10 pistas por vez y pide más cuando quedan menos de 3.
La cola conserva solo las últimas 100 pistas reproducidas, así que la estación nunca se
queda sin música. Las estaciones guardadas se almacenan por nombre y viven en Tus estaciones, en Inicio.

**Pesos** (para sortear las pistas):
- Base: 1.
- ×3 cuando la pista o su álbum tiene ♥.
- ×2 cuando su artista tiene ♥.
- × {1: 0.25, 2: 0.5, 3: 1, 4: 2, 5: 3} según el mayor puntaje (★) entre el de la pista y el del álbum.

#### Inicio

La primera página, que muestra:
- Filas fijas: **Agregados recientemente** (los álbumes más nuevos), **Tus estaciones** (las
  estaciones guardadas, más dos incluidas: "Toda la colección" y "Tus me gusta" cuando hay algo
  con me gusta), **Seguir escuchando** (álbumes por su última reproducción), **Tus canciones favoritas**
  (álbumes con pistas ♥, por el último me gusta) y **Álbumes preferidos**
  (álbumes con marcador, primero los más recientes).
- **Tres filas rotativas**, sorteadas con una semilla a partir de las facetas del catálogo (década,
  país, género, estilo, colecciones, álbumes nunca escuchados, álbumes sin escuchar hace un año,
  álbumes al azar). La mayoría de las filas necesita al menos 4 álbumes para sortearse; la de álbumes al azar
  necesita solo 1. Las filas con pocos elementos se ocultan.

El botón ↻ sortea una semilla nueva y vuelve a armar las filas rotativas; cada visita nueva sortea una
semilla nueva. Solo las recargas después de un escaneo o de una identificación conservan la misma semilla. Cada fila enlaza
a Explorar con su filtro y (salvo las filas fijas no curadas) tiene un botón ▶ para iniciar
su estación.

#### Historial de escucha (estadísticas)

Tu historial de escucha por período: **Última semana** (7 días), **Último mes** (30 días), **Último año**
(365 días), **Siempre**. Rankings de las pistas, álbumes, artistas,
países y géneros más reproducidos; totales (reproducciones, tiempo escuchado, pistas distintas); y
cantidades de **álbumes** nunca escuchados y sin escuchar hace un año en todo el catálogo (con enlace a
Explorar).

Una reproducción se atribuye a **una** pista: el archivo desde el que se reprodujo si está presente
con el mismo audio; si no, el archivo presente de menor número con ese audio. Cada
género de un álbum cuenta como una reproducción.

#### Faceta Escucha

Filtrá álbumes o pistas por `listened`: **never** (nunca escuchado) o **year-ago**
(escuchado, pero la última reproducción tiene más de 365 días). Un álbum es de nunca escuchado solo
si ninguna de sus pistas se reprodujo.

#### Páginas de artista

Muestran las 5 pistas más reproducidas del artista (de todos los tiempos) arriba de sus álbumes.

Las URL, los parámetros de consulta, los nombres de facetas y las rutas de la API están en inglés sea cual sea el idioma
de la interfaz (`/explore`, `/folders`, `/settings`, `/help`…; `?view=albums&order=year`,
`decade`, `listened=never`…).

| Método y ruta | Qué devuelve |
|---|---|
| `GET /api/status` | progreso del escaneo, cantidades del catálogo (las últimas conocidas cuando el catálogo está ocupado; ninguna antes del primero), `preparing` mientras se arma el primer catálogo, estado de la normalización, indicador `setupPending`, `language` |
| `POST /api/scan` | inicia un escaneo (`Content-Type: application/json`) |
| `GET /api/albums?folder=` | álbumes, todos o dentro de una carpeta |
| `GET /api/albums/{id}` | un álbum con sus pistas (con `sizeBytes` por pista), versiones (otros álbumes que comparten grupos de duplicados) y datos técnicos |
| `GET /api/folders?path=` | una carpeta: subcarpetas (con tamaño), álbumes, pistas sueltas y los archivos que contiene directamente (sin `path`: las raíces) |
| `GET /api/folders/tree?path=` | un nivel del árbol de Carpetas: subcarpetas, con las cadenas de paso unidas |
| `GET /api/folders/tree/search?q=` | carpetas por nombre, con las filas del árbol que llevan a ellas |
| `GET /api/collections` | carpetas detectadas como colecciones, con sus álbumes en orden |
| `PUT /api/folders/collection` | `{"path", "collection": false}`: no es una colección (`true` la recupera) |
| `GET /api/explore?view=&<facets>&order=&dir=&from=&limit=` | una página de Explorar (álbumes, artistas o pistas) con las cantidades de las facetas |
| `GET /api/artists/{key}` | un artista: álbumes por año, apariciones en álbumes de otros y sus 5 pistas más reproducidas en `top` |
| `GET /api/search?q=&limit=` | artistas, álbumes, pistas y carpetas que coinciden con `q` |
| `GET /api/genres` | la taxonomía de géneros y estilos |
| `GET /api/genres/unmapped` | géneros crudos de los tags que ningún mapeo conoce (Géneros por mapear) |
| `PUT /api/genres/aliases`, `DELETE /api/genres/aliases?raw=` | mapear un género crudo (o ignorarlo), u olvidar el mapeo |
| `GET /api/review/unidentified` | álbumes que esperan revisión de identificación |
| `PUT /api/albums/{id}/identification` | elegir la identificación de un álbum (`DELETE` vuelve a la automática) |
| `GET /api/review/duplicates?kind=` | grupos de duplicados; `kind` es `exact`, `formats` o `suspected` (por defecto `exact` si falta o es desconocido); devuelve `{counts, recoverable, groups}` |
| `GET /api/review/formats` | pistas que necesitan conversión, por formato |
| `GET /api/marks` | me gusta y puntajes de pistas y álbumes (por id), y artistas y colecciones con me gusta |
| `PUT /api/likes` | poner o quitar un me gusta (♥) (`{kind, id|key, liked}`; `kind` es track/album/artist/collection; `album` es el marcador Preferido) |
| `PUT /api/ratings` | poner o borrar el puntaje de una pista o álbum (`{kind, id, stars}`; `stars` 0–5, 0 lo borra) |
| `POST /api/plays` | registrar una reproducción de una pista (`{id, listenedMs, device}`); se ignora en modo consulta |
| `GET /api/liked` | las pistas con me gusta (♥), los álbumes preferidos (marcador), los artistas y colecciones con me gusta, primero los más recientes |
| `GET /api/gains?track=` | las ganancias de cada pista en dB (`{id: {trackGain, albumGain}}`, de los tags ReplayGain o del volumen medido); las pistas sin medir van primero en la cola de análisis; las pistas marcadas como "no medibles" (sin tag ReplayGain y con la medición fallida de forma permanente) no se vuelven a analizar |
| `GET /api/playlists` | las listas de reproducción del usuario |
| `POST /api/playlists` | crear una lista de reproducción nueva |
| `GET /api/playlists/{id}` | las entradas de una lista |
| `PUT /api/playlists/{id}` | renombrar una lista |
| `DELETE /api/playlists/{id}` | eliminar una lista |
| `POST /api/playlists/{id}/items` | agregar una entrada a una lista |
| `DELETE /api/playlists/{id}/items/{item}` | quitar una entrada |
| `PUT /api/playlists/{id}/order` | reordenar las entradas |
| `GET /api/stations` | estaciones guardadas, primero las actualizadas más recientemente |
| `POST /api/stations` | guardar una estación (cuerpo: `{name, source, mode, artistGap, avoidDays}`) |
| `PUT /api/stations/{id}` | renombrar o actualizar una estación |
| `DELETE /api/stations/{id}` | eliminar una estación guardada |
| `GET /api/station/next?def=<JSON>&recent=1,2,3` | el siguiente lote de pistas de la estación (al menos 10 pistas, posiblemente más en modo álbumes para incluir álbumes completos), con mime y ganancias; `recent` es una lista separada por comas de hasta 200 ids de pistas reproducidas hace poco; la respuesta incluye `station` (la definición de la estación tal como la aplicó el servidor) |
| `GET /api/home?seed=N` | Inicio: filas fijas, filas rotativas y estaciones guardadas; `seed` sortea las filas rotativas, si falta o es inválida se usa una al azar |
| `GET /api/stats?period=&limit=` | Historial de escucha: `period` es `week`/`month`/`year`/`all` (por defecto `month`); `limit` es el largo de cada ranking (por defecto 10, rango 1–100) |
| `GET /api/play?album=&folder=&artist=&track=&shuffle=1&playlist=&liked=1` | las pistas para poner en la cola, en orden (como máximo 5000) |
| `GET /stream/{trackId}` | el audio de una pista (`Range` para adelantar; `?transcode=1` la convierte) |
| `GET /api/covers/album/{id}?size=` | la imagen de un álbum, completa o como miniatura de 160/320/640 px |
| `GET /api/covers/track/{id}?size=` | la imagen incrustada en el archivo de una pista |
| `GET /api/config` | la configuración actual (los ajustes solo se pueden leer desde la computadora local) |
| `PUT /api/config` | guardar la configuración: raíces, patrones a ignorar, intervalo de revisión, token de Discogs, normalización, idioma, ajustes de la LAN (solo desde la computadora local) |
| `POST /api/config/root` | comprobar si una carpeta puede ser una raíz: devuelve su ruta canónica o un error |
| `POST /api/config/token` | comprobar si Discogs acepta un token: devuelve `{valid: true/false/null}` (null sin conexión) |
| `GET /api/lan` | información de la LAN: activada, puerto, escuchando y las URL para que otros dispositivos lleguen a Tordo |
| `GET /api/lan/qr?url=` | código QR en SVG de una URL |
| `GET /api/auth` | siempre 200: `{local, session, device, pinRequired, language}` (la página muestra la pantalla del PIN cuando `pinRequired` es verdadero; las demás rutas `/api/` y `/stream/` responden 401 sin sesión) |
| `POST /api/auth/login` | autenticarse con el PIN desde un dispositivo de la LAN: `{pin, device}` |
| `POST /api/auth/logout` | cerrar esta sesión |
| `GET /api/sessions` | listar las sesiones activas (solo desde la computadora local) |
| `DELETE /api/sessions/{id}` | cerrar una sesión por id (solo desde la computadora local; los dispositivos de la LAN solo pueden cerrar su propia sesión con `POST /api/auth/logout`) |

Los errores son JSON, `{"code", "message", "params"}`, con el mismo estado HTTP de siempre: `message` está en
inglés y `code` es un identificador estable (declarado en `internal/server/codes.go`) que la aplicación web
traduce.

## Cómo funcionan el escaneo y la identificación

La ayuda de la aplicación (Ayuda → *Cómo funciona: escaneo e identificación*) tiene una versión en lenguaje sencillo de esta sección.

### Cuándo corre un escaneo

Al iniciar, cada vez que cambian las carpetas de música o los patrones a ignorar en Ajustes, con
`POST /api/scan` y cada hora (`rescanMinutes`, contada desde el final del último escaneo).
El escaneo inicial espera a que los álbumes estén en pantalla: primero se muestra el catálogo guardado,
y recorrer las carpetas al mismo tiempo haría más lenta su lectura cuando ambos están en el
mismo disco.

La ronda programada recorre las carpetas y compara tamaños y fechas de modificación con el
catálogo, sin abrir ningún archivo y escribiendo solo las filas que difieren. Sin diferencias y
sin nada esperando para leerse, termina ahí: el estado no cambia, no se rearman los álbumes, la identificación no corre y las páginas no se
recargan. Si no, sigue como un escaneo normal y se muestra en la barra superior desde ese momento. También corre en modo escucha, como el
escaneo inicial: ese modo bloquea lo que las personas piden, no que el catálogo se mantenga al día. Un
disco externo dormido se despierta en cada ronda, así que un intervalo más largo o `0` lo evita.

Los escaneos son **incrementales**: solo se vuelven a leer los archivos nuevos y los que cambiaron de tamaño o de
fecha de modificación (también todos los archivos después de una actualización que cambia
el lector, y los archivos que fallaron cuando aparece `ffprobe` en el `PATH`). La barra superior muestra el
paso: `Buscando archivos · n`, `Leyendo archivos · n/m (p %)`, `Armando discos`; mientras se lee, el catálogo se reagrupa cada 30 s, así que los álbumes
aparecen en Inicio y Explorar a medida que avanza el escaneo (las páginas se recargan solas). El escaneo
nunca usa la red.

### 1. Recorrer (`internal/scan`)

Se recorre cada raíz y se registra cada archivo catalogado con su tamaño y fecha de modificación:
audio (`.mp3 .flac .m4a .aac .opus .ogg .oga .wma .wav .aif(f/c) .ape .wv`),
imágenes (`.jpg .jpeg .png .webp`), letras (`.lrc`) y listas de reproducción (`.m3u .m3u8`).
Las carpetas ignoradas (`ignore` en `config.json`, más las carpetas de otras aplicaciones) se saltean.

Los datos faltantes se tratan con cautela:
- una raíz que no está disponible (disco desconectado) se saltea y se conserva su catálogo;
- una raíz que desaparece a mitad del escaneo aborta el escaneo sin marcar nada como faltante;
- una raíz recorrida con errores de lectura se conserva, pero sus archivos no vistos no se marcan como faltantes;
- los archivos que ya no se ven bajo una raíz recorrida sin problemas se marcan como **faltantes**, no se borran
  del catálogo, así que vuelven como estaban cuando el disco regresa.

### 2. Leer (`internal/audiofile`, `internal/fingerprint`)

De cada archivo de audio pendiente, Tordo lee los tags (título, artistas, álbum, artista del álbum,
números de pista y de disco, año, géneros, indicador de compilado, ids de MusicBrainz, carátula incrustada),
los datos técnicos (formato, códec, duración, tasa de bits, frecuencia de muestreo, profundidad de bits, canales,
ReplayGain) y calcula una **huella de audio**: SHA-256 de los primeros y últimos
64 KiB de los datos de audio más su largo, sin los tags ni las imágenes incrustadas
(tampoco los encabezados de página de Ogg). Mover, renombrar o retaguear un archivo conserva su huella;
las copias idénticas bit a bit comparten una. Los lectores nativos se ocupan de los formatos comunes; `ffprobe`
cubre el resto. Los resultados se guardan en lotes de 100, así que un escaneo interrumpido conserva
lo que hizo. Los errores de entrada/salida se reintentan en el próximo escaneo; los archivos leídos pero no entendidos se
guardan con su error.

### 3. Agrupar (`internal/grouping`)

Después de leer, los álbumes, pistas, carpetas y colecciones se rearman a partir de todo el
catálogo (una función pura de los datos guardados, igual en cada corrida). Un resultado igual al
guardado (su hash se conserva en `grouping_stamp`) no escribe nada, así que un escaneo de una
biblioteca sin cambios deja el catálogo y los títulos identificados como estaban:

- Los **campos de la pista** salen de los tags y, si faltan, del nombre del archivo
  (`01 - Artista - Título.mp3`) y del nombre de la carpeta (`Artista - Álbum (1985)`).
  Los nombres de sitios de descarga y los códigos de archivo se descartan de los tags.
- Los **álbumes** se identifican por carpeta + título de álbum normalizado. Las carpetas de disco (`CD1`,
  `Disc 2`) se suman al álbum de su carpeta madre. En una carpeta cuyos tags no coinciden, los grupos de una
  o dos pistas sueltas se suman al álbum principal de la carpeta cuando este tiene dos tercios de las
  pistas; una carpeta formada en su mayoría por álbumes de una o dos pistas se vuelve una única
  **selección** con el nombre de la carpeta. Las pistas sin tag de álbum en una carpeta con
  al menos dos pistas toman el nombre de álbum de la carpeta; el resto son pistas sueltas.
- **Artista del álbum:** el tag de artista del álbum; *Varios artistas* para tags, carpetas
  o varios artistas de pista del estilo "Various"; si no, el artista de la carpeta, el único artista
  de pista, o la carpeta madre cuando se llama como un artista conocido.
- **Carátula:** una imagen llamada `cover`, `folder` o `front`; después una cuyo nombre
  las mencione; después la imagen más grande en las carpetas del álbum (también `Scans/`,
  `Artwork/` debajo); las imágenes `back…` nunca cuentan. También se conserva una carátula incrustada.
- **Colecciones:** carpetas con al menos 3 álbumes en carpetas hijas de al menos 2
  artistas de álbum, con un nombre que no es el de un artista.

Cuando termina el escaneo (salvo que se cancele) dispara la identificación y, con `ffmpeg`,
la medición de volumen.

### 4. Identificar (`internal/identify`)

El proceso de identificación trabaja álbum por álbum contra MusicBrainz, una consulta por
segundo como lo pide MusicBrainz; una primera pasada sobre ~10.000 álbumes lleva unas horas y
continúa donde quedó. La barra superior muestra `Identificando · n/m`.

1. **Primero los ids de los tags.** Si la mayoría de las pistas tienen el mismo id de álbum de MusicBrainz
   y MusicBrainz lo conoce, el álbum se identifica con confianza 1.
2. **Búsqueda.** Si no, se buscan en orden hasta cuatro variantes de consulta: el
   título y artista agrupados, el título y artista de la carpeta, y ambos sin marcas de disco ni
   notas entre paréntesis (`(Remastered)`). Los compilados buscan Various Artists.
3. **Puntaje.** Cada candidato recibe una confianza de 0 a 1:

   | Señal | Peso | Regla |
   |---|---|---|
   | Título | 0.30 | similitud (1 − distancia de edición / largo, después de normalizar) |
   | Duraciones de las pistas | 0.25 | proporción de pistas a menos de 3 s de una pista del lanzamiento (por posición cuando las cantidades coinciden) |
   | Artista | 0.20 | similitud con el crédito o con cualquiera de los nombres acreditados |
   | Cantidad de pistas | 0.15 | igual es lo mejor; menos pistas que el lanzamiento reciben una parte; más reciben 0 |
   | Año | 0.10 | exacto (del lanzamiento o primer lanzamiento) 1, ±1 la mitad; sin año, una mitad neutra |

   Los resultados de la búsqueda no traen duraciones, así que solo los que todavía podrían alcanzar el umbral
   se consultan completos (como máximo 4 consultas por álbum).
4. **Decidir.** Con **0.80** o más, el álbum se identifica automáticamente. Si no,
   va a Revisar → *Sin identificar* con sus cinco mejores candidatos.

Las coincidencias se guardan en caché según su consulta (variantes, año, id del tag y duraciones
redondeadas): un álbum se vuelve a buscar solo cuando cambia lo que lo describe, o cuando
mejora el comparador.

### 5. Completar y aplicar

Para cada lanzamiento en uso, Tordo trae el artista principal (para su **país**), los **géneros
y estilos** de Discogs (a través de los enlaces de MusicBrainz a Discogs, los del lanzamiento o los de
su grupo, con el maestro como alternativa; con un `discogsToken`, también una búsqueda en Discogs)
y una **carátula** del Cover Art Archive (del lanzamiento o, si no, del grupo de lanzamientos) para los álbumes
cuya carpeta no tiene ninguna, guardada en `cache/covers/`. La barra muestra
`Completando datos · n/m` y después `Al día`.

Después, *aplicar* recalcula la identidad de cada álbum solo con los datos guardados (sin consultas):
el título del álbum, el artista, el año original y el país de MusicBrainz; los títulos de las pistas alineados por
posición (o por el título más parecido) cuando las duraciones coinciden dentro de 3 s; los géneros de Discogs,
si no los de MusicBrainz, si no los de los tags. Los géneros crudos de los tags que ningún mapeo conoce van a Revisar →
*Géneros por mapear*. No se escribe nada en los archivos de audio.

### Correcciones y sin conexión

Elegir un candidato, pegar un lanzamiento de musicbrainz.org, o marcar un álbum como que no está en
MusicBrainz (en *Sin identificar* o en *Corregir identificación* en la página del álbum) se
guarda por **huella de pista**: sobrevive a mover, renombrar y retaguear, se aplica
cuando al menos la mitad de las pistas de un álbum la tienen, y la identificación automática nunca
la pisa. Sin red, el proceso muestra `Sin conexión con MusicBrainz` y
reintenta solo (a los 5 minutos, duplicando hasta una hora); todo lo demás sigue funcionando.

### Revisar

#### Duplicados

Tordo detecta tres tipos de grupos de duplicados; nunca borra archivos, solo los lista.

1. **Copias idénticas:** dos o más archivos con audio idéntico (la misma huella de audio). Los bytes recuperables de un grupo son todos sus archivos menos el más grande.
   Los grupos se ordenan por bytes recuperables, de mayor a menor.

2. **Mismo tema en varios formatos:** pistas del mismo álbum con el mismo id de grabación de MusicBrainz, o con el mismo artista y título normalizados y duración dentro de 2 segundos (encadenadas por duración); un grupo necesita al menos dos huellas de audio distintas (los archivos idénticos bit a bit son solo copias idénticas). Los archivos se ordenan por calidad (lossless > lossy; los lossless por profundidad de bits y después frecuencia de muestreo; los lossy por tasa de bits) y después por ruta. El mejor archivo recibe una insignia MEJOR cuando es estrictamente mejor que todos los demás de su grupo.

3. **Sospechas (mismo artista y título en distintos álbumes):** el mismo artista, título y duración (dentro de 2 segundos) en álbumes distintos, con al menos dos huellas de audio distintas. Se ordenan por artista y después por título. Solo informativo. El mejor archivo de cada grupo recibe una insignia MEJOR cuando es estrictamente mejor que todos los demás.

Un álbum se marca como "Con duplicados" (filtro Estado en Explorar) cuando una o más de sus pistas pertenecen a un grupo de duplicados de copias idénticas o de formatos.

#### Formatos a convertir

Detecta qué pistas usan formatos que necesitan conversión para reproducirse: WMA, WAV, AIFF, APE, WavPack y ALAC.
Muestra totales (archivos, bytes) y un elemento por álbum × formato (o carpeta × formato para las pistas sueltas),
ordenados por tamaño. **Solo detección:** no se convierte nada automáticamente.

#### Versiones de un álbum

La página del álbum lista los otros álbumes que comparten con él uno o más grupos de duplicados de copias idénticas o de formatos:
su título, artista, año, los formatos de las pistas compartidas, cuántas pistas de este álbum comparten
y el tipo de grupo (`exact` cuando todas las pistas compartidas son copias idénticas, si no `formats`).

## Normalización de volumen

Tordo normaliza el volumen de toda tu colección a −18 LUFS (EBU R128).
Primero mira los tags ReplayGain; si faltan, `ffmpeg` mide el
volumen en segundo plano (primero las pistas más nuevas), y el audio que está
por sonar se adelanta en la cola. Las pistas que todavía no se midieron suenan a −6 dB. La
normalización solo baja el volumen, nunca lo sube. La ganancia de álbum se aplica cuando
un álbum se reproduce en orden (aleatorio desactivado). Activala o desactivala en Ajustes; el reproductor
lo toma de `/api/status` sin reiniciar.

## Acceso desde la red

Por defecto, Tordo solo escucha en `127.0.0.1` (la computadora que lo corre). En Ajustes
podés activar **Red local** para que otros dispositivos de tu red hogareña puedan acceder.

### Activar el acceso desde la LAN

Cuando activás "Usar Tordo desde otros dispositivos de la red", Tordo te pide
elegir un PIN de 6 a 12 dígitos. El PIN se elige al activar la LAN; un PIN escrito mientras la LAN
está desactivada no se guarda. Después Tordo escucha en `0.0.0.0` en el puerto 4747 (por defecto; cambialo en
Ajustes si hace falta).

La conexión es HTTP simple, sin cifrar: es segura en una red hogareña. Usala solo
en tu casa, no por internet.

### Autenticación con PIN

El PIN protege el acceso desde otros dispositivos de la LAN. Después de 5 intentos fallidos, cada
intento fallido adicional bloquea los ingresos por 30 segundos (duplicando cada vez, hasta 1 hora);
un PIN correcto reinicia el bloqueo. Mientras está bloqueado, el servidor responde 429 con `Retry-After`.

Cambiar el PIN cierra la sesión de todos los dispositivos (tienen que ingresar de nuevo).

### Sesiones

Cuando ingresás desde un dispositivo, el servidor crea una sesión de 90 días (una cookie `tordo_sesion`,
`HttpOnly`, `SameSite=Lax`, sin cifrar). Si no usás un dispositivo durante un día, la
sesión se extiende automáticamente otros 90 días en tu próxima solicitud. Las sesiones de más
de 90 días se eliminan en el siguiente ingreso.

En Ajustes podés ver la lista de sesiones activas (una fila por dispositivo con su nombre
y la fecha del último uso) y cerrar cualquiera desde ese dispositivo.

### Direcciones de la LAN y QR

Ajustes muestra las direcciones donde podés llegar a Tordo en tu red hogareña: `http://<ip>:<puerto>/`
para cada dirección IPv4 local, y después `http://<nombre-del-equipo>.local:<puerto>/`. Cada dirección tiene un código
QR que podés escanear con tu celular para abrirla.

La pantalla del PIN aparece cuando cargás la página en un dispositivo no local (celular, tableta, otra
computadora de la LAN). Una vez que ingresaste, todo lo que podés hacer desde la computadora principal funciona:
reproducir, explorar, puntuar, dar me gusta, armar listas de reproducción, estaciones y mirar tus estadísticas. Lo único
que no podés hacer desde otro dispositivo es cambiar los ajustes; eso solo es posible desde
la computadora que corre Tordo.

### Configuración: el bloque lan

```json
"lan": {
  "enabled": false,
  "port": 4747,
  "pinHash": ""
}
```

- `enabled`: si el oyente de la LAN está corriendo.
- `port`: el puerto en el que escuchar (1024–65535); 0 o ausente significa el puerto por defecto, 4747.
- `pinHash`: el PIN con hash (PBKDF2-SHA256, 100.000 iteraciones). Vacío hasta que se elige un PIN; activar la LAN requiere uno. Cambiar el PIN cierra la sesión de todos los dispositivos.

## Desarrollo

La aplicación web (Svelte 5 + Vite, en `web/`) está embebida en el binario desde
`internal/server/dist/`, que no se versiona: compilala antes de `go run` o
`go build`, o el navegador muestra una página que la pide.

```bash
go test ./...                       # full suite, no network
(cd web && npm ci && npm test && npm run build)   # web app tests and build
go run ./cmd/tordo -dir .run        # config and catalog in .run/
bash scripts/build.sh               # web app, then binaries in dist/tordo/
TORDO_CORPUS=/path/to/music go test ./internal/audiofile -run Corpus -v -timeout 0
```

Para publicar una versión, subí una etiqueta de versión (`git tag v1.0.0 && git push origin v1.0.0`):
el flujo de trabajo `release` corre las pruebas y `scripts/build.sh`, y adjunta los binarios
y `SHA256SUMS.txt` al release de GitHub de esa etiqueta.
