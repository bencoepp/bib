// Package p2p provides P2P networking functionality using libp2p.
package p2p

import (
	"time"

	"bib/internal/domain"

	bibv1 "bib/api/gen/go/bib/v1"
	p2ppb "bib/api/gen/go/bib/v1/p2p"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// Type aliases for cleaner code
type (
	Catalog        = domain.Catalog
	CatalogEntry   = domain.CatalogEntry
	Topic          = domain.Topic
	TopicID        = domain.TopicID
	Dataset        = domain.Dataset
	DatasetID      = domain.DatasetID
	DatasetContent = domain.DatasetContent
	Chunk          = domain.Chunk
)

// =============================================================================
// Domain to Proto Conversions
// =============================================================================

// CatalogToProto converts a domain Catalog to proto Catalog.
func CatalogToProto(c *Catalog) *bibv1.Catalog {
	if c == nil {
		return nil
	}

	entries := make([]*bibv1.CatalogEntry, len(c.Entries))
	for i, e := range c.Entries {
		entries[i] = CatalogEntryToProto(&e)
	}

	return &bibv1.Catalog{
		PeerId:      c.PeerID,
		Entries:     entries,
		LastUpdated: timestamppb.New(c.LastUpdated),
		Version:     c.Version,
	}
}

// CatalogEntryToProto converts a domain CatalogEntry to proto CatalogEntry.
func CatalogEntryToProto(e *CatalogEntry) *bibv1.CatalogEntry {
	if e == nil {
		return nil
	}

	return &bibv1.CatalogEntry{
		TopicId:     string(e.TopicID),
		TopicName:   e.TopicName,
		DatasetId:   string(e.DatasetID),
		DatasetName: e.DatasetName,
		Hash:        e.Hash,
		Size:        e.Size,
		ChunkCount:  int32(e.ChunkCount),
		UpdatedAt:   timestamppb.New(e.UpdatedAt),
	}
}

// PeerInfoToProto converts domain peer info to proto PeerInfo.
func PeerInfoToProto(peerID string, addrs []string, mode NodeMode, version string, lastSeen time.Time) *bibv1.PeerInfo {
	return &bibv1.PeerInfo{
		PeerId:    peerID,
		Addresses: addrs,
		NodeMode:  string(mode),
		Version:   version,
		LastSeen:  timestamppb.New(lastSeen),
	}
}

// DatasetInfoToProto converts a domain Dataset to proto DatasetInfo.
// Note: This only converts basic dataset metadata. For size/hash/chunk info,
// you need to use DatasetVersionToProto with the version's Content.
func DatasetInfoToProto(d *Dataset) *bibv1.DatasetInfo {
	if d == nil {
		return nil
	}

	return &bibv1.DatasetInfo{
		Id:        string(d.ID),
		TopicId:   string(d.TopicID),
		Name:      d.Name,
		CreatedAt: timestamppb.New(d.CreatedAt),
		UpdatedAt: timestamppb.New(d.UpdatedAt),
		Metadata:  d.Metadata,
	}
}

// DatasetWithContentToProto converts a domain Dataset with its version content to proto DatasetInfo.
func DatasetWithContentToProto(d *Dataset, content *DatasetContent) *bibv1.DatasetInfo {
	if d == nil {
		return nil
	}

	info := &bibv1.DatasetInfo{
		Id:        string(d.ID),
		TopicId:   string(d.TopicID),
		Name:      d.Name,
		CreatedAt: timestamppb.New(d.CreatedAt),
		UpdatedAt: timestamppb.New(d.UpdatedAt),
		Metadata:  d.Metadata,
	}

	if content != nil {
		info.Size = content.Size
		info.Hash = content.Hash
		info.ChunkCount = int32(content.ChunkCount)
		info.ChunkSize = content.ChunkSize
	}

	return info
}

// TopicInfoToProto converts a domain Topic to proto TopicInfo.
func TopicInfoToProto(t *Topic) *bibv1.TopicInfo {
	if t == nil {
		return nil
	}

	return &bibv1.TopicInfo{
		Id:           string(t.ID),
		Name:         t.Name,
		Description:  t.Description,
		Schema:       t.TableSchema,
		DatasetCount: int32(t.DatasetCount),
		CreatedAt:    timestamppb.New(t.CreatedAt),
		UpdatedAt:    timestamppb.New(t.UpdatedAt),
	}
}

// ChunkDataToProto converts a domain Chunk to proto ChunkData.
func ChunkDataToProto(c *Chunk) *p2ppb.ChunkData {
	if c == nil {
		return nil
	}

	return &p2ppb.ChunkData{
		DatasetId: string(c.DatasetID),
		Index:     int32(c.Index),
		Hash:      c.Hash,
		Size:      c.Size,
		Data:      c.Data,
	}
}

// ErrorToProto converts an error to proto Error.
func ErrorToProto(code int, message string, details map[string]string) *bibv1.Error {
	return &bibv1.Error{
		Code:    int32(code),
		Message: message,
		Details: details,
	}
}

// =============================================================================
// Proto to Domain Conversions
// =============================================================================

// ProtoToCatalog converts a proto Catalog to domain Catalog.
func ProtoToCatalog(c *bibv1.Catalog) *Catalog {
	if c == nil {
		return nil
	}

	entries := make([]CatalogEntry, len(c.Entries))
	for i, e := range c.Entries {
		entries[i] = *ProtoToCatalogEntry(e)
	}

	lastUpdated := time.Time{}
	if c.LastUpdated != nil {
		lastUpdated = c.LastUpdated.AsTime()
	}

	return &Catalog{
		PeerID:      c.PeerId,
		Entries:     entries,
		LastUpdated: lastUpdated,
		Version:     c.Version,
	}
}

// ProtoToCatalogEntry converts a proto CatalogEntry to domain CatalogEntry.
func ProtoToCatalogEntry(e *bibv1.CatalogEntry) *CatalogEntry {
	if e == nil {
		return nil
	}

	updatedAt := time.Time{}
	if e.UpdatedAt != nil {
		updatedAt = e.UpdatedAt.AsTime()
	}

	return &CatalogEntry{
		TopicID:     TopicID(e.TopicId),
		TopicName:   e.TopicName,
		DatasetID:   DatasetID(e.DatasetId),
		DatasetName: e.DatasetName,
		Hash:        e.Hash,
		Size:        e.Size,
		ChunkCount:  int(e.ChunkCount),
		UpdatedAt:   updatedAt,
	}
}

// ProtoToPeerInfo converts a proto PeerInfo to a map of values.
func ProtoToPeerInfo(p *bibv1.PeerInfo) (peerID string, addrs []string, mode NodeMode, version string, lastSeen time.Time) {
	if p == nil {
		return
	}

	peerID = p.PeerId
	addrs = p.Addresses
	mode = NodeMode(p.NodeMode)
	version = p.Version
	if p.LastSeen != nil {
		lastSeen = p.LastSeen.AsTime()
	}
	return
}

// ProtoToDatasetInfo converts a proto DatasetInfo to domain Dataset.
// Note: Size/Hash/ChunkCount/ChunkSize are not stored on Dataset directly;
// they are on DatasetContent within a DatasetVersion.
func ProtoToDatasetInfo(d *bibv1.DatasetInfo) *Dataset {
	if d == nil {
		return nil
	}

	createdAt := time.Time{}
	if d.CreatedAt != nil {
		createdAt = d.CreatedAt.AsTime()
	}

	updatedAt := time.Time{}
	if d.UpdatedAt != nil {
		updatedAt = d.UpdatedAt.AsTime()
	}

	return &Dataset{
		ID:        DatasetID(d.Id),
		TopicID:   TopicID(d.TopicId),
		Name:      d.Name,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Metadata:  d.Metadata,
	}
}

// ProtoToDatasetContent extracts DatasetContent from a proto DatasetInfo.
func ProtoToDatasetContent(d *bibv1.DatasetInfo) *DatasetContent {
	if d == nil {
		return nil
	}

	return &DatasetContent{
		Hash:       d.Hash,
		Size:       d.Size,
		ChunkCount: int(d.ChunkCount),
		ChunkSize:  d.ChunkSize,
	}
}

// ProtoToChunkData converts a proto ChunkData to domain Chunk.
func ProtoToChunkData(c *p2ppb.ChunkData) *Chunk {
	if c == nil {
		return nil
	}

	return &Chunk{
		DatasetID: DatasetID(c.DatasetId),
		Index:     int(c.Index),
		Hash:      c.Hash,
		Size:      c.Size,
		Data:      c.Data,
	}
}

// ProtoToError converts a proto Error to code/message.
func ProtoToError(e *bibv1.Error) (code int, message string, details map[string]string) {
	if e == nil {
		return
	}
	return int(e.Code), e.Message, e.Details
}
