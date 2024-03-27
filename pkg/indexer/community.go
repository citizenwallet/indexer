package indexer

type Community struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	URL          string `json:"url"`
	Alias        string `json:"alias"`
	Logo         string `json:"logo"`
	Hidden       bool   `json:"hidden,omitempty"`
	CustomDomain string `json:"custom_domain,omitempty"`
	Theme        *Theme `json:"theme,omitempty"`
}

type Theme struct {
	PrimaryColor string `json:"primaryColor"`
}

type CommunityScan struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

type CommunityIndexer struct {
	URL     string `json:"url"`
	IPFSURL string `json:"ipfs_url"`
	Key     string `json:"key"`
}

type CommunityIPFS struct {
	URL string `json:"url"`
}

type CommunityNode struct {
	ChainID int    `json:"chain_id"`
	URL     string `json:"url"`
	WSURL   string `json:"ws_url"`
}

type CommunityERC4337 struct {
	RPCURL                string `json:"rpc_url"`
	PaymasterAddress      string `json:"paymaster_address"`
	EntrypointAddress     string `json:"entrypoint_address"`
	AccountFactoryAddress string `json:"account_factory_address"`
	PaymasterRPCURL       string `json:"paymaster_rpc_url"`
	PaymasterType         string `json:"paymaster_type"`
}

type CommunityToken struct {
	Standard string `json:"standard"`
	Address  string `json:"address"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type CommunityProfile struct {
	Address string `json:"address"`
}

type CommunityPlugin struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
	URL  string `json:"url"`
}

type CommunityConfig struct {
	Community Community         `json:"community"`
	Scan      CommunityScan     `json:"scan"`
	Indexer   CommunityIndexer  `json:"indexer"`
	IPFS      CommunityIPFS     `json:"ipfs"`
	Node      CommunityNode     `json:"node"`
	ERC4337   CommunityERC4337  `json:"erc4337"`
	Token     CommunityToken    `json:"token"`
	Profile   CommunityProfile  `json:"profile"`
	Plugins   []CommunityPlugin `json:"plugins,omitempty"`
	Version   int               `json:"version"`
}
