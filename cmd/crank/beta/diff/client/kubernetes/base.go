package kubernetes

type Clients struct {
	Apply    ApplyClient
	Resource ResourceClient
	Schema   SchemaClient
	Type     TypeConverter
}
