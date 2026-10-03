# M5 contra la colección real — hallazgos

Colección: `/Users/martin/Music` (solo lectura). Fecha: 2026-10-03. Rama `m5-revision-red`. Catálogo: una copia del
de `.run` en `x-projects/tordo-m5-check/run` (mismas rutas relativas); el original no se tocó.

## Migración

- La copia abrió y pasó a v6 (sesiones) sin escanear de nuevo.

## Duplicados

- 79 copias idénticas (1,2 GB recuperables), 15 grupos de un mismo tema en varios formatos, 137 sospechas.
- Ejemplo de copia idéntica: *Beyond* de Herb Alpert, el mismo FLAC 24/88,2 en la edición de 1980 y en la de 2020.
- `/api/review/duplicates`: 56 ms la primera vez (arma el catálogo), menos de 1 ms después.
- `estado=con-duplicados` en Explorar: 26 álbumes.
- Revisado en el navegador por el usuario: "se ve bien".

## Formatos a convertir

- 54 archivos WMA (0,24 GB). Ningún ALAC: los 33 `.m4a` sin pérdida que la nota de M1 contaba como ALAC son FLAC
  dentro de MP4 (`fLaC`, confirmado con ffprobe), así que no van en la lista. La nota de M1 estaba mal.

## Pendiente de M4b (su revisión nunca se hizo)

- `/api/station/next` sobre toda la colección: 9 ms; Brasil en modo álbumes: 2 ms (un álbum entero, 11 pistas).
- `/api/home`: 1 ms. `/api/stats?periodo=siempre`: menos de 1 ms.
- No se escucharon estaciones a oído ni se revisaron las filas de Inicio con el usuario.

## Encontrado y corregido durante M5

- La verificación del token de Discogs aceptaba cualquier token (`/releases/{id}` es público); ahora usa
  `/oauth/identity`, que responde 401 a un token malo.
- La escucha en la red abría `[::]:puerto` (también IPv6 global); ahora solo `0.0.0.0` en IPv4 y la puerta rechaza
  direcciones que no son de la red local.
- Las sesiones con más de 90 días sin uso seguían valiendo; ahora vencen.

## Desde el teléfono (pendiente)

- Entrar con el PIN escaneando el QR, un PIN equivocado, el bloqueo.
- Reproductor: pantalla bloqueada y controles de auriculares (Media Session, pendiente desde M3a).
- Arrastrar con el dedo para reordenar una lista (pendiente desde M4a).
- Carpetas al estilo iPod.
- Cerrar la sesión del teléfono desde Dispositivos en la computadora.
