package config

type P2PConfig struct {
	ListenAddresses    []string `json:"listenAddresses" yaml:"listenAddresses" mapstructure:"listenAddresses"`
	BootstrapPeers     []string `json:"bootstrapPeers" yaml:"bootstrapPeers" mapstructure:"bootstrapPeers"`
	Rendezvous         string   `json:"rendezvous" yaml:"rendezvous" mapstructure:"rendezvous"`
	EnableMDNS         bool     `json:"enableMDNS" yaml:"enableMDNS" mapstructure:"enableMDNS"`
	EnableDHT          bool     `json:"enableDHT" yaml:"enableDHT" mapstructure:"enableDHT"`
	EnableHolePunching bool     `json:"enableHolePunching" yaml:"enableHolePunching" mapstructure:"enableHolePunching"`
	EnableRelay        bool     `json:"enableRelay" yaml:"enableRelay" mapstructure:"enableRelay"`
	GRPCProtocolID     string   `json:"grpcProtocolID" yaml:"grpcProtocolID" mapstructure:"grpcProtocolID"`
}
