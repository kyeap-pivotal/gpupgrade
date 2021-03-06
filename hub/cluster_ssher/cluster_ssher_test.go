package cluster_ssher_test

import (
	"os"
	"path/filepath"

	"github.com/greenplum-db/gpupgrade/testutils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/greenplum-db/gpupgrade/hub/cluster_ssher"
	"github.com/pkg/errors"
)

var _ = Describe("ClusterSsher", func() {
	var (
		errChan       chan error
		outChan       chan []byte
		commandExecer *testutils.FakeCommandExecer
	)
	BeforeEach(func() {
		errChan = make(chan error, 2)
		outChan = make(chan []byte, 2)
		commandExecer = &testutils.FakeCommandExecer{}
		commandExecer.SetOutput(&testutils.FakeCommand{
			Err: errChan,
			Out: outChan,
		})
	})

	// TODO: The story that is in charge of removing the cluster_ssher
	// needs to:
	// 1. move the VerifySoftware() into the hub_check_seginstall file.
	// 2. rewrite these tests to use the checklist manager attached to the hub.
	// 3. Use the localExecers attached to the clusters and not use the
	//    commandExecer in clusterssher
	// 4. rewrite these tests in hub_check_seginstall file.
	// 5. same thing for the start-agents.

	XDescribe("VerifySoftware", func() {
		It("indicates that it is in progress, failed on the hub filesystem", func() {
			outChan <- []byte("stdout/stderr message")
			errChan <- errors.New("host not found")

			cw := testutils.NewMockChecklistManager()
			clusterSsher := cluster_ssher.NewClusterSsher(cw, newSpyAgentPinger(), commandExecer.Exec)
			Expect(cw.IsPending("seginstall")).To(BeTrue())
			clusterSsher.VerifySoftware([]string{"doesnt matter"})

			Expect(cw.WasReset("seginstall")).To(BeTrue())
			Expect(cw.IsFailed("seginstall")).To(BeTrue())
		})

		It("indicates that it is in progress, completed on the hub filesystem", func() {
			outChan <- []byte("completed")

			cw := testutils.NewMockChecklistManager()
			clusterSsher := cluster_ssher.NewClusterSsher(cw, newSpyAgentPinger(), commandExecer.Exec)
			Expect(cw.IsPending("seginstall")).To(BeTrue())
			clusterSsher.VerifySoftware([]string{"doesnt matter"})

			Expect(commandExecer.Command()).To(Equal("ssh"))
			pathToAgent := filepath.Join(os.Getenv("GPHOME"), "bin", "gpupgrade_agent")
			Expect(commandExecer.Args()).To(Equal([]string{
				"-o",
				"StrictHostKeyChecking=no",
				"doesnt matter",
				"ls",
				pathToAgent,
			}))

			Expect(cw.WasReset("seginstall")).To(BeTrue())
			Expect(cw.IsComplete("seginstall")).To(BeTrue())
		})
	})

	Describe("Start", func() {
		It("starts the agents", func() {
			outChan <- []byte("stdout/stderr message")
			errChan <- errors.New("host not found")

			cw := testutils.NewMockChecklistManager()
			clusterSsher := cluster_ssher.NewClusterSsher(cw, newSpyAgentPinger(), commandExecer.Exec)
			Expect(cw.IsPending("start-agents")).To(BeTrue())
			clusterSsher.Start([]string{"doesnt matter"})

			Expect(commandExecer.Command()).To(Equal("ssh"))
			pathToGreenplumPathScript := filepath.Join(os.Getenv("GPHOME"), "greenplum_path.sh")
			pathToAgent := filepath.Join(os.Getenv("GPHOME"), "bin", "gpupgrade_agent")
			Expect(commandExecer.Args()).To(Equal([]string{
				"-o",
				"StrictHostKeyChecking=no",
				"doesnt matter",
				"sh -c '. " + pathToGreenplumPathScript + " ; " + pathToAgent + " --daemonize'",
			}))

			Expect(cw.WasReset("start-agents")).To(BeTrue())
			Expect(cw.IsComplete("start-agents")).To(BeTrue())
		})
	})
})

type spyAgentPinger struct{}

func newSpyAgentPinger() *spyAgentPinger {
	return &spyAgentPinger{}
}

func (s *spyAgentPinger) PingPollAgents() error {
	return nil
}
