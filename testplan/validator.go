package testplan

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/celestiaorg/knuu/pkg/names"
	"github.com/celestiaorg/testwave/pkg/worker"
	"github.com/sirupsen/logrus"
)

const (
	envNodeType                    = "NODE_TYPE"
	nodeTypeValidator              = "VALIDATOR"
	waitForOtherNodesRetryInterval = 5 * time.Second
	waitForFirstBlockInterval      = 2 * time.Second
	P2PListeningPort               = 26656

	// followed by the UID of the worker
	msgGenesisFileIDPrefix = "GENESIS_FILE_ID_"
	msgSeedPrefix          = "SEED_"
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
	defer gfr.Close()

	fid, err := w.Minio.Push(ctx, gfr)
	if err != nil {
		return err
	}

	if err := w.Message.Set(msgGenesisFileIDPrefix+w.UID, fid); err != nil {
		return err
	}

	// Now start the network in another thread
	go func() {
		cmd := exec.CommandContext(ctx, "celestia-appd", "start", "--log_level", "error")
		cmd.Stdout = nil // Discard stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logrus.Errorf("failed to start celestia-appd: %v", err)
		}
	}()

	if err := waitForFirstBlock(ctx); err != nil {
		return err
	}

	// Get the Peer ID and share it with others
	seed, err := validatorSeed(ctx, w)
	if err != nil {
		return err
	}

	if err := w.Message.Set(msgSeedPrefix+w.UID, seed); err != nil {
		return err
	}

	// Keep waiting forever
	<-(chan struct{})(nil)

	return nil
}

func validatorSeed(ctx context.Context, w *worker.Worker) (string, error) {
	cmd := exec.CommandContext(ctx, "celestia-appd", "tendermint", "show-node-id")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command: %v, combined output: %s", err, output)
	}

	ipAddr, err := w.LocalIPAddress()
	if err != nil {
		return "", fmt.Errorf("failed to get local IP address: %v", err)
	}

	pid := strings.TrimSpace(string(output))
	seed := fmt.Sprintf("%s@%s:%d", pid, ipAddr, P2PListeningPort)

	return seed, nil
}

func waitForServerReady() {
	ticker := time.NewTicker(waitForFirstBlockInterval)
	for range ticker.C {
		resp, err := http.Get("http://localhost:26657")
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return
		}
	}
}

func waitForFirstBlock(ctx context.Context) error {
	waitForServerReady()
	ticker := time.NewTicker(waitForFirstBlockInterval)
	defer ticker.Stop()
	for {
		cmd := exec.CommandContext(ctx, "sh", "-c", "celestia-appd query block | jq -r '.block.header.height'")
		res, err := cmd.CombinedOutput()

		if err != nil {
			// if the error is exit status 1, it means the block is not there yet
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				<-ticker.C
				continue
			}

			return fmt.Errorf("command failed: %s, error: %v", cmd.String(), err)
		}

		// Check for stderr and handle it if needed
		if len(res) == 0 {
			return fmt.Errorf("waiting for the first block: no output received")
		}

		heightStr := strings.TrimSpace(string(res))
		if heightStr == "null" {
			<-ticker.C
			continue
		}
		logrus.Infof("Current block height: %s", heightStr)
		height, err := strconv.ParseInt(heightStr, 10, 64)
		if err != nil {
			return err
		}
		if height >= 1 {
			return nil
		}
		<-ticker.C
	}
}

func genesisLocalPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return path.Join(homeDir, "/.celestia-app/config/genesis.json"), nil
}
