// Help content in Brazilian Portuguese. Same structure, ids, keys, links and
// flags as es.js: only title, term and text are translated.
export const SECTIONS = [
  {
    id: 'shortcuts',
    title: 'Atalhos e comandos',
    items: [
      { keys: ['Ctrl+K', '⌘K'], text: 'Abre a busca em qualquer página. As setas e o Enter escolhem um resultado; Esc fecha.' },
      { keys: ['Space'], text: 'Toca ou pausa (a menos que você esteja digitando ou sobre um botão).' },
      { keys: ['/'], text: 'Em Pastas, vai para o filtro da árvore.' },
      { keys: ['Esc'], text: 'Fecha os menus e painéis abertos.' },
      { keys: ['?'], text: 'Abre esta ajuda.' },
      { term: 'Player', text: 'Aleatório, anterior, tocar ou pausar, próxima e repetir (não, toda a fila ou esta faixa).' },
      { term: 'Posição e volume', text: 'Uma barra para avançar ou voltar e outra para o volume, com botão de silenciar. Os formatos que são convertidos na hora não podem ser avançados.' },
      { term: 'Fila', text: 'O botão da fila mostra o que vai tocar; ali você pode remover faixas e reordená-las arrastando (ou com Alt + setas).' },
      { term: 'Nota', text: 'Com o foco nas estrelas, as setas mudam a nota.' },
    ],
  },
  {
    id: 'sections',
    title: 'Seções',
    items: [
      { term: 'Início', href: '/', text: 'Linhas para retomar: suas estações, continuar ouvindo, suas músicas favoritas e seus álbuns preferidos, mais sugestões (como adicionados recentemente) que mudam, assim como a ordem, a cada visita ou com o botão de atualizar.' },
      { term: 'Explorar', href: '/explore', text: 'Toda a coleção por álbuns, artistas ou faixas, com filtros combináveis (década, país, gênero, formato, ♥, ★…) e ordenação, inclusive aleatória.' },
      { term: 'Pastas', href: '/folders', text: 'A coleção como está no disco, com uma árvore de pastas.' },
      { term: 'Listas', href: '/lists', text: 'Suas listas de reprodução e seus ♥.' },
      { term: 'Revisar', href: '/review', blockedInListen: true, text: 'Manutenção: músicas não identificadas, duplicados, gêneros para mapear e formatos para converter.' },
      { term: 'Configurações', href: '/settings', text: 'Pastas de música, token do Discogs, volume, áudio deste computador, modo ouvir e rede local.' },
    ],
  },
  {
    id: 'concepts',
    title: 'Conceitos',
    items: [
      { term: 'Estações', text: 'Uma fila sem fim com músicas parecidas com o que você escolheu (um artista, um país, um gênero, uma pasta, seus ♥ e seus álbuns preferidos…). O Tordo a alimenta de dez em dez faixas, varia os artistas e dá mais peso ao que você gosta. Você pode salvar uma estação e vê-la no Início.' },
      { term: '♥, marcador e ★', text: 'O ♥ marca suas músicas favoritas (e os artistas e coleções de que você gosta); o marcador marca álbuns preferidos, uma lista curta para voltar; a ★ dá nota de 1 a 5 a faixas e álbuns. Servem para filtrar em Explorar (em Álbuns, "Preferidos"), para montar estações e pesam nelas.' },
      { term: 'Duplicados', href: '/review/duplicates', blockedInListen: true, text: 'O Tordo os mostra, nunca os apaga. Exatos: o mesmo áudio em dois ou mais caminhos (mostra o espaço recuperável). Formatos: a mesma gravação em vários formatos dentro de um álbum, com a marca MELHOR na de maior qualidade. Suspeitas: mesmo artista e título em álbuns diferentes; é informativo e costuma ser legítimo (uma faixa do disco e de uma coletânea).' },
      { term: 'Modo ouvir', href: '/settings', text: 'Uma opção das Configurações que oculta Revisar e a curadoria e bloqueia identificações, varreduras e mudanças de gêneros. Os ♥, as ★, as listas e as estações continuam funcionando. É útil para deixar a música para outras pessoas.' },
      { term: 'Jukebox', href: '/settings', text: 'No computador que roda o Tordo, ative "Usar o áudio deste computador para o Tordo" em Configurações → Reprodução e deixe essa aba aberta: o som sai ali. De um celular da rede, o botão 🎵 Jukebox da barra superior faz o player controlar o computador em vez do celular. Se você recarregar essa aba, um clique em "Reativar o áudio" devolve o som.' },
      { term: 'Anfitriões e convidados', href: '/settings#network', text: 'O computador é sempre anfitrião; os demais dispositivos são convidados, a menos que você os marque como "Anfitrião da jukebox" em Configurações → Rede local. Os anfitriões controlam tudo. Os convidados só contribuem: ▶ adiciona à fila (e começa a tocar se a fila estava vazia ou terminada), e podem pausar e remover apenas o que adicionaram.' },
    ],
  },
  {
    id: 'how-it-works',
    title: 'Como funciona: varredura e identificação',
    items: [
      { term: 'Quando faz a varredura', text: 'Ao abrir o Tordo, sempre que você muda as pastas de música ou as pastas a ignorar nas Configurações e, a cada hora, para procurar mudanças no disco (o intervalo é escolhido nas Configurações e pode ser desativado). Essa verificação compara tamanhos e datas sem abrir os arquivos: se não encontra diferenças, termina ali e você nem nota; se encontra, segue como uma varredura normal. A varredura é incremental: só lê os arquivos novos ou modificados (por tamanho e data), então as seguintes são muito mais rápidas que a primeira. Enquanto isso, a barra superior mostra em que etapa está: "Procurando arquivos" (o percurso), "Lendo arquivos" com quantos já leu e "Montando álbuns"; ao passar o mouse ou tocar nesse indicador, abre-se um painel com as etapas, o progresso, quanto falta e os números do catálogo. A música aparece à medida que é lida: o catálogo é remontado a cada meio minuto e as páginas se atualizam sozinhas.' },
      { term: '1. Percurso', text: 'O Tordo percorre as pastas procurando áudio (MP3, FLAC, M4A, Ogg, Opus, WMA, WAV, AIFF, APE, WavPack), imagens de capa, letras e listas M3U. Pula as pastas a ignorar e as de outros aplicativos (Ableton, djay, Apple Music, downloads incompletos do Soulseek). Se um disco não está conectado, deixa-o de fora e mantém o que já conhecia; um arquivo que não está mais lá é marcado como ausente, não esquecido.' },
      { term: '2. Leitura', text: 'De cada arquivo novo lê as tags (título, artista, álbum, número da faixa, ano, gêneros, capa incluída) e os dados técnicos (formato, duração, qualidade), e calcula uma impressão digital do áudio. A impressão digital ignora as tags: um arquivo movido, renomeado ou re-etiquetado continua sendo o mesmo, e duas cópias idênticas são reconhecidas como duplicados. Se o ffprobe estiver instalado, lê também os formatos que o Tordo não lê por conta própria. Um arquivo ilegível é tentado de novo na próxima varredura.' },
      { term: '3. Álbuns', text: 'Monta os álbuns com as tags e, quando faltam, com os nomes de pastas e arquivos ("Artista - Álbum (1985)", "CD1"). Os discos de um mesmo álbum em subpastas são reunidos; algumas faixas com uma tag de álbum diferente são somadas ao álbum da sua pasta; uma pasta de faixas soltas de álbuns diferentes fica como uma seleção com o nome da pasta. Escolhe a capa (cover, folder ou front, senão a maior da pasta) e detecta as coleções: pastas com vários álbuns de artistas diferentes.' },
      { term: '4. Identificação', text: 'Ao terminar cada varredura, o Tordo busca cada álbum no MusicBrainz, uma consulta por segundo, como o MusicBrainz pede (com milhares de álbuns, a primeira vez leva horas; se for interrompida, continua de onde parou). A barra superior mostra "Identificando" e quantos já foram. Se as tags já trazem o identificador do MusicBrainz, ele o usa. Se não, busca por título e artista (também como a pasta os nomeia e sem notas entre parênteses) e pontua cada candidato: título 30%, durações das faixas 25%, artista 20%, quantidade de faixas 15% e ano 10%. Com 80% ou mais, identifica sozinho; senão, fica em Revisar → Não identificados com até cinco candidatos para escolher.' },
      { term: '5. Dados que acrescenta', text: 'Um álbum identificado mostra os títulos do MusicBrainz (também de cada faixa), o ano original, o país do artista e, se a pasta não tem capa, a do Cover Art Archive (guardada em cache/). Enquanto busca esses dados, a barra superior mostra "Completando dados"; quando termina, "Em dia". Os gêneros e estilos vêm do Discogs: pelos links que o MusicBrainz tem e, com um token do Discogs nas Configurações, também buscando. Sem Discogs, do MusicBrainz; sem identificação, das tags. Os gêneros das tags que o Tordo não reconhece ficam em Revisar → Gêneros para mapear.' },
      { term: 'Suas correções', text: 'O que você escolhe à mão (em Não identificados ou com "Corrigir identificação" na página do álbum) é guardado pela impressão digital de cada faixa: sobrevive se você mover, renomear ou re-etiquetar os arquivos, e a identificação automática nunca o sobrescreve. Nada disso mexe nos seus arquivos: as tags não são reescritas.' },
      { term: 'Sem conexão', text: 'A varredura não usa a rede. Se não há conexão, a identificação é pausada (a barra mostra "Sem conexão com o MusicBrainz") e tenta de novo sozinha, cada vez mais espaçada; todo o resto funciona igual. O que já foi identificado não é buscado de novo.' },
    ],
  },
  {
    id: 'guidelines',
    title: 'Orientações de uso',
    items: [
      { term: 'Seus arquivos', text: 'O Tordo nunca escreve, move, renomeia nem apaga seus arquivos de áudio. Só escreve na sua própria pasta: tordo.db (o catálogo), config.json (as configurações) e cache/.' },
      { term: 'Portabilidade', text: 'O catálogo guarda caminhos relativos à pasta do Tordo, então você pode mover a pasta inteira (por exemplo para outro disco) e tudo continua funcionando.' },
      { term: 'De outros dispositivos', href: '/settings#network', text: 'Se você ativar a rede local nas Configurações, pode entrar de um celular ou tablet da mesma rede. Cada dispositivo pede o PIN que você escolheu; trocá-lo encerra todas as sessões. Para tocar nas caixas de som do computador, use a Jukebox.' },
      { term: 'Configurações', text: 'Só podem ser alteradas a partir do computador que roda o Tordo; de outro dispositivo é possível ouvir e consultar, mas não configurar.' },
    ],
  },
]
