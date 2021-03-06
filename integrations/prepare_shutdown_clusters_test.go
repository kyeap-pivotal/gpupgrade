package integrations_test

import (
	"errors"
	"time"

	"github.com/greenplum-db/gp-common-go-libs/testhelper"
	"github.com/greenplum-db/gpupgrade/hub/cluster_ssher"
	"github.com/greenplum-db/gpupgrade/hub/services"
	"github.com/greenplum-db/gpupgrade/hub/upgradestatus"
	pb "github.com/greenplum-db/gpupgrade/idl"
	"github.com/greenplum-db/gpupgrade/testutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"
)

var _ = Describe("prepare shutdown-clusters", func() {
	var (
		hub             *services.Hub
		mockAgent       *testutils.MockAgentServer
		commandExecer   *testutils.FakeCommandExecer
		outChan         chan []byte
		errChan         chan error
		clusterPair     *services.ClusterPair
		testExecutorOld *testhelper.TestExecutor
		testExecutorNew *testhelper.TestExecutor
		cm              *testutils.MockChecklistManager
	)

	BeforeEach(func() {

		var err error
		port, err = testutils.GetOpenPort()
		Expect(err).ToNot(HaveOccurred())

		var agentPort int
		mockAgent, agentPort = testutils.NewMockAgentServer()

		conf := &services.HubConfig{
			CliToHubPort:   port,
			HubToAgentPort: agentPort,
			StateDir:       testStateDir,
		}
		outChan = make(chan []byte, 5)
		errChan = make(chan error, 5)

		commandExecer = &testutils.FakeCommandExecer{}
		commandExecer.SetOutput(&testutils.FakeCommand{
			Out: outChan,
			Err: errChan,
		})
		clusterPair = testutils.InitClusterPairFromDB()
		testExecutorOld = &testhelper.TestExecutor{}
		testExecutorNew = &testhelper.TestExecutor{}
		clusterPair.OldCluster.Executor = testExecutorOld
		clusterPair.NewCluster.Executor = testExecutorNew
		clusterPair.OldBinDir = "/tmpOld"
		clusterPair.NewBinDir = "/tmpNew"
		cm = testutils.NewMockChecklistManager()
		clusterSsher := cluster_ssher.NewClusterSsher(
			cm,
			services.NewPingerManager(conf.StateDir, 500*time.Millisecond),
			commandExecer.Exec,
		)
		hub = services.NewHub(clusterPair, grpc.DialContext, commandExecer.Exec, conf, clusterSsher, cm)
		go hub.Start()
	})

	AfterEach(func() {
		hub.Stop()
		mockAgent.Stop()
	})

	It("updates status PENDING, RUNNING then COMPLETE if successful", func() {
		mockAgent.StatusConversionResponse = &pb.CheckConversionStatusReply{
			Statuses: []string{},
		}

		Expect(cm.IsPending(upgradestatus.SHUTDOWN_CLUSTERS)).To(BeTrue())

		prepareShutdownClustersSession := runCommand("prepare", "shutdown-clusters", "--old-bindir", clusterPair.OldBinDir, "--new-bindir", clusterPair.NewBinDir)
		Eventually(prepareShutdownClustersSession).Should(Exit(0))

		Expect(testExecutorOld.NumExecutions).To(Equal(2))
		Expect(testExecutorOld.LocalCommands[0]).To(ContainSubstring("pgrep"))
		Expect(testExecutorOld.LocalCommands[1]).To(ContainSubstring(clusterPair.OldBinDir + "/gpstop -a"))

		Expect(testExecutorNew.NumExecutions).To(Equal(2))
		Expect(testExecutorNew.LocalCommands[0]).To(ContainSubstring("pgrep"))
		Expect(testExecutorNew.LocalCommands[1]).To(ContainSubstring(clusterPair.NewBinDir + "/gpstop -a"))

		Expect(cm.IsComplete(upgradestatus.SHUTDOWN_CLUSTERS)).To(BeTrue())
	})

	It("updates status to FAILED if it fails to run", func() {
		mockAgent.StatusConversionResponse = &pb.CheckConversionStatusReply{
			Statuses: []string{},
		}

		Expect(cm.IsPending(upgradestatus.SHUTDOWN_CLUSTERS)).To(BeTrue())

		testExecutorOld.ErrorOnExecNum = 2
		testExecutorNew.ErrorOnExecNum = 2
		testExecutorOld.LocalError = errors.New("stop failed")
		testExecutorNew.LocalError = errors.New("stop failed")

		prepareShutdownClustersSession := runCommand("prepare", "shutdown-clusters", "--old-bindir", clusterPair.OldBinDir, "--new-bindir", clusterPair.NewBinDir)
		Eventually(prepareShutdownClustersSession).Should(Exit(0))

		Expect(testExecutorOld.NumExecutions).To(Equal(2))
		Expect(testExecutorOld.LocalCommands[0]).To(ContainSubstring("pgrep"))
		Expect(testExecutorOld.LocalCommands[1]).To(ContainSubstring(clusterPair.OldBinDir + "/gpstop -a"))
		Expect(testExecutorOld.NumExecutions).To(Equal(2))
		Expect(testExecutorNew.LocalCommands[0]).To(ContainSubstring("pgrep"))
		Expect(testExecutorNew.LocalCommands[1]).To(ContainSubstring(clusterPair.NewBinDir + "/gpstop -a"))
		Expect(cm.IsFailed(upgradestatus.SHUTDOWN_CLUSTERS)).To(BeTrue())
	})

	It("fails if the --old-bindir or --new-bindir flags are missing", func() {
		prepareShutdownClustersSession := runCommand("prepare", "shutdown-clusters")
		Expect(prepareShutdownClustersSession).Should(Exit(1))
		Expect(string(prepareShutdownClustersSession.Out.Contents())).To(Equal("Required flag(s) \"new-bindir\", \"old-bindir\" have/has not been set\n"))
	})
})
