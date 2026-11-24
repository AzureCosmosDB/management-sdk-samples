# Azure Management Java SDK Sample for Azure Cosmos DB

This folder contains a Java sample that uses the **Azure Resource Manager (ARM) / Management Plane** SDKs to create and update **Azure Cosmos DB** resources.

This is useful when your application uses **Microsoft Entra ID** and you want to manage Cosmos DB resources (accounts, databases, containers, throughput, and RBAC) via an SDK instead of Bicep/PowerShell/Azure CLI.

> **Important**: This sample uses the *control plane* (resource provider) APIs via ARM. It does **not** use the Cosmos DB *data plane* SDK to create ARM resources.

## Sample features

### Accounts (control plane)

- Create or update a Cosmos DB **SQL (NoSQL)** account.
- Disables local/key auth (`disableLocalAuth`) so **Entra ID + RBAC** is required.

### Database and container (control plane)

- Create or update a SQL database.
- Create or update a SQL container with:
    - Partition key on `/id`.
    - Indexing policy (consistent), including excluded path for `/_etag`.
    - Unique key on `/userName`.
    - Computed property example (`cp_lowerName`).
    - Last-writer-wins conflict resolution (`/_ts`).
    - Autoscale max throughput from configuration.

### Throughput

- Updates **container dedicated throughput** by reading current settings first and then:
    - Updating autoscale max throughput when the container is autoscale, or
    - Updating RU/s when the container is manual throughput.
- Re-reads and logs the applied settings after the update.
- Throws a clear error when the throughput resource doesn’t exist (common for **serverless** accounts or **shared database throughput**).

### Role-based access control (RBAC)

This sample creates **two role assignments by default** for the currently signed-in principal:

- **Azure RBAC (control plane)**: assigns the built-in `Cosmos DB Operator` role at the Cosmos account scope.
- **Cosmos DB SQL RBAC**: assigns the built-in `Cosmos DB Built-in Data Contributor` role.

### Interactive menu + safe delete

- Runs an interactive menu by default.
- Includes a "Run full sample" menu option.
- Supports running a single menu option via command line args (for example `--option=2`).
- Supports deleting the Cosmos DB account:
    - From the menu (requires typing `DELETE` to confirm)
    - From a single-option run only with `--confirm-delete`
    - From a non-interactive run only when `COSMOS_SAMPLE_DELETE_ACCOUNT=true` (opt-in safety guard)

## Prerequisites

- An Azure subscription and a resource group.
- Java Development Kit (JDK) 25 (LTS). The included wrapper downloads Maven 3.10.2.
- Azure identity available to `DefaultAzureCredential` (for example: `az login`, VS Code sign-in, Managed Identity, etc.).
- Permissions:
    - To create/update Cosmos resources: typically **Contributor** on the resource group.
    - To create Azure RBAC role assignments: typically **Owner** or **User Access Administrator** at the target scope.

### Windows quick setup (recommended)

Install JDK 25 and set `JAVA_HOME`.

#### Option A: With Winget

1) Install JDK 25 (LTS):

```powershell
winget install --id Microsoft.OpenJDK.25 -e
```

1) (Optional) Install Maven.

This repo includes Maven Wrapper, so Maven is not required, but having Maven installed can still be useful.

```powershell
winget install --id Apache.Maven -e
```

1) Set `JAVA_HOME` to your JDK 25 install directory (User or System environment variable), then restart VS Code and any terminals.

1) Verify:

```powershell
.\mvnw.cmd -v
```

This should report Java 25.

Note: `java -version` may still show an older Java if your system `PATH` points to it. Check the Java version reported by the wrapper.

#### Option B: With Chocolatey (`choco`)

1) Open PowerShell **as Administrator** (recommended for Chocolatey installs).

1) Install JDK 25 (LTS):

```powershell
choco install Temurin25 -y
```

1) Restart VS Code and any terminals.

1) Verify:

```powershell
.\mvnw.cmd -v
```

This should report Java 25.

#### Option C: MacOS/Linux manual setup

1) Install a JDK 25 (LTS)

Choose a distribution and download the JDK 25 build for your operating system/architecture:

- [Microsoft Build of OpenJDK 25](https://learn.microsoft.com/java/openjdk/download)
- [Eclipse Temurin 25 (Adoptium)](https://adoptium.net/temurin/releases/?version=25)

1) Set `JAVA_HOME`

- Set `JAVA_HOME` to the JDK install directory (not a JRE).
- Restart VS Code and any terminals after changing environment variables.

1) Verify the active Java with the included Maven Wrapper:

```bash
java -version
./mvnw -v
```

`./mvnw -v` should report Java 25.

### VS Code setup (recommended)

If you want to run/debug this sample from VS Code:

1. Install the Java extension pack: `vscjava.vscode-java-pack`
2. Ensure VS Code is using a JDK 25 runtime:
    - Command Palette → **Java: Configure Java Runtime** → set a **JavaSE-25** runtime as default, OR
    - Set `JAVA_HOME` to a JDK 25 path and restart VS Code.

If Maven reports that release 25 is not supported, check `JAVA_HOME` and restart your terminal and VS Code.

## Usage

1. **Open the Java workspace in VS Code**:

    Open [Java.code-workspace](../Java.code-workspace). This configures the recommended VS Code Java settings for this sample (including the Java runtime).

### Configuration

For local runs/debugging, this sample can read configuration from a local Java properties file.

- Copy [Java/application.properties.example](application.properties.example) to `Java/application.properties` and fill in values.
- Alternatively, provide the environment variables listed by the startup error message.

`application.properties` is ignored by git to avoid accidentally committing local values.

## Setup

From the repo root:

```sh
cd Java
```

Build/test (recommended):

```sh
./mvnw test
```

Windows (PowerShell):

```powershell
.\mvnw.cmd test
```

## Running

### Interactive menu

From the `Java/` folder:

```sh
./mvnw exec:java
```

Windows (PowerShell):

```powershell
.\mvnw.cmd exec:java
```

### Run a single option (non-interactive)

Examples:

```sh
./mvnw exec:java -Dexec.args="--option=2"
./mvnw exec:java -Dexec.args="--option=6 --delta=1000"
```

Windows (PowerShell):

```powershell
.\mvnw.cmd exec:java -Dexec.args="--option=2"
.\mvnw.cmd exec:java -Dexec.args="--option=6 --delta=1000"
```

### Delete the account (opt-in)

The delete option is guarded:

- From the menu: requires typing `DELETE`.
- From a single-option run: requires `--confirm-delete`.

Example:

```sh
./mvnw exec:java -Dexec.args="--option=8 --confirm-delete"
```

### Debug in VS Code

- Use **Run and Debug** → **Java: Debug sample**.

## Functionality

See "Sample features" above.

## Code structure

- `CosmosDBManagement.java`: entry point + menu/CLI parsing.
- `CosmosManagement.java`: Cosmos DB management operations.
- `ConfigLoader.java`: configuration loading from `application.properties` and environment variables.
