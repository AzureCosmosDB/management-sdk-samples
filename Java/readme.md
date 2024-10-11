# CosmosDBManagement

## Overview

`CosmosDBManagement.java` is a Java application that utilizes the Azure SDK to manage Azure Cosmos DB resources. This sample demonstrates how to interact with Cosmos DB using the Azure Resource Manager (ARM) APIs, allowing users to create, update, and manage Cosmos DB accounts, databases, and containers.

## Prerequisites

To run this application, ensure you have the following installed:

- **Java Development Kit (JDK)**: Version 8 or higher.
- **Maven**: For managing dependencies.
- **Azure Account**: You need an Azure subscription. If you don’t have one, you can [create a free account](https://azure.com/free).

## Dependencies

The project uses Azure SDK libraries, which can be included in your Maven `pom.xml` file. Below are the relevant dependencies:


<dependencies>
    <dependency>
        <groupId>com.azure.resourcemanager</groupId>
        <artifactId>azure-resourcemanager-resources</artifactId>
        <version>2.41.0</version> <!-- Check for the latest version -->
    </dependency>
    <dependency>
        <groupId>com.azure.resourcemanager</groupId>
        <artifactId>azure-resourcemanager-cosmos</artifactId>
        <version>2.41.0</version>
    </dependency>
    <dependency>
        <groupId>com.azure</groupId>
        <artifactId>azure-identity</artifactId>
        <version>1.12.2</version>
    </dependency>
    <dependency>
        <groupId>org.slf4j</groupId>
        <artifactId>slf4j-simple</artifactId>
        <version>1.7.32</version>
    </dependency>
    <dependency>
        <groupId>com.microsoft.graph</groupId>
        <artifactId>microsoft-graph</artifactId>
        <version>3.7.0</version> <!-- Check for the latest version -->
    </dependency>
</dependencies>


## Usage

1. **Clone the Repository** (if applicable):
   ```bash
   git clone https://github.com/your-repo/CosmosDBManagement.git
   cd CosmosDBManagement
   ```

2. **Build the Project**:
   Use Maven to build the project:
   ```bash
   mvn clean install
   ```

3. **Run the Application**:
   Execute the application with the following command:
   ```bash
   mvn exec:java -Dexec.mainClass="com.example.CosmosDBManagement"
   ```

## Functionality

This sample provides the following functionalities:

- **Authenticate**: Uses `DefaultAzureCredential` to authenticate with Azure.
- **Manage Cosmos DB Accounts**: Create, update, and delete Cosmos DB accounts.
- **Database and Container Management**: Create and manage databases and containers within the Cosmos DB account.
- **Configure Autoscale Settings**: Set autoscale configurations for containers.

## Code Structure

- **CosmosDBManagement.java**: The main class containing the logic for managing Cosmos DB resources.

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue for any enhancements or bug fixes.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Azure SDK for Java
- Azure Cosmos DB Documentation