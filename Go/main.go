package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/AzureCosmosDB/management-sdk-samples/Go/to"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

var (
	subscriptionID         string
	resourceGroupName      string
	accountName            string
	location               string
	databaseName           string
	containerName          string
	maxAutoScaleThroughput int
	credential             *azidentity.DefaultAzureCredential
)

// main is the entry point for the Cosmos DB management sample.
func main() {
	loadConfiguration()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("failed to obtain a credential: %v", err)
	}
	credential = cred

	ctx := context.Background()

	// If we're not running in an interactive terminal (e.g., CI), fall back to the full sample.
	if !isInteractiveTerminal() {
		runFullSample(ctx)
		return
	}

	runInteractiveMenu(ctx)
}

// runFullSample runs the end-to-end management-plane workflow with sensible defaults.
func runFullSample(ctx context.Context) {
	initializeSubscription(ctx)

	createOrUpdateCosmosDBAccount(ctx)
	createOrUpdateAzureRoleAssignment(ctx)
	createOrUpdateCosmosDBDatabase(ctx)
	createOrUpdateCosmosDBContainer(ctx)
	updateThroughput(ctx, 1000)

	// Cosmos DB SQL RBAC (built-in data contributor)
	builtInRoleDefinitionID, err := getBuiltInDataContributorRoleDefinition(ctx)
	if err != nil {
		log.Fatalf("failed to get built-in data contributor role definition: %v", err)
	}
	createOrUpdateRoleAssignment(ctx, builtInRoleDefinitionID)

	// Optional cleanup: set COSMOS_SAMPLE_DELETE_ACCOUNT=true to delete the account at the end of a full run.
	if strings.EqualFold(os.Getenv("COSMOS_SAMPLE_DELETE_ACCOUNT"), "true") {
		deleteCosmosDBAccount(ctx)
	}
}

// runInteractiveMenu runs a simple interactive menu for the sample.
func runInteractiveMenu(ctx context.Context) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println()
		fmt.Println("Cosmos management sample - choose an action:")
		fmt.Println("  1) Run full sample")
		fmt.Println("  2) Create/update Cosmos DB account")
		fmt.Println("  3) Create Azure RBAC assignment (Cosmos DB Operator)")
		fmt.Println("  4) Create/update NoSQL database")
		fmt.Println("  5) Create/update NoSQL container")
		fmt.Println("  6) Update container throughput (+delta)")
		fmt.Println("  7) Create Cosmos NoSQL RBAC assignment (Built-in Data Contributor)")
		fmt.Println("  8) Delete Cosmos DB account")
		fmt.Println("  0) Exit")
		fmt.Print("Selection: ")

		selection, err := readLine(reader)
		if err != nil {
			log.Printf("Failed to read selection: %v", err)
			continue
		}
		selection = strings.ToLower(strings.TrimSpace(selection))
		if selection == "" {
			continue
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Operation panicked: %v", r)
				}
			}()

			switch selection {
			case "0", "q", "quit", "exit":
				os.Exit(0)
			case "1":
				runFullSample(ctx)
			case "2":
				initializeSubscription(ctx)
				createOrUpdateCosmosDBAccount(ctx)
			case "3":
				initializeSubscription(ctx)
				createOrUpdateAzureRoleAssignment(ctx)
			case "4":
				createOrUpdateCosmosDBDatabase(ctx)
			case "5":
				createOrUpdateCosmosDBContainer(ctx)
			case "6":
				delta := promptInt(reader, "Throughput delta to add", 1000)
				updateThroughput(ctx, delta)
			case "7":
				builtInRoleDefinitionID, err := getBuiltInDataContributorRoleDefinition(ctx)
				if err != nil {
					log.Printf("failed to get built-in data contributor role definition: %v", err)
					return
				}
				createOrUpdateRoleAssignment(ctx, builtInRoleDefinitionID)
			case "8":
				if confirmDelete(reader) {
					deleteCosmosDBAccount(ctx)
				} else {
					fmt.Println("Delete cancelled.")
				}
			default:
				fmt.Println("Unknown selection.")
			}
		}()
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err == nil {
		return strings.TrimRight(line, "\r\n"), nil
	}

	if errors.Is(err, io.EOF) {
		// Accept EOF as a valid "last line".
		return strings.TrimRight(line, "\r\n"), nil
	}

	return "", err
}

func promptInt(reader *bufio.Reader, label string, defaultValue int) int {
	fmt.Printf("%s (default %d): ", label, defaultValue)
	raw, err := readLine(reader)
	if err != nil {
		return defaultValue
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return value
}

func confirmDelete(reader *bufio.Reader) bool {
	fmt.Print("Type DELETE to confirm deleting the Cosmos DB account: ")
	raw, err := readLine(reader)
	if err != nil {
		return false
	}
	return strings.TrimSpace(raw) == "DELETE"
}

func isInteractiveTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func loadConfiguration() {
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.SetConfigName("config")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Missing configuration. Copy Go/config.json.sample to Go/config.json and fill it in. Original error: %v", err)
	}

	subscriptionID = strings.TrimSpace(viper.GetString("SubscriptionId"))
	resourceGroupName = strings.TrimSpace(viper.GetString("ResourceGroupName"))
	accountName = strings.TrimSpace(viper.GetString("AccountName"))
	location = strings.TrimSpace(viper.GetString("Location"))
	databaseName = strings.TrimSpace(viper.GetString("DatabaseName"))
	containerName = strings.TrimSpace(viper.GetString("ContainerName"))

	missing := make([]string, 0, 7)
	if subscriptionID == "" {
		missing = append(missing, "SubscriptionId")
	}
	if resourceGroupName == "" {
		missing = append(missing, "ResourceGroupName")
	}
	if accountName == "" {
		missing = append(missing, "AccountName")
	}
	if location == "" {
		missing = append(missing, "Location")
	}
	if databaseName == "" {
		missing = append(missing, "DatabaseName")
	}
	if containerName == "" {
		missing = append(missing, "ContainerName")
	}
	if !viper.IsSet("MaxAutoScaleThroughput") {
		missing = append(missing, "MaxAutoScaleThroughput")
	}
	if len(missing) > 0 {
		log.Fatalf("Missing required configuration values: %s. Copy Go/config.json.sample to Go/config.json and fill it in.", strings.Join(missing, ", "))
	}

	maxAutoScaleThroughput = viper.GetInt("MaxAutoScaleThroughput")
	if maxAutoScaleThroughput < 1000 {
		log.Fatalf("MaxAutoScaleThroughput must be >= 1000 (got %d)", maxAutoScaleThroughput)
	}
}

func initializeSubscription(ctx context.Context) {
	subscriptionClient, err := armsubscriptions.NewClient(credential, nil)
	if err != nil {
		log.Fatalf("failed to create subscription client: %v", err)
	}

	subscription, err := subscriptionClient.Get(ctx, subscriptionID, nil)
	if err != nil {
		log.Fatalf("failed to get subscription: %v", err)
	}

	fmt.Printf("Subscription ID: %s\n", *subscription.ID)
}

func createOrUpdateCosmosDBAccount(ctx context.Context) {
	log.Printf("Starting Cosmos DB account create/update (this can take a couple minutes): account=%s", accountName)

	accountClient, err := armcosmos.NewDatabaseAccountsClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos db account client: %v", err)
	}

	properties := armcosmos.DatabaseAccountCreateUpdateParameters{
		Location: &location,
		Tags: map[string]*string{
			"owner": to.StringPtr(getCurrentUserEmailBestEffort(ctx)),
		},
		Properties: &armcosmos.DatabaseAccountCreateUpdateProperties{
			Locations: []*armcosmos.Location{{
				LocationName:     &location,
				FailoverPriority: to.Int32Ptr(0),
				IsZoneRedundant:  to.BoolPtr(false),
			}},
			Capabilities: []*armcosmos.Capability{{
				Name: to.StringPtr("EnableNoSQLVectorSearch"),
			}},
			// Uncomment to experiment with serverless.
			// Capabilities: append(capabilities, &armcosmos.Capability{Name: to.StringPtr("EnableServerless")}),
			DatabaseAccountOfferType: to.StringPtr("Standard"),
			DisableLocalAuth:         to.BoolPtr(true),
			PublicNetworkAccess:      to.PublicNetworkAccessPtr(armcosmos.PublicNetworkAccessEnabled),
		},
	}

	resourceGroupClient, err := armresources.NewResourceGroupsClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create resource group client: %v", err)
	}
	if _, err := resourceGroupClient.Get(ctx, resourceGroupName, nil); err != nil {
		log.Fatalf("failed to get resource group: %v", err)
	}

	pollerResp, err := accountClient.BeginCreateOrUpdate(ctx, resourceGroupName, accountName, properties, nil)
	if err != nil {
		log.Fatalf("failed to begin create or update cosmos db account: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		log.Fatalf("failed to poll the result: %v", err)
	}
	if resp.ID != nil {
		fmt.Printf("Created/updated Account: %s\n", *resp.ID)
		return
	}
	fmt.Println("Created/updated Account.")
}

func deleteCosmosDBAccount(ctx context.Context) {
	log.Printf("Starting Cosmos DB account delete (this can take a couple minutes): account=%s", accountName)

	accountClient, err := armcosmos.NewDatabaseAccountsClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos db account client: %v", err)
	}

	pollerResp, err := accountClient.BeginDelete(ctx, resourceGroupName, accountName, nil)
	if err != nil {
		log.Fatalf("failed to begin delete cosmos db account: %v", err)
	}

	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		log.Fatalf("failed to delete cosmos db account: %v", err)
	}

	fmt.Printf("Deleted Cosmos DB account: %s\n", accountName)
}

// createOrUpdateCosmosDBDatabase creates or updates a SQL database.
func createOrUpdateCosmosDBDatabase(ctx context.Context) {
	databaseClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos db database client: %v", err)
	}

	properties := armcosmos.SQLDatabaseCreateUpdateParameters{
		Location: &location,
		Properties: &armcosmos.SQLDatabaseCreateUpdateProperties{
			Resource: &armcosmos.SQLDatabaseResource{ID: &databaseName},
		},
	}

	accountClient, err := armcosmos.NewDatabaseAccountsClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos db account client: %v", err)
	}
	if _, err := accountClient.Get(ctx, resourceGroupName, accountName, nil); err != nil {
		log.Fatalf("failed to get cosmos db account: %v", err)
	}

	pollerResp, err := databaseClient.BeginCreateUpdateSQLDatabase(ctx, resourceGroupName, accountName, databaseName, properties, nil)
	if err != nil {
		log.Fatalf("failed to begin create or update cosmos db database: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		log.Fatalf("failed to poll the result: %v", err)
	}

	fmt.Printf("Created/updated Database: %s\n", *resp.ID)
}

// createOrUpdateCosmosDBContainer creates or updates a NoSQL container and configures throughput.
func createOrUpdateCosmosDBContainer(ctx context.Context) {
	containerClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos db container client: %v", err)
	}

	if _, err := containerClient.GetSQLDatabase(ctx, resourceGroupName, accountName, databaseName, nil); err != nil {
		log.Fatalf("failed to get cosmos db database: %v", err)
	}

	// NOTE: The Go `armcosmos` management SDK does not currently expose some newer SQL container fields
	// (computed properties and vector settings like vectorEmbeddingPolicy / indexingPolicy.vectorIndexes).
	// This sample creates the container using only fields currently supported by the SDK.
	// When these features become supported in the Go management SDK, we will add them here.

	partitionKind := armcosmos.PartitionKindMultiHash
	indexingMode := armcosmos.IndexingModeConsistent
	conflictResolutionModeLastWriterWins := armcosmos.ConflictResolutionModeLastWriterWins

	properties := armcosmos.SQLContainerCreateUpdateParameters{
		Location: &location,
		Properties: &armcosmos.SQLContainerCreateUpdateProperties{
			Resource: &armcosmos.SQLContainerResource{
				ID:         &containerName,
				DefaultTTL: to.Int32Ptr(-1),
				PartitionKey: &armcosmos.ContainerPartitionKey{
					Paths:   []*string{to.StringPtr("/companyId"), to.StringPtr("/departmentId"), to.StringPtr("/userId")},
					Kind:    &partitionKind,
					Version: to.Int32Ptr(2),
				},
				IndexingPolicy: &armcosmos.IndexingPolicy{
					Automatic:     to.BoolPtr(true),
					IndexingMode:  &indexingMode,
					IncludedPaths: []*armcosmos.IncludedPath{{Path: to.StringPtr("/*")}},
					ExcludedPaths: []*armcosmos.ExcludedPath{{Path: to.StringPtr("/\"_etag\"/?")}},
				},
				UniqueKeyPolicy: &armcosmos.UniqueKeyPolicy{
					UniqueKeys: []*armcosmos.UniqueKey{{Paths: []*string{to.StringPtr("/userId")}}},
				},
				ConflictResolutionPolicy: &armcosmos.ConflictResolutionPolicy{
					Mode:                   &conflictResolutionModeLastWriterWins,
					ConflictResolutionPath: to.StringPtr("/_ts"),
				},
			},
			Options: &armcosmos.CreateUpdateOptions{
				AutoscaleSettings: &armcosmos.AutoscaleSettings{MaxThroughput: to.Int32Ptr(int32(maxAutoScaleThroughput))},
			},
		},
	}

	pollerResp, err := containerClient.BeginCreateUpdateSQLContainer(ctx, resourceGroupName, accountName, databaseName, containerName, properties, nil)
	if err != nil {
		log.Fatalf("failed to begin create or update cosmos db container: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		log.Fatalf("failed to poll the result: %v", err)
	}

	fmt.Printf("Created/updated Collection: %s\n", *resp.ID)
}

// updateThroughput updates the container throughput by a delta, handling autoscale vs manual throughput.
func updateThroughput(ctx context.Context, addThroughput int) {
	log.Printf(
		"Starting throughput update (this can take a couple minutes): account=%s, database=%s, container=%s, delta=%d",
		accountName,
		databaseName,
		containerName,
		addThroughput,
	)

	throughputClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create throughput client: %v", err)
	}

	existing, err := throughputClient.GetSQLContainerThroughput(ctx, resourceGroupName, accountName, databaseName, containerName, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 404 {
			log.Fatalf("Container throughput settings were not found. This usually means the container uses shared database throughput or serverless, and therefore does not have a dedicated throughput resource to update. Create the container with dedicated throughput (or update database throughput instead), then retry.")
		}
		log.Fatalf("failed to read existing container throughput settings: %v", err)
	}

	existingResource := (*armcosmos.ThroughputSettingsGetPropertiesResource)(nil)
	if existing.Properties != nil {
		existingResource = existing.Properties.Resource
	}
	if existingResource == nil {
		log.Fatalf("Container throughput settings did not include a resource payload. The container likely uses shared database throughput or serverless.")
	}

	currentAutoscaleMax := (*int32)(nil)
	if existingResource.AutoscaleSettings != nil {
		currentAutoscaleMax = existingResource.AutoscaleSettings.MaxThroughput
	}
	currentManualThroughput := existingResource.Throughput

	throughput := armcosmos.ThroughputSettingsUpdateParameters{
		Location: &location,
		Properties: &armcosmos.ThroughputSettingsUpdateProperties{
			Resource: &armcosmos.ThroughputSettingsResource{},
		},
	}

	if currentAutoscaleMax != nil {
		baseline := int64(*currentAutoscaleMax)
		if baseline == 0 {
			baseline = int64(maxAutoScaleThroughput)
		}
		newAutoscaleMax := baseline + int64(addThroughput)
		if newAutoscaleMax < 1000 {
			newAutoscaleMax = 1000
		}

		fmt.Printf("Updating container autoscale max throughput from %d to %d\n", *currentAutoscaleMax, newAutoscaleMax)
		throughput.Properties.Resource.AutoscaleSettings = &armcosmos.AutoscaleSettingsResource{MaxThroughput: to.Int32Ptr(int32(newAutoscaleMax))}
	} else {
		currentManual := int64(0)
		if currentManualThroughput != nil {
			currentManual = int64(*currentManualThroughput)
		}

		baseline := currentManual
		adjustedDelta := int64(addThroughput)
		if baseline == 0 {
			baseline = adjustedDelta
			adjustedDelta = 0
		}
		newManualThroughput := baseline + adjustedDelta
		if newManualThroughput < 400 {
			newManualThroughput = 400
		}

		fmt.Printf("Updating container manual throughput from %d to %d\n", currentManual, newManualThroughput)
		throughput.Properties.Resource.Throughput = to.Int32Ptr(int32(newManualThroughput))
	}

	pollerResp, err := throughputClient.BeginUpdateSQLContainerThroughput(ctx, resourceGroupName, accountName, databaseName, containerName, throughput, nil)
	if err != nil {
		log.Fatalf("failed to update throughput: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		log.Fatalf("failed to poll the result: %v", err)
	}
	fmt.Printf("Updated collection throughput for: %s\n", *resp.ID)

	applied, err := throughputClient.GetSQLContainerThroughput(ctx, resourceGroupName, accountName, databaseName, containerName, nil)
	if err != nil {
		log.Fatalf("failed to read applied container throughput settings: %v", err)
	}

	var appliedAutoscaleMax any
	var appliedManual any
	if applied.Properties != nil && applied.Properties.Resource != nil {
		if applied.Properties.Resource.AutoscaleSettings != nil && applied.Properties.Resource.AutoscaleSettings.MaxThroughput != nil {
			appliedAutoscaleMax = *applied.Properties.Resource.AutoscaleSettings.MaxThroughput
		}
		if applied.Properties.Resource.Throughput != nil {
			appliedManual = *applied.Properties.Resource.Throughput
		}
	}
	fmt.Printf("Applied throughput settings: autoscaleMax=%v, manual=%v\n", appliedAutoscaleMax, appliedManual)
}

// createOrUpdateRoleAssignment creates or updates a Cosmos SQL RBAC role assignment for the current principal.
func createOrUpdateRoleAssignment(ctx context.Context, roleDefinitionID string) {
	roleAssignmentClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create role assignment client: %v", err)
	}

	principalID, err := getCurrentPrincipalObjectID(ctx)
	if err != nil {
		log.Fatalf("failed to get current user principal ID: %v", err)
	}

	assignableScope := getAssignableScope(Account)

	properties := armcosmos.SQLRoleAssignmentCreateUpdateParameters{Properties: &armcosmos.SQLRoleAssignmentResource{RoleDefinitionID: &roleDefinitionID, Scope: &assignableScope, PrincipalID: to.StringPtr(principalID)}}
	roleAssignmentID := uuid5Name(fmt.Sprintf("%s|%s|%s", assignableScope, roleDefinitionID, principalID))

	pollerResp, err := roleAssignmentClient.BeginCreateUpdateSQLRoleAssignment(ctx, roleAssignmentID, resourceGroupName, accountName, properties, nil)
	if err != nil {
		log.Fatalf("failed to create or update role assignment: %v", err)
	}

	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		log.Fatalf("failed to poll the result: %v", err)
	}

	fmt.Println("Created/updated Cosmos SQL RBAC role assignment.")
}

// createOrUpdateAzureRoleAssignment assigns the built-in Azure RBAC role (Cosmos DB Operator) at account scope.
func createOrUpdateAzureRoleAssignment(ctx context.Context) {
	principalObjectID, err := getCurrentPrincipalObjectID(ctx)
	if err != nil {
		log.Fatalf("failed to get current principal object id: %v", err)
	}

	roleDefinitionResourceID, err := getBuiltInCosmosDbOperatorRoleDefinitionID(ctx)
	if err != nil {
		log.Fatalf("failed to resolve Azure RBAC role definition (Cosmos DB Operator): %v", err)
	}

	scope := getAssignableScope(Account)
	createOrUpdateAzureRoleAssignmentWithDefinition(ctx, scope, roleDefinitionResourceID, principalObjectID)
}

// createOrUpdateAzureRoleAssignmentWithDefinition creates or updates an Azure RBAC role assignment idempotently.
func createOrUpdateAzureRoleAssignmentWithDefinition(ctx context.Context, scope string, roleDefinitionResourceID string, principalObjectID string) {
	roleAssignmentsClient, err := armauthorization.NewRoleAssignmentsClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create Azure RBAC role assignments client: %v", err)
	}

	roleAssignmentName := uuid5Name(fmt.Sprintf("%s|%s|%s", scope, roleDefinitionResourceID, principalObjectID))
	properties := armauthorization.RoleAssignmentCreateParameters{Properties: &armauthorization.RoleAssignmentProperties{RoleDefinitionID: to.StringPtr(roleDefinitionResourceID), PrincipalID: to.StringPtr(principalObjectID)}}

	resp, err := roleAssignmentsClient.Create(ctx, scope, roleAssignmentName, properties, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) {
			if respErr.StatusCode == 409 {
				existing, getErr := roleAssignmentsClient.Get(ctx, scope, roleAssignmentName, nil)
				if getErr == nil && existing.ID != nil {
					fmt.Printf("Azure RBAC role assignment already exists: %s\n", *existing.ID)
					return
				}
				fmt.Println("Azure RBAC role assignment already exists.")
				return
			}
			log.Fatalf("failed to create Azure RBAC role assignment (status %d): %v", respErr.StatusCode, err)
		}
		log.Fatalf("failed to create Azure RBAC role assignment: %v", err)
	}

	if resp.ID != nil {
		fmt.Printf("Created Azure RBAC role assignment: %s\n", *resp.ID)
		return
	}
	fmt.Println("Created Azure RBAC role assignment.")
}

// getBuiltInCosmosDbOperatorRoleDefinitionID resolves the Azure RBAC role definition ID by role name.
func getBuiltInCosmosDbOperatorRoleDefinitionID(ctx context.Context) (string, error) {
	subscriptionScope := getAssignableScope(Subscription)
	return getAzureRoleDefinitionIDByName(ctx, subscriptionScope, "Cosmos DB Operator")
}

// getAzureRoleDefinitionIDByName returns a role definition resource ID for a role name at the given scope.
func getAzureRoleDefinitionIDByName(ctx context.Context, scope string, roleName string) (string, error) {
	roleDefinitionsClient, err := armauthorization.NewRoleDefinitionsClient(credential, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create Azure RBAC role definitions client: %w", err)
	}

	filter := fmt.Sprintf("roleName eq '%s'", strings.ReplaceAll(roleName, "'", "''"))
	pager := roleDefinitionsClient.NewListPager(scope, &armauthorization.RoleDefinitionsClientListOptions{Filter: to.StringPtr(filter)})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to list Azure RBAC role definitions: %w", err)
		}

		for _, def := range page.Value {
			if def == nil || def.Properties == nil || def.Properties.RoleName == nil || def.ID == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(*def.Properties.RoleName), roleName) {
				return *def.ID, nil
			}
		}
	}

	return "", fmt.Errorf("Azure RBAC role definition not found at scope %q: %s", scope, roleName)
}

// uuid5Name creates a deterministic UUID v5 (name-based) for stable IDs across reruns.
func uuid5Name(name string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(name)).String()
}

// getBuiltInDataContributorRoleDefinition returns the Cosmos SQL RBAC built-in data contributor role definition ID.
func getBuiltInDataContributorRoleDefinition(ctx context.Context) (string, error) {
	roleDefinitionClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create role definition client: %v", err)
	}

	roleDefinitionID := "00000000-0000-0000-0000-000000000002"
	roleDefinition, err := roleDefinitionClient.GetSQLRoleDefinition(ctx, roleDefinitionID, resourceGroupName, accountName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get role definition: %v", err)
	}

	fmt.Printf("Found Built In Role Definition : %s\n", *roleDefinition.ID)
	return *roleDefinition.ID, nil
}

// createOrUpdateCustomRoleDefinition creates a custom Cosmos SQL RBAC role definition (delete action commented out).
func createOrUpdateCustomRoleDefinition(ctx context.Context) (string, error) {
	roleDefinitionClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create role definition client: %v", err)
	}

	assignableScope := []*string{to.StringPtr(getAssignableScope(Account))}
	roleDefinitionTypeCustomRole := armcosmos.RoleDefinitionTypeCustomRole

	properties := armcosmos.SQLRoleDefinitionCreateUpdateParameters{
		Properties: &armcosmos.SQLRoleDefinitionResource{
			RoleName:         to.StringPtr("My Custom Cosmos DB Data Contributor Except Delete"),
			Type:             &roleDefinitionTypeCustomRole,
			AssignableScopes: assignableScope,
			Permissions: []*armcosmos.Permission{{
				DataActions: []*string{
					to.StringPtr("Microsoft.DocumentDB/databaseAccounts/readMetadata"),
					to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/create"),
					// to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/delete"),
					to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/read"),
					to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/replace"),
					to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/upsert"),
					to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeQuery"),
					to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/readChangeFeed"),
					to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeStoredProcedure"),
					to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/manageConflicts"),
				},
			}},
		},
	}

	roleDefinitionID := uuid.New().String()
	pollerResp, err := roleDefinitionClient.BeginCreateUpdateSQLRoleDefinition(ctx, roleDefinitionID, resourceGroupName, accountName, properties, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new role definition: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to poll the result: %v", err)
	}

	fmt.Printf("Created Custom Role Definition: %s\n", *resp.ID)
	return *resp.ID, nil
}

const armTokenScope = "https://management.azure.com/.default"

// getCurrentPrincipalObjectID returns the current principal object ID from the ARM token (or env override).
func getCurrentPrincipalObjectID(ctx context.Context) (string, error) {
	if override := strings.TrimSpace(os.Getenv("AZURE_PRINCIPAL_OBJECT_ID")); override != "" {
		return override, nil
	}

	claims, err := getArmTokenClaims(ctx)
	if err != nil {
		return "", err
	}

	if oid, ok := claims["oid"].(string); ok && strings.TrimSpace(oid) != "" {
		return strings.TrimSpace(oid), nil
	}

	return "", fmt.Errorf("could not determine current principal object id (oid) from the ARM access token")
}

// getCurrentUserEmailBestEffort extracts a user identifier (UPN/email) from the ARM token claims.
func getCurrentUserEmailBestEffort(ctx context.Context) string {
	claims, err := getArmTokenClaims(ctx)
	if err != nil {
		return ""
	}

	for _, key := range []string{"preferred_username", "upn", "unique_name"} {
		if value, ok := claims[key].(string); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}

	return ""
}

// getArmTokenClaims acquires an ARM access token and parses its JWT claims.
func getArmTokenClaims(ctx context.Context) (map[string]any, error) {
	token, err := credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{armTokenScope}})
	if err != nil {
		return nil, fmt.Errorf("failed to acquire ARM access token: %w", err)
	}

	parts := strings.Split(token.Token, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid JWT token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to base64url-decode token payload: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse token claims: %w", err)
	}

	return claims, nil
}

type Scope string

const (
	Subscription  Scope = "Subscription"
	ResourceGroup Scope = "ResourceGroup"
	Account       Scope = "Account"
	Database      Scope = "Database"
	Container     Scope = "Container"
)

// getAssignableScope returns the ARM scope string used for role definitions/assignments.
func getAssignableScope(scope Scope) string {
	switch scope {
	case Subscription:
		return fmt.Sprintf("/subscriptions/%s", subscriptionID)
	case ResourceGroup:
		return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", subscriptionID, resourceGroupName)
	case Account:
		return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s", subscriptionID, resourceGroupName, accountName)
	case Database:
		return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s/dbs/%s", subscriptionID, resourceGroupName, accountName, databaseName)
	case Container:
		return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s/dbs/%s/colls/%s", subscriptionID, resourceGroupName, accountName, databaseName, containerName)
	default:
		return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.DocumentDB/databaseAccounts/%s", subscriptionID, resourceGroupName, accountName)
	}
}
