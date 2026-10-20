# mcjava-manager

Gerenciador de conteúdo para instâncias do Minecraft Java, com interface de linha de comando (CLI) e interface interativa no terminal (TUI).

O projeto instala, lista, atualiza e remove mods, shader packs e resource packs diretamente nas pastas da instância, mantendo um manifesto local do que foi gerenciado. O provedor Modrinth está implementado; o suporte ao CurseForge está planejado.

> **Status:** projeto em desenvolvimento ativo. A CLI e a TUI já cobrem o fluxo principal com Modrinth.

## Índice

- [Visão geral](#visão-geral)
- [Funcionalidades](#funcionalidades)
- [Conteúdo suportado](#conteúdo-suportado)
- [Instalação rápida](#instalação-rápida)
- [Primeiros passos](#primeiros-passos)
- [Interface interativa](#interface-interativa)
- [Comandos](#comandos)
- [Configuração e chaves de API](#configuração-e-chaves-de-api)
- [Manifesto e segurança](#manifesto-e-segurança)
- [Como o projeto funciona](#como-o-projeto-funciona)
- [Desenvolvimento](#desenvolvimento)
- [Documentação detalhada](#documentação-detalhada)
- [Roadmap](#roadmap)

## Visão geral

O `mcjava-manager` foi feito para quem quer cuidar do conteúdo de uma instância sem abrir o navegador, sem baixar arquivos manualmente e sem depender de ferramentas externas como `curl`, `wget` ou `unzip`.

O programa:

- recebe o caminho de qualquer diretório de instância existente;
- cria automaticamente `mods/`, `shaderpacks/`, `resourcepacks/` e `saves/` quando necessário;
- consulta o Modrinth para buscar projetos e versões compatíveis;
- baixa os arquivos por HTTP com verificação de checksum;
- registra cada instalação em `.mcjava-manager/installed.json`;
- distingue conteúdo gerenciado de arquivos que já estavam na pasta;
- pode ser compilado como um binário estático, sem `cgo`.

Não há validação baseada no nome do loader ou no nome da pasta da instância. Qualquer diretório existente pode ser usado, desde que o usuário tenha permissão de escrita nele.

## Funcionalidades

- **CLI completa:** busca, metadados, versões, instalação, listagem, atualização e remoção.
- **TUI com Bubble Tea:** busca, navegação por projetos e versões, instalação, atualização, verificação, remoção e detalhes do conteúdo instalado.
- **Seleção de versão compatível:** filtros por versão do Minecraft, loader e canal (`release`, `beta`, `alpha`).
- **Versão exata:** instalação de um ID de versão específico com `--version`.
- **Dependências obrigatórias:** resolução recursiva, com detecção de ciclos e limite de profundidade.
- **Verificação de integridade:** SHA-512 e SHA-1 publicados pelo provedor são conferidos antes da instalação.
- **Download atômico:** o arquivo é gravado em um temporário e só substitui o destino depois de validado.
- **Manifesto local:** sabe exatamente o que foi instalado, por qual projeto e versão.
- **Conteúdo não rastreado:** arquivos que já estavam nas pastas gerenciadas aparecem na listagem e não são apagados sem uma ação explícita.
- **Proteção contra sobrescrita silenciosa:** arquivos não rastreados exigem `--overwrite`; dependências em versão diferente são recusadas na instalação.
- **Paginação:** `search` e `versions` aceitam `--limit` e `--offset`.
- **Multiplataforma:** Linux, macOS, Windows, WSL e Termux/Android.
- **Fallback DNS:** usa o DNS do sistema e recorre a resolvedores públicos quando ele está inacessível, como no Shizuku/Android.
- **Mensagens de permissão contextuais:** orientação específica para Windows, macOS, Linux e Termux.
- **Autocompletar de shell:** comandos de conclusão gerados pelo Cobra para Bash, Zsh, Fish e PowerShell.
- **Sem ferramentas externas:** o download e a manipulação de arquivos usam a biblioteca padrão do Go.

## Conteúdo suportado

| Tipo | Busca e metadados | Instalação | Pasta gerenciada |
| --- | :---: | :---: | --- |
| Mod | Sim | Sim | `mods/` |
| Shader pack | Sim | Sim | `shaderpacks/` |
| Resource pack | Sim | Sim | `resourcepacks/` |
| Mapa/mundo | Não | Não | `saves/` é criada, mas não gerenciada |
| Modpack | Sim | Não | — |
| Plugin | Sim | Não | — |
| Datapack | Sim | Não | — |

A pasta `saves/` é criada para manter o layout esperado da instância. A instalação e o gerenciamento de mapas ainda não estão implementados, e o Modrinth não hospeda mundos/mapas.

## Instalação rápida

### Opção 1: instalar com Go

Requer Go 1.24.2 ou superior:

```bash
go install github.com/wedemmmdev/mcjava-manager/cmd/mcjava-manager@latest
```

O binário é colocado em `$(go env GOPATH)/bin`, normalmente `~/go/bin`. Se essa pasta não estiver no `PATH`:

```bash
# Bash/Zsh
export PATH="$HOME/go/bin:$PATH"
```

```powershell
# PowerShell
$env:Path += ";$env:USERPROFILE\go\bin"
```

### Opção 2: compilar o código-fonte

```bash
git clone https://github.com/wedemmmdev/mcjava-manager.git
cd mcjava-manager
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager ./cmd/mcjava-manager
```

No Windows (PowerShell):

```powershell
git clone https://github.com/wedemmmdev/mcjava-manager.git
cd mcjava-manager
$env:CGO_ENABLED = "0"
go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager.exe .\cmd\mcjava-manager
```

O projeto ainda não publica releases binárias oficiais. As instruções completas para cada plataforma, incluindo Termux, macOS, Windows e compilação cruzada, estão em [docs/INSTALL.md](docs/INSTALL.md).

### Verificar a instalação

```bash
mcjava-manager version
mcjava-manager --help
```

## Primeiros passos

### Abrir a interface interativa

Informe o caminho de uma instância existente:

```bash
mcjava-manager --instance /caminho/da/instancia
```

ou, usando o argumento posicional:

```bash
mcjava-manager /caminho/da/instancia
```

Sem uma instância, o comando raiz exibe a ajuda. A TUI só é aberta quando um caminho é informado. A instância pode ser de qualquer loader ou até uma pasta organizada manualmente; o programa não tenta adivinhar o loader.

### Fluxo básico pela CLI

```bash
# Buscar um mod compatível
mcjava-manager search sodium --loader fabric --game-version 1.21.1

# Ver detalhes e versões
mcjava-manager info sodium
mcjava-manager versions sodium --loader fabric --game-version 1.21.1 --channel release

# Instalar
mcjava-manager install sodium \
  --instance /caminho/da/instancia \
  --game-version 1.21.1 \
  --loader fabric

# Conferir o que está instalado
mcjava-manager list --instance /caminho/da/instancia

# Verificar atualizações sem baixar nada
mcjava-manager update --instance /caminho/da/instancia --check

# Atualizar
mcjava-manager update --instance /caminho/da/instancia

# Remover
mcjava-manager remove sodium --instance /caminho/da/instancia
```

A instância também pode ser definida pela variável de ambiente:

```bash
export MCJAVA_MANAGER_INSTANCE="$HOME/.local/share/PrismLauncher/instances/MinhaInstancia/minecraft"
mcjava-manager list
```

## Interface interativa

A TUI possui duas telas principais:

- **Buscar:** formulário de busca, filtros de versão, loader e tipo, além da lista de resultados.
- **Instalados:** conteúdo rastreado e arquivos não rastreados, com ações de detalhes, atualização, verificação e remoção.

### Atalhos principais

| Tecla | Ação |
| --- | --- |
| `F1` / `Alt+1` | Ir para a tela **Buscar** |
| `F2` / `Alt+2` | Ir para a tela **Instalados** |
| `1` / `2` | Alternar telas quando o foco não está em um campo de texto |
| `Tab` / `Shift+Tab` | Navegar entre os campos da busca |
| `Enter` | Buscar, abrir projeto, instalar ou abrir detalhes, conforme o foco |
| `Esc` | Voltar, fechar overlay ou cancelar |
| `↑` / `↓` | Navegar nas listas |
| `/` | Focar o campo de busca |
| `o` | Alternar sobrescrita |
| `d` | Alternar instalação de dependências obrigatórias |
| `r` | Repetir a busca, recarregar o projeto ou recarregar o manifesto |
| `v` | Abrir a última saída de uma operação (fora de campos de texto) |
| `?` | Abrir a ajuda (fora de campos de texto) |
| `q` | Sair (fora de campos de texto) |
| `Ctrl+C` | Sair imediatamente |
| `u` | Atualizar o item selecionado em **Instalados** |
| `U` ou `a` | Atualizar todos os itens rastreados |
| `c` | Verificar atualizações do item selecionado |
| `C` | Verificar atualizações de todos os itens |
| `x` / `Delete` | Remover o item selecionado, com confirmação |

Na tela de saída, use `↑`/`↓` para rolar, `Esc`/`Enter` para fechar e `o` para ativar a sobrescrita e repetir a operação.

Por padrão, a sobrescrita fica **desligada** e a instalação de dependências obrigatórias fica **ligada**. A TUI nunca substitui um arquivo não rastreado sem que a sobrescrita seja ativada.

A busca da TUI mostra até os 50 primeiros resultados do Modrinth e informa o total encontrado. Para paginar resultados pela CLI, use `search --limit` e `search --offset`.

## Comandos

| Comando | Descrição |
| --- | --- |
| `mcjava-manager [instância]` | Abre a TUI para a instância informada |
| `search [termo]` | Busca projetos no provedor |
| `info <slug\|id>` | Exibe detalhes de um projeto |
| `versions <slug\|id>` | Lista versões disponíveis |
| `install <slug\|id>` | Instala um mod, shader ou resource pack |
| `list` | Lista conteúdo rastreado e arquivos não rastreados |
| `update [itens...]` | Atualiza um item ou todo o conteúdo rastreado |
| `remove <arquivo\|slug\|id>` | Remove conteúdo da instância |
| `version` | Exibe versão, commit e ambiente de compilação |
| `completion <shell>` | Gera autocompletar para Bash, Zsh, Fish ou PowerShell |

A referência completa de flags, exemplos e comportamentos está em [docs/USAGE.md](docs/USAGE.md).

### Flags globais

| Flag | Descrição |
| --- | --- |
| `-i, --instance <caminho>` | Caminho da instância (env: `MCJAVA_MANAGER_INSTANCE`) |
| `--config <arquivo>` | Arquivo de configuração (env: `MCJAVA_MANAGER_CONFIG`) |
| `--no-color` | Desativa cores e estilos visuais |
| `-v, --verbose` | Flag reservada para logs detalhados; nesta versão ainda não altera a saída |
| `-h, --help` | Ajuda |
| `--version` | Versão curta do binário |

A instância pode ser passada como argumento posicional ou por `--instance`. Se as duas formas forem usadas, elas precisam apontar para o mesmo caminho.

## Configuração e chaves de API

O Modrinth não exige chave de API. Nenhuma configuração é necessária para usar os comandos atuais.

A estrutura de configuração do CurseForge já existe e segue esta precedência:

1. variável de ambiente `CURSEFORGE_API_KEY`;
2. arquivo `~/.mcjava-manager.yaml` (ou o caminho informado em `--config`).

Como o adaptador CurseForge ainda não foi conectado à CLI, nenhum comando atual lê o arquivo de configuração. A flag `--config` e a variável `MCJAVA_MANAGER_CONFIG` já são resolvidas e ficam reservadas para essa integração.

Exemplo de arquivo:

```yaml
curseforge:
  api_key: "SUA_CHAVE_AQUI"
```

Restrinja o acesso ao arquivo em sistemas Unix:

```bash
chmod 600 ~/.mcjava-manager.yaml
```

> **Importante:** o adaptador CurseForge ainda não está implementado. Hoje, `--provider curseforge` retorna uma mensagem informando que o provedor não está disponível. A configuração e a resolução de chave estão preparadas para quando o adaptador for adicionado.

Nunca faça commit de chaves de API. O arquivo `.mcjava-manager.yaml`, `.env` e arquivos semelhantes já estão no `.gitignore`.

## Manifesto e segurança

O estado do conteúdo gerenciado fica dentro da instância:

```text
<caminho-da-instância>/
├── mods/
├── shaderpacks/
├── resourcepacks/
├── saves/
└── .mcjava-manager/
    └── installed.json
```

O manifesto registra, para cada arquivo:

- provedor, ID, slug e nome do projeto;
- ID e nome da versão;
- tipo de conteúdo e canal;
- versão do Minecraft e loader usados na seleção;
- nome e caminho relativo do arquivo;
- tamanho e checksums;
- datas de instalação e atualização.

O arquivo é validado e salvo de forma atômica. Não edite o manifesto manualmente enquanto o gerenciador estiver em uso. Se precisar fazer backup, copie `.mcjava-manager/installed.json` com a instância fechada.

Medidas de segurança implementadas:

- verificação de SHA-512/SHA-1 antes de mover o arquivo para o destino;
- recusa de downloads sem checksum utilizável;
- gravação em arquivo temporário e renomeação atômica;
- validação de nomes de arquivo contra separadores de caminho, caracteres de controle e nomes reservados do Windows;
- validação de caminhos do manifesto contra as pastas gerenciadas;
- limite de tamanho para downloads e para o arquivo de manifesto;
- tratamento explícito de conflitos e necessidade de `--overwrite` para arquivos não rastreados;
- mensagens de permissão específicas para Windows, macOS, Linux e Termux.

## Como o projeto funciona

```text
cmd/mcjava-manager/   Ponto de entrada
internal/cli/         Comandos Cobra, TUI e regras de instalação/update/remove
internal/config/      Configuração local e resolução segura de credenciais
internal/download/    Download HTTP, checksums, nomes seguros e escrita atômica
internal/instance/    Validação da instância, pastas e erros de permissão
internal/manifest/    Manifesto JSON versionado e persistência atômica
internal/netdns/      Resolver DNS com fallback para ambientes restritos
internal/version/     Metadados de build
pkg/modsource/        Interfaces e tipos independentes de provedor
pkg/modrinth/         Cliente da API v2 do Modrinth
```

A CLI depende da interface `modsource.Client`, que abstrai busca, projetos, versões, arquivos e dependências. O cliente Modrinth implementa essa interface; o adaptador CurseForge poderá ser adicionado sem alterar os comandos principais.

## Desenvolvimento

Requisitos:

- Go 1.24.2 ou superior;
- Git;
- acesso à internet para os testes de integração manual com o Modrinth.

Comandos úteis:

```bash
gofmt -w .
go vet ./...
go test ./...
go test -race ./...
```

Build estático:

```bash
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager ./cmd/mcjava-manager
```

Build cruzado para Windows:

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager.exe ./cmd/mcjava-manager
```

Os testes de clientes HTTP usam `httptest.Server` e não dependem da API real. Contribuições devem seguir Conventional Commits, manter o código formatado e incluir testes para lógica crítica.

## Documentação detalhada

- [Instalação por plataforma](docs/INSTALL.md)
- [Referência de uso e comandos](docs/USAGE.md)

## Roadmap

- Implementar o adaptador CurseForge e usar a chave configurada.
- Adicionar instalação e gerenciamento de mapas/mundos em `saves/`.
- Avaliar suporte a modpacks, plugins e datapacks.
- Publicar releases binárias para Linux, macOS e Windows.
- Ampliar a cobertura de testes de integração e smoke tests da TUI.

## Contribuição

Antes de abrir uma contribuição:

1. execute `gofmt`, `go vet`, `go test ./...` e, quando possível, `go test -race ./...`;
2. não inclua chaves de API, caminhos pessoais ou arquivos de instância no commit;
3. use mensagens de commit no padrão Conventional Commits;
4. descreva claramente o comportamento alterado e os testes adicionados.
