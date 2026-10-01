# M1 contra la colección real — hallazgos

Colección: `/Users/martin/Music` (solo lectura). Fecha: 2026-10-01.

## Lectores propios vs ffprobe

- 6.697 archivos, **0 rechazados** por el lector propio, 286 con diferencias. Todas esperables:
  - Acentos: ffprobe muestra mojibake (`FÃ¡bio`, `D�as`); tordo decodifica bien.
  - ID3v1 como respaldo: cuando el ID3v2 no trae título/artista/álbum, tordo los toma del ID3v1 y ffprobe no
    (p. ej. `Selección de murgas`, `La patria, compañero`). Títulos ID3v1 truncados a 30 caracteres.
  - Duración de 6 MP3 VBR sin cabecera Xing o dañados (ambas herramientas estiman).
- Sin cambios en `audiofile`; `audiofile.Version` sigue en 1.

## Escaneo

- Primer escaneo: **~7–9 s** (9.026 archivos, 6.685 de audio, caché de disco caliente tras la prueba con ffprobe).
- Reescaneo sin cambios: **0,5 s**, `toRead: 0`.
- La colección no se modificó (`find -newer` vacío).
- Conteos: audio 6.685 (mp3 5.770, opus 411, flac 372, m4a 78 [45 AAC + 33 ALAC], wma 54); **0 ilegibles**;
  461 álbumes (35 compilados), 549 carpetas, 13 colecciones. Ningún archivo de `Ableton/`, `djay/`,
  `Audio Music Apps/` ni `.musiclibrary`. Pistas sueltas: `Criosho` en la raíz, `youtube/` y 7 más.
- `Latinoamérica 600` es colección con posiciones 598–600 (en disco solo hay esos 3 discos).

## Corregido (con prueba primero)

- **Nombres NFD de macOS**: `Norm` cortaba las palabras en el acento combinante (`Império` → `impe rio`).
  Ahora lo ignora. Efecto: `Fito Páez` y `Tótem` dejan de contar como "varios artistas" y ya no son colecciones.
- **Juegos de discos con tags distintos por disco**: `Noel pela primeira vez` (CD 03 `(03)`, CD 05 `- disc`) se
  partía en 4 álbumes; las carpetas de disco ahora se agrupan por el álbum que nombra la carpeta, sin notas entre
  corchetes (`Candombe de llamadas CD 1 (selección…)` + `CD 2`). Resultado: 1 álbum, 14 discos, 230 pistas.
- **Carpetas con tags de álbum dispares** (`settleFolders`):
  - Tags sueltos (grupos de 1–2 pistas) se suman al álbum principal si este tiene ⅔ de la carpeta:
    `Na Roda Do Samba` (26+1+1), `La patria, compañero`, `Radio Centenario`, `Donga e Os Primitivos`, etc.
  - Carpeta hecha mayormente de álbumes de 1–2 pistas = una selección, un álbum con el nombre de la carpeta:
    `Selección de murgas`, `VA - Global Sampler vol. 56`, `AAVV - Cantamos a Zitarrosa`, `Casi en privado`
    (tags `P.1`…`P.35`), `baulera` (698 pistas, antes 279 álbumes).
  - Siguen separados los álbumes reales en una carpeta: `VA - Cumbia Colombiana` (6), `Anthology of American
    Folk Music, Volumes 1-3` (3), `infantiles` (35).
- **Nombres de carpeta**: `[1962] …` era posición 1962 (ahora año); `[1980-1981] …` toma 1980; no se separa
  artista/álbum en un guion dentro de paréntesis (`Casi en privado (… - Mexico)`).
- Álbumes: 852 → 461; carpetas con más de un álbum: 22 → 7, todas legítimas.

## Pendiente para M2

- **Tags basura**: álbum `WWW.TODOMUSICACD.8K.COM` (ID3v1 de *Naturaleza Sangre*), `(LoMusical.Com)`, artistas
  `M031CAS_e_01_01` en *Casi en privado* (sale "Varios artistas"; la carpeta padre dice Zitarrosa), tags
  cruzados en *Raras Partituras* (álbum `Epsa Music`, artista `Raras Partituras 15…`).
- **Títulos feos de tags**: `FITO PAEZ - 2004 - mi vida con ellas 1`, títulos ID3v1 truncados
  (`La Ley es Tela de Araña y Melo`, año 2000 en vez de 1984 de la carpeta).
- **História da MPB**: cada volumen tiene `albumArtist` = compositor (`Noel Rosa`), así que no sale como
  compilado; MusicBrainz debería resolverlo.
- **Carpetas-cajón** (`baulera`, `voces`, `infantiles`): mejor como selección o como álbumes reales según la
  identificación.
- **Unicode**: rutas y textos de nombres de carpeta quedan en NFD tal como los da macOS; normalizar a NFC para
  mostrar y buscar (M3) sin tocar la ruta real.
- **Géneros**: 207 valores distintos sin normalizar.
- 61 álbumes sin carátula (ni imagen ni embebida).
