package testplan

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/celestiaorg/knuu/pkg/names"
	"github.com/celestiaorg/testwave/pkg/worker"
	"github.com/sirupsen/logrus"
)

const (
	nodeTypeFullNode              = "FULL_NODE"
	envValidatorUID               = "VALIDATOR_UID"
	waitForValidatorRetryInterval = 5 * time.Second
)

var fullNodeUID string

func init() {
	var err error
	fullNodeUID, err = names.NewRandomK8("fullnode")
	if err != nil {
		logrus.Fatalf("failed to generate full node UID: %v", err)
	}
}

func fullNodeWorkerSetup() *worker.Worker {
	return &worker.Worker{
		UID: fullNodeUID,
		// key is the name of the Env variable, and the value is the value
		Envs: map[string]string{
			"KEY":           "VALUE",
			envNodeType:     nodeTypeFullNode,
			envValidatorUID: validatorUID,
		},

		Files: map[string]string{
			"./resources/fullnode.sh": "/opt/fullnode.sh",
		},
	}
}

func iAmFullNodeNode() bool {
	return os.Getenv(envNodeType) == nodeTypeFullNode
}

func fullNodeWorkerRun(w *worker.Worker) error {
	logrus.Infof("Running the test according to playbook %s", Name)
	logrus.Infof("FullNode Worker UID: %v", w.UID)

	plCancel, err := w.BitTwister.SetPacketLossRate(10)
	if err != nil {
		return err
	}
	defer plCancel()

	validatorUID, ok := os.LookupEnv(envValidatorUID)
	if !ok {
		return fmt.Errorf("failed to get validator UID from ENV var `%s`", envValidatorUID)
	}
	logrus.Infof("Validator UID: %s", validatorUID)

	logrus.Info("Getting the Validator IP...")
	ctx := context.TODO()
	validatorIp, err := w.Message.GetIPWaiting(ctx, validatorUID)
	if err != nil {
		return err
	}
	logrus.Infof("Validator IP: %s", validatorIp)

	cmd := exec.CommandContext(ctx, "sh", "/opt/fullnode.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	// Receive the genesis file from the validator node
	if err := getGenesisFileFromValidator(w); err != nil {
		return err
	}

	// Now start the node
	cmd = exec.CommandContext(ctx, "celestia-appd", "start")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func getGenesisFileFromValidator(w *worker.Worker) error {
	value, err := w.Message.ReceiveGlobal(genesisFileID)
	if err != nil {
		return err
	}

	contentID, ok := value.(string)
	if !ok {
		return fmt.Errorf("received invalid genesis file: %v", value)
	}
	logrus.Infof("Received genesis file: %s", contentID)

	// Download the genesis file
	ctx := context.TODO()
	genesisFileContent, err := w.Minio.Pull(ctx, contentID)
	if err != nil {
		return fmt.Errorf("failed to download the genesis file: %v", err)
	}

	genesisPath, err := genesisLocalPath()
	if err != nil {
		return err
	}
	file, err := os.Create(genesisPath)
	if err != nil {
		return fmt.Errorf("failed to create the genesis file: %v", err)

	}
	defer file.Close()

	if _, err := io.Copy(file, genesisFileContent); err != nil {
		return fmt.Errorf("failed to write the genesis file: %v", err)
	}
	return nil
}
