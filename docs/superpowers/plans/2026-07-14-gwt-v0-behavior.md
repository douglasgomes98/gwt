# GWT v0 Behavior Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fazer CLI e TUI obedecerem a todas as regras definidas em `SPEC.md`.

**Architecture:** A configuração validada é carregada uma vez no binário e passada para CLI/TUI. `internal/worktree` concentra operações Git e texto de status; a CLI concentra a gramática; a TUI mantém seleção por path e executa somente essa seleção.

**Tech Stack:** Go 1.26, stdlib, yaml.v3, Bubble Tea v2, Git.

## Global Constraints

- `SPEC.md` é a fonte normativa; toda linha `Definido` é obrigatória.
- Projeto v0: remover compatibilidade, aliases e flags antigos.
- Sem novas dependências.
- TDD, `gofmt`, `make test` e `make coverage`; cobertura total acima de 90%.
- Nunca remover checkout primário; validar todos os alvos antes do primeiro `rm --all`.
- Na TUI, nenhuma mutação afeta item não selecionado.

---

## File map

| File | Responsabilidade |
| --- | --- |
| `cmd/gwt/main.go` | Config uma vez, TUI/CLI e somente `gwt version`. |
| `internal/config/config.go` | Discovery e validação estrita. |
| `internal/git/git.go` | Validação mínima de branch Git, se necessária. |
| `internal/worktree/worktree.go` | Status, detached, lookup, remoção e update seguro. |
| `internal/cli/cli.go` | Gramática, output e lotes da CLI. |
| `internal/tui/model.go` | Seleção, palette e ações limitadas à seleção. |
| Arquivos `*_test.go` pares | Testes observáveis com repos Git temporários. |
| `SPEC.md` | Não mudar sem decisão nova do usuário. |

## Task 1: Configuração estrita e bootstrap

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/gwt/main.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Produces: `func Load(start string) (Config, error)`.
- Produces: `func New(out, err io.Writer, dir, version string, cfg config.Config) App`.

- [ ] **Step 1: Escrever testes que falham para config inválida e defaults**

```go
func TestLoadRejectsInvalidConfig(t *testing.T) {
	for _, text := range []string{
		"layout: unknown\n", "layout: ''\n", "baseBranch: ''\n",
		"unknown: value\n", "layout: [inside]\n", "layout: [\n",
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte(text), 0644); err != nil { t.Fatal(err) }
		if _, err := Load(dir); err == nil { t.Fatalf("Load accepted %q", text) }
	}
}
func TestLoadDefaultsAndOptionalCommands(t *testing.T) {
	dir := t.TempDir()
	got, err := Load(dir)
	if err != nil || got != (Config{Layout: "sibling", BaseBranch: "main", Editor: "code", Agent: "claude"}) { t.Fatalf("%+v %v", got, err) }
	if err := os.WriteFile(filepath.Join(dir, "gwt.yml"), []byte("editor: ''\nagent: ''\n"), 0644); err != nil { t.Fatal(err) }
	got, err = Load(dir)
	if err != nil || got.Editor != "" || got.Agent != "" { t.Fatalf("%+v %v", got, err) }
}
```

- [ ] **Step 2: Confirmar que falham**

Run: `go test ./internal/config -run 'TestLoadRejectsInvalidConfig|TestLoadDefaultsAndOptionalCommands' -count=1`

Expected: FAIL; `Load` hoje retorna somente `Config` e ignora YAML inválido.

- [ ] **Step 3: Implementar o carregamento mínimo e estrito**

```go
type fileConfig struct {
	Layout *string `yaml:"layout"`
	BaseBranch *string `yaml:"baseBranch"`
	Editor *string `yaml:"editor"`
	Agent *string `yaml:"agent"`
}
func Load(start string) (Config, error) {
	defaults := Config{Layout: "sibling", BaseBranch: "main", Editor: "code", Agent: "claude"}
	for _, path := range configPaths(start) {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) { continue }
		if err != nil { return Config{}, fmt.Errorf("read config %s: %w", path, err) }
		var raw fileConfig
		dec := yaml.NewDecoder(bytes.NewReader(data)); dec.KnownFields(true)
		if err := dec.Decode(&raw); err != nil { return Config{}, fmt.Errorf("parse config %s: %w", path, err) }
		return apply(raw, defaults)
	}
	return defaults, nil
}
```

`configPaths` preserva local e depois home. `apply` aceita somente layouts definidos; `layout` e `baseBranch` presentes não podem ser vazios; `baseBranch` usa `git check-ref-format --branch`; editor/agent presentes podem ser vazios. Não consultar `PATH`.

- [ ] **Step 4: Propagar o erro e versão**

```go
cwd, _ := os.Getwd()
cfg, err := config.Load(cwd)
if err != nil { fmt.Fprintln(os.Stderr, "gwt:", err); os.Exit(1) }
if len(args) == 1 && args[0] == "version" { fmt.Println(version); return }
if len(args) == 0 { _, err := tea.NewProgram(tui.New(cwd, cfg)).Run(); /* handle err */ }
err = cli.New(os.Stdout, os.Stderr, cwd, version, cfg).Run(args)
```

Excluir `--version` e `-version`. Testar `version` sem/ com argumentos extras.

- [ ] **Step 5: Verificar e commitar**

Run: `go test ./internal/config ./internal/cli -count=1`
Expected: PASS.

```bash
git add cmd/gwt/main.go internal/config internal/cli internal/git
git commit -m "feat(config): validate v0 configuration"
```

## Task 2: Primitivas compartilhadas de worktree

**Files:**
- Modify: `internal/worktree/worktree.go`
- Modify: `internal/worktree/worktree_test.go`

**Interfaces:**
- Produces: `func Status(Item) string`.
- Produces: `func Find(repo, branch string) (Item, error)`.
- Produces: `func Update(path, base string) error`.

- [ ] **Step 1: Escrever testes reais de Git que falham**

```go
func TestStatusAndDetachedWorktree(t *testing.T) {
	if got := worktree.Status(worktree.Item{Changes: 2, Ahead: 1, Behind: 3}); got != "(2 files changed · ahead 1 · behind 3)" { t.Fatal(got) }
	if got := worktree.Status(worktree.Item{}); got != "(clean)" { t.Fatal(got) }
	// Adicione worktree, faça checkout --detach e exija Item.Detached em ListFast.
}
func TestUpdateRejectsDirtyBeforeFetch(t *testing.T) {
	r := repo(t)
	if err := os.WriteFile(filepath.Join(r, "README"), []byte("dirty"), 0644); err != nil { t.Fatal(err) }
	if err := worktree.Update(r, "main"); err == nil || strings.Contains(err.Error(), "fetch") { t.Fatal(err) }
}
func TestFindReturnsPrimary(t *testing.T) {
	item, err := worktree.Find(repo(t), "main")
	if err != nil || !item.Primary { t.Fatalf("%+v %v", item, err) }
}
```

- [ ] **Step 2: Confirmar falha**

Run: `go test ./internal/worktree -run 'TestStatusAndDetachedWorktree|TestUpdateRejectsDirtyBeforeFetch|TestFindReturnsPrimary' -count=1`
Expected: FAIL; as APIs/precondições ainda não existem.

- [ ] **Step 3: Implementar uma fonte única para status, detached e lookup**

```go
type Item struct {
	Repo, Branch, Path string
	Primary, Detached bool
	Dirty bool
	Changes, Ahead, Behind int
}
func Status(item Item) string { /* changed, ahead, behind; default "(clean)" */ }
func Find(repo, branch string) (Item, error) {
	items, err := List(repo)
	if err != nil { return Item{}, err }
	for _, item := range items { if item.Branch == branch { return item, nil } }
	return Item{}, fmt.Errorf("worktree for branch %q not found", branch)
}
```

Em `ListFast`, interpretar a linha porcelain `detached`. Na fronteira de apresentação, formatar branch detached como `(detached)`; não inventar nome de branch. `Remove` busca primeiro, recusa `Primary` ou `Detached`, depois remove o path.

- [ ] **Step 4: Tornar update seguro**

```go
func Update(path, base string) error {
	status, err := git.Run(path, "status", "--porcelain")
	if err != nil { return err }
	if strings.TrimSpace(status) != "" { return fmt.Errorf("root has uncommitted changes") }
	branch, err := git.Run(path, "branch", "--show-current")
	if err != nil { return err }
	if strings.TrimSpace(branch) != base { return fmt.Errorf("root must be on %s", base) }
	if err := Fetch(path, base); err != nil { return err }
	_, err = git.Run(path, "merge", "--ff-only", "origin/"+base)
	return err
}
```

Criar fixture com bare remote e dois clones; testar fast-forward e histórico divergente.

- [ ] **Step 5: Verificar e commitar**

Run: `go test ./internal/worktree -count=1`
Expected: PASS.

```bash
git add internal/worktree/worktree.go internal/worktree/worktree_test.go
git commit -m "feat(worktree): add safe v0 operations"
```

## Task 3: Gramática e operações da CLI

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `worktree.Status`, `Find`, `Remove`, `Repos`, `Update`.
- Produces: somente `help`, `version`, `add`, `open`, `rm`, `list`, `prune`, `update`.

- [ ] **Step 1: Escrever testes de gramática e lote**

```go
func TestAddFlagsCanAppearEitherSideButCannotMix(t *testing.T) {
	a := testApp(t, testRepo(t))
	for _, args := range [][]string{{"add", "AG-1", "-e"}, {"add", "-e", "AG-2"}} {
		if err := a.Run(args); err != nil { t.Fatal(err) }
	}
	for _, args := range [][]string{{"add", "AG-3", "-e", "-a"}, {"add", "AG-3", "--all", "-e"}} {
		if err := a.Run(args); err == nil { t.Fatalf("accepted %v", args) }
	}
}
func TestListPrintsPathBranchAndStatus(t *testing.T) { /* require path + "\tAG-1\t(clean)\n" */ }
func TestRemoveAllPrevalidatesPrimary(t *testing.T) { /* rm main --all fails; roots remain */ }
```

Também testar: `open -p`, flags de versão antigas e argumentos em `prune` falham; `rm --all` ignora irmão sem branch; `add --all` não exige remote.

- [ ] **Step 2: Confirmar falha**

Run: `go test ./internal/cli -run 'TestAddFlagsCanAppearEitherSideButCannotMix|TestListPrintsPathBranchAndStatus|TestRemoveAllPrevalidatesPrimary' -count=1`
Expected: FAIL.

- [ ] **Step 3: Substituir flag por parser explícito**

```go
func parse(args []string, allowed ...string) (map[string]bool, []string, error) {
	known, flags := map[string]bool{}, map[string]bool{}
	for _, name := range allowed { known[name] = true }
	var values []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if !known[arg] { return nil, nil, fmt.Errorf("unknown flag %q", arg) }
			flags[arg] = true
		} else { values = append(values, arg) }
	}
	return flags, values, nil
}
```

Remover `flagsFirst` e `-p`. `add` aceita no máximo uma entre `--all`, `-e`, `-a`; `open` no máximo uma entre `-e`, `-a`; `rm` só `--all`; demais nenhuma flag. `prune` rejeita qualquer argumento.

- [ ] **Step 4: Implementar os lotes e output**

Para `add --all`: iterar `Repos`, sem `Fetch`, parar no primeiro erro e embrulhar como “result may be partial”. Para `rm --all`: coletar todos os `Find` existentes, falhar se vazio, validar que nenhum é root/detached e só então remover sequencialmente; erro posterior informa parcial. `list` escreve:

```go
fmt.Fprintf(a.Out, "%s\t%s\t%s\n", item.Path, displayBranch(item), worktree.Status(item))
```

`update` resolve apenas `CurrentRepo(a.Dir)` e chama `worktree.Update`. Atualizar help com `version` e `update`.

- [ ] **Step 5: Verificar e commitar**

Run: `go test ./internal/cli -count=1`
Expected: PASS.

```bash
git add internal/cli/cli.go internal/cli/cli_test.go
git commit -m "feat(cli): implement v0 command behavior"
```

## Task 4: Seleção e palette contextual da TUI

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`

**Interfaces:**
- Produces: `selected map[string]bool`, `feature string`, `availableActions() []action`.
- Consumes: `Item.Detached` e config de editor/agent.

- [ ] **Step 1: Escrever testes que falham**

```go
func TestFeatureSelectionStartsWholeGroupThenTogglesOne(t *testing.T) {
	m := modelWith([]worktree.Item{{Repo:"api", Branch:"AG-1", Path:"/api.A"}, {Repo:"web", Branch:"AG-1", Path:"/web.A"}})
	m = press(m, "space")
	if !m.selected["/api.A"] || !m.selected["/web.A"] { t.Fatal("group not selected") }
	m.cursor = 1; m = press(m, "space")
	if m.selected["/web.A"] || !m.selected["/api.A"] { t.Fatal("row toggle failed") }
}
func TestRootAndFeatureSelectionsAreExclusive(t *testing.T) { /* selecting either clears the other */ }
func TestPaletteOnlyShowsValidCLICommands(t *testing.T) { /* one feature: open/rm/prune; roots: add/update/prune */ }
```

- [ ] **Step 2: Confirmar falha**

Run: `go test ./internal/tui -run 'TestFeatureSelectionStartsWholeGroupThenTogglesOne|TestRootAndFeatureSelectionsAreExclusive|TestPaletteOnlyShowsValidCLICommands' -count=1`
Expected: FAIL; estado atual usa grupo inteiro e não há palette.

- [ ] **Step 3: Implementar o estado mínimo e ações**

```go
type action string
const (
	actionAdd action = "add"; actionAddAll action = "add --all"
	actionOpen action = "open"; actionOpenEditor action = "open -e"; actionOpenAgent action = "open -a"
	actionRemove action = "rm"; actionRemoveAll action = "rm --all"
	actionPrune action = "prune"; actionUpdate action = "update"
)
func (m Model) availableActions() []action {
	if roots := m.selectedRoots(); len(roots) > 0 {
		if len(roots) == 1 { return []action{actionAdd, actionPrune, actionUpdate} }
		return []action{actionAddAll, actionPrune, actionUpdate}
	}
	if items := m.selectedFeatureItems(); len(items) == 1 { return []action{actionOpen, actionOpenEditor, actionOpenAgent, actionRemove, actionPrune} }
	if len(items) > 1 { return []action{actionRemoveAll, actionPrune} }
	return nil
}
```

Filtrar editor/agent vazios. Feature seleciona todos os paths da branch; novo Space dentro dela alterna somente o path. Root limpa feature; feature limpa roots. Detached não é selecionável. Enter abre palette se houver ação; j/k/setas movimentam; Esc fecha preservando seleção; Enter executa. Remover atalhos diretos n/d/p/u/e/a e mostrar os nomes CLI na palette.

- [ ] **Step 4: Verificar e commitar**

Run: `go test ./internal/tui -count=1`
Expected: PASS.

```bash
git add internal/tui/model.go internal/tui/model_test.go
git commit -m "feat(tui): add contextual command palette"
```

## Task 5: Ações selecionadas, reload e render da TUI

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`

**Interfaces:**
- Produces: `execute(action) (Model, tea.Cmd)` e `operationResult{..., reload:true}`.
- Consumes: seleção da Task 4 e operações da Task 2.

- [ ] **Step 1: Escrever testes que falham**

```go
func TestTUIAddUsesOnlySelectedRootsAndNoFetch(t *testing.T) { /* dois irmãos, um root selecionado; só ele recebe branch */ }
func TestTUIRemoveAllUsesOnlySelectedFeatureRows(t *testing.T) { /* desmarque uma linha; rm --all preserva-a */ }
func TestOperationResultClearsSelectionAndReloads(t *testing.T) {
	m := selectedFeatureModel()
	updated, cmd := m.Update(operationResult{message:"done", reload:true})
	got := updated.(Model)
	if got.feature != "" || len(got.selected) != 0 || cmd == nil { t.Fatal("operation did not reset") }
}
```

- [ ] **Step 2: Confirmar falha**

Run: `go test ./internal/tui -run 'TestTUIAddUsesOnlySelectedRootsAndNoFetch|TestTUIRemoveAllUsesOnlySelectedFeatureRows|TestOperationResultClearsSelectionAndReloads' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implementar executor de seleção**

```go
func (m Model) execute(a action) (Model, tea.Cmd) {
	switch a {
	case actionAdd, actionAddAll:
		m.input, m.branch = true, ""
		return m, nil
	case actionRemove, actionRemoveAll:
		m.confirm, m.pending = true, a
		return m, nil
	case actionPrune:
		return m, m.pruneSelected()
	case actionUpdate:
		return m, m.updateSelectedRoots()
	default:
		return m, m.openSelected(a)
	}
}
```

No Enter do input, iterar apenas `selectedRoots()` e chamar `worktree.Add(root.Path, branch, base, config)`: sem Fetch e sem `origin/`. `rm`/ `rm --all` usam somente `selectedFeatureItems()`, após confirmação. `prune` deduplica repos representados na seleção. `update` chama `worktree.Update(root.Path, base)`. Loops param no primeiro erro e informam resultado parcial.

- [ ] **Step 4: Centralizar encerramento e render**

```go
type operationResult struct { err error; message string; reload bool }
case operationResult:
	m.paletteOpen, m.confirm = false, false
	m.clearSelection()
	if x.err != nil { m.message = x.err.Error() } else { m.message = x.message }
	if x.reload { return m, m.reload() }
	return m, nil
```

Usar esse callback também após shell/editor/agent. Manter input imediatamente depois de `branch:`, status textual igual à CLI e detached como `(detached)`.

- [ ] **Step 5: Verificar e commitar**

Run: `go test ./internal/tui -count=1`
Expected: PASS.

```bash
git add internal/tui/model.go internal/tui/model_test.go
git commit -m "feat(tui): execute selected worktree commands"
```

## Task 6: Verificação completa e auditoria da spec

**Files:**
- Modify: `internal/cli/cli_test.go`
- Modify: `internal/worktree/worktree_test.go`
- Modify: `internal/tui/model_test.go`
- Modify: `SPEC.md` somente se uma decisão nova for aprovada.

- [ ] **Step 1: Adicionar fronteiras finais da CLI**

```go
func TestV0CLIRejectsRemovedCompatibilityForms(t *testing.T) {
	a := testApp(t, testRepo(t))
	for _, args := range [][]string{{"open","AG-1","-p"}, {"prune","extra"}, {"version","extra"}} {
		if err := a.Run(args); err == nil { t.Fatalf("accepted %v", args) }
	}
}
```

Cobrir também: `add --all` sem remote; `rm --all` com branch ausente em um irmão; update fora da base antes de fetch; e list de detached.

- [ ] **Step 2: Formatar e validar**

Run: `gofmt -w cmd/gwt/main.go internal/config/*.go internal/git/*.go internal/worktree/*.go internal/cli/*.go internal/tui/*.go`
Run: `make test`
Expected: PASS.

Run: `make coverage`
Expected: cobertura total acima de 90%.

- [ ] **Step 3: Auditar cada regra**

Ler `SPEC.md` de cima a baixo e associar cada linha `Definido` a teste ou primitiva compartilhada. Confirmar que o único item `Adiado` — diretórios vazios quando Git falha — permaneceu intacto.

- [ ] **Step 4: Commit final de testes, se houver**

```bash
git add internal/cli/cli_test.go internal/worktree/worktree_test.go internal/tui/model_test.go
git commit -m "test: cover v0 behavior boundaries"
```

## Self-review

- **Cobertura:** Task 1 trata configuração/bootstrap; Task 2 trata Git, detached e update; Task 3 cobre todos os comandos e flags da CLI; Tasks 4–5 cobrem seleção, palette, operações e render TUI; Task 6 fecha regressões e audita a tabela.
- **Consistência:** `Load` retorna `(Config,error)`; `App` recebe `Config`; `Status`, `Find` e `Update` são compartilhados; palette usa `action` em todo fluxo.
- **Escopo:** nenhum pacote novo, nenhuma feature fora de `SPEC.md`, e nenhum comportamento de retrocompatibilidade.

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-14-gwt-v0-behavior.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks.
2. **Inline Execution** — execute tasks in this session using `superpowers:executing-plans`, with checkpoints.

Which approach?
