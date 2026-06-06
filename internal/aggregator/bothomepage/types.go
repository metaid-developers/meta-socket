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
	URI          string `json:"uri"`
	PinId        string `json:"pinId"`
	ContentType  string `json:"contentType"`
	Renderer     string `json:"renderer"`
	Txid         string `json:"txid,omitempty"`
	ProtocolPath string `json:"protocolPath"`
}

type Presence struct {
	State     string `json:"state"`
	UpdatedAt *int64 `json:"updatedAt"`
	Source    string `json:"source"`
}

type Service struct {
	Id                 string        `json:"id"`
	CurrentPinId       string        `json:"currentPinId"`
	SourceServicePinId string        `json:"sourceServicePinId"`
	DisplayName        string        `json:"displayName"`
	ServiceName        string        `json:"serviceName"`
	Description        string        `json:"description"`
	ServiceIcon        string        `json:"serviceIcon"`
	ProviderSkill      string        `json:"providerSkill"`
	OutputType         string        `json:"outputType"`
	Price              string        `json:"price"`
	Currency           string        `json:"currency"`
	SettlementKind     string        `json:"settlementKind"`
	PaymentChain       string        `json:"paymentChain"`
	MRC20Ticker        any           `json:"mrc20Ticker"`
	MRC20Id            any           `json:"mrc20Id"`
	PaymentAddress     string        `json:"paymentAddress"`
	RatingAvg          float64       `json:"ratingAvg"`
	RatingCount        int64         `json:"ratingCount"`
	Status             int           `json:"status"`
	Operation          string        `json:"operation"`
	Disabled           bool          `json:"disabled"`
	ChainName          string        `json:"chainName"`
	CreatedAt          int64         `json:"createdAt"`
	UpdatedAt          int64         `json:"updatedAt"`
	Proof              *ServiceProof `json:"proof,omitempty"`
}

type Action struct {
	Id                    string `json:"id"`
	Label                 string `json:"label"`
	Kind                  string `json:"kind"`
	Enabled               bool   `json:"enabled"`
	RequiresUsingIdentity bool   `json:"requiresUsingIdentity"`
	URI                   string `json:"uri,omitempty"`
}

type Proofs struct {
	VerificationState string         `json:"verificationState"`
	Identity          *ProofSummary  `json:"identity"`
	Profile           []ProfileProof `json:"profile"`
	Homepage          *ProofSummary  `json:"homepage"`
	Services          []ServiceProof `json:"services"`
}

type ProofSummary struct {
	Txid                  string `json:"txid,omitempty"`
	PinId                 string `json:"pinId,omitempty"`
	ProtocolPath          string `json:"protocolPath"`
	PublisherGlobalMetaId string `json:"publisherGlobalMetaId,omitempty"`
	ContentHash           string `json:"contentHash,omitempty"`
	ExplorerURL           string `json:"explorerUrl,omitempty"`
}

type ProfileProof struct {
	Field                 string `json:"field"`
	Txid                  string `json:"txid,omitempty"`
	PinId                 string `json:"pinId,omitempty"`
	ProtocolPath          string `json:"protocolPath"`
	ContentHash           string `json:"contentHash,omitempty"`
	PublisherGlobalMetaId string `json:"publisherGlobalMetaId,omitempty"`
}

type ServiceProof struct {
	ServiceId             string `json:"serviceId,omitempty"`
	Txid                  string `json:"txid,omitempty"`
	PinId                 string `json:"pinId"`
	SourceServicePinId    string `json:"sourceServicePinId,omitempty"`
	ProtocolPath          string `json:"protocolPath"`
	PublisherGlobalMetaId string `json:"publisherGlobalMetaId"`
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
