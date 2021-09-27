package containerimagebuilderpusher

import (
	"github.com/nuclio/logger"
	"k8s.io/client-go/kubernetes"
)

func NewClient(logger logger.Logger,
	configuration *ContainerBuilderConfiguration,
	kubeClientSet kubernetes.Interface) (BuilderPusher, error) {

	switch configuration.Kind {
	case ContainerBuilderKindKaniko:
		return NewKaniko(logger, kubeClientSet, configuration)
	case ContainerBuilderKindDocker:
		return NewDocker(logger, configuration)
	case ContainerBuilderKindNop:
		return NewNop(logger, configuration)

	default:
		logger.WarnWith("Unknown container image builder pusher kind was given, defaulting to Docker",
			"kind", configuration.Kind)
		return NewDocker(logger, configuration)
	}
}
