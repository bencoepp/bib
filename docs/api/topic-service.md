# TopicService API

The TopicService provides operations for managing research topics - the primary organizational unit in BIB.

## Service Definition

```protobuf
service TopicService {
  // CRUD Operations
  rpc CreateTopic(CreateTopicRequest) returns (CreateTopicResponse);
  rpc GetTopic(GetTopicRequest) returns (GetTopicResponse);
  rpc ListTopics(ListTopicsRequest) returns (ListTopicsResponse);
  rpc UpdateTopic(UpdateTopicRequest) returns (UpdateTopicResponse);
  rpc DeleteTopic(DeleteTopicRequest) returns (DeleteTopicResponse);
  
  // Subscription
  rpc Subscribe(SubscribeRequest) returns (SubscribeResponse);
  rpc Unsubscribe(UnsubscribeRequest) returns (UnsubscribeResponse);
  rpc ListSubscriptions(ListSubscriptionsRequest) returns (ListSubscriptionsResponse);
  
  // Real-time
  rpc StreamTopicUpdates(StreamTopicUpdatesRequest) returns (stream TopicUpdate);
  
  // Search & Stats
  rpc SearchTopics(SearchTopicsRequest) returns (SearchTopicsResponse);
  rpc GetTopicStats(GetTopicStatsRequest) returns (GetTopicStatsResponse);
}
```

## Methods

### CreateTopic

Create a new topic. Requires admin role.

**Authentication:** Required (Admin)

**Request:**
```protobuf
message CreateTopicRequest {
  string name = 1;                // Unique topic name
  string description = 2;         // Topic description
  string schema = 3;              // Optional: table schema definition
  repeated string tags = 4;       // Topic tags
  map<string, string> metadata = 5;
}
```

**Response:**
```protobuf
message CreateTopicResponse {
  Topic topic = 1;
}
```

**Example:**
```go
resp, err := topicClient.CreateTopic(ctx, &services.CreateTopicRequest{
    Name:        "genomics-research",
    Description: "Human genomics research data",
    Tags:        []string{"genomics", "research", "human"},
    Metadata: map[string]string{
        "department": "biology",
        "project":    "HGP-2025",
    },
})
```

### GetTopic

Retrieve a topic by ID or name.

**Authentication:** Required

**Request:**
```protobuf
message GetTopicRequest {
  string id = 1;    // Topic ID (UUID)
  string name = 2;  // Or topic name
}
```

**Response:**
```protobuf
message GetTopicResponse {
  Topic topic = 1;
  bool subscribed = 2;              // Current user subscribed
  Subscription subscription = 3;    // Subscription details if subscribed
}
```

**Example:**
```go
// By ID
resp, err := topicClient.GetTopic(ctx, &services.GetTopicRequest{
    Id: "550e8400-e29b-41d4-a716-446655440000",
})

// By name
resp, err := topicClient.GetTopic(ctx, &services.GetTopicRequest{
    Name: "genomics-research",
})
```

### ListTopics

List topics with filtering and pagination.

**Authentication:** Required

**Request:**
```protobuf
message ListTopicsRequest {
  string status = 1;           // Filter by status
  string owner_id = 2;         // Filter by owner
  repeated string tags = 3;    // Filter by tags
  bool public_only = 4;        // Only public topics
  bool subscribed_only = 5;    // Only subscribed topics
  PageRequest page = 6;
  SortRequest sort = 7;
}
```

**Response:**
```protobuf
message ListTopicsResponse {
  repeated Topic topics = 1;
  PageInfo page_info = 2;
}
```

**Example:**
```go
resp, err := topicClient.ListTopics(ctx, &services.ListTopicsRequest{
    Tags:           []string{"genomics"},
    SubscribedOnly: true,
    Page:           &services.PageRequest{Limit: 20},
    Sort:           &services.SortRequest{Field: "updated_at", Descending: true},
})
```

### UpdateTopic

Update topic metadata. Requires owner role.

**Authentication:** Required (Owner)

**Request:**
```protobuf
message UpdateTopicRequest {
  string id = 1;
  string description = 2;
  repeated string tags = 3;
  map<string, string> metadata = 4;
}
```

### DeleteTopic

Delete (archive) a topic. Requires owner role.

**Authentication:** Required (Owner)

**Request:**
```protobuf
message DeleteTopicRequest {
  string id = 1;
  bool force = 2;  // Delete even if has datasets
}
```

### Subscribe

Subscribe to a topic to receive updates and sync data.

**Authentication:** Required

**Request:**
```protobuf
message SubscribeRequest {
  string topic_id = 1;
  SubscriptionOptions options = 2;
}

message SubscriptionOptions {
  bool sync_existing = 1;     // Sync existing datasets
  bool real_time = 2;         // Receive real-time updates
  string priority = 3;        // Sync priority: "high", "normal", "low"
}
```

**Response:**
```protobuf
message SubscribeResponse {
  Subscription subscription = 1;
}
```

### Unsubscribe

Unsubscribe from a topic.

**Authentication:** Required

**Request:**
```protobuf
message UnsubscribeRequest {
  string topic_id = 1;
  bool delete_local_data = 2;  // Remove local copies
}
```

### StreamTopicUpdates

Stream real-time topic updates.

**Authentication:** Required

**Request:**
```protobuf
message StreamTopicUpdatesRequest {
  repeated string topic_ids = 1;  // Empty = all subscribed
  repeated string event_types = 2; // Filter event types
}
```

**Response Stream:**
```protobuf
message TopicUpdate {
  string topic_id = 1;
  string event_type = 2;  // "dataset_added", "dataset_updated", etc.
  Timestamp timestamp = 3;
  oneof payload {
    Dataset dataset = 4;
    DatasetVersion version = 5;
  }
}
```

**Example:**
```go
stream, err := topicClient.StreamTopicUpdates(ctx, &services.StreamTopicUpdatesRequest{
    TopicIds:   []string{"topic-1", "topic-2"},
    EventTypes: []string{"dataset_added", "dataset_updated"},
})

for {
    update, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Update: %s on topic %s", update.EventType, update.TopicId)
}
```

## Topic Model

```protobuf
message Topic {
  string id = 1;
  string name = 2;
  string description = 3;
  string status = 4;          // "active", "archived", "deleted"
  string schema = 5;          // Table schema definition
  repeated string owners = 6; // Owner user IDs
  string created_by = 7;
  Timestamp created_at = 8;
  Timestamp updated_at = 9;
  repeated string tags = 10;
  map<string, string> metadata = 11;
  TopicStats stats = 12;
}

message TopicStats {
  int64 dataset_count = 1;
  int64 subscriber_count = 2;
  int64 total_size_bytes = 3;
  Timestamp last_activity = 4;
}
```

## Membership Roles

| Role | Permissions |
|------|-------------|
| `owner` | Full control, can delete topic |
| `editor` | Create/update datasets, invite members |
| `member` | Read access, subscribe |

## Error Codes

| Error | Code | Description |
|-------|------|-------------|
| Topic not found | `NOT_FOUND` | Topic doesn't exist |
| Already subscribed | `ALREADY_EXISTS` | Already subscribed |
| Topic archived | `FAILED_PRECONDITION` | Topic is archived |
| Permission denied | `PERMISSION_DENIED` | Insufficient role |

