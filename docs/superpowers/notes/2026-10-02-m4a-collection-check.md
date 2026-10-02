# M4a contra la colección real — hallazgos

Colección: `/Users/martin/Music` (solo lectura, con `ffmpeg` 9.0.2 en `/opt/homebrew/bin`). Fecha: 2026-10-02.
Rama `m4-escucha`. Catálogo: una copia del de `.run` (M3b, esquema v3) en una carpeta aparte, con las mismas
rutas relativas; el original no se tocó.

## Migración

- La copia v3 abrió y pasó a v4 sin escanear de nuevo: 9.026 archivos, 6.685 pistas, 460 álbumes, 13
  colecciones, como antes.

## Volumen (análisis en segundo plano)

- 6.685 pistas: **262 con ReplayGain en los tags** (no se analizan), **6.423 para medir**.
- Velocidad: **~2,75 pistas por segundo (~165 por minuto)**; la colección entera, ~40 min. Un solo `ffmpeg`
  con un hilo a la vez: más o menos un núcleo. Explorar y la reproducción no se notaron más lentos mientras corría.
- Primeras 309 medidas: **0 errores**; de −24,4 a −0,8 LUFS, media −13,6.
- *Los grandes éxitos en español* (Cypress Hill, FLAC con tags): ganancias de pista de −4,7 a −7 dB, de álbum
  −5,88 dB (calculada). *Guitarra negra* (Zitarrosa, WMA): −7,4 y −5,5 dB.
- **Hallazgo 1 — compilados:** *Selección de murgas* (álbum 454) junta grabaciones de −24,4 a −0,8 LUFS. Escuchado
  en orden, la ganancia de álbum respeta esa diferencia de 23 dB entre pistas. En un compilado conviene la
  ganancia de pista aunque se escuche en orden.
- **Límite de diseño:** la normalización solo baja el volumen. Las grabaciones más bajas que −18 LUFS (Héctor
  Numa Moraes, −23,4) suenan igual que antes, así que un disco moderno normalizado (~−18) sigue sonando ~5 dB más
  fuerte que esas. Subir el volumen pediría Web Audio (un `GainNode`), como el gapless real: queda para "Después".

## Explorar con datos personales

- `/api/explore`: 64 ms la primera vez (arma el catálogo), 2 ms después. Después de un ♥: **12 ms** (recarga solo
  los datos personales; el catálogo no se rearma). `megusta=si` encontró el álbum.
- `ver=pistas&orden=escuchado&limite=500` sobre las 6.685 pistas: 24 ms.

## Mudanza (copia de prueba aparte)

Un álbum (*Silvia Barrios - Itiyuro*, 11 pistas) copiado a una raíz descartable, servido por otra instancia. Con ♥
y ★4 en el álbum, ♥ y ★5 en una pista, una reproducción y una lista de dos pistas: se renombró la carpeta y se la
movió a una subcarpeta (`Argentina/Itiyuro (Silvia Barrios)`) y se volvió a escanear.

- Ids nuevos (pista 1 → 12, álbum 1 → 2) y **todo siguió**: ♥ y ★ del álbum y de la pista, Me gusta en Listas,
  `megusta=si`, `puntaje=4`, la reproducción ("Más escuchado") y las dos entradas de la lista, que se reproduce.
- Aparte (no es de M4a): en ese álbum varios `.opus` de YouTube tienen como título del tag el nombre del álbum
  ("Itiyuro"), no el del tema; el nombre del archivo trae el bueno.

## Teléfono (375 px)

- Álbum: ♥ y ★ arriba de las acciones, se ven bien. Listas: listas vacías con la tarjeta "Nueva lista", Me gusta
  con sus cuatro pestañas.
- **Hallazgo 2 — filas de pistas apretadas:** con ♥, duración, + y ⋯ en cada fila, el título queda en ~16
  caracteres ("Yo quiero fumar…"). En el teléfono el + sobra: "A la cola" ya está en el menú ⋯.

## Queda para el usuario

- Escuchar seguido un disco fuerte y uno más bajo (pero no menos de −18 LUFS) y comparar con `"normalize": false`.
- Arrastrar para reordenar una lista con el dedo en un teléfono de verdad (con LAN, en M5; antes, con el
  emulador táctil del navegador).
