# Plano: `gwt` — TUI Go para worktrees multi-repo (testável ponta a ponta, publicável)

## Context

O gerenciamento de worktrees hoje é feito por funções zsh no `~/.zshrc` (`gwa`/`gwo`/`gwd`/`gwl`, ~linhas 256-456). São boas, mas operam **um repo por vez** e são pessoais/hardcoded. O fluxo real é por **ticket**, que vira worktree em vários repos ao mesmo tempo (ex.: `AG-5517` → `bff-app`+`bff-cockpit`+`ui-app`; `AG-5234` → `api-share`+`ui-app`).

Objetivo: transformar isso numa **TUI com CLI, totalmente testável e publicável** (Homebrew). A TUI é uma camada visual sobre as mesmas operações da CLI; a instalação não depende de aliases ou configuração de shell. Dores a resolver (confirmadas):
1. **Multi-repo por ticket** — criar/abrir/remover a mesma branch em N repos de uma vez.
2. **Visibilidade global** — listar worktrees de todos os repos (branch, dirty, ahead/behind), agrupadas por branch.
3. **Remoção em massa** — apagar, mediante confirmação, os worktrees de uma branch em todos os repos.
4. **Layouts de worktree** — manter o padrão atual ou escolher onde os worktrees são criados.

**Decisões do usuário:**
- Stack **Go** — objetivo secundário: aprender a linguagem. Usar a stdlib onde ela basta, com `Bubble Tea v2` para o ciclo de eventos da TUI; `cmd/`+`internal/` é a estrutura mínima. O Makefile mantém os comandos locais `build`, `test`, `install` e `version`.
- **Totalmente testável ponta a ponta.**
- **Repo dedicado: `~/dev/personal/gwt`** (fora do agentguru).
- Escopo: **estruturar pronto-pra-publicar** — binário + README + testes. A fórmula Homebrew é preparada após existir uma release versionada; publicar o tap fica como passo manual final.

Vantagem do Go aqui: **binário estático único, zero runtime dep** → formula Homebrew só compila da fonte (`depends_on "go" => :build`). Distribuição mais limpa que Node.

## Estratégia de testabilidade (o requisito central)

Três camadas, todas com `testing` da stdlib:
1. **Funções puras** (parse de `worktree list --porcelain`, resolução de layout, seleção de repos-alvo e resumo dirty) → table-driven tests, sem I/O.
2. **Operações Git reais** em repositórios descartáveis criados com `t.TempDir()` → valida Add/Remove/List/Prune no disco, sem mocks da dependência central do produto.
3. **CLI e TUI** → a CLI testa argumentos, stdout/stderr e códigos de saída; o modelo da TUI recebe mensagens de teclado e resultados de operações como valores. Testa-se navegação, seleção e confirmação sem terminal real. Poucos casos críticos também compilam e invocam o binário.

## Estrutura do repo (`~/dev/personal/gwt`)

```
gwt/
  go.mod                      # module github.com/<user>/gwt ; go 1.26
  Makefile                    # build/test/install
  README.md  LICENSE          # MIT + instruções de instalação, TUI e compatibilidade CLI
  cmd/gwt/
    main.go                   # inicia TUI ou despacha subcomando; versão via -ldflags
  internal/
    cli/
      cli.go                  # flags, streams injetáveis e códigos de saída
      {add,open,rm,list}.go   # operações não interativas da TUI
    git/
      git.go                  # invocação fina de git via os/exec
    worktree/
      context.go              # deriva base_root/repo_name/sibling_root do cwd (porta do zsh)
      discover.go             # descobre repos irmãos + ParsePorcelain()
      manager.go              # Add/Remove/List/Prune sobre Git
      *_test.go               # testes puros e com repos temporários
    config/config.go          # load config + defaults (layout, baseBranch, editor, agent)
    tui/
      model.go                # estado, atalhos e telas
      operations.go           # tea.Cmd para carregar/fetch/criar/remover/abrir
      model_test.go           # navegação, seleção e confirmação
```

## CLI e experiência da TUI

Sem subcomando, `gwt` abre a TUI. A CLI continua disponível e é a fonte única das operações chamadas também pela TUI:

- `gwt add <branch> [base] [-e|-a]` cria no repo atual; `--all` habilita fan-out. As flags preservam os usos atuais de editor/agent.
- `gwt open <branch> [-e|-a|-p]` abre um subshell no worktree por padrão; `-e` abre editor, `-a` abre agent e `-p` imprime o path. Não tenta mudar o shell pai.
- `gwt rm <branch>` remove forçado, sem prompt, o worktree vinculado do repo atual; nunca o checkout principal.
- `gwt list` lista os worktrees do repo atual.

Se o cwd estiver num checkout Git (principal ou worktree), a TUI deriva seu `sibling-root`; se o cwd não for Git, usa o próprio cwd como pasta agrupadora. Nos dois casos, descobre apenas checkouts principais Git nos diretórios imediatos desse root; worktrees vinculados são ignorados.

A tela inicial agrupa worktrees por branch e mostra repo, path, dirty e ahead/behind. O carregamento tenta `git fetch origin <base>` em cada repo; uma falha deixa o estado visível como potencialmente desatualizado, sem impedir a navegação.

Atalhos da v1:
- `Enter`: abre um shell no worktree selecionado; a TUI é suspensa enquanto o shell estiver aberto e retorna ao sair. Isso substitui o impossível `cd` no shell pai.
- `e` / `a`: abre o editor ou agent configurado no worktree selecionado.
- `n`: pede branch, seleciona repos e cria em cada um após `git fetch origin <base>`, usando `origin/<base>`. Falhas parciais são reportadas, sem rollback.
- `d`: exibe todos os paths e indicadores dirty da branch selecionada; só `y` confirma `git worktree remove --force --force`. O checkout principal nunca é alvo.
- `p`: executa somente o `git worktree prune` nativo nos repos descobertos.
- `q`: sai.

As ações da TUI chamam os mesmos serviços internos usados pela CLI — não executam outro fluxo nem montam comandos por texto.

## Layout e config

`gwt.yml` / `~/.config/gwt/config.yml` (YAML) é opcional e contém somente defaults:
- `layout`: `sibling` (default, `<pai>/<repo>.<branch>`), `grouped` (`<pai>/<repo>.worktrees/<repo>.<branch>`) ou `inside` (`<repo>/.worktrees/<branch>`).
- `baseBranch` (`main`), `editor` (`code`) e `agent`.

O layout `inside` não modifica `.gitignore`; o usuário deve ignorar `.worktrees/` se não quiser vê-lo como arquivo não rastreado no checkout principal.

Nada de nome de repo agentguru hardcoded — root derivado do contexto git.

Setup pós-criação (cópia de `.env` e comandos por linguagem) fica fora da v1: exige uma política segura e portável para comandos, arquivos e falhas.

## Distribuição (estruturada, não publicada nesta entrega)

- **Homebrew:** após a primeira tag, a fórmula no tap pessoal aponta para o tarball da release e seu SHA256; compila com Go como dependência de build.
- `Makefile`: `build`, `test`, `install` e `version`. `build` usa `-trimpath` e injeta `main.version` com `git describe`; cross-compilação, GoReleaser e bottles entram quando houver release.

## Padrões adotados

- Portar `_gwt_repo_context_line` / `_gwt_resolve_existing_worktree_path` (`.zshrc:265-333`) para `internal/worktree/context.go` — base_root/sibling_root + fallback via `worktree list --porcelain`.
- Defaults atuais (`GWT_EDITOR=code`, `GWT_DEFAULT_BRANCH=main`, `GWT_AGENT`) viram defaults do `config`.
- `Bubble Tea v2` contém o loop de terminal e isola as transições de estado no `Model`; a lógica Git e de worktree continua independente da TUI.
- `cmd/gwt/main.go` apenas monta dependências e inicia a TUI ou a CLI. `internal/cli` preserva as opções úteis atuais sem depender de aliases; `gwt open` usa um subshell porque um binário não consegue executar `cd` no shell pai.

## Verificação

1. `make test` (= `go vet ./... && go test ./...`) — testes puros, Git real, CLI e transições da TUI verdes.
2. Casos críticos criam repos Git reais em `t.TempDir()` e verificam create/remove/dirty/layout; a CLI preserva as flags úteis de abertura e o modelo verifica seleção e a confirmação obrigatória antes de remover.
3. Smoke real: `gwt` em `~/dev/agentguru` e dentro de um de seus repos abre a mesma visão de worktrees agrupadas por branch.
4. Instalação limpa (sem aliases ou configuração de shell): `gwt open <branch>` abre um shell no worktree; `Enter` faz o mesmo pela TUI e, ao sair, retorna para ela.
5. Após uma release: `brew install <tap>/gwt --build-from-source` compila e instala o binário.

## Fora de escopo (add depois se doer)

- Publicar o tap Homebrew / releases com bottles (passo manual final).
- Setup pós-criação e integração PR/GitHub.
