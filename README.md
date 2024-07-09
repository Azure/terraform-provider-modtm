# Terraform ModTM Telemetry Provider

This Terraform provider, named ModTM, is designed to assist with tracking the usage of Terraform modules. It creates a custom `modtm_telemetry` resource that gathers and sends telemetry data to a specified endpoint. The aim is to provide visibility into the lifecycle of your Terraform modules - whether they are being created, updated, or deleted. This data can be invaluable in understanding the usage patterns of your modules, identifying popular modules, and recognizing those that are no longer in use.

In essence, the ModTM provider enhances your Terraform modules with telemetry capabilities, enabling you to make data-driven decisions while ensuring smooth operations and respect for your data privacy. Its non-blocking nature and controlled data collection make it a safe and valuable addition to your Terraform toolkit.

## Minimal and Controlled Data Collection

The ModTM provider is designed with respect for data privacy and control. The only data collected and transmitted are the tags you define in your `modtm_telemetry` resource, and an uuid which represents a module instance's identifier. No other data from your Terraform modules or your environment is collected or transmitted. This gives you full control over the data you wish to collect for telemetry purposes.

## Usage

To use this provider, include the `modtm_telemetry` resource in your Terraform modules. This resource accepts a map of tags, which can include any data relevant to your needs, such as module name, version, cloud provider, etc. During the lifecycle operations (create, read, update, delete) of your Terraform modules, these tags are sent via a HTTP POST request to a specified endpoint.

This resource could be used along with `modtm_module_source` data source to retrieve the current module's version and source:

```hcl
data "azurerm_client_config" "telemetry" {
  count = var.enable_telemetry ? 1 : 0
}

data "modtm_module_source" "telemetry" {
  count       = var.enable_telemetry ? 1 : 0
  module_path = path.module
}

resource "random_uuid" "telemetry" {
  count = var.enable_telemetry ? 1 : 0
}

resource "modtm_telemetry" "this" {
  count = var.enable_telemetry ? 1 : 0

  tags = {
    subscription_id = one(data.azurerm_client_config.telemetry).subscription_id
    tenant_id       = one(data.azurerm_client_config.telemetry).tenant_id
    module_source   = one(data.modtm_module_source.telemetry).module_source
    module_version  = one(data.modtm_module_source.telemetry).module_version
    random_id       = one(random_uuid.telemetry).result
  }
}
```

Or, you can use provider function instead, if your Terraform version supports so:

```hcl
data "azurerm_client_config" "telemetry" {
  count = var.enable_telemetry ? 1 : 0
}

resource "random_uuid" "telemetry" {
  count = var.enable_telemetry ? 1 : 0
}

resource "modtm_telemetry" "this" {
  count = var.enable_telemetry ? 1 : 0

  tags = {
    subscription_id = one(data.azurerm_client_config.telemetry).subscription_id
    tenant_id       = one(data.azurerm_client_config.telemetry).tenant_id
    module_source   = provider::modtm::module_source(path.module)
    module_version  = provider::modtm::module_version(path.module)
    random_id       = one(random_uuid.telemetry).result
  }
}
```

## Safe Operations

One of the primary design principles of the ModTM provider is its non-blocking nature. The provider is designed to work in a way that any network disconnectedness or errors during the telemetry data sending process will not cause a Terraform error or interrupt your Terraform operations. This makes the ModTM provider safe to use even in network-restricted or air-gaped environments.

If the telemetry data cannot be sent due to network issues, the failure will be logged, but it will not affect the Terraform operation in progress(it might delay your operations for no more than 5 seconds). This ensures that your Terraform operations always run smoothly and without interruptions, regardless of the network conditions.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the provider

Fill this in for each provider

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

## Chaos testing

This provider uses [Toxiproxy](https://github.com/Shopify/toxiproxy) so simulate different network issues, now we've tested:

[latency](https://github.com/Shopify/toxiproxy#latency)
[down](https://github.com/Shopify/toxiproxy#down)
[reset_peer](https://github.com/Shopify/toxiproxy#reset_peer)

To run chaos tests, you must [install Toxiproxy](https://github.com/Shopify/toxiproxy#1-installing-toxiproxy), or run Toxiproxy's docker container on linux:

```shell
docker run -d --rm --network=host ghcr.io/shopify/toxiproxy
```

You must set environment `CHAOS` to a non-empty string to enable the chaos tests.