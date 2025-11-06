#!/bin/bash
# must be used with docker image mcr.microsoft.com/azterraform
set -e

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
echo "Created temp directory at $TEMP_DIR"

# Clone the repositories
REPOS=(
  "https://github.com/Azure/terraform-azurerm-avm-res-cognitiveservices-account.git"
  "https://github.com/Azure/terraform-azurerm-avm-res-keyvault-vault.git"
  "https://github.com/Azure/terraform-azurerm-avm-res-network-virtualnetwork.git"
)

for REPO in "${REPOS[@]}"; do
  git clone "$REPO" "$TEMP_DIR/$(basename "$REPO" .git)"
done

cat <<EOL > $HOME/.terraformrc
provider_installation {
  dev_overrides {
   "Azure/modtm" = "/tmp"
  }

  # Install all other providers directly from their origin provider registry as normal.
  # If this is omittet, terraform will only use the dev_overrides block.
  direct {}
}
disable_checkpoint = true
EOL

go build -o /tmp/terraform-provider-modtm

for REPO in "${REPOS[@]}"; do
  REPO_DIR="$TEMP_DIR/$(basename "$REPO" .git)"
  cd "$REPO_DIR"/examples/default
  terraform init
  terraform plan
done

echo "Script completed successfully."