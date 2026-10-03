# Tordo — Diseño (v1)

Fecha: 2026-10-01
Estado: aprobado en brainstorming, pendiente de revisión escrita

## 1. Objetivo

Herramienta local y portable para catalogar, explorar y **escuchar** una colección de música repartida en una o más carpetas, independientemente de cómo estén organizadas en disco. Reúne los archivos, lee y completa sus metadatos, detecta duplicados (y sospechas de duplicado) y ofrece herramientas para escuchar y buscar por artista, álbum, año, país, género, carpeta, colección, al azar, etc.

Proyecto hermano de Cinexplorer: mismo modelo de distribución y mismas convenciones, pero **repositorio completamente separado**. Se copia y adapta código de Cinexplorer cuando conviene; nunca se comparten módulos.

### Principios

- **Portable:** carpeta `tordo/` en el mismo disco que la música, con un binario por plataforma, sin instalador ni dependencias obligatorias.
- **Solo lectura sobre la colección en v1.** Escribir en archivos (tags, conversión de formatos) llega en una etapa posterior, siempre como acción explícita con vista previa y sin borrar nunca nada.
- **Herramientas externas opcionales:** `ffmpeg`/`ffprobe`, si están en el `PATH`, se usan para reproducir formatos no soportados por el navegador, para leer lo que el lector nativo no entiende y para analizar volumen. Sin ellas todo lo demás funciona.
- **Escucha en la red local** desde otros dispositivos (teléfono, otra computadora), protegida con PIN. El audio suena en el dispositivo que tiene la página abierta.
- **Un solo usuario** (el dueño, en varios dispositivos).
- Interfaz en **español**.

### Datos de referencia

Colección completa: ~1,5 TB, en 1–5 carpetas de un mismo disco. Muestra analizada: `/Users/martin/Music` (52 GB, ~7.000 archivos de audio):

- Formatos: mp3 (5.775), opus (411, descargas de YouTube), flac (372), m4a (84), wma (54).
- Cobertura de tags (muestra de 300): artista/título/álbum ~93%, número de pista 84%, año 74%, género 69%, album_artist 60%, IDs de MusicBrainz ~1%, país ~1%, carátula embebida 60% (más ~2.300 `.jpg` sueltos).
- Géneros caóticos: `Samba`, `Other`, `Folklore`/`Folclore`, `Latin`/`Latino`, `Latin, Folk, World, & Country`, `Uruguay`, `Pop Pop Internacional`.
- Nombres: `Artista - (2008) Álbum`, `[LA-599] Rata Blanca - (1990) Magos, Espadas Y Rosas`, `(1990) Tercer Mundo/05-Fue Amor.mp3`, `Noel Rosa - Noel pela primeira vez Cd 02 -x`, `… (256kb)`, `(aac vbr)`, `[24-88.2]`, créditos en el título (`antonico(alcides gerardi)`, `Curare - Nilze Carvalho`, `… c- Almirante`).
- Compilados: `VA - …`, `AAVV - …`, `Varios - …`, `Historia musical de… la cumbia colombiana cd 2`.
- Carpetas con curaduría propia: `Latinoamérica 600`, `15 discos de los 60`, `Selección de murgas`, `Sambas de enredo RJ 2020`; organización parcial por país (`Brasil/`, `Uruguay/`, `USA/`).
- Ruido no musical: carpetas de otras apps (`Ableton/`, `Audio Music Apps/`, `djay/`, biblioteca de Apple Music, `Soulseek/downloading`), 482 `.epub`, `.tagset`, `.strings`, `.db`. Útiles a futuro: `.lrc` (letras), `Scans/` (booklets).

### Fuera de alcance v1 (etapa "Después")

- Conversión de formatos (WMA/WAV → FLAC/MP3) y escritura de tags.
- API compatible con Subsonic (apps nativas de teléfono, escucha offline), dentro del mismo binario.
- AcoustID/Chromaprint bajo demanda.
- Gapless real (Web Audio / MediaSource).
- Letras (`.lrc`, embebidas), booklets y escaneos.
- Reproducción en los parlantes del equipo que corre tordo (modo control remoto / jukebox).
- Importación de `.m3u`.
- Multiusuario.
- **Favoritos separados por tipo** (pedido 2026-10-02, después de M5): el ♥ de pistas y el ♥ de álbumes dejan de ser una sola bolsa "Me gusta". Las pistas con ♥ forman una lista de reproducción automática (*Canciones favoritas*); los álbumes con ♥ van a una sección propia, una **colección de cabecera** (*comfort collection*) para volver a ellos, no mezclada con la de pistas.
- **Modo escucha / fiesta** (pedido 2026-10-02, después de M5): modo de solo lectura que oculta la curaduría y el mantenimiento: sin **Revisar** (sin identificar, duplicados, géneros por mapear, formatos a convertir) y sin las marcas de duplicados y versiones en Álbum y Explorar. Queda lo de escuchar: Inicio, Explorar, Carpetas, Listas, búsqueda, cola y estaciones.

## 2. Distribución y portabilidad

### 2.1 Forma

Un ejecutable por plataforma escrito en **Go**, con la interfaz (Svelte 5 + Vite) embebida con `go:embed`. Al ejecutarse levanta un servidor HTTP y abre el navegador. Binarios: Windows amd64, macOS universal (amd64+arm64), Linux amd64 y **Linux arm64** (Raspberry Pi con el disco conectado).

### 2.2 Estructura en disco

```
<raíz del disco>/
├── musica/
├── musica-ordenar/
└── tordo/
    ├── tordo-windows.exe
    ├── tordo-macos
    ├── tordo-linux-amd64
    ├── tordo-linux-arm64
    ├── config.json
    ├── tordo.db            (SQLite: catálogo, datos de usuario, correcciones)
    └── cache/              (carátulas, fotos de artistas)
```

- Directorio de datos = directorio del ejecutable (sobreescribible con `-dir` o `TORDO_DIR`).
- Rutas guardadas **relativas al directorio de la app**, con `/` como separador.
- `config.json`: `{"roots": ["../musica"], "ignore": [], "discogsToken": "", "lan": {"enabled": false, "port": 4747, "pinHash": ""}, "normalize": true}`.
- SQLite (`modernc.org/sqlite`, Go puro) en journal `DELETE`.
- **Modo consulta** si el directorio de la app no es escribible (como Cinexplorer).
- Raíz no disponible: se saltea sin tocar el catálogo.

## 3. Arquitectura

```
cmd/tordo/            entrada
internal/appdir       carpeta de la app y rutas relativas portables   (copiado de Cinexplorer)
internal/config       config.json
internal/httpx        HTTP con rate limiting y reintentos            (copiado)
internal/scan         escaneo incremental, clasificación, reglas de ignorado
internal/audiofile    lectura nativa en una sola pasada de tags (ID3v1/v2, Vorbis en FLAC/Ogg/Opus, MP4, ASF, RIFF/AIFF)
                      y datos técnicos (formato, codec, bitrate, sample rate, bit depth, duración, lossless,
                      rango de bytes del audio); ffprobe como alternativa
internal/natsort      orden natural de nombres ([LA-9] antes de [LA-10])
internal/fingerprint  huella del payload de AUDIO (excluye tags y carátulas)
internal/nameparse    parseo de nombres de carpetas y archivos
internal/grouping     archivos → pistas → álbumes → carpetas → colecciones
internal/musicbrainz  cliente MusicBrainz (1 req/s) + Cover Art Archive
internal/discogs      cliente Discogs (géneros y estilos)
internal/genres       normalización de géneros: tag crudo → taxonomía género/estilo
internal/identify     puntaje por álbum, enriquecimiento, worker en segundo plano
internal/loudness     ReplayGain desde tags; análisis ebur128 con ffmpeg
internal/engine       todo lo que depende de config.json, intercambiable en caliente (adaptado)
internal/store        catálogo SQLite
internal/catalog      facetas, páginas, Inicio, estaciones, duplicados, estadísticas
internal/search       índice de búsqueda FTS5                        (adaptado)
internal/stream       entrega de audio con Range; transcodificación con ffmpeg
internal/auth         PIN, sesiones, modo LAN
internal/server       API JSON + interfaz embebida
internal/platform     abrir carpeta en el sistema                    (copiado)
web/                  interfaz Svelte 5 + Vite
```

## 4. Modelo de datos

- **Archivo** — ruta relativa, tamaño, mtime, **huella de audio**, tipo (`audio`, `imagen`, `letra`, `playlist`, `basura`, `otro`), estado (`presente`, `no encontrado`), datos técnicos, tags crudos (JSON, para re-derivar sin releer el disco).
- **Pista** — un archivo de audio = una pista. Título, disco/número, duración, artistas acreditados (con rol: principal, invitado, extraído del título), álbum, carpeta, MBID de grabación si existe, ganancia (pista y álbum).
- **Álbum** — título, artista del álbum, año y año original, compilado (sí/no), cantidad de pistas, MB release / release group, id Discogs, carátula, géneros/estilos, carpeta, estado (`identificado`, `sin identificar`, `sueltos`).
- **Artista** — nombre, nombre de orden, MBID, **país**, tipo (persona/grupo), años de actividad, foto (si MB/Discogs la tienen).
- **Carpeta** — ruta relativa, padre, nombre, totales recursivos (pistas, álbumes, duración), carátula (imagen propia o mosaico de sus álbumes), marca de colección (detectada / descartada por el usuario).
- **Género / Estilo** — taxonomía de dos niveles basada en Discogs. Tabla `genre_alias` (valor crudo → género/estilo). Cada vínculo álbum–género guarda su **origen** (`tag`, `discogs`, `mb`, `manual`).
- **Datos de usuario** — reproducciones (pista, fecha, dispositivo, % escuchado); **me gusta** (♥) sobre pistas, álbumes, artistas y colecciones; **puntaje** (★ 1–5) sobre pistas y álbumes; listas de reproducción y sus entradas ordenadas; estaciones guardadas.
- **Corrección** — atada a la **huella de cada pista**. Una corrección a nivel álbum se guarda en cada una de sus pistas: sobrevive a mover, renombrar y re-taggear. La identificación automática nunca pisa una corrección.

### 4.1 Huella de audio

Hash de los primeros y últimos 64 KiB del **payload de audio** más su tamaño (con un MB por extremo, 1,5 TB de música implicarían leer ~300 GB en cada escaneo completo), saltando bloques de metadatos (ID3v2/ID3v1/APEv2 en MP3, bloques METADATA en FLAC, cabeceras de comentarios en Ogg/Opus —donde además se hashea el contenido de las páginas sin sus cabeceras, porque reescribir comentarios renumera las páginas siguientes—, átomos de metadatos en MP4, objetos de cabecera en ASF). Editar tags no cambia la identidad del archivo, y dos copias idénticas con tags distintos se reconocen como la misma.

### 4.2 Agrupación en álbumes

- Señal principal: carpeta + tag `album` (+ `album_artist`).
- Carpetas hermanas `CD1`/`CD 2`/`Disc N`/`Disco N` con el mismo álbum se fusionan; el álbum pertenece a la carpeta padre.
- Carpeta con pistas de artistas distintos sin `album_artist` → **compilado** ("Varios artistas"), reforzado por nombres `VA - …`, `AAVV - …`, `Varios - …`.
- Archivos sueltos (`youtube/`, `.opus` aislados) → **sueltos**: no forman un álbum artificial.

### 4.3 Carpetas y colecciones

Las carpetas son entidad de primera clase porque contienen curaduría propia del usuario.

- **Colección detectada:** una carpeta que contiene **varios álbumes de artistas distintos** y cuyo nombre no es el de un artista (según el catálogo o MusicBrainz). Ej.: `Latinoamérica 600`, `15 discos de los 60`. No lo son: `Uruguay/Alfredo Zitarrosa` (carpeta de artista) ni `Sambas de enredo RJ 2020` (un compilado = un álbum).
- Se detectan **automáticamente**; cada una tiene la acción "no es una colección" (persistida).
- **Prefijos de orden** (`[LA-599]`, `01 -`, `100.`) se interpretan como la **posición** del álbum en la colección; reproducir la colección respeta ese orden. Orden natural de nombres (`[LA-9]` antes de `[LA-10]`).

## 5. Pipeline de catalogación

En segundo plano, con progreso visible en la UI. Incremental: un archivo se reprocesa solo si cambiaron tamaño o mtime.

1. **Escaneo** — recorre las raíces; clasifica por extensión: audio (`mp3 flac m4a opus ogg wma wav aiff ape wv`), imagen, `.lrc`, `.m3u`, basura (`.DS_Store`, `Thumbs.db`, `desktop.ini`, `*.db-wal`…), otro (`.epub`, `.pdf`…). **Ignora carpetas de apps conocidas** (`Ableton/`, `Audio Music Apps/`, `djay/`, `*.musiclibrary`, `*.localized`, `Soulseek/downloading`…) más la lista de ignorados del usuario.
2. **Audio y tags** — lectura nativa de tags y datos técnicos; `ffprobe` como alternativa si un header no se entiende.
3. **Huella** — §4.1.
4. **Nombres** — parseo de carpeta y archivo para completar lo que falta en los tags: título, artista, año, número de pista y de disco, créditos en el título, posición en colección; limpia ruido de calidad (`(256kb)`, `[24-88.2]`, `(aac vbr)`, `(digital)`, `(lp)`).
5. **Agrupación** — pistas → álbumes → carpetas → colecciones (§4.2, §4.3).
6. **Identificación (por álbum)** — búsqueda en **MusicBrainz** con álbum, artista, año y cantidad de pistas. Puntaje de confianza = similitud de título + artista + año (±1) + cantidad de pistas + **coincidencia de duraciones de pistas**. Sobre el umbral: automático; debajo: cola **Sin identificar** con candidatos. Variantes: nombre de carpeta en lugar del tag, álbum sin "Disco N"; compilados buscados como "Various Artists".
7. **Enriquecimiento** — MB: año original, release group, **país del artista** (área), lista de pistas, MBIDs, foto. **Discogs** (vía relación MB→Discogs, o búsqueda): **géneros y estilos**. Carátulas: embebida → imagen de carpeta (`cover`, `folder`, `front`) → Cover Art Archive. Géneros crudos normalizados vía `genre_alias`; los desconocidos van a la cola **Géneros por mapear**.
8. **Volumen** — ReplayGain desde tags si existe; si no, análisis `ebur128` con ffmpeg en segundo plano, a baja prioridad.

Escala esperada: ~100–150k pistas, ~8–12k álbumes. Con 1 req/s en MusicBrainz, la identificación por álbum completa la primera pasada en pocas horas.

### 5.1 Errores

- Sin red o sin token de Discogs: los pasos 1–5 funcionan; identificación y enriquecimiento quedan pendientes y se reintentan solos (5 min, duplicando hasta 1 h) y en cada escaneo.
- Archivo ilegible: se marca, se registra, no detiene el pipeline.
- Disco desconectado durante el escaneo: se aborta; la base queda en el último estado consistente (transacciones por lote).
- Archivos desaparecidos: `no encontrado`, no se borran (preserva reproducciones, me gusta, listas y correcciones).
- Rate limits: cliente con limitación y reintentos con backoff.

## 6. Duplicados

En **Revisar → Duplicados**:

1. **Copia idéntica** — misma huella de audio en dos o más rutas, aunque los tags difieran. Muestra el espacio recuperable.
2. **Misma grabación en varios formatos** — mismo MBID de grabación, o artista + título normalizados con duración ±2 s, **dentro del mismo álbum identificado**. Badge **MEJOR**: lossless > lossy, luego bitrate / bit depth. Incluye "WMA y FLAC de lo mismo".
3. **Sospecha** — artista + título normalizados + duración ±2 s **entre álbumes distintos**. Lista aparte e informativa: suele ser legítimo (tema del álbum y del compilado, *História da MPB* vs. el disco original).

Tordo nunca borra: solo muestra.

## 7. Reproducción y escucha

### 7.1 Reproductor (simple en v1)

- **Barra persistente** abajo (hoja desplegable en móvil): carátula, título/artista, ⏮ ⏯ ⏭, barra de progreso, volumen, ⤮ aleatorio, 🔁 repetir, botón de **cola**.
- Un elemento `<audio>` activo y un segundo que **precarga** la siguiente pista y toma el relevo al terminar (casi-gapless).
- **Cola en el navegador** (`localStorage`): cada dispositivo tiene la suya y sobrevive a recargar. "Reproducir" reemplaza la cola; "Agregar a la cola" y "Reproducir a continuación" agregan.
- **Media Session API**: controles en pantalla bloqueada y auriculares; teclas multimedia.
- **Streaming:** `GET /stream/{trackId}` con **Range**; los formatos que el navegador reproduce (mp3, flac, m4a/aac, opus, ogg, wav) se sirven tal cual. WMA, APE, WV, AIFF, ALAC se transcodifican al vuelo con ffmpeg a Opus o MP3 (sin seek). Sin ffmpeg: "no reproducible". Siempre por id, nunca por ruta.
- **Normalización:** ganancia por pista (por álbum si se escucha un álbum en orden), aplicada con `audio.volume` (solo atenúa), objetivo ~−18 LUFS. Desactivable.

### 7.2 Estaciones

- Una estación es **consulta + regla de ritmo**. Fuente: cualquier faceta o combinación (país, género/estilo, década, artista, carpeta/colección, lista, me gusta, puntaje ≥ N) o "toda la biblioteca".
- Modos: **pistas** (aleatorio pista a pista) o **álbumes** (un álbum al azar completo, después otro).
- Reglas de variedad: no repetir artista en N pistas; evitar lo escuchado en los últimos X días; mayor peso a me gusta y puntajes altos.
- El servidor alimenta la cola de a 10 pistas (`GET /api/station/next`): nunca se termina.
- Botón "▶ Estación" en artista, álbum, país, género, carpeta, colección y resultados de Explorar. Estaciones guardables, visibles en Inicio.

### 7.3 Historial y estadísticas

- Una reproducción cuenta si se escuchó ≥ 50% o ≥ 4 minutos.
- Más escuchados (pistas, álbumes, artistas, países, géneros) por período; **nunca escuchados**; **sin escuchar hace más de un año**. Todo esto alimenta filas de Inicio y fuentes de estaciones.

### 7.4 Me gusta, puntajes y listas

- **♥ Me gusta** en pistas, álbumes, artistas y colecciones. **★ 1–5** en pistas y álbumes.
- Ambos son faceta, fuente de estación ("Me gusta") y peso en estaciones. Fila *Tus me gusta* en Inicio; sección "Me gusta" en Listas, filtrable por tipo.
- **Listas de reproducción**: crear, renombrar, reordenar (arrastrar), agregar desde el menú de pista/álbum, reproducir completas o como estación. Sin importación de `.m3u` en v1.

## 8. Interfaz

Estética de la familia de Cinexplorer: tema oscuro, Inter, imagen primero, pero con **carátulas cuadradas**. Responsiva desde el inicio (barra de navegación inferior en móvil).

Navegación: **Inicio · Explorar · Carpetas · Listas · Revisar** + ⌕ (Ctrl+K / ⌘K) + ⚙.

- **Inicio** — filas fijas (*Agregados recientemente*, *Tus estaciones*, *Seguir escuchando*, *Tus me gusta*) y rotativas por visita con ↻ (una década, un país, un género/estilo, una colección, *nunca escuchados*, *sin escuchar hace un año*, *álbumes al azar*). Cada fila: "ver todas →" (Explorar con esa faceta) y "▶ estación".
- **Explorar** — pestañas **Álbumes / Artistas / Pistas**; facetas combinables: década/año, artista, país, género → estilo, colección, carpeta, formato (lossless/lossy/codec), me gusta, puntaje, estado (sin identificar, con duplicados, no reproducible). Orden: año, título, artista, agregado, más escuchado. Facetas en la URL.
- **Carpetas** — árbol del disco tal cual, orden natural, migas de pan; ▶ y ⤮ en cada nivel (recursivo, en orden).
- **Listas** — listas de reproducción, colecciones detectadas y Me gusta.
- **Álbum** — carátula, artista, año original, país, géneros/estilos, pistas con formato, ♥ y ★, versiones/duplicados, carpeta. Acciones: ▶, ⤮, + cola, + lista, estación, ♥, ⋯ (corregir identificación).
- **Artista** — foto, país, años, álbumes por año, apariciones en compilados, más escuchadas, ♥, ▶ estación.
- **Colección / Carpeta** — álbumes en el orden de la colección, ♥ (colección), ▶, ⤮, estación, ⋯ "no es una colección".
- **Revisar** — Sin identificar · Duplicados · Géneros por mapear · Formatos a convertir (solo detección: WMA, WAV, APE, AIFF, WV, ALAC, con tamaño total).
- **Búsqueda** — instantánea sobre artistas, álbumes, pistas y carpetas, sin importar acentos ni mayúsculas.
- **Ajustes / asistente inicial** — carpetas (sugiere las vecinas a `tordo/`, lista de ignorados), token de Discogs (opcional, con verificación), LAN on/off + PIN, normalización on/off.

Indicador de estado arriba a la derecha con escaneo e identificación en curso.

## 9. Red local y seguridad

- **LAN apagada (default):** escucha en `127.0.0.1`, sin PIN, con el control Host/Origin de Cinexplorer.
- **LAN encendida:** escucha en `0.0.0.0` en un **puerto fijo** configurable (4747 por defecto). Ajustes muestra la URL (`http://192.168.x.x:4747`) y un **QR**.
- Desde localhost no se pide PIN; desde otros dispositivos sí. PIN de ≥ 6 dígitos, guardado como hash, con **límite de intentos** y bloqueo creciente. Sesión por cookie `HttpOnly` de 90 días; Ajustes lista dispositivos con "cerrar sesión".
- **Endpoints de configuración** (carpetas, tokens, LAN, PIN) **solo desde localhost**. Desde el teléfono se escucha y se cura (me gusta, listas, correcciones), no se reconfigura.
- HTTP plano en LAN: aceptable para red doméstica, documentado. Implica que no hay PWA instalable (requiere HTTPS), coherente con dejar "offline en teléfono" para después.

## 10. Etapas

| # | Alcance |
|---|---|
| **M1 Núcleo** | escaneo incremental, clasificación e ignorados, lectura nativa de tags y datos técnicos, huella de audio, parseo de nombres, agrupación (álbumes, carpetas, colecciones), catálogo SQLite, API JSON |
| **M2 Identificación** | MusicBrainz por álbum con puntaje y cola Sin identificar, Discogs, normalización de géneros, país del artista, año original, carátulas, correcciones por huella |
| **M3 Biblioteca y reproductor** | interfaz Svelte: Explorar con facetas, Carpetas, páginas de álbum/artista/colección, búsqueda, barra de reproducción con cola y aleatorio, streaming con Range, transcodificación WMA |
| **M4 Escucha** | Inicio, estaciones, historial y estadísticas, me gusta y puntajes, listas de reproducción, normalización de volumen |
| **M5 Revisión y red** | duplicados (3 tipos, MEJOR), formatos a convertir (detección), asistente y Ajustes, LAN + PIN + QR |
| **Después** | conversión de formatos, escritura de tags con vista previa, API Subsonic, AcoustID bajo demanda, gapless real, letras, booklets, reproducción en parlantes del servidor, importación `.m3u` |

Cada etapa tiene su propio diseño detallado (si hace falta) y plan de implementación.

## 11. Pruebas

- **Go:** tests unitarios por paquete con fixtures (archivos de audio mínimos con tags armados a mano en `testdata/`); **corpus de nombres** reales de la colección para `nameparse`; respuestas **grabadas** de MusicBrainz y Discogs para `identify` (sin red en CI); tests de corpus opcionales sobre una colección real (`TORDO_CORPUS=/ruta`).
- **Interfaz:** Vitest para lógica de cola, estaciones y facetas.
- **CI:** GitHub Actions en Windows, macOS y Linux (`go vet`, `go test`), y job aparte para tests y build de la interfaz.
