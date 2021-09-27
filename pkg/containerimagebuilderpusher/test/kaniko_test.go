// +build test_integration
// +build test_kube

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/containerimagebuilderpusher"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	"github.com/nuclio/nuclio/pkg/platform/kube/test"

	"github.com/stretchr/testify/suite"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KanikoTestSuite struct {
	test.KubeTestSuite

	nginxPodName       string
	functionWorkingDir string
}

func (suite *KanikoTestSuite) SetupTest() {
	suite.KubeTestSuite.SetupTest()
	var err error

	// we use a directory that can be volumized onto a pod
	suite.functionWorkingDir, err = ioutil.TempDir(suite.GetNuclioHostSourceDir(), "nuclio-kaniko-test-*")
	suite.Require().NoError(err)

	kubePlatform := suite.Platform.(*kube.Platform)
	suite.nginxPodName = "nginx-server"
	suite.createNginxServerPod()

	// build images with kaniko
	newContainerBuilderPusherConfiguration := containerimagebuilderpusher.NewContainerBuilderConfiguration()
	newContainerBuilderPusherConfiguration.Kind = containerimagebuilderpusher.ContainerBuilderKindKaniko
	newContainerBuilderPusherConfiguration.CreateFunctionTarSymlinkOntoNginxAssetsDir = false
	newContainerBuilderPusherConfiguration.KanikoImage = "gcr.io/kaniko-project/executor:v1.6.0"
	newContainerBuilderPusherConfiguration.BusyBoxImage = "busybox:1.33.1"
	newContainerBuilderPusherConfiguration.InsecurePullRegistry = true
	newContainerBuilderPusherConfiguration.InsecurePushRegistry = true
	newContainerBuilderPusherConfiguration.NginxAssetsURL = fmt.Sprintf(
		"http://%s:80/assets/tar", suite.nginxPodName)
	suite.PlatformConfiguration.ContainerBuilderConfiguration = newContainerBuilderPusherConfiguration
	kubePlatform.ContainerBuilder, err = containerimagebuilderpusher.NewClient(kubePlatform.Logger,
		suite.PlatformConfiguration.ContainerBuilderConfiguration,
		suite.KubeClientSet)
	suite.Require().NoError(err)
}

func (suite *KanikoTestSuite) TearDownTest() {
	suite.ExecuteKubectl([]string{"delete", "svc", suite.nginxPodName}, nil)
	suite.ExecuteKubectl([]string{"delete", "pod", suite.nginxPodName, "--grace-period=0"}, nil)
	err := os.RemoveAll(suite.functionWorkingDir)
	suite.Require().NoError(err)

	suite.KubeTestSuite.TearDownTest()
}

func (suite *KanikoTestSuite) TestBuildSanity() {
	functionName := "build-with-kaniko"

	// compile function config
	createFunctionOptions := suite.CompileCreateFunctionOptions(functionName)

	// deploy function
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// function is running - build succeeded
		suite.Require().Equal(functionconfig.FunctionStateReady, deployResult.FunctionStatus.State)
		return true
	})
}

func (suite *KanikoTestSuite) CompileCreateFunctionOptions(functionName string) *platform.CreateFunctionOptions {
	createFunctionOptions := suite.KubeTestSuite.CompileCreateFunctionOptions(functionName)
	createFunctionOptions.FunctionConfig.Spec.Build.TempDir = suite.functionWorkingDir
	createFunctionOptions.FunctionConfig.Spec.Build.Registry = suite.RegistryURL
	return createFunctionOptions
}

func (suite *KanikoTestSuite) createNginxServerPod() {
	overrides := fmt.Sprintf(`{
		"apiVersion": "v1",
		"spec": {
			"containers": [
			  {
				"name": "%s",
				"image": "nginx:latest",
				"ports": [
				  {
					"containerPort": 80
				  }
				],
				"volumeMounts": [
				  {
					"mountPath": "/usr/share/nginx/html/assets",
					"name": "assets"
				  }
				]
			  }
			],
			"volumes": [
			  {
				"name": "assets",
				"hostPath": {
				  "path": "%s",
				  "type": "DirectoryOrCreate"
				}
			  }
			]
		  }
		}`, suite.nginxPodName, suite.functionWorkingDir)
	compactedBuffer := &bytes.Buffer{}
	err := json.Compact(compactedBuffer, []byte(overrides))
	suite.Require().NoError(err)

	_, err = suite.ExecuteKubectl([]string{"run", suite.nginxPodName, "--expose"}, map[string]string{
		"labels":    "nuclio.io/app=test-nginx-server",
		"image":     "nginx:latest",
		"port":      "80",
		"overrides": fmt.Sprintf("'%s'", compactedBuffer.String()),
	})
	suite.Require().NoError(err)

	// wait for pod to be run
	err = common.RetryUntilSuccessful(3*time.Minute, time.Second, func() bool {
		pod, getPodErr := suite.KubeClientSet.CoreV1().Pods(suite.Namespace).Get(suite.nginxPodName, metav1.GetOptions{})
		suite.Require().NoError(getPodErr)
		return pod.Status.Phase == v1.PodRunning
	})
	suite.Require().NoError(err)
}

func TestKanikoTestSuite(t *testing.T) {
	if testing.Short() {
		return
	}
	suite.Run(t, new(KanikoTestSuite))

}
