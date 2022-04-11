package containerimagebuilderpusher

import (
	"strconv"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"

	"github.com/nuclio/errors"
	"k8s.io/api/core/v1"
)

type BuilderKind string

const (
	BuilderKindKaniko = "kaniko"
	BuilderKindNop    = "nop"
	BuilderKindDocker = "docker"
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

	// kaniko options
	KanikoOptions *KanikoOptions

	// kaniko pod runtime configuration
	Affinity                *v1.Affinity
	NodeSelector            map[string]string
	NodeName                string
	PriorityClassName       string
	Tolerations             []v1.Toleration
	ReadinessTimeoutSeconds int
}

type KanikoOptions struct {

	// Set this flag to cache copy layers.
	// https://github.com/GoogleContainerTools/kaniko#--cache-copy-layers
	CacheCopyLayers bool

	// Set this to false in order to prevent tar compression for cached layers
	// https://github.com/GoogleContainerTools/kaniko#--compressed-caching
	CompressedCaching bool

	// This flag takes a single snapshot of the filesystem at the end of the build,
	// so only one layer will be appended to the base image.
	// https://github.com/GoogleContainerTools/kaniko#--single-snapshot
	SingleSnapshot bool

	// Use this flag to set how kaniko will snapshot the filesystem.
	// - full (default): the full file contents and metadata are considered when snapshotting.
	//   This is the least performant option, but also the most robust.
	// - redo: the file mtime, size, mode, owner uid and gid will be considered when snapshotting.
	//   This may be up to 50% faster than "full", particularly if your project has a large number files.
	// - time: only file mtime will be considered when snapshotting
	// https://github.com/GoogleContainerTools/kaniko#--snapshotmode
	SnapshotMode string

	// Set this flag to the number of retries that should happen for the extracting an image filesystem. Defaults to 0.
	// https://github.com/GoogleContainerTools/kaniko#--image-fs-extract-retry
	ImageFSExtractRetry int

	// Set this flag to strip timestamps out of the built image and make it reproducible.
	// https://github.com/GoogleContainerTools/kaniko#--reproducible
	Reproducible bool

	// Use the experimental run implementation for detecting changes without requiring file system snapshots.
	// In some cases, this may improve build performance by 75%.
	// https://github.com/GoogleContainerTools/kaniko#--use-new-run
	RunV2 bool
}

type ContainerBuilderConfiguration struct {
	Kind                                 string
	BusyBoxImage                         string
	KanikoImage                          string
	KanikoImagePullPolicy                string
	JobPrefix                            string
	JobDeletionTimeout                   time.Duration
	DefaultRegistryCredentialsSecretName string
	DefaultBaseRegistryURL               string
	DefaultOnbuildRegistryURL            string
	CacheRepo                            string
	InsecurePushRegistry                 bool
	InsecurePullRegistry                 bool
	PushImagesRetries                    int
}

func NewContainerBuilderConfiguration() (*ContainerBuilderConfiguration, error) {
	var containerBuilderConfiguration ContainerBuilderConfiguration
	var err error

	// if some of the parameters are undefined, try environment variables
	if containerBuilderConfiguration.Kind == "" {
		containerBuilderConfiguration.Kind = common.GetEnvOrDefaultString("NUCLIO_CONTAINER_BUILDER_KIND",
			"docker")
	}
	if containerBuilderConfiguration.BusyBoxImage == "" {
		containerBuilderConfiguration.BusyBoxImage = common.GetEnvOrDefaultString("NUCLIO_BUSYBOX_CONTAINER_IMAGE",
			"busybox:1.31")
	}
	if containerBuilderConfiguration.KanikoImage == "" {
		containerBuilderConfiguration.KanikoImage = common.GetEnvOrDefaultString("NUCLIO_KANIKO_CONTAINER_IMAGE",
			"gcr.io/kaniko-project/executor:v1.7.0")
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

	containerBuilderConfiguration.PushImagesRetries, err =
		strconv.Atoi(common.GetEnvOrDefaultString("NUCLIO_KANIKO_PUSH_IMAGES_RETRIES", "3"))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve number of push images retries")
	}

	jobDeletionTimeout := common.GetEnvOrDefaultString("NUCLIO_KANIKO_JOB_DELETION_TIMEOUT", "30m")
	containerBuilderConfiguration.JobDeletionTimeout, err = time.ParseDuration(jobDeletionTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse job deletion timeout duration")
	}

	return &containerBuilderConfiguration, nil
}
