package containerimagebuilderpusher

import "github.com/nuclio/nuclio/pkg/processor/build/runtime"

// BuilderPusher is a builder of container images
type BuilderPusher interface {

	// GetKind returns the kind (docker/kaniko)
	GetKind() string

	// BuildAndPushContainerImage builds container image and pushes it into container registry
	BuildAndPushContainerImage(buildOptions *BuildOptions, namespace string) error

	// GetOnbuildStages gets onbuild stage for multistage builds
	GetOnbuildStages(onbuildArtifacts []runtime.Artifact) ([]string, error)

	// TransformOnbuildArtifactPaths changes Onbuild artifact paths depending on the type of the builder used
	TransformOnbuildArtifactPaths(onbuildArtifacts []runtime.Artifact) (map[string]string, error)

	// GetBaseImageRegistry returns base image registry
	GetBaseImageRegistry(registry string) string

	// GetOnbuildImageRegistry returns onbuild base registry
	GetOnbuildImageRegistry(registry string) string

	// GetDefaultRegistryCredentialsSecretName returns secret with credentials to push/pull from docker registry
	GetDefaultRegistryCredentialsSecretName() string
}
