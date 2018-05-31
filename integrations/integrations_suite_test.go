package integrations_test

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/greenplum-db/gp-common-go-libs/gplog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

func TestCommands(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Tests Suite")
}

var (
	cliBinaryPath            string
	hubBinaryPath            string
	agentBinaryPath          string
	userPreviousPathVariable string
	port                     int
	testlog                  *gbytes.Buffer
)

var _ = BeforeSuite(func() {
	var err error
	cliBinaryPath, err = Build("github.com/greenplum-db/gpupgrade/cli") // if you want build flags, do a separate Build() in a specific integration test
	Expect(err).NotTo(HaveOccurred())
	cliDirectoryPath := path.Dir(cliBinaryPath)

	hubBinaryPath, err = Build("github.com/greenplum-db/gpupgrade/hub")
	Expect(err).NotTo(HaveOccurred())
	hubDirectoryPath := path.Dir(hubBinaryPath)

	agentBinaryPath, err = Build("github.com/greenplum-db/gpupgrade/agent")
	Expect(err).NotTo(HaveOccurred())
	// move the agent binary into the hub directory and rename to match expected name
	renamedAgentBinaryPath := filepath.Join(hubDirectoryPath, "/gpupgrade_agent")
	err = os.Rename(agentBinaryPath, renamedAgentBinaryPath)
	Expect(err).NotTo(HaveOccurred())

	// hub gets built as "hub", but rename for integration tests that expect
	// "gpupgrade_hub" to be on the path
	renamedHubBinaryPath := hubDirectoryPath + "/gpupgrade_hub"
	err = os.Rename(hubBinaryPath, renamedHubBinaryPath)
	Expect(err).NotTo(HaveOccurred())
	hubBinaryPath = renamedHubBinaryPath

	// put the gpupgrade_hub on the path don't need to rename the cli nor put
	// it on the path: integration tests should use RunCommand() below
	userPreviousPathVariable = os.Getenv("PATH")
	os.Setenv("PATH", cliDirectoryPath+":"+hubDirectoryPath+":"+userPreviousPathVariable)

	testlog = gbytes.NewBuffer()
	testlogger := gplog.NewLogger(GinkgoWriter, GinkgoWriter, testlog, "gbytes.Buffer", gplog.LOGINFO, "testProgram")
	gplog.SetLogger(testlogger)
})

var _ = BeforeEach(func() {
	port = 7527
	killAll()
})

var _ = AfterSuite(func() {
	/* for a developer who runs `make integration` and then goes on to manually
	* test things out they should start their own up under a different HOME dir
	* setting than what ginkgo has been using */
	killAll()
	CleanupBuildArtifacts()
})

func runCommand(args ...string) *Session {
	// IMPORTANT TEST INFO: exec.Command forks and runs in a separate process,
	// which has its own Golang context; any mocks/fakes you set up in
	// the test context will NOT be meaningful in the new exec.Command context.
	cmd := exec.Command(cliBinaryPath, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GPUPGRADE_HUB_PORT=%d", port))
	session, err := Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	<-session.Exited

	return session
}

func killAll() {
	pkillCmd := exec.Command("pkill", "-9", "gpupgrade_*")
	pkillCmd.Run()
}

func runStatusUpgrade() string {
	return string(runCommand("status", "upgrade").Out.Contents())
}

func checkPortIsAvailable(port int) bool {
	t := time.After(2 * time.Second)
	select {
	case <-t:
		fmt.Println("timed out")
		break
	default:
		cmd := exec.Command("/bin/sh", "-c", "'lsof | grep "+strconv.Itoa(port)+"'")
		err := cmd.Run()
		output, _ := cmd.CombinedOutput()
		if _, ok := err.(*exec.ExitError); ok && string(output) == "" {
			return true
		}

		time.Sleep(250 * time.Millisecond)
	}

	return false
}
