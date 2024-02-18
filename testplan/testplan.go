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

var _ = playbook.Playbook(&Playbook{})

func (p *Playbook) Name() string {
	return Name
}

func (p *Playbook) NodeSets() []*playbook.NodeSet {
	return p.nodeSets
}

// Setup is called by the dispatcher before the test starts.
// It is used to prepare the environment for the test.
func (p *Playbook) Setup() error {
	podUID1, err := names.NewRandomK8(Name + "-nodeset1")
	if err != nil {
		return err
	}
	podUID2, err := names.NewRandomK8(Name + "-nodeset2")
	if err != nil {
		return err
	}
	//TODO: Since I figured out that containers in a pod share the same network stack and
	// as we have to use privileged mode and NET_ADMIN capability, I can't run two same
	// applications on the same pods because there will be conflicts with the ports
	// and traffic shape of one container will affect the others.
	p.nodeSets = []*playbook.NodeSet{
		{
			UID: podUID1,
			Workers: []*worker.Worker{
				validatorWorkerSetup(),
			},
		},
		{
			UID: podUID2,
			Workers: []*worker.Worker{
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
