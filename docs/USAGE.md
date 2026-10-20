# Guia de uso

Este documento descreve os comandos, flags, fluxos de trabalho e comportamentos do `mcjava-manager`.

## Conceitos

- **Instância:** diretório existente que contém (ou receberá) as pastas `mods/`, `shaderpacks/`, `resourcepacks/` e `saves/`. O programa não valida o nome do loader nem a estrutura interna da instância.
- **Provedor:** origem dos projetos e arquivos. Atualmente apenas o Modrinth está implementado.
- **Projeto:** mod, shader, resource pack etc. identificado por slug ou ID.
- **Versão:** lançamento concreto de um projeto, com arquivos, loaders, versões do jogo e canal.
- **Canal:** estabilidade da versão: `release`, `beta` ou `alpha`.
- **Conteúdo rastreado:** arquivo instalado pelo `mcjava-manager` e registrado no manifesto.
- **Conteúdo não rastreado:** arquivo que já estava em uma pasta gerenciada, mas não foi instalado pelo programa. Ele é listado, mas não é alterado sem uma ação explícita.
- **Manifesto:** arquivo `.mcjava-manager/installed.json` dentro da instância.

## Flags globais

```text
-i, --instance <caminho>   Caminho da instância (env: MCJAVA_MANAGER_INSTANCE)
    --config <arquivo>     Arquivo de configuração (env: MCJAVA_MANAGER_CONFIG)
    --no-color             Desativa cores e estilos visuais
-v, --verbose              Reservada para logs detalhados (ainda sem efeito)
-h, --help                 Ajuda
    --version              Versão curta do binário
```

### Formas de informar a instância

```bash
# Argumento posicional
mcjava-manager /caminho/da/instancia

# Flag
mcjava-manager --instance /caminho/da/instancia list

# Variável de ambiente
export MCJAVA_MANAGER_INSTANCE=/caminho/da/instancia
mcjava-manager list
```

Se `--instance` e o argumento posicional forem usados juntos, eles precisam apontar para o mesmo caminho. Caminhos começando com `~` são expandidos. Caminhos relativos são convertidos para absolutos.

A instância precisa existir e ser um diretório. O programa cria as pastas de conteúdo quando necessário; ele não cria a pasta da instância em si.

## Comando raiz

```bash
mcjava-manager [instância]
```

Sem subcomando e com uma instância informada, abre a TUI.

Sem instância, exibe a ajuda:

```bash
mcjava-manager
```

## `search`

Busca projetos no Modrinth sem precisar de uma instância.

```bash
mcjava-manager search [termo] [flags]
```

| Flag | Descrição |
| --- | --- |
| `-t, --type <tipo>` | `mod`, `modpack`, `resourcepack`, `shader`, `datapack` ou `plugin` (`world` é aceito pela CLI, mas o Modrinth não hospeda mundos) |
| `-l, --loader <loader>` | Loader, como `fabric`, `forge`, `neoforge` ou `quilt` |
| `-g, --game-version <versão>` | Versão do Minecraft, como `1.21.1` |
| `-c, --category <categoria>` | Categoria; pode repetir ou separar por vírgula |
| `-s, --sort <ordem>` | `relevance`, `downloads`, `follows`, `newest` ou `updated` |
| `-n, --limit <1-100>` | Quantidade de resultados (padrão: 20) |
| `--offset <n>` | Deslocamento inicial dos resultados |
| `--provider <nome>` | Provedor; atualmente apenas `modrinth` |

Exemplos:

```bash
mcjava-manager search sodium
mcjava-manager search iris --loader fabric --game-version 1.21.1
mcjava-manager search --type shader --game-version 1.21.1
mcjava-manager search "resource pack" --category vanilla-like --limit 10
mcjava-manager search sodium --sort downloads --limit 5
mcjava-manager search sodium --offset 20
```

Sem um termo, a busca lista projetos em destaque para os filtros informados. A saída mostra total, tabela de resultados e a indicação da próxima página quando houver.

## `info`

Exibe metadados de um projeto.

```bash
mcjava-manager info <slug|id> [flags]
```

| Flag | Descrição |
| --- | --- |
| `--provider <nome>` | Provedor; atualmente apenas `modrinth` |
| `--description` | Inclui a descrição completa do projeto |

Exemplos:

```bash
mcjava-manager info sodium
mcjava-manager info sodium --description
mcjava-manager info AANobbMI
```

## `versions`

Lista as versões de um projeto.

```bash
mcjava-manager versions <slug|id> [flags]
```

| Flag | Descrição |
| --- | --- |
| `--provider <nome>` | Provedor; atualmente apenas `modrinth` |
| `-g, --game-version <versão>` | Filtra pela versão do Minecraft |
| `-l, --loader <loader>` | Filtra pelo loader |
| `-c, --channel <canal>` | `release`, `beta` ou `alpha`; pode repetir ou separar por vírgula |
| `-n, --limit <1-100>` | Quantidade de versões exibidas (padrão: 20) |
| `--offset <n>` | Deslocamento inicial |

Exemplos:

```bash
mcjava-manager versions sodium
mcjava-manager versions sodium --game-version 1.21.1 --loader fabric
mcjava-manager versions sodium --channel release --limit 10
mcjava-manager versions sodium --channel release,beta
mcjava-manager versions sodium --offset 20
```

As versões são retornadas pelo provedor e paginadas localmente. A saída inclui o ID da versão, que pode ser usado em `install --version`.

## `install`

Instala um mod, shader pack ou resource pack.

```bash
mcjava-manager install <slug|id> [flags]
```

| Flag | Descrição |
| --- | --- |
| `--provider <nome>` | Provedor; atualmente apenas `modrinth` |
| `--version <id>` | ID exato da versão a instalar |
| `-g, --game-version <versão>` | Versão do Minecraft usada na seleção |
| `-l, --loader <loader>` | Loader usado na seleção |
| `-c, --channel <canal>` | `release`, `beta` ou `alpha`; pode repetir ou separar por vírgula |
| `--overwrite` | Substitui arquivo não rastreado e força a reinstalação |
| `--no-dependencies` | Não instala dependências obrigatórias automaticamente |

> `--version` em `install` recebe o ID de uma versão do projeto. Para ver a versão do binário, use `mcjava-manager --version` ou `mcjava-manager version`.

### Seleção automática de versão

Sem `--version`, o programa escolhe a versão mais recente compatível com os filtros:

- `--game-version` e `--loader`, quando informados;
- canal `release` por padrão;
- `--channel` pode alterar ou ampliar os canais aceitos.

Exemplo:

```bash
mcjava-manager install sodium \
  --instance /caminho/da/instancia \
  --game-version 1.21.1 \
  --loader fabric
```

> **Dica:** informe `--game-version` e `--loader` sempre que possível. Sem esses filtros, a versão release mais recente pode ser de outro loader ou de uma versão do Minecraft diferente da sua instância.

### Instalação de versão exata

Use o ID retornado por `versions`:

```bash
mcjava-manager versions sodium --game-version 1.21.1 --loader fabric
mcjava-manager install sodium \
  --instance /caminho/da/instancia \
  --version SMxNOGZ6
```

Com `--version`, os filtros informados são validados contra a versão escolhida; eles não são usados para escolher outra versão.

### Dependências obrigatórias

Antes do projeto principal, dependências marcadas como `required` são resolvidas e instaladas. A resolução:

- usa a versão explícita da dependência, quando informada pelo provedor;
- caso contrário, escolhe a versão mais recente instalável compatível com os filtros atuais;
- detecta ciclos de dependência;
- limita a profundidade da árvore a 24 níveis;
- instala dependências antes do projeto que as exige.

Se uma dependência já estiver instalada em outra versão, o `install` recusa a substituição automática para evitar mudanças silenciosas. Atualize ou remova a dependência explicitamente antes de continuar, ou use `update` para permitir a atualização de dependências.

Para instalar apenas o arquivo selecionado:

```bash
mcjava-manager install sodium --instance /caminho/da/instancia --no-dependencies
```

### Conflitos e sobrescrita

- Arquivos rastreados do mesmo projeto são substituídos automaticamente quando a versão muda.
- A versão antiga do mesmo projeto é removida depois que a nova é instalada e registrada.
- Um arquivo **não rastreado** com o mesmo nome no destino só é substituído com `--overwrite`.
- Um caminho que já pertence a outro projeto no manifesto é recusado.
- Diretórios e links simbólicos para diretórios são recusados; a sobrescrita substitui o link, não o arquivo apontado.

Use `--overwrite` somente depois de conferir o arquivo existente:

```bash
mcjava-manager install sodium --instance /caminho/da/instancia --overwrite
```

### Checksums e download

O arquivo precisa ter SHA-512 ou SHA-1 publicado pelo provedor. O download:

1. grava em um arquivo temporário na pasta de destino;
2. calcula e compara os checksums;
3. move o arquivo para o destino de forma atômica;
4. registra os metadados no manifesto.

Downloads sem checksum utilizável são recusados.

## `list`

Lista o conteúdo rastreado e os arquivos não rastreados nas pastas gerenciadas.

```bash
mcjava-manager list [flags]
```

| Flag | Descrição |
| --- | --- |
| `-t, --type <tipo>` | `mod`, `shader` ou `resourcepack` |

Exemplos:

```bash
mcjava-manager list --instance /caminho/da/instancia
mcjava-manager list --instance /caminho/da/instancia --type mod
mcjava-manager list --instance /caminho/da/instancia --type shader
```

A saída mostra:

- total de conteúdo rastreado;
- tabela com tipo, nome, versão, provedor, arquivo e status;
- total de arquivos não rastreados;
- tabela com tipo, caminho relativo e tamanho dos arquivos não rastreados.

Status possíveis de um item rastreado:

| Status | Significado |
| --- | --- |
| `ok` | Arquivo presente no destino |
| `ausente` | O manifesto aponta para um arquivo que não existe mais |
| `diretório` | O destino existe, mas é um diretório |
| `inválido` | O caminho do manifesto não é válido |
| `erro` | Não foi possível inspecionar o arquivo |

## `update`

Verifica e instala versões mais recentes do conteúdo rastreado.

```bash
mcjava-manager update [slug|id|arquivo ...] [flags]
```

Sem argumentos, atualiza todos os itens rastreados. Com argumentos, limita a atualização aos itens correspondentes.

Um argumento pode ser:

- slug do projeto;
- ID do projeto;
- nome do projeto;
- nome do arquivo;
- caminho relativo do arquivo, como `mods/sodium.jar`.

Se houver mais de uma correspondência, o comando pede um caminho mais específico.

| Flag | Descrição |
| --- | --- |
| `-g, --game-version <versão>` | Sobrescreve a versão do Minecraft usada na seleção |
| `-l, --loader <loader>` | Sobrescreve o loader usado na seleção |
| `-c, --channel <canal>` | Sobrescreve o canal; `release`, `beta` ou `alpha`, repetindo ou separando por vírgula |
| `--check` | Apenas verifica e exibe o resultado, sem baixar nem alterar arquivos |
| `--overwrite` | Permite substituir arquivos não rastreados com o mesmo nome |
| `--no-dependencies` | Não instala nem atualiza dependências obrigatórias |

Exemplos:

```bash
# Verificar todo o conteúdo
mcjava-manager update --instance /caminho/da/instancia --check

# Atualizar todo o conteúdo
mcjava-manager update --instance /caminho/da/instancia

# Atualizar somente um projeto
mcjava-manager update sodium --instance /caminho/da/instancia

# Atualizar um arquivo específico
mcjava-manager update mods/sodium.jar --instance /caminho/da/instancia

# Forçar a busca em outro canal
mcjava-manager update sodium --instance /caminho/da/instancia --channel beta
```

### Filtros padrão

Por padrão, cada item mantém os filtros usados quando foi instalado:

- versão do Minecraft registrada no manifesto;
- loader registrado no manifesto;
- canal da versão instalada.

As flags `--game-version`, `--loader` e `--channel` substituem esses valores para a execução atual.

### `--check`

`--check` não faz escritas no disco. Ele informa:

- `Já atualizado` quando a versão atual é a mais recente e o arquivo existe;
- `Atualização disponível` com a versão atual e a nova;
- `Reinstalação necessária` quando o manifesto aponta para a versão atual, mas o arquivo está ausente.

`--overwrite` não pode ser combinado com `--check`:

```text
--overwrite não pode ser usado junto com --check
```

### Dependências no update

Dependências obrigatórias das novas versões podem ser instaladas ou atualizadas automaticamente. Diferentemente do `install`, o `update` permite substituir uma dependência já instalada por uma versão exigida pela nova versão do item principal.

Use `--no-dependencies` para limitar a operação aos itens selecionados.

> **Limitação atual:** se o item já está na versão mais recente e o arquivo existe, o `update` não revalida dependências ausentes desse item. Use `install` novamente para reparar o conteúdo completo.

## `remove`

Remove conteúdo de uma instância.

```bash
mcjava-manager remove <arquivo|slug|id> [flags]
```

| Flag | Descrição |
| --- | --- |
| `-t, --type <tipo>` | Desambigua por tipo: `mod`, `shader` ou `resourcepack` |

O argumento pode ser:

- slug, ID ou nome do projeto;
- nome do arquivo;
- caminho relativo, como `mods/sodium.jar`;
- caminho absoluto dentro de uma pasta gerenciada.

Exemplos:

```bash
mcjava-manager remove sodium --instance /caminho/da/instancia
mcjava-manager remove sodium.jar --instance /caminho/da/instancia --type mod
mcjava-manager remove mods/sodium.jar --instance /caminho/da/instancia
mcjava-manager remove resourcepacks/meu-pack.zip --instance /caminho/da/instancia
```

Comportamento:

- Se o item for rastreado, o arquivo é removido e a entrada do manifesto é apagada.
- Se o arquivo já não existir, mas estiver rastreado, o registro é removido e o comando informa que o arquivo já não existia.
- Arquivos não rastreados também podem ser removidos pelo nome ou caminho.
- Diretórios não são removidos; remova-os manualmente se tiver certeza.
- A CLI **não pede confirmação**. A TUI pede confirmação antes de remover.
- Se houver ambiguidade entre arquivos de pastas diferentes, use o caminho ou `--type`.

## `version`

```bash
mcjava-manager version
```

Mostra versão, commit, data de build e versão do Go usada na compilação. Parte das informações depende de metadados injetados no build ou do módulo instalado com `go install`.

## `completion`

Gera o script de autocompletar do Cobra para o shell:

```bash
# Bash
mcjava-manager completion bash

# Zsh
mcjava-manager completion zsh

# Fish
mcjava-manager completion fish

# PowerShell
mcjava-manager completion powershell
```

Exemplos de instalação:

```bash
# Bash (carregamento no próximo shell)
mcjava-manager completion bash > ~/.local/share/bash-completion/completions/mcjava-manager
```

```zsh
# Zsh
mcjava-manager completion zsh > "${fpath[1]}/_mcjava-manager"
```

```fish
# Fish
mcjava-manager completion fish > ~/.config/fish/completions/mcjava-manager.fish
```

```powershell
# PowerShell (adicionar ao seu profile)
mcjava-manager completion powershell | Out-String | Invoke-Expression
```

## Interface interativa (TUI)

Abra com:

```bash
mcjava-manager --instance /caminho/da/instancia
```

A instância é preparada no início: as pastas `mods/`, `shaderpacks/`, `resourcepacks/` e `saves/` são criadas se necessário.

A TUI força o perfil **True Color (24-bit)**, independentemente de `TERM`, `COLORTERM` ou das capacidades informadas pelo terminal. Isso mantém o tema roxo consistente em PCs, Termux e Shizuku. Use `--no-color` para desativar as cores explicitamente.

### Telas

- **Buscar (F1):** campos de busca, versão do Minecraft, loader, tipo e canal; lista de resultados. O seletor **Tipo** oferece Todos, Mod, Shader e Pacote de recursos; **Canal** oferece release, beta, alpha e todos.
- **Instalados (F2):** conteúdo rastreado e arquivos não rastreados; ações de detalhes, atualização, verificação e remoção.

### Navegação

| Tecla | Ação |
| --- | --- |
| `F1` ou `Alt+1` | Tela Buscar |
| `F2` ou `Alt+2` | Tela Instalados |
| `1` / `2` | Alternar telas quando não está digitando |
| `Tab` / `Shift+Tab` | Próximo/anterior campo na busca |
| `↑` / `↓` | Navegar em listas |
| `PgUp` / `PgDn` | Página anterior/próxima |
| `Home` / `End` | Início/fim da lista |
| `Enter` | Confirmar a ação do contexto |
| `Esc` | Voltar, fechar overlay ou sair de um campo |
| `/` | Focar o campo de busca |
| `?` | Ajuda (fora de campos de texto) |
| `q` | Sair (fora de campos de texto) |
| `Ctrl+C` | Sair imediatamente |

### Busca

1. Digite o termo no campo **Busca**.
2. Opcionalmente preencha **Versão** e **Loader**.
3. Escolha **Tipo** e **Canal** com as setas.
4. Pressione `Enter`.
5. Use `↑`/`↓` para escolher um projeto e `Enter` ou `i` para abrir suas versões.
6. Escolha a versão e pressione `Enter` ou `i` para instalar.

Na lista de versões:

- `o` alterna a sobrescrita;
- `d` alterna a instalação de dependências;
- `r` recarrega as versões do projeto;
- `Esc` volta aos resultados.

A TUI mostra até 50 resultados de busca e informa o total. Para ver mais páginas, use a CLI com `search --offset`.

### Instalados

| Tecla | Ação |
| --- | --- |
| `Enter` | Abrir detalhes do item |
| `u` | Atualizar o item selecionado |
| `U` ou `a` | Atualizar todos os itens rastreados |
| `c` | Verificar atualizações do item selecionado |
| `C` | Verificar atualizações de todos os itens |
| `x` ou `Delete` | Remover o item selecionado (com confirmação) |
| `o` | Alternar sobrescrita |
| `d` | Alternar atualização de dependências |
| `r` | Recarregar o manifesto |
| `v` | Abrir a última saída de operação |

A remoção abre uma confirmação. Use `s`, `y` ou `Enter` para confirmar e `n`, `Esc` ou `q` para cancelar.

### Overlays

- **Ajuda:** `Esc`, `?`, `q` ou `Enter` fecha; `↑`/`↓` rola.
- **Saída:** `Esc`, `q` ou `Enter` fecha; `↑`/`↓` rola; `o` ativa/desativa a sobrescrita e fecha.
- **Confirmação:** `s`/`y`/`Enter` confirma; `n`/`Esc`/`q` cancela.

### Indicadores

A tela **Instalados** mostra:

- quantidade de itens rastreados;
- quantidade de arquivos não rastreados;
- estado da sobrescrita (`ligada`/`desligada`);
- estado das dependências (`sim`/`não`).

Operações assíncronas exibem um spinner e uma mensagem de status. Enquanto uma operação está em andamento, as teclas de navegação são ignoradas, exceto `Ctrl+C`.

## Configuração

O Modrinth não exige chave. A configuração atual é voltada ao futuro adaptador CurseForge e ainda não é lida pelos comandos da CLI.

### Precedência da chave CurseForge

1. `CURSEFORGE_API_KEY`;
2. `curseforge.api_key` em `~/.mcjava-manager.yaml` ou no arquivo indicado por `--config`.

Exemplo:

```yaml
curseforge:
  api_key: "SUA_CHAVE_AQUI"
```

Restrinja o arquivo:

```bash
chmod 600 ~/.mcjava-manager.yaml
```

A camada de configuração detecta permissões inseguras e redige chaves em logs e serializações. Como o adaptador CurseForge ainda não está implementado, os comandos atuais só funcionam com `--provider modrinth` e não leem o arquivo de configuração.

## Manifesto

Local:

```text
<caminho-da-instância>/.mcjava-manager/installed.json
```

Formato resumido:

```json
{
  "schema": 1,
  "updated_at": "2026-01-01T12:00:00Z",
  "entries": [
    {
      "provider": "modrinth",
      "project_id": "AANobbMI",
      "project_slug": "sodium",
      "project_name": "Sodium",
      "type": "mod",
      "version_id": "SMxNOGZ6",
      "version_number": "0.6.13+mc1.21.1",
      "channel": "release",
      "game_version": "1.21.1",
      "loader": "fabric",
      "file_name": "sodium-fabric-0.6.13+mc1.21.1.jar",
      "file_path": "mods/sodium-fabric-0.6.13+mc1.21.1.jar",
      "file_size": 123456,
      "hashes": {
        "sha1": "...",
        "sha512": "..."
      },
      "installed_at": "2026-01-01T12:00:00Z",
      "updated_at": "2026-01-01T12:00:00Z"
    }
  ]
}
```

O manifesto é validado antes de ser usado. Caminhos precisam apontar diretamente para `mods/`, `shaderpacks/` ou `resourcepacks/` e não podem sair da instância. Em Windows, a comparação de caminhos é feita sem diferenciar maiúsculas de minúsculas.

Não edite o manifesto manualmente enquanto o gerenciador estiver em uso. Para backup, copie o arquivo com a instância fechada.

## Limitações atuais

- Apenas o provedor Modrinth está implementado.
- A instalação é suportada para mods, shaders e resource packs.
- Modpacks, plugins e datapacks podem ser buscados e inspecionados, mas não instalados.
- Mundos/mapas não são hospedados pelo Modrinth; a pasta `saves/` é criada, mas não é gerenciada.
- A TUI mostra até 50 resultados de busca por vez.
- `--verbose` ainda não altera a saída.
- Não há pipeline de releases binárias publicadas.

## Códigos de saída

| Código | Significado |
| --- | --- |
| `0` | Sucesso |
| `1` | Erro de validação, rede, permissão ou operação |
| `130` | Operação interrompida por `Ctrl+C` na CLI |

## Erros comuns

### `provedor CurseForge ainda não está implementado`

O adaptador CurseForge ainda não existe. Use o provedor padrão:

```bash
mcjava-manager search sodium --provider modrinth
```

### `nenhuma versão instalável foi encontrada`

Nenhuma versão atende aos filtros. Tente:

- conferir a versão do Minecraft;
- conferir o loader;
- permitir outros canais com `--channel release,beta,alpha`;
- remover filtros para ver o que existe:

```bash
mcjava-manager versions <projeto>
```

### `o arquivo ... já está registrado para ...`

O caminho de destino já pertence a outro projeto no manifesto. Verifique com:

```bash
mcjava-manager list --instance /caminho/da/instancia
```

Remova o conteúdo antigo ou escolha outro projeto/versão.

### `mais de um conteúdo instalado corresponde a ...`

Use o caminho relativo ou `--type` para desambiguar:

```bash
mcjava-manager remove mods/sodium.jar --instance /caminho/da/instancia
mcjava-manager remove sodium --instance /caminho/da/instancia --type mod
```

### `manifesto de conteúdo instalado inválido`

O arquivo `.mcjava-manager/installed.json` está corrompido ou usa um esquema desconhecido. Se você tiver backup, restaure-o. Caso contrário, remova o manifesto com a instância fechada; os arquivos de conteúdo permanecerão e passarão a ser tratados como não rastreados.

Para problemas de instalação, permissões e plataforma, consulte [INSTALL.md](INSTALL.md).
