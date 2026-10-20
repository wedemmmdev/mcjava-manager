# Diretrizes do Projeto: mcjava-manager (CLI)

Este documento define as regras de arquitetura, estilo de código, segurança e comportamento da IA (OpenCode) para o desenvolvimento do `mcjava-manager`, um gerenciador de mods, shaders e resourcepacks para Minecraft Java via terminal.

## 1. Identidade e Comportamento da IA
- **Autonomia Controlada:** Escreva e refatore código livremente, mas **NUNCA** exclua arquivos, diretórios inteiros ou mude a arquitetura principal do projeto sem solicitar confirmação explícita do usuário.
- **Transparência em Comandos:** Antes de sugerir a execução de comandos de terminal com efeitos colaterais (como `rm -rf`, alterações de permissões com `chmod`, ou testes que modifiquem disco), explique o que o comando fará.
- **Foco em CLI/TUI:** A experiência do usuário no terminal é prioritária. Respostas interativas devem usar os componentes visuais do `charmbracelet/bubbletea`.

## 2. Stack de Desenvolvimento e Regras de Go (Golang)
- **Versão:** Go 1.21 ou superior.
- **Independência de SO:** O binário deve ser 100% estático. Proibido usar `os/exec` para chamar comandos externos do sistema (como `unzip`, `tar`, `wget` ou `curl`). Use sempre a Standard Library do Go (`archive/zip`, `net/http`).
- **Tratamento de Caminhos:** Sempre use o pacote `path/filepath` para manipular caminhos de arquivos, garantindo compatibilidade entre Windows (barras invertidas) e sistemas Unix/Android (barras normais).
- **Tratamento de Erros:** Não ignore erros (evite usar `_` para abafar retornos de erro). Sempre envolva erros com contexto útil utilizando `fmt.Errorf("falha ao [ação]: %w", err)`. Evite o uso de `panic`; retorne os erros graciosamente para a CLI.
- **Estilo:** Siga as diretrizes oficiais de formatação do Go. Todo código deve ser aprovado pelo `gofmt` e `golangci-lint`.

## 3. Arquitetura e Comandos da CLI
- **Argumento de Instância (Caminho do Jogo):** O comando raiz deve receber o caminho do diretório da instância do jogo de forma totalmente dinâmica como argumento posicional (ex: `mcjava-manager /qualquer/pasta/do/jogo`). **NÃO DEVE** haver validações hardcoded restritas a nomes de loaders (como "fabric", "forge", etc). Basta que o caminho fornecido seja válido para operar as pastas do jogo.
- **Estrutura de Diretórios:**
  - `/cmd`: Ponto de entrada e comandos CLI (via `spf13/cobra`).
  - `/internal`: Lógica de negócios privada (gerenciamento de arquivos, extração, resolução de conflitos).
  - `/pkg`: Código que pode ser útil externamente (ex: clientes da API do Modrinth e CurseForge).
- **Interface de Usuário (TUI):** Use a stack da Charmbracelet. `bubbletea` para a lógica de estado/renderização da interface de busca, `lipgloss` para estilização de cores/bordas e `bubbles` para componentes prontos (listas, inputs).

## 4. Integrações e Segurança (APIs)
- **Modrinth e CurseForge:** Todas as integrações REST devem ter *timeouts* definidos no `http.Client`.
- **Segurança de Chaves (API Keys):** A API do CurseForge exige autenticação. **NUNCA** adicione chaves de API em código limpo ou arquivos testáveis.
- O programa deve tentar ler a `API Key` na seguinte ordem:
  1. Variável de ambiente (ex: `CURSEFORGE_API_KEY`).
  2. Arquivo de configuração local (ex: `~/.mcjava-manager.yaml`).
- Se a chave não for encontrada, exiba uma mensagem instrutiva de como o usuário pode obtê-la e configurá-la.

## 5. Sistema de Arquivos e Resolução de Conflitos
- **Criação de Pastas:** Ao apontar para a pasta do jogo, o CLI deve verificar e criar as pastas base (ex: `mods/`, `shaderpacks/`) usando `os.MkdirAll` dentro do caminho fornecido.
- **Tratamento de Permissões:** Se ocorrer erro de permissão `os.ErrPermission` (comum no Android/Termux ou Windows), capture o erro e instrua o usuário de forma específica para aquela plataforma (ex: `termux-setup-storage` no Android, ou executar como administrador no Windows).
- **Substituição de Arquivos:** Ao baixar um mod já existente, pause a execução e use a TUI para perguntar: `[Substituir] [Ignorar] [Cancelar]`.

## 6. Testes e Qualidade
- O desenvolvimento deve seguir a cultura de testes. Lógicas complexas como resolução de versões e extração de ZIPs devem ter testes unitários correspondentes no formato `_test.go`.
- Ao implementar clientes de API (Modrinth/CurseForge), utilize `httptest.Server` para simular as respostas das APIs durante os testes automatizados.

## 7. Controle de Versão
- Todos os commits devem seguir rigorosamente o padrão **Conventional Commits**:
  - `feat:` Novas funcionalidades.
  - `fix:` Correções de bugs.
  - `refactor:` Mudanças no código que não alteram o comportamento.
  - `test:` Adição ou modificação de testes.
  - `chore:` Atualizações de dependências e ferramentas.
