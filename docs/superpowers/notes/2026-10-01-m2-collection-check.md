# M2 contra la colección real (2026-10-01)

Corrida de `tordo` sobre `/Users/martin/Music` (catálogo M1 migrado a v2), contra MusicBrainz, Cover Art Archive y los
enlaces de MusicBrainz a Discogs. Sin token de Discogs.

## Números

- Primera identificación completa (arranque + identificar + enriquecer): ~21 min 40 s (1299 s). Identificar 460
  álbumes ~17 min; enriquecer 172 releases + artistas ~4 min. No se contaron las peticiones (el servidor no las registra).
- 460 álbumes: **174 identificados (auto, 38 %)**, **280 sin identificar (unmatched, 61 %)**, **6 selección**,
  0 ignorados, 0 manuales (antes de la prueba de corrección).
- De los 280 sin identificar: 30 sin ningún candidato; 34 con un primer candidato de puntaje >= 0,65 (26 >= 0,70), casi
  todos visiblemente correctos.
- Segunda corrida de `POST /api/identify`: `toIdentify: 0`, terminó en segundos, sin peticiones nuevas.
- Rescan con corrección manual: el álbum sigue `manual`; `DELETE` lo devuelve a la cola (unmatched, 0,76).
- La colección no se modificó (`find -newer` vacío).

## Falsos positivos

- Revisados 20 aleatorios + los 20 de menor confianza (0,80–0,86): ninguno incorrecto.
- Casos con diferencias legítimas: *Macumba* (MusicBrainz acredita a varios artistas), *Steamin'* (artista "Miles Davis
  Quintet"), *Bon Iver* ("Bon Iver, Bon Iver"), *Neo* (artista "HA Duo").
- No se cambió `BuildQuery` ni el puntaje; `MatcherVersion` sigue en 1.

## Pendiente de M1

- *Naturaleza Sangre*: identificado (1,0), título de MusicBrainz.
- *Casi en privado*: queda en `selection`, pero el artista es "Parte" (basura de tags), no Alfredo Zitarrosa. Es del
  agrupador de M1.
- *FITO PAEZ - 2004 - mi vida con ellas 1*: identificado (0,85) por la variante de carpeta; título "Mi vida con ellas".
- Títulos truncados por ID3v1: *La Ley es Tela de Araña y Melo* queda sin identificar (candidato correcto
  *Melodía larga*, 0,76); al elegirlo a mano los títulos de pista salen de MusicBrainz.
- *História da MPB*: 48 volúmenes, todos sin identificar y **no** son compilaciones. Cada volumen figura con el
  compositor como artista y MusicBrainz tiene otra serie ("Nova História da Música Popular Brasileira", 0,6–0,7),
  que no es la misma edición. Dato de MusicBrainz, no un bug.
- *Raras Partituras* (15): ninguno encontrado (títulos de tag son códigos EPSA). Probablemente no están en MusicBrainz.
- `baulera`: selection. `voces`: 8 sin identificar. `infantiles`: 10 identificados, 29 sin identificar, 1 selección.

## Géneros

- Valores crudos sin mapear: 88 (de 207 en M1). Fuentes por álbum: discogs 100, mb 12, tag 186.
- Se agregaron alias integrados con `TestMap`: canciones de cuna, progressive rock, psych, scores de películas, poesía,
  tango canción / latino, música popular latinoamericana, "género desconocido" (ignorado).
- Quedan para el usuario: MPU / Música uruguaya / Música nacional, Vals, Cueca, Bailecito, Chamarrita, Yaraví,
  Highlife, Soukous, Calypso, Singer-Songwriter, "World Folk", "Art Pop", y combinaciones largas (`Mpb;Samba;...`).
- `PUT /api/genres/aliases` probado con "Cueca": el álbum pasó a listar Folk, World, & Country / Folk, y los sin mapear
  bajaron de 88 a 87. No hay `DELETE` de alias (la prueba se borró a mano de la base).

## Países y portadas

- Países: AR 42, US 33, BR 29, UY 26, GB 13, CO 9, CA 4 (AR, UY, BR, US lideran, GB y CO también).
- Portadas: de 61 álbumes sin portada, 15 la tienen ahora (Cover Art Archive). `/api/albums/{id}/cover` responde 200 image/jpeg.

## Para M3 y más

- **Decisión de diseño pendiente:** 34 álbumes sin identificar con candidato correcto a 0,65–0,79 (p. ej. *Monk's Dream*
  0,78, *Os Originais do Samba* 0,79, *Psiglo - Ideación* 0,78, *Sansueña* 0,5). Pierden puntos por año (tag de
  reedición) y duraciones de otra edición. No se bajó el umbral (0,80): preferible revisarlos en la cola; M3 podría
  ofrecer "aceptar el primer candidato" en lote para los >= 0,70.
- Alineación de pistas por posición: ~10 de 2042 pistas (por ejemplo en los álbumes 17, 87, 176, 249) quedan con un
  título de MusicBrainz distinto del tag; la duración dentro de 3 s no basta. Considerar exigir además similitud de
  título cuando el título del tag no es un placeholder ("Pista N").
- Consulta: el artista de la carpeta padre no se usa (*La ciudad liberada*, tag de artista basura, está bajo
  `Fito Páez/`): sin identificar con candidatos absurdos.
- Artistas con tags basura o múltiples ("Carmen Sandiego", "Les Mains Noires", Zitarrosa en discos poco catalogados):
  gran parte simplemente no está en MusicBrainz.
- Nombres NFC/NFD en la búsqueda, portadas embebidas y fotos de artista siguen pendientes.
