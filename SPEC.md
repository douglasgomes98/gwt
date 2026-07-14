# Especificação de comportamento

Esta é a tabela viva de regras do `gwt`. Atualize primeiro este documento; a
implementação deve seguir somente as regras marcadas como definidas.

Status: `Definido` é uma regra aprovada para implementação. `Adiado` fica fora
da implementação atual e será revisado depois.

## Compatibilidade

O projeto está em v0. Não há compromisso de retrocompatibilidade: cada regra
em **Novo definido** substitui o comportamento atual, inclusive comandos,
flags, formatos de saída, arquivos de configuração e mensagens de erro. A
versão estável só será publicada quando esta especificação estiver consolidada.

## Entrada, versão e configuração

| Funcionalidade | Atual | Novo definido | Status |
| --- | --- | --- | --- |
| `gwt` sem argumentos | Abre a TUI. | Mantém. | Definido |
| Ajuda | `gwt help` lista comandos atuais e rejeita argumentos extras. | Incluir `update` e `version`; continuar rejeitando extras. | Definido |
| Versão | Aceita `--version` e `-version`. | Aceitar somente `gwt version`; flags de versão deixam de ser aceitas. | Definido |
| Comando desconhecido | Retorna erro com prefixo `gwt:` no binário. | Mantém. | Definido |
| Localização da configuração | Primeiro `./gwt.yml`; se ausente, `~/.config/gwt/config.yml`; não busca diretórios pais. | Mantém. | Definido |
| Arquivo ausente | Segue para a próxima localização ou usa defaults. | Mantém. | Definido |
| Arquivo presente, ilegível | É ignorado e pode cair em defaults. | Falha com erro contextual. | Definido |
| YAML inválido | É ignorado silenciosamente. | Falha com erro contextual. | Definido |
| Chave desconhecida | É ignorada pelo decoder YAML. | Falha; schema estrito. | Definido |
| Tipo inválido | Pode resultar em configuração parcialmente carregada. | Falha; schema estrito. | Definido |
| `layout` | `sibling`, `grouped`, `inside`; outro valor acaba usando `sibling`. | Campo presente deve ser exatamente `sibling`, `grouped` ou `inside`; vazio ou inválido falha. | Definido |
| `baseBranch` | Ausente ou vazia vira `main`. | Ausente vira `main`; presente deve ser uma branch Git válida e não vazia. | Definido |
| `editor` | Ausente usa `code`; vazio atualmente desabilita na prática. | Opcional: ausente usa `code`; vazio desabilita editor. | Definido |
| `agent` | Ausente ou vazio causa erro apenas ao tentar abrir agent. | Ausente usa `claude`; vazio desabilita agent. | Definido |
| Executáveis de editor/agent | Só são verificados ao tentar executar. | Mantém; a validação de configuração não consulta `PATH`. | Definido |

## CLI

| Comando | Atual | Novo definido | Status |
| --- | --- | --- | --- |
| `gwt add <branch> [base]` | Cria no root atual a partir da base local; sem fetch. | Mantém. | Definido |
| Base de `add` | Usa `[base]` ou `baseBranch`. | Sempre referência local, inclusive na TUI e com `--all`. | Definido |
| `gwt add --all` | Para cada root irmão, tenta `fetch origin <base>` e ignora erro; depois cria usando a base local. | Não acessa rede; cria sequencialmente em todos os roots irmãos. | Definido |
| Falha em `add --all` | Para no primeiro erro; anteriores ficam criados; sem rollback. | Mantém; a mensagem deve informar que o resultado pode ser parcial. | Definido |
| Branch existente | Anexa o worktree à branch existente. | Mantém. | Definido |
| Branch nova | Cria branch e worktree a partir da base. | Mantém. | Definido |
| `add --all` / `-e` / `-a` | As combinações são aceitas; `-e` vence e, com `--all`, abre somente o primeiro path. | Formam um único grupo mutuamente exclusivo: é aceito no máximo um deles; qualquer combinação retorna erro de uso. | Definido |
| Posição das flags | Flags conhecidas funcionam antes ou depois dos argumentos. | Mantém: `add` aceita `--all`, `-e` e `-a`; `open` aceita `-e` e `-a`; `rm` aceita `--all`. | Definido |
| `gwt open <branch>` | Procura no repo atual; sem modo abre shell. | Mantém. | Definido |
| `gwt open -e` / `-a` | Abre editor/agent; se combinadas, há precedência implícita. | `-e` e `-a` são mutuamente exclusivas; combinação retorna erro de uso. | Definido |
| Worktree inexistente em `open` | Erro de branch não encontrada. | Mantém. | Definido |
| `gwt rm <branch>` | Remove forçadamente, sem confirmação. | Mantém para o repositório atual. | Definido |
| `gwt rm <branch> --all` | Não existe. | Remove a mesma branch de todos os repos irmãos em que ela existir; ausência da branch em um irmão não é erro. | Definido |
| Falha em `rm --all` | — | Se a branch não existir em nenhum repo, falha. Após as validações, remove sequencialmente e para no primeiro erro Git; remoções anteriores permanecem. | Definido |
| Proteção de root | Recusa remover checkout principal. | Mantém obrigatoriamente. Em `rm --all`, verifica todos os alvos antes de remover qualquer um e falha se a branch for um root. | Definido |
| `gwt list` | Imprime `<path>\t<branch>`. | Formato normativo: `<path>\t<branch>\t<status>`, com o mesmo status textual da TUI: `(clean)` ou `(N files changed · ahead N · behind N)`. | Definido |
| `gwt prune` | Poda todos os roots descobertos; argumentos extras são ignorados. | Não aceita argumentos ou flags; poda todos os roots descobertos e para no primeiro erro. | Definido |
| `gwt update` | Não existe. | Atualiza somente o root do repositório atual. Não há `--all`. | Definido |
| Pré-condições de `update` | — | Root deve estar limpo e em `baseBranch`; caso contrário falha antes de acessar a rede. | Definido |
| Operação de `update` | — | `fetch origin <baseBranch>` seguido de `merge --ff-only origin/<baseBranch>`. Nunca cria merge commit. | Definido |
| Falha de `update` | — | Falha para root dirty, branch diferente da base, remoto ausente, base remota ausente ou histórico não-fast-forward. | Definido |

## TUI: carregamento, navegação e seleção

| Funcionalidade | Atual | Novo definido | Status |
| --- | --- | --- | --- |
| Carregamento | Faz listagem rápida e detalhada em paralelo. | Mantém. | Definido |
| Status detalhado | Mostra dirty, número de arquivos alterados, ahead e behind. | Mantém. | Definido |
| Ordenação | Branch e depois repositório. | Mantém. | Definido |
| Navegação | `j` ou seta para baixo avança e `k` ou seta para cima recua; não altera seleção. | Navega worktrees; com a palette aberta, navega os comandos dela. | Definido |
| Sair | `q` ou `Ctrl+C`. | Mantém. | Definido |
| `Esc` fora de input | Limpa grupo de feature e roots marcados. | Fecha a palette sem alterar a seleção; sem palette aberta, limpa grupo de feature e roots marcados. | Definido |
| Seleção de root | Espaço marca o checkout principal quando ele está em `baseBranch`. | Marca ou desmarca o root; ao marcar qualquer root, limpa a feature selecionada. Ações em lote usam somente os roots marcados. | Definido |
| Seleção de feature | Espaço seleciona uma única branch de feature; trocar de grupo exige `Esc`. | Seleciona todos os worktrees da mesma branch; ao selecioná-la, limpa todos os roots marcados. Só uma feature pode estar ativa. | Definido |
| Ajuste da seleção de feature | Não existe: o grupo inteiro é selecionado ou limpo. | Espaço alterna o worktree sob o cursor dentro da feature ativa; se o último for desmarcado, a seleção de feature é limpa. | Definido |
| Seleções simultâneas | Roots marcados e grupo de feature podem coexistir. | Não permitidas: a seleção de roots e a de feature são mutuamente exclusivas. | Definido |
| Comandos contextuais | A TUI tem ações próprias. | Usa os nomes dos comandos CLI, mas só mostra os habilitados para a seleção atual; o escopo da seleção prevalece sobre o escopo padrão da CLI. | Definido |
| Palette de comandos | Não existe. | Enter abre a palette para a seleção atual; setas ou `j`/`k` escolhem um comando e Enter o executa. | Definido |
| `help`, `version` e `list` na TUI | Não existem como ações. | Não são exibidos: a própria tela já lista worktrees e mostra os comandos habilitados. | Definido |
| Escopo de lote na TUI | Operações usam todos os repos descobertos ou o grupo de feature. | `--all` e `prune` usam somente os itens selecionados; nenhum repo não selecionado é alterado. | Definido |
| Pós-comando | Alguns comandos recarregam a lista; a seleção pode permanecer. | Ao concluir uma ação, com sucesso ou falha, fecha a palette, recarrega a lista, limpa todas as seleções e recalcula os comandos habilitados. | Definido |
| Cores | Desativadas por `NO_COLOR` ou `TERM=dumb`. | Mantém. | Definido |
| Erro de carga | Mostra o erro, com proteção contra resposta rápida atrasada sobrescrever carga detalhada. | Mantém. | Definido |

## TUI: criar, abrir e remover

| Funcionalidade | Atual | Novo definido | Status |
| --- | --- | --- | --- |
| `add` sem roots marcados | `n` mostra orientação para selecionar projetos. | Não é exibido na palette. | Definido |
| `add` com roots marcados | `n` solicita nome da branch. | Solicita nome da branch; com um root executa `add`, com vários executa `add --all` restrito aos roots selecionados. | Definido |
| Input de branch | O texto digitado é renderizado no fim da tela, depois dos atalhos. | É renderizado imediatamente após `branch:`, antes dos comandos contextuais. | Definido |
| Criar pela TUI | Para cada root: tenta fetch, ignora erro e cria de `origin/<baseBranch>`. | Não faz fetch; cria da `baseBranch` local somente nos roots selecionados. | Definido |
| Falha parcial na criação | Para no primeiro erro, sem reload nesse caminho. | Para no primeiro erro, informa parcial e recarrega. | Definido |
| `open` | Enter abre shell no worktree sob cursor; se cursor estiver fora do grupo, abre o primeiro do grupo. | Só fica habilitado na palette quando a seleção contém exatamente um worktree. | Definido |
| `open -e` / `open -a` | `e` e `a` abrem editor/agent no worktree ativo. | Só ficam habilitados na palette com exatamente um worktree selecionado e comando configurado. | Definido |
| Abrir com seleção vazia ou múltipla | Não faz nada sem feature; com grupo abre um item ativo. | Não exibe comandos `open` enquanto a seleção não tiver exatamente um worktree. | Definido |
| `rm` | `d` pede confirmação e remove o grupo de feature. | Fica habilitado na palette com exatamente um worktree de feature selecionado; pede confirmação antes de remover. | Definido |
| `rm --all` | Não existe como comando explícito; `d` remove o grupo inteiro. | Fica habilitado na palette com dois ou mais worktrees da mesma feature selecionados; pede confirmação e remove somente os selecionados. | Definido |
| Proteção de root na remoção | Não oferece remoção para grupo contendo root. | Roots não podem integrar seleção de feature nem habilitam `rm`; a proteção permanece obrigatória. | Definido |
| Falha parcial na remoção | Pode remover parte do grupo e parar no primeiro erro. | Mantém; informa parcial e recarrega. | Definido |

## TUI: manutenção

| Funcionalidade | Atual | Novo definido | Status |
| --- | --- | --- | --- |
| `prune` | Executa prune em todos os roots e ignora erros. | Fica habilitado na palette e executa nos repositórios únicos da seleção; para no primeiro erro, informa resultado parcial e recarrega. | Definido |
| Atualização de feature | `u` faz fetch e merge normal em todos os worktrees da feature; pode gerar merge commit. | Removida. | Definido |
| `update` com roots marcados | Não existe. | Fica habilitado na palette e atualiza os roots marcados, sequencialmente, com a mesma regra segura da CLI. | Definido |
| `update` sem roots marcados | `u` não faz nada se não houver grupo. | Não é exibido na palette. | Definido |
| Falha parcial de `update` | — | Para no primeiro root com falha, sem rollback, informa parcial e recarrega. | Definido |

## Git, paths e casos especiais

| Funcionalidade | Atual | Novo definido | Status |
| --- | --- | --- | --- |
| Root a partir de worktree | Resolve checkout principal via `git worktree list`. | Mantém. | Definido |
| Descoberta de irmãos | Examina o diretório pai do root e inclui diretórios com `.git` como diretório. | Mantém. | Definido |
| Layout `sibling` | `../api.AG-123`. | Mantém. | Definido |
| Layout `grouped` | `../api.worktrees/api.AG-123`. | Mantém. | Definido |
| Layout `inside` | `api/.worktrees/AG-123`. | Mantém. | Definido |
| Checkout detached | Branch fica vazia na listagem e TUI. | Exibir como `(detached)`, com status visível. | Definido |
| Ações em detached | Comportamento ambíguo. | Bloquear abertura, criação, atualização e remoção que usem esse checkout como alvo; permitir somente listagem e status. | Definido |
| Diretórios criados antes do Git | Pode deixar `.worktrees/` ou diretório agrupado vazio se Git falhar. | Manter por enquanto; não é uma regra de produto nesta etapa. | Adiado |
