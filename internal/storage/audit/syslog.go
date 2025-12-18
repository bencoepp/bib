package audit

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

// SyslogExporter exports audit entries to a syslog server.
type SyslogExporter struct {
	config   SyslogConfig
	conn     net.Conn
	mu       sync.Mutex
	closed   bool
	facility SyslogFacility
	severity SyslogSeverity
}

// SyslogConfig holds syslog export configuration.
type SyslogConfig struct {
	// Enabled controls whether syslog export is active.
	Enabled bool `mapstructure:"enabled"`

	// Network is the network type: "tcp", "udp", or "unix".
	Network string `mapstructure:"network"`

	// Address is the syslog server address (e.g., "localhost:514").
	Address string `mapstructure:"address"`

	// TLS enables TLS for TCP connections.
	TLS bool `mapstructure:"tls"`

	// TLSConfig holds TLS configuration (nil uses defaults).
	TLSConfig *tls.Config `mapstructure:"-"`

	// Facility is the syslog facility (default: LOG_LOCAL0).
	Facility SyslogFacility `mapstructure:"facility"`

	// Tag is the syslog tag/program name.
	Tag string `mapstructure:"tag"`

	// Hostname is the hostname to report (empty = auto-detect).
	Hostname string `mapstructure:"hostname"`

	// ReconnectInterval is how often to retry on connection failure.
	ReconnectInterval time.Duration `mapstructure:"reconnect_interval"`

	// MaxRetries is the maximum number of send retries.
	MaxRetries int `mapstructure:"max_retries"`
}

// SyslogFacility represents syslog facility values.
type SyslogFacility int

const (
	FacilityKern     SyslogFacility = 0
	FacilityUser     SyslogFacility = 1
	FacilityMail     SyslogFacility = 2
	FacilityDaemon   SyslogFacility = 3
	FacilityAuth     SyslogFacility = 4
	FacilitySyslog   SyslogFacility = 5
	FacilityLPR      SyslogFacility = 6
	FacilityNews     SyslogFacility = 7
	FacilityUUCP     SyslogFacility = 8
	FacilityCron     SyslogFacility = 9
	FacilityAuthPriv SyslogFacility = 10
	FacilityFTP      SyslogFacility = 11
	FacilityLocal0   SyslogFacility = 16
	FacilityLocal1   SyslogFacility = 17
	FacilityLocal2   SyslogFacility = 18
	FacilityLocal3   SyslogFacility = 19
	FacilityLocal4   SyslogFacility = 20
	FacilityLocal5   SyslogFacility = 21
	FacilityLocal6   SyslogFacility = 22
	FacilityLocal7   SyslogFacility = 23
)

// SyslogSeverity represents syslog severity values.
type SyslogSeverity int

const (
	SeverityEmergency SyslogSeverity = 0
	SeverityAlert     SyslogSeverity = 1
	SeverityCritical  SyslogSeverity = 2
	SeverityError     SyslogSeverity = 3
	SeverityWarning   SyslogSeverity = 4
	SeverityNotice    SyslogSeverity = 5
	SeverityInfo      SyslogSeverity = 6
	SeverityDebug     SyslogSeverity = 7
)

// DefaultSyslogConfig returns the default syslog configuration.
func DefaultSyslogConfig() SyslogConfig {
	return SyslogConfig{
		Enabled:           false,
		Network:           "udp",
		Address:           "localhost:514",
		TLS:               false,
		Facility:          FacilityLocal0,
		Tag:               "bibd",
		ReconnectInterval: 30 * time.Second,
		MaxRetries:        3,
	}
}

// NewSyslogExporter creates a new syslog exporter.
func NewSyslogExporter(cfg SyslogConfig) (*SyslogExporter, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	exporter := &SyslogExporter{
		config:   cfg,
		facility: cfg.Facility,
		severity: SeverityInfo,
	}

	if err := exporter.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to syslog: %w", err)
	}

	return exporter, nil
}

// connect establishes a connection to the syslog server.
func (e *SyslogExporter) connect() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.conn != nil {
		e.conn.Close()
	}

	var conn net.Conn
	var err error

	switch e.config.Network {
	case "tcp":
		if e.config.TLS {
			tlsCfg := e.config.TLSConfig
			if tlsCfg == nil {
				tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
			}
			conn, err = tls.Dial("tcp", e.config.Address, tlsCfg)
		} else {
			conn, err = net.Dial("tcp", e.config.Address)
		}
	case "udp":
		conn, err = net.Dial("udp", e.config.Address)
	case "unix":
		conn, err = net.Dial("unix", e.config.Address)
	default:
		return fmt.Errorf("unsupported network type: %s", e.config.Network)
	}

	if err != nil {
		return err
	}

	e.conn = conn
	return nil
}

// Export sends an audit entry to syslog.
func (e *SyslogExporter) Export(ctx context.Context, entry *Entry) error {
	if e == nil || e.closed {
		return nil
	}

	message := e.formatMessage(entry)
	return e.send(message)
}

// ExportBatch sends multiple entries to syslog.
func (e *SyslogExporter) ExportBatch(ctx context.Context, entries []*Entry) error {
	if e == nil || e.closed {
		return nil
	}

	for _, entry := range entries {
		if err := e.Export(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

// formatMessage formats an audit entry as an RFC 5424 syslog message.
func (e *SyslogExporter) formatMessage(entry *Entry) string {
	// RFC 5424 format:
	// <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID [SD-ID SD-PARAM...] MSG

	pri := int(e.facility)*8 + int(e.determineSeverity(entry))
	version := 1
	timestamp := entry.Timestamp.Format(time.RFC3339Nano)
	hostname := e.config.Hostname
	if hostname == "" {
		hostname = "-"
	}
	appName := e.config.Tag
	procID := "-"
	msgID := entry.OperationID

	// Structured data
	sd := e.formatStructuredData(entry)

	// Message content
	msg := fmt.Sprintf("action=%s table=%s role=%s rows=%d duration=%dms",
		entry.Action,
		entry.TableName,
		entry.RoleUsed,
		entry.RowsAffected,
		entry.DurationMS,
	)

	return fmt.Sprintf("<%d>%d %s %s %s %s %s %s %s\n",
		pri, version, timestamp, hostname, appName, procID, msgID, sd, msg)
}

// formatStructuredData formats the structured data portion.
func (e *SyslogExporter) formatStructuredData(entry *Entry) string {
	// [bibd@0 node_id="..." job_id="..." action="..." flags="..."]
	sd := fmt.Sprintf(`[bibd@0 node_id="%s" op_id="%s" action="%s" role="%s"`,
		escapeSDValue(entry.NodeID),
		escapeSDValue(entry.OperationID),
		escapeSDValue(string(entry.Action)),
		escapeSDValue(entry.RoleUsed),
	)

	if entry.JobID != "" {
		sd += fmt.Sprintf(` job_id="%s"`, escapeSDValue(entry.JobID))
	}

	if entry.TableName != "" {
		sd += fmt.Sprintf(` table="%s"`, escapeSDValue(entry.TableName))
	}

	if entry.Actor != "" {
		sd += fmt.Sprintf(` actor="%s"`, escapeSDValue(entry.Actor))
	}

	// Add flags
	if entry.Flags.Suspicious {
		sd += ` suspicious="true"`
	}
	if entry.Flags.RateLimited {
		sd += ` rate_limited="true"`
	}
	if entry.Flags.BreakGlass {
		sd += ` break_glass="true"`
	}

	sd += "]"
	return sd
}

// escapeSDValue escapes special characters in structured data values.
func escapeSDValue(s string) string {
	// Escape \, ", and ]
	result := s
	result = replaceAll(result, `\`, `\\`)
	result = replaceAll(result, `"`, `\"`)
	result = replaceAll(result, `]`, `\]`)
	return result
}

// replaceAll is a simple string replace helper.
func replaceAll(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i <= len(s)-len(old) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}

// determineSeverity determines the syslog severity based on entry.
func (e *SyslogExporter) determineSeverity(entry *Entry) SyslogSeverity {
	if entry.Flags.Suspicious || entry.Flags.AlertTriggered {
		return SeverityWarning
	}
	if entry.Flags.BreakGlass {
		return SeverityNotice
	}
	if entry.Action == ActionDDL {
		return SeverityNotice
	}
	return SeverityInfo
}

// send writes a message to the syslog connection with retry logic.
func (e *SyslogExporter) send(message string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("syslog exporter is closed")
	}

	var lastErr error
	for i := 0; i <= e.config.MaxRetries; i++ {
		if e.conn == nil {
			if err := e.connectLocked(); err != nil {
				lastErr = err
				continue
			}
		}

		_, err := e.conn.Write([]byte(message))
		if err == nil {
			return nil
		}

		lastErr = err
		e.conn.Close()
		e.conn = nil
	}

	return fmt.Errorf("failed to send syslog message after %d retries: %w", e.config.MaxRetries, lastErr)
}

// connectLocked establishes connection (caller must hold lock).
func (e *SyslogExporter) connectLocked() error {
	var conn net.Conn
	var err error

	switch e.config.Network {
	case "tcp":
		if e.config.TLS {
			tlsCfg := e.config.TLSConfig
			if tlsCfg == nil {
				tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
			}
			conn, err = tls.Dial("tcp", e.config.Address, tlsCfg)
		} else {
			conn, err = net.Dial("tcp", e.config.Address)
		}
	case "udp":
		conn, err = net.Dial("udp", e.config.Address)
	case "unix":
		conn, err = net.Dial("unix", e.config.Address)
	default:
		return fmt.Errorf("unsupported network type: %s", e.config.Network)
	}

	if err != nil {
		return err
	}

	e.conn = conn
	return nil
}

// Close closes the syslog exporter.
func (e *SyslogExporter) Close() error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.closed = true
	if e.conn != nil {
		return e.conn.Close()
	}
	return nil
}
