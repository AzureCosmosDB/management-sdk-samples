package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"management-sdk-samples/to"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
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
	err                    error
)

func main() {
	loadConfiguration()

	credential, err = azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("failed to obtain a credential: %v", err)
	}

	ctx := context.Background()

	initializeSubscription(ctx)

	createOrUpdateCosmosDBAccount(ctx)
	createOrUpdateCosmosDBDatabase(ctx)
	createOrUpdateCosmosDBContainer(ctx)
	updateThroughput(ctx, 1000)

	fmt.Printf("\n*******Built In Role Definition***************\n")
	builtInRoleDefinitionID, err := getBuiltInDataContributorRoleDefinition(ctx)
	if err != nil {
		log.Fatalf("failed to get built-in data contributor role definition: %v", err)
	}
	createOrUpdateRoleAssignment(ctx, builtInRoleDefinitionID)

	fmt.Printf("\n*******Custom Role Definition***************\n")
	customRoleDefinitionID, err := createOrUpdateCustomRoleDefinition(ctx)
	if err != nil {
		log.Fatalf("failed to create custom role definition: %v", err)
	}
	createOrUpdateRoleAssignment(ctx, customRoleDefinitionID)

}

func loadConfiguration() {
	viper.SetConfigName("appsettings")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	subscriptionID = viper.GetString("SubscriptionId")
	resourceGroupName = viper.GetString("ResourceGroupName")
	accountName = viper.GetString("AccountName")
	location = viper.GetString("Location")
	databaseName = viper.GetString("DatabaseName")
	containerName = viper.GetString("ContainerName")
	maxAutoScaleThroughput = viper.GetInt("MaxAutoScaleThroughput")
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

	accountClient, err := armcosmos.NewDatabaseAccountsClient(subscriptionID, credential, nil)

	if err != nil {
		log.Fatalf("failed to create cosmos db account client: %v", err)
	}

	properties := armcosmos.DatabaseAccountCreateUpdateParameters{
		Location: &location,
		Tags: map[string]*string{
			"key1": to.StringPtr("value1"),
			"key2": to.StringPtr("value2"),
		},
		Properties: &armcosmos.DatabaseAccountCreateUpdateProperties{
			Locations: []*armcosmos.Location{
				{
					LocationName:     &location,
					FailoverPriority: to.Int32Ptr(0),
					IsZoneRedundant:  to.BoolPtr(false),
				},
			},
			Capabilities: []*armcosmos.Capability{
				{
					Name: to.StringPtr("EnableNoSQLVectorSearch"),
				},
			},
			DatabaseAccountOfferType: to.StringPtr("Standard"),
			DisableLocalAuth:         to.BoolPtr(false),
			PublicNetworkAccess:      to.PublicNetworkAccessPtr(armcosmos.PublicNetworkAccessEnabled),
		},
	}

	resourceGroupClient, err := armresources.NewResourceGroupsClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create resource group client: %v", err)
	}

	_, err = resourceGroupClient.Get(ctx, resourceGroupName, nil)
	if err != nil {
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
	fmt.Printf("Created new Account: %s\n", *resp.ID)

}

func createOrUpdateCosmosDBDatabase(ctx context.Context) {

	databaseClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos db database client: %v", err)
	}

	properties := armcosmos.SQLDatabaseCreateUpdateParameters{
		Location: &location,
		Properties: &armcosmos.SQLDatabaseCreateUpdateProperties{
			Resource: &armcosmos.SQLDatabaseResource{
				ID: &databaseName,
			},
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

	fmt.Printf("Created new Database: %s\n", *resp.ID)
}

func createOrUpdateCosmosDBContainer(ctx context.Context) {
	containerClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos db container client: %v", err)
	}

	partitionKind := armcosmos.PartitionKindMultiHash
	indexingMode := armcosmos.IndexingModeConsistent
	conflictResolutionModeLastWriterWins := armcosmos.ConflictResolutionModeLastWriterWins

	properties := armcosmos.SQLContainerCreateUpdateParameters{
		Location: &location,
		Properties: &armcosmos.SQLContainerCreateUpdateProperties{
			Resource: &armcosmos.SQLContainerResource{
				ID: &containerName,
				PartitionKey: &armcosmos.ContainerPartitionKey{
					Paths:   []*string{to.StringPtr("/companyId"), to.StringPtr("/departmentId"), to.StringPtr("/userId")},
					Kind:    &partitionKind,
					Version: to.Int32Ptr(2),
				},
				IndexingPolicy: &armcosmos.IndexingPolicy{
					Automatic:    to.BoolPtr(true),
					IndexingMode: &indexingMode,
					IncludedPaths: []*armcosmos.IncludedPath{
						{Path: to.StringPtr("/*")},
					},
					ExcludedPaths: []*armcosmos.ExcludedPath{
						{Path: to.StringPtr("/\"_etag\"/?")},
					},
				},
				UniqueKeyPolicy: &armcosmos.UniqueKeyPolicy{
					UniqueKeys: []*armcosmos.UniqueKey{
						{Paths: []*string{to.StringPtr("/userId")}},
					},
				},
				ConflictResolutionPolicy: &armcosmos.ConflictResolutionPolicy{
					Mode:                   &conflictResolutionModeLastWriterWins,
					ConflictResolutionPath: to.StringPtr("/_ts"),
				},
			},
			Options: &armcosmos.CreateUpdateOptions{
				AutoscaleSettings: &armcosmos.AutoscaleSettings{
					MaxThroughput: to.Int32Ptr(int32(maxAutoScaleThroughput)),
				},
			},
		},
	}

	databaseClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos db database client: %v", err)
	}

	if _, err := databaseClient.GetSQLDatabase(ctx, resourceGroupName, accountName, databaseName, nil); err != nil {
		log.Fatalf("failed to get cosmos db database: %v", err)
	}

	pollerResp, err := containerClient.BeginCreateUpdateSQLContainer(ctx, resourceGroupName, accountName, databaseName, containerName, properties, nil)
	if err != nil {
		log.Fatalf("failed to begin create or update cosmos db container: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		log.Fatalf("failed to poll the result: %v", err)
	}

	fmt.Printf("Created new Collection: %s\n", *resp.ID)

}

func updateThroughput(ctx context.Context, addThroughput int) {
	throughputClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create throughput client: %v", err)
	}

	throughput := armcosmos.ThroughputSettingsUpdateParameters{
		Location: &location,
		Properties: &armcosmos.ThroughputSettingsUpdateProperties{
			Resource: &armcosmos.ThroughputSettingsResource{
				AutoscaleSettings: &armcosmos.AutoscaleSettingsResource{
					MaxThroughput: to.Int32Ptr(int32(maxAutoScaleThroughput + addThroughput)),
				},
			},
		},
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
}

func createOrUpdateRoleAssignment(ctx context.Context, roleDefinitionID string) {
	roleAssignmentClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		log.Fatalf("failed to create role assignment client: %v", err)
	}

	principalID, err := getCurrentUserPrincipalID(ctx)
	if err != nil {
		log.Fatalf("failed to get current user principal ID: %v", err)
	}

	assignableScope := getAssignableScope(Account)

	properties := armcosmos.SQLRoleAssignmentCreateUpdateParameters{
		Properties: &armcosmos.SQLRoleAssignmentResource{
			RoleDefinitionID: &roleDefinitionID,
			Scope:            &assignableScope,
			PrincipalID:      principalID,
		},
	}

	roleAssignmentID := uuid.New().String()
	pollerResp, err := roleAssignmentClient.BeginCreateUpdateSQLRoleAssignment(ctx, roleAssignmentID, resourceGroupName, accountName, properties, nil)
	if err != nil {
		log.Fatalf("failed to create or update role assignment: %v", err)
	}

	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		log.Fatalf("failed to poll the result: %v", err)
	}

	fmt.Printf("Created new Role Assignment.\n")
}

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

func createOrUpdateCustomRoleDefinition(ctx context.Context) (string, error) {
	roleDefinitionClient, err := armcosmos.NewSQLResourcesClient(subscriptionID, credential, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create role definition client: %v", err)
	}

	assignableScope := []*string{to.StringPtr(getAssignableScope("Account"))}
	roleDefinitionTypeCustomRole := armcosmos.RoleDefinitionTypeCustomRole

	properties := armcosmos.SQLRoleDefinitionCreateUpdateParameters{
		Properties: &armcosmos.SQLRoleDefinitionResource{
			RoleName:         to.StringPtr("My Custom Cosmos DB Data Contributor Except Delete"),
			Type:             &roleDefinitionTypeCustomRole,
			AssignableScopes: assignableScope,
			Permissions: []*armcosmos.Permission{
				{
					DataActions: []*string{
						to.StringPtr("Microsoft.DocumentDB/databaseAccounts/readMetadata"),
						to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/create"),
						to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/read"),
						to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/replace"),
						to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/items/upsert"),
						to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeQuery"),
						to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/readChangeFeed"),
						to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/executeStoredProcedure"),
						to.StringPtr("Microsoft.DocumentDB/databaseAccounts/sqlDatabases/containers/manageConflicts"),
					},
				},
			},
		},
	}

	roleDefinitionID := uuid.New().String()
	pollerResp, err := roleDefinitionClient.BeginCreateUpdateSQLRoleDefinition(ctx, roleDefinitionID, resourceGroupName, accountName, properties, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new role definition: %v", err)
	}

	resp, err := pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		log.Fatalf("failed to poll the result: %v", err)
	}

	fmt.Printf("Created Custom Role Definition: %s\n", *resp.ID)
	return *resp.ID, nil

}

// Define the structure for the Graph API response
type GraphResponse struct {
	ID                string `json:"id"` // This is the Principal ID (Object ID)
	DisplayName       string `json:"displayName"`
	UserPrincipalName string `json:"userPrincipalName"`
}

func getCurrentUserPrincipalID(ctx context.Context) (*string, error) {

	// Create an access token for Microsoft Graph API (scope: https://graph.microsoft.com/.default)
	token, err := credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://graph.microsoft.com/.default"},
	})
	if err != nil {
		log.Fatalf("failed to get access token: %v", err)
	}

	// Prepare the request to the Microsoft Graph API /me endpoint
	url := "https://graph.microsoft.com/v1.0/me"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("failed to create request: %v", err)
	}

	// Add authorization header with the access token
	req.Header.Add("Authorization", "Bearer "+token.Token)

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Read and parse the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read response body: %v", err)
	}

	// Parse JSON response
	var user GraphResponse
	if err := json.Unmarshal(body, &user); err != nil {
		log.Fatalf("failed to parse JSON: %v", err)
	}

	// Output the Principal ID (Object ID)
	fmt.Printf("Principal ID (Object ID): %s, Display Name: %s\n", user.ID, user.DisplayName)

	return &user.ID, nil
}

func getLocalIPAddress() (string, error) {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return "", fmt.Errorf("failed to get local IP address: %v", err)
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read IP address response: %v", err)
	}

	return string(ip), nil
}

type Scope string

const (
	Subscription  Scope = "Subscription"
	ResourceGroup Scope = "ResourceGroup"
	Account       Scope = "Account"
	Database      Scope = "Database"
	Container     Scope = "Container"
)

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
