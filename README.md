# Azure Management SDK Samples for Azure Cosmos DB

This repository contains of Azure Management SDK samples for Azure Cosmos DB to create and update Azure Cosmos DB resources through it's resource provider (control plane). Azure resource providers are used to provision and manage resources in Azure. 

The samples can be used to support users who are adopting Microsoft Entra Id and disabling key-based access to their data and want to manage their Cosmos resources using an SDK rather than use Bicep templates, PowerShell or Azure CLI.

## Resource Operations

These samples demonstrate create or update operations on the following Cosmos DB resources, including:

- Accounts:
    - Serverless, IP firewall rules, and disabling local auth (key-based data plane access) to force Entra Id for Auth-N and role-based access control (RBAC) for Auth-Z
- Databases
- Containers:
    - Hierarchical partition keys, index policies, unique keys, ttl, conflict resolution policies, autoscale throughput
- Throughput
    - Update autoscale throughput
- RBAC definitions:
    - Creating a built-in definition, creating a custom RBAC definition
- RBAC assignments

The samples also include some other handy conveniences for developers working with Cosmos resources including: 

- Getting your IP address to add to a Firewall Rule, add Portal Access and Azure data center access
- Getting your principal id for RBAC assignments 

## Languages available 

The samples are available in 5 languages including:

- [C#](/Csharp/)
- [Python](/Python/)
- [Go](/Go/)
- [Java](/Java)
- [JavaScript](/JavaScript/) Not yet available


## Azure Management SDK source code repositories

All Azure Management SDK's are open source. For more information and the source for the underlying Azure Management SDK's, visit their GitHub repositories.

- [C#](https://github.com/Azure/azure-sdk-for-net/tree/main/sdk/cosmosdb/Azure.ResourceManager.CosmosDB)
- [Python](https://github.com/Azure/azure-sdk-for-python/tree/main/sdk/cosmos/azure-mgmt-cosmosdb)
- [Go](https://github.com/Azure/azure-sdk-for-go/tree/main/sdk/resourcemanager/cosmos/armcosmos)
- [Java](https://github.com/Azure/azure-sdk-for-java/tree/main/sdk/resourcemanager/azure-resourcemanager-cosmos)
- [JavaScript](https://github.com/Azure/azure-sdk-for-js/tree/main/sdk/cosmosdb/arm-cosmosdb)


## How to file issues and get help  

Azure Management SDKs are fully supported by Microsoft. For issues related to them please open a support ticket. The GitHub issues here is not the correct channel for these.

For questions, feature requests or issues related to just the samples themselves, please use GitHub Issues. 
Please search the existing issues before filing new ones to avoid duplicates.


## Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.opensource.microsoft.com.

When you submit a pull request, a CLA bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., status check, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

## Trademarks

This project may contain trademarks or logos for projects, products, or services. Authorized use of Microsoft 
trademarks or logos is subject to and must follow 
[Microsoft's Trademark & Brand Guidelines](https://www.microsoft.com/en-us/legal/intellectualproperty/trademarks/usage/general).
Use of Microsoft trademarks or logos in modified versions of this project must not cause confusion or imply Microsoft sponsorship.
Any use of third-party trademarks or logos are subject to those third-party's policies.
