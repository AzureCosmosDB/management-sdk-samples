package to

import "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos"

func PublicNetworkAccessPtr(p armcosmos.PublicNetworkAccess) *armcosmos.PublicNetworkAccess {
	return &p
}

func StringPtr(s string) *string {
	return &s
}

func Int32Ptr(i int32) *int32 {
	return &i
}

func BoolPtr(b bool) *bool {
	return &b
}

func StringPtrSlice(slice []string) []*string {
	ptrSlice := make([]*string, len(slice))
	for i, v := range slice {
		ptrSlice[i] = StringPtr(v)
	}
	return ptrSlice
}
