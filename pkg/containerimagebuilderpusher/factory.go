package containerimagebuilderpusher

import (
	"github.com/nuclio/logger"
	"k8s.io/client-go/kubernetes"
)

func NewClient(logger logger.Logger,
	configuration *ContainerBuilderConfiguration,
	kubeClientSet kubernetes.Interface) (BuilderPusher, error) {

	var kind ContainerBuilderKind
	if configuration == nil {
		kind = ContainerBuilderKindDocker
	} else {
		kind = configuration.Kind
	}

	switch kind {
	case ContainerBuilderKindKaniko:
		return NewKaniko(logger, kubeClientSet, configuration)
	case ContainerBuilderKindDocker:
		return NewDocker(logger, configuration)
	case ContainerBuilderKindNop:
		return NewNop(logger, configuration)

	default:
		logger.WarnWith("No explicit container image builder pusher client was given, defaulting to Docker")
		return NewDocker(logger, configuration)
	}
}
