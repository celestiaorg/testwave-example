package testplan

import (
	"github.com/celestiaorg/knuu/pkg/names"
	"github.com/celestiaorg/testwave/pkg/playbook"
	"github.com/celestiaorg/testwave/pkg/worker"
)

// The name must match with this regex: [a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
const Name = "testplan01"

type Playbook struct {
	nodeSets []*playbook.NodeSet
}

func (p *Playbook) Name() string {
	return Name
}

func (p *Playbook) NodeSets() []*playbook.NodeSet {
	return p.nodeSets
}

// Setup is called by the dispatcher before the test starts.
// It is used to prepare the environment for the test.
func (p *Playbook) Setup() error {
	podUID, err := names.NewRandomK8(Name + "-consensus")
	if err != nil {
		return err
	}
	p.nodeSets = []*playbook.NodeSet{
		{
			UID: podUID,
			Workers: []*worker.Worker{
				validatorWorkerSetup(),
				fullNodeWorkerSetup(),
			},
		},
	}
	return nil
}

// RunWorker is called by every worker node to run the test.
// The test logic goes here.
func (p *Playbook) RunWorker(w *worker.Worker) error {
	if iAmValidatorNode() {
		return validatorWorkerRun(w)
	}

	if iAmFullNodeNode() {
		return fullNodeWorkerRun(w)
	}

	return nil
}
