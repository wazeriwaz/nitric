package job

import (
	"net"
	"os"
	"os/exec"
	"strings"

	queuespb "github.com/nitrictech/nitric/core/pkg/proto/queues/v1"
	secretspb "github.com/nitrictech/nitric/core/pkg/proto/secrets/v1"
	sqlpb "github.com/nitrictech/nitric/core/pkg/proto/sql/v1"
	storagepb "github.com/nitrictech/nitric/core/pkg/proto/storage/v1"
	topicspb "github.com/nitrictech/nitric/core/pkg/proto/topics/v1"
	"google.golang.org/grpc"
)

// JobMembrane is a membrane for job
// Created as a sepearate membrane type to avoid overloading the service membrane
type JobMembrane struct {
	// The Command that will be executed to run the job
	cmd string

	// Runtime plugins (for reading/writing to cloud services)
	topicServer   topicspb.TopicsServer
	storageServer storagepb.StorageServer
	queueServer   queuespb.QueuesServer
	secretServer  secretspb.SecretManagerServer
	sqlServer     sqlpb.SqlServer
}

func (j *JobMembrane) Run() error {
	// Start the gRPC server for runtime services
	grpcServer := grpc.NewServer()

	topicspb.RegisterTopicsServer(grpcServer, j.topicServer)
	storagepb.RegisterStorageServer(grpcServer, j.storageServer)
	queuespb.RegisterQueuesServer(grpcServer, j.queueServer)
	secretspb.RegisterSecretManagerServer(grpcServer, j.secretServer)
	sqlpb.RegisterSqlServer(grpcServer, j.sqlServer)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		return err
	}

	// Start the grpc services
	go grpcServer.Serve(lis)

	defer grpcServer.GracefulStop()

	// Run the command and wait for it to exit
	cmdParts := strings.Split(j.cmd, " ")
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)

	// copy the current environment variables
	cmd.Env = os.Environ()

	return cmd.Run()
}

func NewJobMembrane(cmd string, options ...JobMembraneOption) *JobMembrane {
	membrane := &JobMembrane{
		cmd:           cmd,
		topicServer:   topicspb.UnimplementedTopicsServer{},
		storageServer: storagepb.UnimplementedStorageServer{},
		queueServer:   queuespb.UnimplementedQueuesServer{},
		secretServer:  secretspb.UnimplementedSecretManagerServer{},
		sqlServer:     sqlpb.UnimplementedSqlServer{},
	}

	for _, option := range options {
		// Apply the option to the membrane
		option(membrane)
	}

	return membrane
}
