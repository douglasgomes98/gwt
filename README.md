# gwt

Gerenciador de Git worktrees para um diretório com vários repositórios.

`gwt` abre a TUI. Use `gwt add <branch> [base] [--all]`, `gwt open <branch> [-e|-a|-p]`, `gwt rm <branch>` e `gwt list` para automação. `gwt open` abre um subshell no worktree; um binário não pode mudar o diretório do shell pai.

## Instalação

```sh
go install github.com/douglasgomes/gwt/cmd/gwt@latest
```

Opcionalmente crie `gwt.yml` na pasta de repos ou `~/.config/gwt/config.yml`:

```yaml
layout: sibling # sibling, grouped ou inside
baseBranch: main
editor: code
agent: claude
```

`inside` usa `.worktrees/`; adicione-a ao `.gitignore` se necessário.
