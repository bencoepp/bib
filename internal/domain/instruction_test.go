package domain

import (
	"testing"
)

func TestInstructionOp_IsValid(t *testing.T) {
	validOps := []InstructionOp{
		OpHTTPGet, OpHTTPPost,
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
		OpOutputStore, OpOutputAppend,
	}

	for _, op := range validOps {
		t.Run(string(op), func(t *testing.T) {
			if !op.IsValid() {
				t.Errorf("expected %q to be valid", op)
			}
		})
	}

	invalidOps := []InstructionOp{
		"invalid",
		"shell.exec",
		"",
	}

	for _, op := range invalidOps {
		t.Run(string(op), func(t *testing.T) {
			if op.IsValid() {
				t.Errorf("expected %q to be invalid", op)
			}
		})
	}
}

func TestInstructionOp_IsSafe(t *testing.T) {
	// All valid operations should be safe
	validOps := []InstructionOp{
		OpHTTPGet, OpHTTPPost,
		OpFileUnzip, OpFileRead,
		OpCSVParse, OpJSONParse,
		OpTransformMap, OpTransformFilter,
		OpValidateSchema,
		OpStringSplit, OpStringJoin,
		OpControlIf, OpControlForEach,
		OpVarSet, OpVarGet,
		OpOutputStore, OpOutputAppend,
	}

	for _, op := range validOps {
		t.Run(string(op), func(t *testing.T) {
			if !op.IsSafe() {
				t.Errorf("expected %q to be safe", op)
			}
		})
	}

	// Invalid operations should not be safe
	invalidOps := []InstructionOp{
		"shell.exec",
		"system.command",
	}

	for _, op := range invalidOps {
		t.Run(string(op), func(t *testing.T) {
			if op.IsSafe() {
				t.Errorf("expected %q to be unsafe", op)
			}
		})
	}
}

func TestInstruction_Validate(t *testing.T) {
	tests := []struct {
		name        string
		instruction *Instruction
		wantErr     error
	}{
		{
			name: "valid instruction",
			instruction: &Instruction{
				ID:        "step-1",
				Operation: OpHTTPGet,
				Params:    map[string]any{"url": "https://example.com"},
				OutputVar: "response",
			},
			wantErr: nil,
		},
		{
			name: "valid instruction with condition",
			instruction: &Instruction{
				Operation: OpFileWrite,
				Params:    map[string]any{"path": "/tmp/out.txt"},
				Condition: "response.status == 200",
			},
			wantErr: nil,
		},
		{
			name: "invalid operation",
			instruction: &Instruction{
				Operation: InstructionOp("invalid"),
			},
			wantErr: ErrInvalidInstruction,
		},
		{
			name: "empty operation",
			instruction: &Instruction{
				Operation: "",
			},
			wantErr: ErrInvalidInstruction,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.instruction.Validate()
			if err != tt.wantErr {
				t.Errorf("Instruction.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTaskID_String(t *testing.T) {
	id := TaskID("task-123")
	if id.String() != "task-123" {
		t.Errorf("expected 'task-123', got %q", id.String())
	}
}

func TestInstruction_Fields(t *testing.T) {
	inst := Instruction{
		ID:          "step-1",
		Operation:   OpHTTPGet,
		Params:      map[string]any{"url": "https://example.com", "headers": map[string]string{"X-Api-Key": "secret"}},
		OutputVar:   "result",
		Condition:   "enabled == true",
		OnError:     "retry",
		RetryCount:  3,
		Description: "Fetch data from API",
	}

	if inst.ID != "step-1" {
		t.Errorf("expected 'step-1', got %q", inst.ID)
	}
	if inst.Operation != OpHTTPGet {
		t.Errorf("expected OpHTTPGet, got %q", inst.Operation)
	}
	if inst.OutputVar != "result" {
		t.Errorf("expected 'result', got %q", inst.OutputVar)
	}
	if inst.Condition != "enabled == true" {
		t.Errorf("expected 'enabled == true', got %q", inst.Condition)
	}
	if inst.OnError != "retry" {
		t.Errorf("expected 'retry', got %q", inst.OnError)
	}
	if inst.RetryCount != 3 {
		t.Errorf("expected 3, got %d", inst.RetryCount)
	}
	if inst.Description != "Fetch data from API" {
		t.Errorf("expected 'Fetch data from API', got %q", inst.Description)
	}
	if inst.Params["url"] != "https://example.com" {
		t.Errorf("expected 'https://example.com', got %v", inst.Params["url"])
	}
}

func TestHTTPOperations(t *testing.T) {
	if OpHTTPGet != "http.get" {
		t.Errorf("expected 'http.get', got %q", OpHTTPGet)
	}
	if OpHTTPPost != "http.post" {
		t.Errorf("expected 'http.post', got %q", OpHTTPPost)
	}
}

func TestFileOperations(t *testing.T) {
	ops := map[InstructionOp]string{
		OpFileUnzip:   "file.unzip",
		OpFileUntar:   "file.untar",
		OpFileRead:    "file.read",
		OpFileWrite:   "file.write",
		OpFileDelete:  "file.delete",
		OpFileList:    "file.list",
		OpFileMove:    "file.move",
		OpFileCopy:    "file.copy",
		OpFileHash:    "file.hash",
		OpFileSize:    "file.size",
		OpFileExists:  "file.exists",
		OpFileGunzip:  "file.gunzip",
		OpFileBunzip2: "file.bunzip2",
	}

	for op, expected := range ops {
		if string(op) != expected {
			t.Errorf("expected %q, got %q", expected, op)
		}
	}
}

func TestParseOperations(t *testing.T) {
	if OpCSVParse != "csv.parse" {
		t.Errorf("expected 'csv.parse', got %q", OpCSVParse)
	}
	if OpJSONParse != "json.parse" {
		t.Errorf("expected 'json.parse', got %q", OpJSONParse)
	}
	if OpXMLParse != "xml.parse" {
		t.Errorf("expected 'xml.parse', got %q", OpXMLParse)
	}
}

func TestTransformOperations(t *testing.T) {
	ops := map[InstructionOp]string{
		OpTransformMap:     "transform.map",
		OpTransformFilter:  "transform.filter",
		OpTransformReduce:  "transform.reduce",
		OpTransformFlatten: "transform.flatten",
		OpTransformSort:    "transform.sort",
		OpTransformUnique:  "transform.unique",
		OpTransformGroup:   "transform.group",
		OpTransformJoin:    "transform.join",
	}

	for op, expected := range ops {
		if string(op) != expected {
			t.Errorf("expected %q, got %q", expected, op)
		}
	}
}

func TestValidateOperations(t *testing.T) {
	ops := map[InstructionOp]string{
		OpValidateSchema:   "validate.schema",
		OpValidateNotNull:  "validate.not_null",
		OpValidateRange:    "validate.range",
		OpValidateRegex:    "validate.regex",
		OpValidateType:     "validate.type",
		OpValidateChecksum: "validate.checksum",
	}

	for op, expected := range ops {
		if string(op) != expected {
			t.Errorf("expected %q, got %q", expected, op)
		}
	}
}

func TestStringOperations(t *testing.T) {
	ops := map[InstructionOp]string{
		OpStringSplit:   "string.split",
		OpStringJoin:    "string.join",
		OpStringReplace: "string.replace",
		OpStringTrim:    "string.trim",
		OpStringLower:   "string.lower",
		OpStringUpper:   "string.upper",
	}

	for op, expected := range ops {
		if string(op) != expected {
			t.Errorf("expected %q, got %q", expected, op)
		}
	}
}

func TestControlOperations(t *testing.T) {
	ops := map[InstructionOp]string{
		OpControlIf:      "control.if",
		OpControlForEach: "control.foreach",
		OpControlRetry:   "control.retry",
		OpControlSleep:   "control.sleep",
	}

	for op, expected := range ops {
		if string(op) != expected {
			t.Errorf("expected %q, got %q", expected, op)
		}
	}
}

func TestVarOperations(t *testing.T) {
	if OpVarSet != "var.set" {
		t.Errorf("expected 'var.set', got %q", OpVarSet)
	}
	if OpVarGet != "var.get" {
		t.Errorf("expected 'var.get', got %q", OpVarGet)
	}
	if OpVarDelete != "var.delete" {
		t.Errorf("expected 'var.delete', got %q", OpVarDelete)
	}
}

func TestOutputOperations(t *testing.T) {
	if OpOutputStore != "output.store" {
		t.Errorf("expected 'output.store', got %q", OpOutputStore)
	}
	if OpOutputAppend != "output.append" {
		t.Errorf("expected 'output.append', got %q", OpOutputAppend)
	}
}
