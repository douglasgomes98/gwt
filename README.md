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

# imprime o caminho do worktree atual
gwt open AG-123 -p

# abre um subshell no worktree
gwt open AG-123

# remove o worktree do repositório atual, sem confirmação
gwt rm AG-123
```

`gwt open` não consegue mudar o diretório do shell que o chamou. Por isso ele
abre um subshell no worktree; ao sair, você volta ao diretório anterior.

## Comandos

| Comando | Descrição |
| --- | --- |
| `gwt` | Abre a TUI. |
| `gwt add <branch> [base] [--all] [-e\|-a]` | Cria um worktree. `--all` cria nos repositórios irmãos. |
| `gwt open <branch> [-e\|-a\|-p]` | Abre subshell (padrão), editor, agent ou apenas imprime o caminho. |
| `gwt rm <branch>` | Remove forçadamente o worktree do repositório atual. O checkout principal nunca é removido. |
| `gwt list` | Lista os worktrees do repositório atual. |
| `gwt prune` | Executa `git worktree prune` nos repositórios descobertos. |
| `gwt help` | Mostra a ajuda da CLI. |
| `gwt version` | Mostra a versão do binário. |

As flags de abertura são mutuamente exclusivas:

- `-e`: usa o editor configurado.
- `-a`: usa o agent configurado.
- `-p`: imprime o caminho, sem abrir processo.

## TUI

| Tecla | Ação |
| --- | --- |
| `n` | Informa a branch, seleciona os repositórios e cria os worktrees. |
| `Enter` | Abre um subshell no worktree selecionado. |
| `e` / `a` | Abre editor ou agent no worktree selecionado. |
| `d`, depois `y` | Remove todos os worktrees da branch selecionada. A confirmação é obrigatória. |
| `p` | Executa prune nos repositórios descobertos. |
| `j` / `k` ou setas | Move a seleção. |
| `q` | Sai. |

Ao criar worktrees pela TUI, o `gwt` atualiza a branch base dos repositórios
selecionados antes da criação. A tela inicial usa o estado local para abrir
imediatamente, sem esperar pela rede.

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
