package testplan

import (
	"context"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/celestiaorg/knuu/pkg/names"
	"github.com/celestiaorg/testwave/pkg/worker"
	"github.com/sirupsen/logrus"
)

const (
	envNodeType                    = "NODE_TYPE"
	nodeTypeValidator              = "VALIDATOR"
	genesisFileID                  = "GENESIS_FILE_ID"
	waitForOtherNodesRetryInterval = 5 * time.Second
)

var validatorUID string

func init() {
	var err error
	validatorUID, err = names.NewRandomK8("validator")
	if err != nil {
		logrus.Fatalf("failed to generate validator UID: %v", err)
	}
}

func validatorWorkerSetup() *worker.Worker {
	return &worker.Worker{
		UID: validatorUID,
		// key is the name of the Env variable, and the value is the value
		Envs: map[string]string{
			"KEY":       "VALUE",
			envNodeType: nodeTypeValidator,
		},

		// you can mount files into each worker this way
		// the key is the path on the host, and the value is the path in the container
		// the file will be copied to the container before the test starts
		// it uses ConfigMap to mount the files, so the file size is vert limited
		// to have a better performance or for large files, you should copy them into the
		// docker image in the Dockerfile
		Files: map[string]string{
			"./resources/validator.sh": "/opt/validator.sh",
		},
	}
}

func iAmValidatorNode() bool {
	return os.Getenv(envNodeType) == nodeTypeValidator
}

func validatorWorkerRun(w *worker.Worker) error {
	logrus.Infof("Running the test according to playbook %s", Name)
	logrus.Infof("Validator Worker UID: %v", w.UID)

	ctx := context.TODO()

	// Prepare the genesis environment for validator
	cmd := exec.CommandContext(ctx, "sh", "/opt/validator.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Share the genesis file with other nodes
	genesisPath, err := genesisLocalPath()
	if err != nil {
		return err
	}
	gfr, err := os.Open(genesisPath)
	if err != nil {
		return err
	}
	fid, err := w.Minio.Push(ctx, gfr)
	if err != nil {
		return err
	}

	if err := w.Message.Set(genesisFileID, fid); err != nil {
		return err
	}

	// Now start the network
	cmd = exec.CommandContext(ctx, "celestia-appd", "start")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func genesisLocalPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return path.Join(homeDir, "/.celestia-app/config/genesis.json"), nil
}
