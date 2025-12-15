package domain

import (
	"encoding/json"
	"time"
)

// InstructionOp represents a safe CEL operation type.
type InstructionOp string

const (
	// HTTP operations
	OpHTTPGet  InstructionOp = "http.get"
	OpHTTPPost InstructionOp = "http.post"

	// File operations
	OpFileUnzip   InstructionOp = "file.unzip"
	OpFileUntar   InstructionOp = "file.untar"
	OpFileRead    InstructionOp = "file.read"
	OpFileWrite   InstructionOp = "file.write"
	OpFileDelete  InstructionOp = "file.delete"
	OpFileList    InstructionOp = "file.list"
	OpFileMove    InstructionOp = "file.move"
	OpFileCopy    InstructionOp = "file.copy"
	OpFileHash    InstructionOp = "file.hash"
	OpFileSize    InstructionOp = "file.size"
	OpFileExists  InstructionOp = "file.exists"
	OpFileGunzip  InstructionOp = "file.gunzip"
	OpFileBunzip2 InstructionOp = "file.bunzip2"

	// Parse operations
	OpCSVParse  InstructionOp = "csv.parse"
	OpJSONParse InstructionOp = "json.parse"
	OpXMLParse  InstructionOp = "xml.parse"

	// Transform operations
	OpTransformMap     InstructionOp = "transform.map"
	OpTransformFilter  InstructionOp = "transform.filter"
	OpTransformReduce  InstructionOp = "transform.reduce"
	OpTransformFlatten InstructionOp = "transform.flatten"
	OpTransformSort    InstructionOp = "transform.sort"
	OpTransformUnique  InstructionOp = "transform.unique"
	OpTransformGroup   InstructionOp = "transform.group"
	OpTransformJoin    InstructionOp = "transform.join"

	// Validation operations
	OpValidateSchema   InstructionOp = "validate.schema"
	OpValidateNotNull  InstructionOp = "validate.not_null"
	OpValidateRange    InstructionOp = "validate.range"
	OpValidateRegex    InstructionOp = "validate.regex"
	OpValidateType     InstructionOp = "validate.type"
	OpValidateChecksum InstructionOp = "validate.checksum"

	// String operations
	OpStringSplit   InstructionOp = "string.split"
	OpStringJoin    InstructionOp = "string.join"
	OpStringReplace InstructionOp = "string.replace"
	OpStringTrim    InstructionOp = "string.trim"
	OpStringLower   InstructionOp = "string.lower"
	OpStringUpper   InstructionOp = "string.upper"

	// Control flow
	OpControlIf      InstructionOp = "control.if"
	OpControlForEach InstructionOp = "control.foreach"
	OpControlRetry   InstructionOp = "control.retry"
	OpControlSleep   InstructionOp = "control.sleep"

	// Variable operations
	OpVarSet    InstructionOp = "var.set"
	OpVarGet    InstructionOp = "var.get"
	OpVarDelete InstructionOp = "var.delete"

	// Output operations (signal to bibd to store data)
	OpOutputStore  InstructionOp = "output.store"
	OpOutputAppend InstructionOp = "output.append"
)

// IsValid checks if the operation is a known valid operation.
func (op InstructionOp) IsValid() bool {
	switch op {
	case OpHTTPGet, OpHTTPPost,
		OpFileUnzip, OpFileUntar, OpFileRead, OpFileWrite, OpFileDelete,
		OpFileList, OpFileMove, OpFileCopy, OpFileHash, OpFileSize, OpFileExists,
		OpFileGunzip, OpFileBunzip2,
		OpCSVParse, OpJSONParse, OpXMLParse,
		OpTransformMap, OpTransformFilter, OpTransformReduce, OpTransformFlatten,
		OpTransformSort, OpTransformUnique, OpTransformGroup, OpTransformJoin,
		OpValidateSchema, OpValidateNotNull, OpValidateRange, OpValidateRegex,
		OpValidateType, OpValidateChecksum,
		OpStringSplit, OpStringJoin, OpStringReplace, OpStringTrim, OpStringLower, OpStringUpper,
		OpControlIf, OpControlForEach, OpControlRetry, OpControlSleep,
		OpVarSet, OpVarGet, OpVarDelete,
		OpOutputStore, OpOutputAppend:
		return true
	default:
		return false
	}
}

// IsSafe checks if the operation is safe (no side effects outside sandbox).
func (op InstructionOp) IsSafe() bool {
	// All defined operations are designed to be safe
	// Unsafe operations like shell exec are not included
	return op.IsValid()
}

// Instruction represents a single CEL-based instruction.
type Instruction struct {
	// ID is an optional identifier for this instruction within a task.
	ID string `json:"id,omitempty"`

	// Operation is the CEL operation to execute.
	Operation InstructionOp `json:"operation"`

	// Params are the operation parameters.
	// Keys and expected types depend on the operation.
	Params map[string]any `json:"params,omitempty"`

	// OutputVar is the variable name to store the result.
	OutputVar string `json:"output_var,omitempty"`

	// Condition is an optional CEL expression that must be true to execute.
	Condition string `json:"condition,omitempty"`

	// OnError defines error handling: "fail", "skip", "retry".
	OnError string `json:"on_error,omitempty"`

	// RetryCount is the number of retries if OnError is "retry".
	RetryCount int `json:"retry_count,omitempty"`

	// Description is a human-readable description.
	Description string `json:"description,omitempty"`
}

// Validate validates the instruction.
func (i *Instruction) Validate() error {
	if !i.Operation.IsValid() {
		return ErrInvalidInstruction
	}
	return nil
}

// TaskID is a unique identifier for a task.
type TaskID string

// String returns the string representation.
func (id TaskID) String() string {
	return string(id)
}

// Task represents a reusable sequence of instructions.
// Tasks are templates that Jobs execute.
type Task struct {
	// ID is the unique identifier for the task.
	ID TaskID `json:"id"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Description provides details about what the task does.
	Description string `json:"description,omitempty"`

	// Version is the task version (semver).
	Version string `json:"version"`

	// Instructions is the ordered sequence of instructions.
	Instructions []Instruction `json:"instructions"`

	// InputSchema defines expected input variables (JSON Schema).
	InputSchema string `json:"input_schema,omitempty"`

	// OutputSchema defines expected output structure (JSON Schema).
	OutputSchema string `json:"output_schema,omitempty"`

	// CreatedBy is the user who created this task.
	CreatedBy UserID `json:"created_by"`

	// CreatedAt is when the task was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the task was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// Tags are optional labels for categorization.
	Tags []string `json:"tags,omitempty"`

	// Metadata holds additional task-specific data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the task.
func (t *Task) Validate() error {
	if t.ID == "" {
		return ErrInvalidTaskID
	}
	if t.Name == "" {
		return ErrInvalidTaskName
	}
	if len(t.Instructions) == 0 {
		return ErrEmptyTask
	}
	for _, inst := range t.Instructions {
		if err := inst.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ToJSON serializes the task to JSON.
func (t *Task) ToJSON() ([]byte, error) {
	return json.Marshal(t)
}

// TaskFromJSON deserializes a task from JSON.
func TaskFromJSON(data []byte) (*Task, error) {
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}
