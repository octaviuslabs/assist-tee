{
  description = "Assist Environment";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = { self, nixpkgs, ... }:
    let
      # Import nixpkgs for aarch64-darwin packages
      pkgsM1 = import nixpkgs {
        system = "aarch64-darwin";
      };
    in {
      devShell.x86_64-darwin = pkgsM1.mkShell {
        name = "ardis-environment";
        buildInputs = [
          pkgsM1.nodejs_22
          pkgsM1.pnpm
          pkgsM1.mailhog
        ];
        
        shellHook = ''
          echo "Setting up custom npm prefix in ~/.npm-global"
          mkdir -p ~/.npm-global
          npm config set prefix ~/.npm-global
          export PATH=$HOME/.npm-global/bin:$PATH
          echo "Installing @anthropic-ai/claude-code globally..."
          npm install -g @anthropic-ai/claude-code
          echo "Claude Code CLI installed at: $HOME/.npm-global/bin/claude-code"
          npm install -g @google/gemini-cli
          export GOOGLE_CLOUD_PROJECT=assist-464816
          echo "Gemini installed"
          curl https://cursor.com/install -fsS | bash
          export PATH="$HOME/.local/bin:$PATH"
          echo "Cursor Cli installed"
          npm install -g @openai/codex
          echo "Codex installed"

        '';
      };
    };

}
