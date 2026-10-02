# Tordo — Carpetas como explorador de archivos (diseño)

Fecha: 2026-10-02
Estado: aprobado en brainstorming, pendiente de revisión escrita
Reemplaza la página **Carpetas** de M3a (§8 del diseño general: "árbol del disco tal cual, orden natural, migas de pan; ▶ y ⤮ en cada nivel").

## 1. Problema

Carpetas muestra cada carpeta como una grilla de tarjetas cuadradas. En la colección real eso mezcla, en la misma grilla y con el mismo peso, carpetas de tamaños muy distintos: en la raíz `Soulseek` (248 álbumes, 3.884 pistas, 30 GB) aparece junto a ocho carpetas de un solo álbum, una de 2 pistas y `youtube` (1 pista). El árbol llega a 6 niveles, 30 carpetas solo contienen otra carpeta (`Soulseek/complete/<usuario>/…`) y algunas tienen listas largas (*História da MPB* 49, *Zitarrosa* 46). Explorar, Artista, Álbum y Listas ya cubren la navegación por álbumes y artistas: Carpetas tiene que servir para **ubicarse en el disco**, como un explorador de archivos.

## 2. Diseño

Dos paneles, como el Explorador de Windows o el Finder con barra lateral.

```
┌ Filtrar carpetas…           ┐ Music / Soulseek / complete / Low_ID       26 álbumes · 378 pistas · 3,0 GB  ▶ ⤮ +
│ ▾ Soulseek › complete  3.884│ Nombre                              Tamaño rel.  Pistas  Duración  Formato   Tamaño
│     ▸ benteveo         1.133│ 📁 RARAS PARTITURAS                 ██████        183    11 h      MP3      1,4 GB
│     ▾ Low_ID             378│ 📁 Lhasa de Sela                    █              35     2 h      FLAC     410 MB
│         ▸ RARAS PART…    183│ 💿 music of the crusades…           ▌              19    52 min    MP3 320  120 MB
│           Lhasa de Sela   35│ 🎵 suelto.mp3                                             3:07     MP3 320  7,2 MB
│         💿 music of the…  19│ 🖼  folder.jpg                                                              84 KB
│ ▸ Uruguay              1.672│
│ ▸ Brasil  colección      953│
└─────────────────────────────┘
```

### 2.1 Árbol (panel izquierdo)

- **Solo carpetas**, y solo las que tienen audio adentro (a cualquier profundidad): son exactamente las filas de la tabla `folders`. Las carpetas de álbum son hojas marcadas con 💿; las de disco (`CD1`, `CD2`) aparecen como subcarpetas de su álbum, como en el disco.
- **Niveles de paso fusionados:** una carpeta sin pistas propias y con una sola subcarpeta se muestra en la misma fila que su hija (`Soulseek › complete`), repetidamente si la cadena sigue. Seleccionar la fila selecciona la carpeta más profunda; las migas de pan muestran todos los niveles.
- Con una sola raíz (el caso de la colección real), el árbol empieza en sus hijas, como hoy.
- Cada fila: ▸/▾ si tiene subcarpetas, ícono (📁, 💿, o la marca "colección"), nombre y cantidad de pistas (recursiva). Orden natural (`[LA-9]` antes de `[LA-10]`).
- Se carga **por nivel**, al expandir. Las ramas expandidas se recuerdan por dispositivo (`localStorage`); la carpeta seleccionada está en la URL (`/carpetas?ruta=…`) y su rama se abre sola al entrar, al volver atrás o desde un enlace.
- **Filtrar carpetas…**: filtra por nombre (sin acentos ni mayúsculas, como la búsqueda) sobre todas las carpetas y muestra las coincidencias con sus ramas abiertas. Es la única operación que pide el árbol completo de nombres.
- El panel tiene su propio desplazamiento, independiente del derecho.

### 2.2 Contenido (panel derecho)

- **Encabezado:** migas de pan (cada nivel es un enlace), nombre, totales (álbumes, pistas, duración, tamaño), ▶ Reproducir, ⤮ Aleatorio, + A la cola, y "No es una colección / Es una colección" cuando corresponde. En una carpeta de álbum, además, la tapa chica y un enlace a la página del álbum.
- **Lista de detalle** con columnas Nombre · Tamaño relativo · Pistas · Duración · Formato · Tamaño:
  - primero las **subcarpetas** (📁/💿) con una barra de tamaño relativa a la carpeta actual (por bytes); un clic la selecciona (y la abre en el árbol), ▶ al pasar el mouse la reproduce;
  - después los **archivos** que están directamente en la carpeta: los de audio por **nombre de archivo** (no el título del tag), con duración y formato ("MP3 320", "FLAC 24/96"); un clic reproduce desde ese archivo los de la carpeta; + los agrega a la cola. Los que no son reproducibles (falta ffmpeg) se ven atenuados;
  - los demás archivos del catálogo (imágenes, `.lrc`, listas `.m3u`) atenuados, con su tamaño y sin acción.
- Orden por defecto: subcarpetas y archivos en orden natural; un clic en el encabezado de una columna ordena por ella (y otro invierte), dentro de cada grupo.
- Carpeta vacía de archivos propios: solo las subcarpetas. Carpeta inexistente: "Esta carpeta ya no está en el catálogo."

### 2.3 Teléfono

No hay lugar para dos paneles: el árbol es la página. Tocar ▸ expande en su lugar; tocar el nombre abre el panel derecho a pantalla completa (con la ruta en la URL); volver regresa al árbol donde estaba. Columnas visibles en el teléfono: Nombre, barra y Pistas (o la duración, en los archivos).

## 3. Datos y API

Sin cambios de esquema.

- `GET /api/folders/tree?path=` → las subcarpetas directas de `path` (sin `path`: las raíces, o las hijas de la única raíz) con las cadenas de paso ya resueltas: `[{path, label, name, tracks, sizeBytes, album (id o 0), collection, hasChildren}]`, donde `path` es la carpeta más profunda de la cadena y `label` une los nombres con " › ".
- `GET /api/folders/tree/search?q=` → las carpetas cuyo nombre coincide (plegado con `nameparse.Norm`, todas las palabras, la última como prefijo), como lista de rutas con sus ancestros, para abrir esas ramas. Tope de 200 coincidencias.
- `GET /api/folders?path=` (existente) suma `sizeBytes`, y `files`: los archivos presentes directamente en la carpeta, `[{id, name, kind, sizeBytes}]`, con, para el audio, `durationMs`, formato, códec, bitrate, sample rate, bit depth y `playable`. Cada subcarpeta (`children`) suma `sizeBytes` y `album` (id o 0).
- **Tamaño de una carpeta:** suma de `files.size` de los archivos presentes bajo su ruta (todo lo que tordo catalogó, menos `basura`), con una consulta por rango sobre el índice de `files.path` (`path >= p || '/' AND path < p || '0'`): ~10 ms para `Soulseek` (30 GB, ~9.000 archivos).
- Reproducir: los archivos de audio son pistas (mismo id); ▶ de carpeta usa el `GET /api/play?folder=` existente; un clic en un archivo usa `?track=` con los de la carpeta.

## 4. Lo que no cambia

- Explorar, Álbum, Artista, Listas y la búsqueda.
- `GET /api/folders` sigue sirviendo a lo que ya lo usa (los campos nuevos se suman).
- Solo lectura sobre la colección: no se abre ningún archivo de música para mostrar el explorador.

## 5. Pruebas

- **Go (store/server):** subcarpetas por nivel; fusión de cadenas (una y varias), sin fusionar cuando la carpeta tiene pistas propias o más de una hija; tamaños recursivos (sin `basura`, sin archivos faltantes); archivos directos con su tipo y datos de audio; `playable` sin ffmpeg; la búsqueda de carpetas con nombres NFD y acentos; carpeta desconocida → 404.
- **Vitest:** estado del árbol (expandir/colapsar, recordar en `localStorage`, abrir la rama de `?ruta=`, combinar los resultados del filtro), orden de columnas, formato de tamaños ("7,2 MB", "1,4 GB") y la barra relativa.
- **En el navegador, contra la colección real:** la raíz no mezcla `Soulseek` con carpetas de una pista; `Soulseek › complete` en una fila; *História da MPB* (49) y *Zitarrosa* (46) navegables; filtro por un nombre con acento; ancho de teléfono.
