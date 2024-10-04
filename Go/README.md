# Azure Management Go SDK Samples for Azure Cosmos DB

Repository of Azure Management Go SDK samples for creating and updating Azure Cosmos DB resources.

## Azure SDK for Python for Azure Cosmos DB

You can find the source code and for this and other Azure SDKs at:
[azure-mgmt-cosmosdb](https://github.com/Azure/azure-sdk-for-python/tree/main/sdk/cosmos/azure-mgmt-cosmosdb)


## Prerequisites
 - Go 1.16 or later

## Setup

### Windows

1. Download the Go installer from the official website.
2. Run the installer and follow the instructions.
3. Verify the installation by opening Command Prompt and running:

```dos
go version
```

### Mac

Install Go using Homebrew:

``` bash
brew install go
```

Verify the installation by opening Terminal and running:

``` bash
go version
```


### Set Up the Project

Initialize a new Go module by opening a terminal (Command Prompt on Windows or Terminal on Mac) and navigate to the project directory.

``` bash
go mod init your-module-name
```

This command creates a go.mod file, which tracks your project's dependencies.

Install the required dependencies:

``` bash

go get github.com/Azure/azure-sdk-for-go/sdk/azidentity
go get github.com/Azure/azure-sdk-for-go/sdk/azcore/policy
go get github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos
go get github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources
go get github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions
go get github.com/spf13/viper
go get github.com/google/uuid

```

The go get command downloads the specified packages and adds them to your go.mod file.

### Configuration

Update the `appsettings.json` in the project directory with the following content:

```json
{
  "SubscriptionId": "{SubscriptionId}",
  "ResourceGroupName": "{ResourceGroupName}",
  "AccountName": "{AccountName}",
  "Location": "West US 3",
  "DatabaseName": "{DatabaseName}",
  "ContainerName": "{ContainerName}",
  "MaxAutoScaleThroughput": 1000
}
```

### Running the Code

Run the Go program:

``` bash
go run main.go
```

### Project Structure

 - main.go: The main application file containing the logic to manage Cosmos DB resources.
 - appsettings.json: Configuration file for the project.
 - to/to.go: Helper functions for converting values to pointers.


## Notes
 - Ensure you have the necessary permissions in your Azure subscription to create and manage Cosmos DB resources.
 - For Go-related issues, refer to the official [Go documentation](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos#section-documentation).