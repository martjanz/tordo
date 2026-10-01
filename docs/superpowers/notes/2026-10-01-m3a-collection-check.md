# M3a contra la colección real — hallazgos

Colección: `/Users/martin/Music` (solo lectura, con `ffmpeg` en `/opt/homebrew/bin`). Fecha: 2026-10-01.

## Imágenes incrustadas (corpus)

- `TestCorpusPictures`: **3.733** archivos con imagen; 40 vacías o enlazadas; **0 sin extraer**.
  Tipos: `image/jpeg` 3.589, `image/png` 104.

## Compilación

- `bash scripts/build.sh` compila la web y los cuatro binarios en `dist/tordo/`, sin errores:
  - `tordo-macos` (binario universal): 23,9 MB
  - `tordo-linux-amd64`: 12,1 MB
  - `tordo-linux-arm64`: 11,5 MB
  - `tordo-windows.exe`: 12,3 MB

## Escaneo

- 9.026 archivos, 6.685 de audio, **0 ilegibles**, 6.685 pistas, 461 álbumes, 549 carpetas, 13 colecciones; ~10 s.

## Prueba en el navegador

Rama `m3a-reproductor` en 575e9d4. El ancho de teléfono (375 px) se revisó al escribir el plan, con el mismo
código de página (pestañas abajo, minireproductor, hoja a pantalla completa, panel de cola).

1. **Carpetas:** la raíz única "Music" abre directo (la URL se reemplaza). 13 tarjetas de carpeta, todas las
   imágenes cargan; 6 sin imagen: Império Serrano, music-republic-world, Silvia Barrios - Cantos, - Folclore y
   tango, - Itiyuro (álbumes `.opus` de YouTube) y `youtube` (solo pistas sueltas). Se comprobó que esos álbumes
   no tienen `cover.*` ni imagen incrustada (`cover_file_id` 0). `/api/folders` de la raíz respondió en 18 ms.
2. **Colección Latinoamérica 600:** el orden de ▶ es [LA-598] Cypress Hill, [LA-599] Rata Blanca,
   [LA-600] (Alcachofa), correcto. Ojo: en disco el nombre de la carpeta está en NFD (`Latinoamérica`); la
   interfaz siempre usa las rutas de la API, así que anda, pero una ruta NFC escrita a mano no encuentra nada.
3. **Álbum FLAC (Cypress Hill, `/album/84`):** reproduce, el slider está habilitado; al buscar 4 s antes del
   final pasa a la pista 2 sin pausa (0:04 en la pista 2 a los 8 s).
4. **Álbum WMA (Zitarrosa, `/album/373`):** reproduce convertido, con el slider deshabilitado. Sin ffmpeg
   (`PATH=/usr/bin:/bin`): `/api/play` devuelve 0 ítems, 11 omitidos, `ffmpeg` false; `/stream/7490` da 415; el
   log dice "sin ffmpeg: WMA, APE, WavPack, AIFF y ALAC no se pueden reproducir" y la página avisa "11 pistas no
   se pueden reproducir: falta ffmpeg."
5. **Pistas sueltas de YouTube (`Music/youtube`):** se reproducen desde la lista de sueltas. `Juízo Final
   [lU8j7lj4RdU].mp3` no tiene imagen incrustada (`hasCover` null), así que la barra muestra el marcador; es lo
   esperado.
6. **Recarga a mitad de pista:** la barra conserva la misma pista en 0:09 / 3:24, en pausa, lista para seguir.
7. **Teclas multimedia / pantalla de bloqueo:** no se probó acá (hace falta un dispositivo real; el modo LAN
   llega con M5).
8. **Tiempos:** la raíz carga en 107 ms con las miniaturas ya en caché; la primera vez, las miniaturas se
   midieron en ~65 ms cada una al escribir el plan.

## Pendiente

- Rutas NFD escritas a mano.
- Media Session en un teléfono (M5).
- La caché de miniaturas nunca se poda (nota de la revisión).
