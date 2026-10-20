# Instalação

Este guia cobre a instalação do `mcjava-manager` em Linux, macOS, Windows, WSL e Termux/Android, além de compilação cruzada e solução de problemas.

## Requisitos

### Para usar um binário já compilado

- Nenhuma dependência de runtime.
- Acesso à internet para consultar o Modrinth e baixar conteúdo.
- Permissão de escrita na pasta da instância do Minecraft.

### Para compilar

- Go 1.24.2 ou superior.
- Git (para clonar o repositório).
- Nenhum compilador C, `unzip`, `curl` ou `wget` é necessário.

Confira a versão do Go:

```bash
go version
```

## Formas de instalação

### Instalar com `go install`

Esta é a forma mais simples de obter o binário mais recente:

```bash
go install github.com/wedemmmdev/mcjava-manager/cmd/mcjava-manager@latest
```

O Go instala o executável em `$(go env GOPATH)/bin` (ou em `GOBIN`, se essa variável estiver definida):

```bash
go env GOPATH
```

Normalmente o caminho é `~/go/bin` em Linux/macOS e `%USERPROFILE%\go\bin` no Windows. Adicione essa pasta ao `PATH` se o comando `mcjava-manager` não for encontrado.

### Compilar a partir do código-fonte

```bash
git clone https://github.com/wedemmmdev/mcjava-manager.git
cd mcjava-manager
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager ./cmd/mcjava-manager
```

Com `CGO_ENABLED=0`, o resultado não depende de bibliotecas C externas e é adequado para distribuição como binário único.

Para builds de desenvolvimento com metadados de versão:

```bash
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w \
  -X github.com/wedemmmdev/mcjava-manager/internal/version.Version=dev" \
  -o mcjava-manager ./cmd/mcjava-manager
```

### Releases binárias

O repositório ainda não publica releases binárias oficiais. Até que isso exista, use `go install` ou compile o código-fonte.

## Linux

### Instalar em `~/.local/bin`

```bash
mkdir -p "$HOME/.local/bin"
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" \
  -o "$HOME/.local/bin/mcjava-manager" ./cmd/mcjava-manager
```

Adicione `~/.local/bin` ao `PATH` caso ainda não esteja:

```bash
# Bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

```zsh
# Zsh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

```fish
# Fish
fish_add_path "$HOME/.local/bin"
```

### Instalar em `/usr/local/bin`

```bash
sudo install -m 0755 mcjava-manager /usr/local/bin/mcjava-manager
```

### Verificar se o binário é estático

```bash
file mcjava-manager
ldd mcjava-manager
```

Em um build estático, `file` mostra `statically linked` e `ldd` informa que o binário não é um executável dinâmico.

### Distribuições ARM

Para Raspberry Pi e outros sistemas ARM, compile na própria máquina:

```bash
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager ./cmd/mcjava-manager
```

Ou faça compilação cruzada com `GOARCH=arm64` ou `GOARCH=arm`.

## macOS

### Instalar com Go

```bash
go install github.com/wedemmmdev/mcjava-manager/cmd/mcjava-manager@latest
```

Adicione `$HOME/go/bin` ao `PATH` se necessário:

```zsh
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

### Compilar localmente

```bash
git clone https://github.com/wedemmmdev/mcjava-manager.git
cd mcjava-manager
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager ./cmd/mcjava-manager
```

Em Macs com Apple Silicon, o build nativo gera `arm64`. Para um Mac Intel, use `GOARCH=amd64` se estiver compilando em outra máquina.

### Gatekeeper

Se você baixar um binário pronto e o macOS bloquear a execução, remova o atributo de quarentena depois de confirmar a origem do arquivo:

```bash
chmod +x mcjava-manager
xattr -d com.apple.quarantine mcjava-manager
```

Também é possível liberar pelo diálogo **Ajustes do Sistema > Privacidade e Segurança**.

### Permissões de arquivos e pastas

Se o macOS negar acesso à pasta da instância, verifique **Ajustes do Sistema > Privacidade e Segurança > Arquivos e Pastas** (ou **Acesso Total ao Disco**) e permita o acesso ao terminal usado para executar o programa.

## Windows

### Instalar com Go

No PowerShell:

```powershell
go install github.com/wedemmmdev/mcjava-manager/cmd/mcjava-manager@latest
```

Adicione `%USERPROFILE%\go\bin` ao `PATH` do usuário. Em uma sessão atual do PowerShell:

```powershell
$env:Path += ";$env:USERPROFILE\go\bin"
```

Para tornar a alteração permanente, use a interface de **Variáveis de Ambiente** do Windows ou o comando `setx` com cuidado (o `setx` tem limite de tamanho e pode truncar um `PATH` muito longo).

### Compilar localmente

```powershell
git clone https://github.com/wedemmmdev/mcjava-manager.git
cd mcjava-manager
$env:CGO_ENABLED = "0"
go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager.exe .\cmd\mcjava-manager
```

Execute no PowerShell, no Prompt de Comando ou no Windows Terminal:

```powershell
.\mcjava-manager.exe --help
.\mcjava-manager.exe --instance "C:\Users\SeuUsuario\AppData\Roaming\PrismLauncher\instances\MinhaInstancia\minecraft"
```

### PATH e pastas com espaços

Coloque o `.exe` em uma pasta como `C:\Tools\mcjava-manager` e adicione essa pasta ao `PATH`. Sempre use aspas quando o caminho da instância tiver espaços:

```powershell
mcjava-manager --instance "D:\Jogos\Minecraft\Instâncias\Fabric 1.21"
```

### Antivírus e Proteção contra Ransomware

O Windows pode bloquear a gravação dentro de pastas protegidas. Se ocorrer erro de permissão:

- prefira uma instância dentro do perfil do usuário, como `%APPDATA%`;
- verifique a **Proteção contra Ransomware** (Controlled Folder Access);
- adicione uma exceção para a pasta da instância ou para o terminal usado;
- se necessário, execute o terminal como Administrador.

Não é recomendado desativar o antivírus de forma permanente. Prefira liberar apenas a pasta necessária.

## WSL

No WSL, use o binário Linux. Instale o Go dentro da distribuição ou compile em outra máquina e copie o binário:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager ./cmd/mcjava-manager
```

Instâncias que ficam no Windows podem ser acessadas por `/mnt/c/...`:

```bash
mcjava-manager --instance "/mnt/c/Users/SeuUsuario/AppData/Roaming/PrismLauncher/instances/MinhaInstancia/minecraft"
```

O acesso a `/mnt/c` é mais lento que ao sistema de arquivos do WSL, mas funciona. Se possível, mantenha a instância no sistema de arquivos Linux para melhor desempenho.

## Termux / Android

### Instalar dependências

```bash
pkg update
pkg install golang git
```

### Compilar e instalar

```bash
git clone https://github.com/wedemmmdev/mcjava-manager.git
cd mcjava-manager
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager ./cmd/mcjava-manager
mkdir -p "$PREFIX/bin"
mv mcjava-manager "$PREFIX/bin/mcjava-manager"
chmod +x "$PREFIX/bin/mcjava-manager"
```

Verifique:

```bash
mcjava-manager version
```

### Acesso ao armazenamento

Para acessar pastas compartilhadas do Android:

```bash
termux-setup-storage
```

Depois use um caminho acessível, como:

```bash
mcjava-manager --instance "$HOME/storage/shared/.../minha-instancia"
```

O Android pode restringir o acesso a `Android/data` e `Android/obb` em versões recentes. Se a instância estiver nessas pastas, mova-a para uma área compartilhada permitida ou use um gerenciador de arquivos com acesso autorizado.

### Permissões no Termux

Se aparecer `permissão negada`, confirme que:

1. `termux-setup-storage` foi executado e a permissão foi concedida;
2. o caminho usado está dentro de `$HOME/storage/shared/...`;
3. o usuário do Termux consegue criar um arquivo de teste na pasta:

```bash
touch "$HOME/storage/shared/teste-mcjava-manager"
rm "$HOME/storage/shared/teste-mcjava-manager"
```

### Execução via Shizuku (rish) para instâncias restritas

Em algumas instalações, o Termux não consegue acessar ou executar arquivos em pastas como `Android/data`. O Shizuku pode executar comandos como o usuário `shell` do Android, mas o SELinux impede que esse usuário execute binários que estejam:

- dentro da pasta privada do Termux;
- em armazenamento público montado como `noexec`, como `/sdcard`.

Para contornar essa restrição, copie o binário para `/data/local/tmp/`, que permite execução nativa pelo usuário do Shizuku.

O binário tenta primeiro o DNS configurado no sistema. Se a conexão com esse DNS falhar, ele usa `8.8.8.8:53` e `1.1.1.1:53` como fallback, evitando o erro de `connection refused` no ambiente do Shizuku.

1. Copie o binário do Termux para uma pasta pública:

   ```bash
   cp "$PREFIX/bin/mcjava-manager" /sdcard/Download/
   ```

2. Use o Shizuku para mover o binário para o diretório executável do Android:

   ```bash
   rish -c "cp /sdcard/Download/mcjava-manager /data/local/tmp/"
   ```

3. Dê permissão de execução nativa via Shizuku:

   ```bash
   rish -c "chmod +x /data/local/tmp/mcjava-manager"
   ```

4. Execute o programa pelo Shizuku, passando o caminho absoluto da instância:

   ```bash
   rish -c "/data/local/tmp/mcjava-manager --instance /sdcard/Android/data/nome.do.seu.launcher/files/sua_instancia"
   ```

> **Atenção:** sempre que atualizar o aplicativo — por exemplo, após um novo `git pull` e uma nova compilação — repita a cópia do binário para `/data/local/tmp/` e o `chmod +x`. O Shizuku executa a cópia que está em `/data/local/tmp/`, não o binário instalado em `$PREFIX/bin`.

## Compilação cruzada

Como o projeto não usa `cgo`, é possível gerar binários independentes para outras plataformas. Crie a pasta de saída antes:

```bash
mkdir -p dist
```

```bash
# Linux x86_64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -buildvcs=false -trimpath -ldflags="-s -w" -o dist/mcjava-manager-linux-amd64 ./cmd/mcjava-manager

# Linux ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -buildvcs=false -trimpath -ldflags="-s -w" -o dist/mcjava-manager-linux-arm64 ./cmd/mcjava-manager

# Linux ARMv7
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -buildvcs=false -trimpath -ldflags="-s -w" -o dist/mcjava-manager-linux-armv7 ./cmd/mcjava-manager

# macOS Intel
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
  go build -buildvcs=false -trimpath -ldflags="-s -w" -o dist/mcjava-manager-darwin-amd64 ./cmd/mcjava-manager

# macOS Apple Silicon
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
  go build -buildvcs=false -trimpath -ldflags="-s -w" -o dist/mcjava-manager-darwin-arm64 ./cmd/mcjava-manager

# Windows x86_64
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -buildvcs=false -trimpath -ldflags="-s -w" -o dist/mcjava-manager-windows-amd64.exe ./cmd/mcjava-manager
```

## Verificação pós-instalação

```bash
mcjava-manager version
mcjava-manager --help
```

Saída esperada do `version` (os valores variam conforme o build):

```text
mcjava-manager dev (commit none, build unknown, go1.24.2)
```

Em um build com metadados injetados, a versão e o commit aparecem preenchidos.

## Primeiro uso

1. Feche o Minecraft antes de instalar ou atualizar conteúdo.
2. Informe o caminho da instância:

   ```bash
   mcjava-manager --instance /caminho/da/instancia
   ```

3. A TUI cria `mods/`, `shaderpacks/`, `resourcepacks/` e `saves/` se ainda não existirem.
4. Use a tela **Buscar** para encontrar um projeto e instalar uma versão.
5. Use **Instalados** para conferir o conteúdo, verificar atualizações e remover arquivos.

Também é possível começar pela CLI:

```bash
mcjava-manager search sodium --game-version 1.21.1 --loader fabric
mcjava-manager install sodium --instance /caminho/da/instancia --game-version 1.21.1 --loader fabric
mcjava-manager list --instance /caminho/da/instancia
```

## Atualizar o programa

### Instalação via `go install`

```bash
go install github.com/wedemmmdev/mcjava-manager/cmd/mcjava-manager@latest
```

### Build local

```bash
git pull
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager ./cmd/mcjava-manager
```

Substitua o binário antigo. O manifesto e o conteúdo das instâncias não são alterados ao atualizar o executável.

## Desinstalar

Remova o binário:

```bash
# Linux/macOS
rm ~/go/bin/mcjava-manager
# ou
sudo rm /usr/local/bin/mcjava-manager
```

```powershell
# Windows
Remove-Item "$env:USERPROFILE\go\bin\mcjava-manager.exe"
```

```bash
# Termux
rm "$PREFIX/bin/mcjava-manager"
```

Remover o programa **não** remove mods, shader packs, resource packs nem o manifesto. Se quiser que uma instância deixe de ter o conteúdo rastreado, remova os itens pela CLI/TUI ou apague manualmente `.mcjava-manager/installed.json` depois de fechar o programa. Os arquivos de conteúdo continuam na instância e passarão a aparecer como não rastreados.

## Solução de problemas

### `mcjava-manager: command not found`

O binário não está no `PATH`. Confira onde o Go o instalou:

```bash
go env GOPATH
```

Adicione `$(go env GOPATH)/bin` ao `PATH` e abra um novo terminal.

No Windows, confirme que `%USERPROFILE%\go\bin` está na variável `Path` do usuário.

### `permissão negada`

As mensagens do programa incluem orientação por plataforma:

- **Windows:** execute o terminal como Administrador ou escolha uma pasta dentro do perfil do usuário; verifique a Proteção contra Ransomware e o antivírus.
- **Termux:** execute `termux-setup-storage` e use uma pasta em `~/storage/shared/...`. Se o erro ocorrer ao executar o próprio `mcjava-manager`, conceda permissão de execução:

  ```bash
  chmod +x "$PREFIX/bin/mcjava-manager"
  ```
- **macOS:** verifique **Privacidade e Segurança > Arquivos e Pastas** ou **Acesso Total ao Disco**.
- **Linux:** confira as permissões com `ls -ld` e, se for seguro, conceda escrita com `chmod u+w`.

### `diretório da instância não encontrado`

O caminho não existe ou está incorreto. Use o caminho da pasta que contém `mods/`, `saves/`, `options.txt` etc. Em launchers como PrismLauncher e MultiMC, normalmente é a subpasta `minecraft`.

### `o caminho da instância não é um diretório`

Foi passado um arquivo em vez de uma pasta. Informe o diretório.

### O aplicativo não abre a TUI

A TUI só é aberta quando uma instância é informada. Sem `--instance` nem argumento posicional, o comando raiz mostra a ajuda. Confirme também que o terminal é interativo e tem suporte a cores; use `--no-color` se a renderização estiver estranha.

### Erro de rede ou TLS

Verifique a conexão com a internet e se o acesso a `api.modrinth.com` e aos servidores de download do Modrinth não está bloqueado por firewall, proxy ou antivírus.

### `error obtaining VCS status: exit status 128`

O Go tenta incorporar informações do Git no binário durante o build. Em alguns ambientes, especialmente Termux com o projeto em armazenamento compartilhado, o comando `git status` pode falhar por diferença de proprietário ou permissões, e o build é abortado.

Desative o carimbo de VCS:

```bash
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" \
  -o mcjava-manager ./cmd/mcjava-manager
```

Para `go install`, o equivalente é:

```bash
GOFLAGS=-buildvcs=false go install github.com/wedemmmdev/mcjava-manager/cmd/mcjava-manager@latest
```

Se você precisa manter as informações de VCS e confia no diretório, marque-o como seguro:

```bash
git config --global --add safe.directory "$(pwd)"
```

Só use `safe.directory` em repositórios que você mesmo clonou ou controla.

### `mv: cannot stat 'mcjava-manager': No such file or directory`

O `mv` só funciona depois que o build termina com sucesso. Se o `go build` falhou, corrija o erro exibido antes de executar o `mv` novamente.

### Go antigo demais

Se o build falhar com erro de versão do módulo, atualize o Go para 1.24.2 ou superior e tente novamente.

### Build no Windows sem `CGO`

Use:

```powershell
$env:CGO_ENABLED = "0"
go build -buildvcs=false -trimpath -ldflags="-s -w" -o mcjava-manager.exe .\cmd\mcjava-manager
```

### `arquivo já existe` ao instalar

O destino já contém um arquivo não rastreado com o mesmo nome. Confirme que o arquivo pode ser substituído e use:

```bash
mcjava-manager install <projeto> --instance <caminho> --overwrite
```

Na TUI, pressione `o` na tela do projeto ou na tela de saída para ativar a sobrescrita e repita a operação.

### Conflito de dependência

Se uma dependência já estiver instalada em outra versão, o `install` recusa substituí-la automaticamente. Atualize ou remova a dependência explicitamente e tente novamente. O comando `update` permite atualizar dependências junto com o item principal.

Mais detalhes sobre comandos, filtros e comportamento de conflitos estão em [USAGE.md](USAGE.md).
