# gwt

`gwt` gerencia Git worktrees em uma pasta com vários repositórios. É útil
quando um ticket precisa da mesma branch em mais de um projeto.

Execute `gwt` na pasta que agrupa os repositórios ou dentro de um deles. A
interface mostra os worktrees por branch, com estado dirty e ahead/behind.

## Instalação

Com Go instalado:

```sh
go install github.com/douglasgomes98/gwt/cmd/gwt@latest
```

Para desenvolver localmente:

```sh
git clone https://github.com/douglasgomes98/gwt.git
cd gwt
make install
```

`go install` coloca o binário em `GOBIN` ou em `GOPATH/bin`. Garanta que esse
diretório esteja no `PATH` do seu ambiente.

## Uso rápido

Considere esta estrutura:

```text
projects/
  api/
  web/
```

Dentro de `projects/`, execute `gwt` para abrir a TUI. Dentro de `api/`, os
comandos CLI operam naquele repositório; `--all` aplica a criação aos
repositórios irmãos.

```sh
# cria AG-123 no repositório atual
gwt add AG-123

# cria AG-123 em api e web
gwt add AG-123 --all

# abre um subshell no worktree
gwt open AG-123

# remove o worktree do repositório atual, sem confirmação
gwt rm AG-123

# remove AG-123 de todos os repositórios irmãos que tiverem essa branch
gwt rm AG-123 --all

# atualiza o checkout principal do repositório atual
gwt update

# volta o checkout principal para a branch base
gwt checkout-base

# descarta todas as mudanças locais do checkout principal
gwt discard
```

`gwt open` não consegue mudar o diretório do shell que o chamou. Por isso ele
abre um subshell no worktree; ao sair, você volta ao diretório anterior.

## Comandos

| Comando | Descrição |
| --- | --- |
| `gwt` | Abre a TUI. |
| `gwt add <branch> [base] [--all] [-e\|-a]` | Cria um worktree. `--all` cria nos repositórios irmãos. |
| `gwt open <branch> [-e\|-a]` | Abre subshell (padrão), editor ou agent. |
| `gwt rm <branch> [--all]` | Remove forçadamente o worktree atual ou a mesma branch dos repositórios irmãos. O checkout principal nunca é removido. |
| `gwt list` | Lista os worktrees do repositório atual. |
| `gwt prune` | Executa `git worktree prune` nos repositórios descobertos. |
| `gwt update` | Atualiza o checkout principal limpo, na branch base, do repositório atual. |
| `gwt checkout-base` | Faz checkout da branch base no checkout principal limpo do repositório atual. |
| `gwt discard` | Descarta todas as mudanças locais do checkout principal: rastreadas, não rastreadas e ignoradas. |
| `gwt help` | Mostra a ajuda da CLI. |
| `gwt version` | Mostra a versão do binário. |

As flags de abertura são mutuamente exclusivas:

- `-e`: usa o editor configurado.
- `-a`: usa o agent configurado.

## TUI

| Tecla | Ação |
| --- | --- |
| `Space` | Seleciona um checkout principal ou uma feature. A primeira seleção de feature marca todos os seus worktrees; os próximos toques alternam só a linha focada. Checkouts detached não são selecionáveis. |
| `Enter` | Abre a palette contextual. Em roots ela mostra `add`, `add --all`, `prune` e `update`; em features mostra `open`, `open -e`, `open -a`, `rm`, `rm --all` e `prune`, conforme a seleção e a configuração. Escolher `add` abre o prompt de branch. |
| `j` / `k` ou setas | Move o foco da lista ou da palette. |
| `Esc` | Fecha a palette sem limpar a seleção. |
| `q` | Sai. |

A TUI mantém a seleção e a escolha contextual de comando antes de executar
operações de Git. Durante criação, remoção, limpeza ou atualização, ela mostra
um indicador de progresso até a operação terminar.

## Configuração

Crie `gwt.yml` na pasta em que executar o comando ou
`~/.config/gwt/config.yml`:

```yaml
layout: sibling
baseBranch: main
editor: code
agent: claude
```

Todos os campos são opcionais. Os valores padrão são `sibling`, `main`, `code`
e `claude`.

### Layouts

| Layout | Destino para `api` e branch `AG-123` |
| --- | --- |
| `sibling` (padrão) | `../api.AG-123` |
| `grouped` | `../api.worktrees/api.AG-123` |
| `inside` | `api/.worktrees/AG-123` |

O layout `inside` não altera o `.gitignore`; adicione `.worktrees/` se não
quiser que ela apareça como conteúdo não rastreado no checkout principal.

## Desenvolvimento

```sh
make test
make build
make install
make version
```

Os testes criam repositórios Git temporários e exercitam criação, remoção e a
proteção do checkout principal.

## Segurança da remoção

`gwt rm` é deliberadamente não interativo e usa remoção forçada, como o fluxo
de aliases que ele substitui. Revise alterações não commitadas antes de
confirmar uma remoção na TUI ou executar `gwt rm`.

## Licença

[MIT](LICENSE).
