package bothomepage

// Data is the stable response payload for the bot homepage read model.
type Data struct {
	SchemaVersion string            `json:"schemaVersion"`
	ResolvedAt    int64             `json:"resolvedAt"`
	GlobalMetaId  string            `json:"globalMetaId"`
	Canonical     CanonicalIdentity `json:"canonical"`
	Profile       Profile           `json:"profile"`
	Homepage      Homepage          `json:"homepage"`
	Presence      Presence          `json:"presence"`
	Services      []Service         `json:"services"`
	Actions       []Action          `json:"actions"`
	Proofs        Proofs            `json:"proofs"`
	Source        Source            `json:"source"`
	Warnings      []string          `json:"warnings"`
}

type CanonicalIdentity struct {
	GlobalMetaId string `json:"globalMetaId"`
	MetaId       string `json:"metaid"`
	Address      string `json:"address"`
	ChainName    string `json:"chainName"`
}

type Profile struct {
	Name            string `json:"name"`
	NamePinId       string `json:"namePinId,omitempty"`
	Avatar          string `json:"avatar"`
	AvatarPinId     string `json:"avatarPinId,omitempty"`
	Background      string `json:"background"`
	BackgroundPinId string `json:"backgroundPinId,omitempty"`
	Bio             string `json:"bio"`
	BioPinId        string `json:"bioPinId,omitempty"`
	ChatPubkey      string `json:"chatPubkey"`
	ChatPubkeyPinId string `json:"chatPubkeyPinId,omitempty"`
	NftAvatar       string `json:"nftAvatar,omitempty"`
	DisplayGlobalId string `json:"displayGlobalMetaId"`
}

type Homepage struct {
	Mode    string          `json:"mode"`
	Title   string          `json:"title"`
	Summary string          `json:"summary"`
	Custom  *CustomHomepage `json:"custom"`
}

type CustomHomepage struct {
	Title       string   `json:"title,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	HeroImage   string   `json:"heroImage,omitempty"`
	Sections    []string `json:"sections,omitempty"`
	UpdatedAt   *int64   `json:"updatedAt,omitempty"`
	SourcePinId string   `json:"sourcePinId,omitempty"`
}

type Presence struct {
	State     string `json:"state"`
	UpdatedAt *int64 `json:"updatedAt"`
}

type Service struct {
	Id                   string  `json:"id"`
	CurrentPinId         string  `json:"currentPinId"`
	SourceServicePinId   string  `json:"sourceServicePinId"`
	ServiceName          string  `json:"serviceName"`
	DisplayName          string  `json:"displayName"`
	Description          string  `json:"description"`
	ServiceIcon          string  `json:"serviceIcon"`
	ProviderSkill        string  `json:"providerSkill"`
	OutputType           string  `json:"outputType"`
	Price                string  `json:"price"`
	Currency             string  `json:"currency"`
	SettlementKind       string  `json:"settlementKind"`
	PaymentChain         string  `json:"paymentChain"`
	MRC20Ticker          any     `json:"mrc20Ticker"`
	MRC20Id              any     `json:"mrc20Id"`
	PaymentAddress       string  `json:"paymentAddress"`
	ProviderMetaId       string  `json:"providerMetaId"`
	ProviderGlobalMetaId string  `json:"providerGlobalMetaId"`
	ProviderAddress      string  `json:"providerAddress"`
	ProviderName         string  `json:"providerName"`
	ProviderAvatar       string  `json:"providerAvatar"`
	ProviderAvatarId     string  `json:"providerAvatarId,omitempty"`
	ProviderChatPubkey   string  `json:"providerChatPubkey"`
	RatingAvg            float64 `json:"ratingAvg"`
	RatingCount          int64   `json:"ratingCount"`
	Status               int     `json:"status"`
	Operation            string  `json:"operation"`
	Disabled             bool    `json:"disabled"`
	ChainName            string  `json:"chainName"`
	CreatedAt            int64   `json:"createdAt"`
	UpdatedAt            int64   `json:"updatedAt"`
}

type Action struct {
	Kind    string `json:"kind"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
	URI     string `json:"uri,omitempty"`
}

type Proofs struct {
	VerificationState string         `json:"verificationState"`
	Summary           ProofSummary   `json:"summary"`
	Profile           []ProfileProof `json:"profile"`
	Services          []ServiceProof `json:"services"`
}

type ProofSummary struct {
	ProfileCount int    `json:"profileCount"`
	ServiceCount int    `json:"serviceCount"`
	ExplorerURL  string `json:"explorerUrl,omitempty"`
}

type ProfileProof struct {
	Field                 string `json:"field"`
	ProtocolPath          string `json:"protocolPath"`
	PinId                 string `json:"pinId"`
	TxId                  string `json:"txid,omitempty"`
	ContentHash           string `json:"contentHash,omitempty"`
	PublisherGlobalMetaId string `json:"publisherGlobalMetaId"`
	ExplorerURL           string `json:"explorerUrl,omitempty"`
}

type ServiceProof struct {
	ServiceId             string `json:"serviceId"`
	ProtocolPath          string `json:"protocolPath"`
	PinId                 string `json:"pinId"`
	TxId                  string `json:"txid,omitempty"`
	ContentHash           string `json:"contentHash,omitempty"`
	PublisherGlobalMetaId string `json:"publisherGlobalMetaId"`
	ExplorerURL           string `json:"explorerUrl,omitempty"`
}

type Source struct {
	Resolver        string `json:"resolver"`
	Node            string `json:"node"`
	ProfileEndpoint string `json:"profileEndpoint"`
	ServiceEndpoint string `json:"serviceEndpoint"`
	ContentBaseURL  string `json:"contentBaseUrl"`
	FetchedAt       int64  `json:"fetchedAt"`
	Stale           bool   `json:"stale"`
}
