package config

type P2PConfig struct {
	Discovery       DiscoveryConfig `mapstructure:"discovery" yaml:"discovery"`
	RTT             RTTConfig       `mapstructure:"rtt" yaml:"rtt"`
	Preferences     Preferences     `mapstructure:"preferences" yaml:"preferences"`
	ListenAddresses []string        `mapstructure:"listen_addresses" yaml:"listen_addresses"`
}

type DiscoveryConfig struct {
	Rendezvous            string   `mapstructure:"rendezvous" yaml:"rendezvous"`
	EnableMDNS            bool     `mapstructure:"enable_mdns" yaml:"enable_mdns"`
	MDNSServiceTag        string   `mapstructure:"mdns_service_tag" yaml:"mdns_service_tag"`
	DHTServer             bool     `mapstructure:"dht_server" yaml:"dht_server"`
	BootstrapPeers        []string `mapstructure:"bootstrap_peers" yaml:"bootstrap_peers"`
	AdvertiseInterval     int      `mapstructure:"advertise_interval" yaml:"advertise_interval"`
	SkipMDNSIfNoMulticast bool     `mapstructure:"skip_mdns_if_no_multicast" yaml:"skip_mdns_if_no_multicast"`
	RequireMDNS           bool     `mapstructure:"require_mdns" yaml:"require_mdns"`
}

type RTTConfig struct {
	EnableRTTProbing bool `mapstructure:"enable_rtt_probing" yaml:"enable_rtt_probing"`
	Interval         int  `mapstructure:"interval" yaml:"interval"`
	Concurrency      int  `mapstructure:"concurrency" yaml:"concurrency"`
	PingsPerPeer     int  `mapstructure:"pings_per_peer" yaml:"pings_per_peer"`
	ConnectTimeout   int  `mapstructure:"connect_timeout" yaml:"connect_timeout"`
	PingTimeout      int  `mapstructure:"ping_timeout" yaml:"ping_timeout"`
}

type Preferences struct {
	Region           string   `mapstructure:"region" yaml:"region"`
	Tags             []string `mapstructure:"tags" yaml:"tags"`
	WeightLatency    float64  `mapstructure:"weight_latency" yaml:"weight_latency"`
	WeightLoad       float64  `mapstructure:"weight_load" yaml:"weight_load"`
	WeightRegion     float64  `mapstructure:"weight_region" yaml:"weight_region"`
	WeightTags       float64  `mapstructure:"weight_tags" yaml:"weight_tags"`
	MinSamplesForRTT int      `mapstructure:"min_samples_for_rtt" yaml:"min_samples_for_rtt"`
}
