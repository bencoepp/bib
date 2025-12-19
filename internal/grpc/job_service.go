// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JobServiceServer implements the JobService gRPC service.
// Note: This is a placeholder until Phase 3 (Scheduler) is complete.
type JobServiceServer struct {
	services.UnimplementedJobServiceServer
}

// NewJobServiceServer creates a new JobServiceServer.
func NewJobServiceServer() *JobServiceServer {
	return &JobServiceServer{}
}

// CreateJob creates a new job.
func (s *JobServiceServer) CreateJob(ctx context.Context, req *services.CreateJobRequest) (*services.CreateJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateJob not implemented - awaiting Phase 3")
}

// GetJob retrieves job status.
func (s *JobServiceServer) GetJob(ctx context.Context, req *services.GetJobRequest) (*services.GetJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetJob not implemented - awaiting Phase 3")
}

// ListJobs lists jobs with filtering.
func (s *JobServiceServer) ListJobs(ctx context.Context, req *services.ListJobsRequest) (*services.ListJobsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListJobs not implemented - awaiting Phase 3")
}

// CancelJob cancels a running job.
func (s *JobServiceServer) CancelJob(ctx context.Context, req *services.CancelJobRequest) (*services.CancelJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CancelJob not implemented - awaiting Phase 3")
}

// RetryJob retries a failed job.
func (s *JobServiceServer) RetryJob(ctx context.Context, req *services.RetryJobRequest) (*services.RetryJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RetryJob not implemented - awaiting Phase 3")
}

// StreamJobLogs streams job output in real-time.
func (s *JobServiceServer) StreamJobLogs(req *services.StreamJobLogsRequest, stream services.JobService_StreamJobLogsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamJobLogs not implemented - awaiting Phase 3")
}

// StreamJobStatus streams job status updates.
func (s *JobServiceServer) StreamJobStatus(req *services.StreamJobStatusRequest, stream services.JobService_StreamJobStatusServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamJobStatus not implemented - awaiting Phase 3")
}

// PauseJob pauses a running job.
func (s *JobServiceServer) PauseJob(ctx context.Context, req *services.PauseJobRequest) (*services.PauseJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PauseJob not implemented - awaiting Phase 3")
}

// ResumeJob resumes a paused job.
func (s *JobServiceServer) ResumeJob(ctx context.Context, req *services.ResumeJobRequest) (*services.ResumeJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ResumeJob not implemented - awaiting Phase 3")
}

// GetJobResult retrieves job result/output.
func (s *JobServiceServer) GetJobResult(ctx context.Context, req *services.GetJobResultRequest) (*services.GetJobResultResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetJobResult not implemented - awaiting Phase 3")
}

// DeleteJob deletes a job and its artifacts.
func (s *JobServiceServer) DeleteJob(ctx context.Context, req *services.DeleteJobRequest) (*services.DeleteJobResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteJob not implemented - awaiting Phase 3")
}
