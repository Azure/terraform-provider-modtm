#!/bin/bash
# must be used with docker image mcr.microsoft.com/azterraform
set -e

MINIBLUE_IMAGE="ghcr.io/lonegunmanb/miniblue:sha-11ef0e8"

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
echo "Created temp directory at $TEMP_DIR"

cleanup() {
  if docker ps -a --format '{{.Names}}' | grep -q '^miniblue$'; then
    docker rm -f miniblue >/dev/null 2>&1 || true
  fi
  rm -rf "$TEMP_DIR"
}

trap cleanup EXIT

wait_miniblue() {
  echo "Waiting for miniblue to be ready..."
  for i in $(seq 1 60); do
    if curl -sf http://localhost:4566/health >/dev/null 2>&1; then
      echo "miniblue is ready."
      return 0
    fi
    sleep 2
  done

  echo "miniblue did not become healthy within 120 seconds"
  docker logs miniblue || true
  return 1
}

setup_miniblue_cert() {
  mkdir -p "$HOME/.miniblue"

  # Trigger cert generation.
  curl -sk https://localhost:4567/health >/dev/null 2>&1 || true

  for i in $(seq 1 30); do
    for p in /home/nonroot/.miniblue/cert.pem /root/.miniblue/cert.pem /app/.miniblue/cert.pem; do
      if docker cp "miniblue:$p" "$HOME/.miniblue/cert.pem" >/dev/null 2>&1; then
        break 2
      fi
    done
    sleep 1
  done

  if [ ! -f "$HOME/.miniblue/cert.pem" ]; then
    echo "WARNING: failed to copy miniblue cert.pem out of container"
    return 0
  fi

  chmod 644 "$HOME/.miniblue/cert.pem"
  export SSL_CERT_FILE="$HOME/.miniblue/cert.pem"

  # Also install into system trust store when available.
  if command -v update-ca-certificates >/dev/null 2>&1; then
    if command -v sudo >/dev/null 2>&1; then
      sudo cp "$HOME/.miniblue/cert.pem" /usr/local/share/ca-certificates/miniblue.crt
      sudo update-ca-certificates >/dev/null 2>&1 || true
    else
      cp "$HOME/.miniblue/cert.pem" /usr/local/share/ca-certificates/miniblue.crt
      update-ca-certificates >/dev/null 2>&1 || true
    fi
  fi
}

create_override_file() {
  local dir="$1"

  cat > "$dir/override.tf" <<'EOL'
terraform {
  required_providers {
    azapi = {
      source = "azure/azapi"
      version = "~> 2.0"
    }
  }
}

provider "azurerm" {
  features {}

  metadata_host               = "localhost:4567"
  skip_provider_registration  = true
  subscription_id             = "00000000-0000-0000-0000-000000000000"
  tenant_id                   = "00000000-0000-0000-0000-000000000001"
  client_id                   = "miniblue"
  client_secret               = "miniblue"
}

provider "azapi" {
  subscription_id = "00000000-0000-0000-0000-000000000000"
  tenant_id       = "00000000-0000-0000-0000-000000000001"
  client_id       = "miniblue"
  client_secret   = "miniblue"
  endpoint = [{
    resource_manager_endpoint = "https://localhost:4567"
  }]
}
EOL
}

# Clone the repositories
REPOS=(
  "https://github.com/Azure/terraform-azurerm-avm-res-cognitiveservices-account.git"
  "https://github.com/Azure/terraform-azurerm-avm-res-keyvault-vault.git"
  "https://github.com/Azure/terraform-azurerm-avm-res-network-virtualnetwork.git"
)

for REPO in "${REPOS[@]}"; do
  git clone "$REPO" "$TEMP_DIR/$(basename "$REPO" .git)"
done

docker pull "$MINIBLUE_IMAGE"
docker run -d --rm --name miniblue -p 4566:4566 -p 4567:4567 -p 443:4567 "$MINIBLUE_IMAGE"
wait_miniblue
setup_miniblue_cert

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

export ARM_USE_OIDC=false
export ARM_SUBSCRIPTION_ID="00000000-0000-0000-0000-000000000000"
export ARM_TENANT_ID="00000000-0000-0000-0000-000000000001"
export ARM_CLIENT_ID="miniblue"
export ARM_CLIENT_SECRET="miniblue"
export ARM_METADATA_HOST="localhost:4567"

for REPO in "${REPOS[@]}"; do
  REPO_DIR="$TEMP_DIR/$(basename "$REPO" .git)"
  cd "$REPO_DIR"/examples/default
  create_override_file "$PWD"
  terraform init
  terraform plan
done

echo "Script completed successfully."