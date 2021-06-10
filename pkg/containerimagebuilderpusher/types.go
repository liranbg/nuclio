package containerimagebuilderpusher

import (
	"fmt"
	"os"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
)

// BuildOptions are options for building a container image
type BuildOptions struct {
	Image               string
	ContextDir          string
	TempDir             string
	DockerfileInfo      *runtime.ProcessorDockerfileInfo
	NoCache             bool
	Pull                bool
	NoBaseImagePull     bool
	BuildArgs           map[string]string
	RegistryURL         string
	SecretName          string
	OutputImageFile     string
	BuildTimeoutSeconds int64
}

type ContainerBuilderKind string

const (
	ContainerBuilderKindDocker = "docker"
	ContainerBuilderKindKaniko = "kaniko"
	ContainerBuilderKindNop    = "nop"
)

type ContainerBuilderConfiguration struct {
	Kind                                       ContainerBuilderKind
	BusyBoxImage                               string
	KanikoImage                                string
	KanikoImagePullPolicy                      string
	CreateFunctionTarSymlinkOntoNginxAssetsDir bool
	NginxAssetsURL                             string
	JobPrefix                                  string
	DefaultRegistryCredentialsSecretName       string
	DefaultBaseRegistryURL                     string
	DefaultOnbuildRegistryURL                  string
	CacheRepo                                  string
	InsecurePushRegistry                       bool
	InsecurePullRegistry                       bool
}

func NewContainerBuilderConfiguration() *ContainerBuilderConfiguration {
	containerBuilderConfiguration := ContainerBuilderConfiguration{}

	// if some of the parameters are undefined, try environment variables
	if containerBuilderConfiguration.Kind == "" {
		containerBuilderConfiguration.Kind = ContainerBuilderKind(common.GetEnvOrDefaultString("NUCLIO_CONTAINER_BUILDER_KIND",
			"docker"))
	}
	if containerBuilderConfiguration.BusyBoxImage == "" {
		containerBuilderConfiguration.BusyBoxImage = common.GetEnvOrDefaultString("NUCLIO_BUSYBOX_CONTAINER_IMAGE",
			"busybox:1.31")
	}
	if containerBuilderConfiguration.KanikoImage == "" {
		containerBuilderConfiguration.KanikoImage = common.GetEnvOrDefaultString("NUCLIO_KANIKO_CONTAINER_IMAGE",
			"gcr.io/kaniko-project/executor:v0.17.1")
	}
	if containerBuilderConfiguration.KanikoImagePullPolicy == "" {
		containerBuilderConfiguration.KanikoImagePullPolicy = common.GetEnvOrDefaultString(
			"NUCLIO_KANIKO_CONTAINER_IMAGE_PULL_POLICY", "IfNotPresent")
	}
	if containerBuilderConfiguration.JobPrefix == "" {
		containerBuilderConfiguration.JobPrefix = common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_JOB_NAME_PREFIX",
			"kanikojob")
	}

	containerBuilderConfiguration.InsecurePushRegistry =
		common.GetEnvOrDefaultBool("NUCLIO_KANIKO_INSECURE_PUSH_REGISTRY", false)
	containerBuilderConfiguration.InsecurePullRegistry =
		common.GetEnvOrDefaultBool("NUCLIO_KANIKO_INSECURE_PULL_REGISTRY", false)

	containerBuilderConfiguration.DefaultRegistryCredentialsSecretName =
		common.GetEnvOrDefaultString("NUCLIO_REGISTRY_CREDENTIALS_SECRET_NAME", "")

	if containerBuilderConfiguration.DefaultBaseRegistryURL == "" {
		containerBuilderConfiguration.DefaultBaseRegistryURL =
			common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_DEFAULT_BASE_REGISTRY_URL", "")
	}

	if containerBuilderConfiguration.DefaultOnbuildRegistryURL == "" {
		containerBuilderConfiguration.DefaultOnbuildRegistryURL =
			common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_DEFAULT_ONBUILD_REGISTRY_URL", "quay.io")
	}

	containerBuilderConfiguration.CacheRepo =
		common.GetEnvOrDefaultString("NUCLIO_DASHBOARD_KANIKO_CACHE_REPO", "")

	containerBuilderConfiguration.CreateFunctionTarSymlinkOntoNginxAssetsDir =
		common.GetEnvOrDefaultBool("NUCLIO_CREATE_FUNCTION_TAR_SYMLINK_ONTO_NGINX_ASSETS_DIR", true)

	nuclioDashboardDeploymentName := os.Getenv("NUCLIO_DASHBOARD_DEPLOYMENT_NAME")
	containerBuilderConfiguration.NginxAssetsURL = common.GetEnvOrDefaultString("NUCLIO_NGINX_ASSETS_URL",
		fmt.Sprintf("http://%s:8070/assets", nuclioDashboardDeploymentName))

	return &containerBuilderConfiguration
}
