package config

type P2PConfig struct {
	Discovery   DiscoveryConfig `yaml:"discovery"`
	RTT         RTTConfig       `yaml:"rtt"`
	Preferences Preferences     `yaml:"preferences"`
}

type DiscoveryConfig struct {
	Rendezvous            string   `yaml:"rendezvous"`
	EnableMDNS            bool     `yaml:"enable_mdns"`
	MDNSServiceTag        string   `yaml:"mdns_service_tag"`
	DHTServer             bool     `yaml:"dht_server"`
	BootstrapPeers        []string `yaml:"bootstrap_peers"`
	AdvertiseInterval     int      `yaml:"advertise_interval"`
	SkipMDNSIfNoMulticast bool     `yaml:"skip_mdns_if_no_multicast"`
	RequireMDNS           bool     `yaml:"require_mdns"`
}

type RTTConfig struct {
	EnableRTTProbing bool `yaml:"enable_rtt_probing"`
	Interval         int  `yaml:"interval"`
	Concurrency      int  `yaml:"concurrency"`
	PingsPerPeer     int  `yaml:"pings_per_peer"`
	ConnectTimeout   int  `yaml:"connect_timeout"`
	PingTimeout      int  `yaml:"ping_timeout"`
}

type Preferences struct {
	Region           string   `yaml:"region"`
	Tags             []string `yaml:"tags"`
	WeightLatency    float64  `yaml:"weight_latency"`
	WeightLoad       float64  `yaml:"weight_load"`
	WeightRegion     float64  `yaml:"weight_region"`
	WeightTags       float64  `yaml:"weight_tags"`
	MinSamplesForRTT int      `yaml:"min_samples_for_rtt"`
}
