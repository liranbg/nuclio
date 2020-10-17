package dockerclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/pkg/archive"
	"github.com/nuclio/nuclio/pkg/cmdrunner"
	"github.com/nuclio/nuclio/pkg/common"
	"golang.org/x/sync/errgroup"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/builder"
	"github.com/docker/docker/client"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

// ApiClient is a docker client that uses docker api
type ApiClient struct {
	logger    logger.Logger
	apiClient client.APIClient

	redactedValues     []string
	buildTimeout       time.Duration
	buildRetryInterval time.Duration
}

// NewApiClient creates a new docker api client
func NewApiClient(parentLogger logger.Logger) (*ApiClient, error) {
	var err error

	// Use DOCKER_HOST to set the url to the docker server.
	// Use DOCKER_API_VERSION to set the version of the API to reach, leave empty for latest.
	// Use DOCKER_CERT_PATH to load the tls certificates from.
	// Use DOCKER_TLS_VERIFY to enable or disable TLS verification, off by default.
	apiClientInstance, err := client.NewEnvClient()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create new api client instance")
	}

	newClient := &ApiClient{
		logger:             parentLogger.GetChild("docker_api"),
		apiClient:          apiClientInstance,
		buildTimeout:       1 * time.Hour,
		buildRetryInterval: 3 * time.Second,
	}

	return newClient, nil
}

func (ac *ApiClient) Build(buildOptions *BuildOptions) error {
	ac.logger.DebugWith("Building image", "buildOptions", buildOptions)

	// if context dir is not passed, use the dir containing the dockerfile
	if buildOptions.ContextDir == "" && buildOptions.DockerfilePath != "" {
		buildOptions.ContextDir = path.Dir(buildOptions.DockerfilePath)
	}

	// user can only specify context directory
	if buildOptions.DockerfilePath == "" && buildOptions.ContextDir != "" {
		buildOptions.DockerfilePath = path.Join(buildOptions.ContextDir, "Dockerfile")
	}

	return ac.build(buildOptions)
}

func (ac *ApiClient) CopyObjectsFromImage(imageName string,
	objectsToCopy map[string]string,
	allowCopyErrors bool) error {

	// create container from image
	containerID, err := ac.createContainer(imageName)
	if err != nil {
		return errors.Wrapf(err, "Failed to create container from %s", imageName)
	}

	// delete once done copying objects
	defer ac.RemoveContainer(containerID) // nolint: errcheck

	// copy objects
	errGroup := errgroup.Group{}
	for objectImagePath, objectLocalPath := range objectsToCopy {
		errGroup.Go(func() error {
			outFile, err := os.Create(objectLocalPath)
			if err != nil {
				return errors.Wrap(err, "Failed to create file")
			}

			defer outFile.Close() // nolint: errcheck

			reader, _, err := ac.apiClient.CopyFromContainer(context.TODO(), containerID, objectImagePath)

			_, err = io.Copy(outFile, reader)
			if err != nil {
				return errors.Wrapf(err, "Failed to copy container contents to file %s", objectLocalPath)
			}

			return nil
		})
	}

	if err := errGroup.Wait(); err != nil && !allowCopyErrors {
		return errors.Wrap(err, "Failed to copy objects from image")
	}

	return nil
}

func (ac *ApiClient) PushImage(imageName string, registryURL string) error {
	taggedImage := common.CompileImageName(registryURL, imageName)

	ac.logger.InfoWith("Pushing image", "from", imageName, "to", taggedImage)

	if err := ac.apiClient.ImageTag(context.TODO(), imageName, taggedImage); err != nil {
		return errors.Wrap(err, "Failed to tag image")
	}

	responseBody, err := ac.apiClient.ImagePush(context.TODO(), taggedImage, types.ImagePushOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to push image")
	}
	return responseBody.Close()

}

func (ac *ApiClient) PullImage(imageURL string) error {
	ac.logger.InfoWith("Pulling image", "imageName", imageURL)
	responseBody, err := ac.apiClient.ImagePull(context.TODO(), imageURL, types.ImagePullOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to pull image")
	}
	return responseBody.Close()
}

func (ac *ApiClient) RemoveImage(imageName string) error {
	ac.logger.InfoWith("Removing image", "imageName", imageName)
	deletedImages, err := ac.apiClient.ImageRemove(context.TODO(), imageName, types.ImageRemoveOptions{
		Force: true,
	})
	if err != nil {
		return errors.Wrap(err, "Failed to remove image")
	}
	ac.logger.DebugWith("Deleted image", "deletedImages", deletedImages)
	return err
}

func (ac *ApiClient) RunContainer(imageName string, runOptions *RunOptions) (string, error) {
	containerCreatedBody, err := ac.apiClient.ContainerCreate(context.TODO(),
		&container.Config{
			Image: imageName,

		},
		&container.HostConfig{
AutoRemove: runOptions.Remove,

		},
		&network.NetworkingConfig{

		}, runOptions.ContainerName)
	if err != nil {
		return "", errors.Wrap(err, "Failed to create container")
	}
	if err := ac.apiClient.ContainerStart(context.TODO(), containerCreatedBody.ID, types.ContainerStartOptions{


	}); err != nil {
		return "", errors.Wrap(err, "Failed to start container")
	}

	if runOptions.Attach {
		// TODO wait for container
		// TODO Attach
	}

	for localPort, dockerPort := range runOptions.Ports {
		if localPort == RunOptionsNoPort {
			dockerArguments = append(dockerArguments, fmt.Sprintf("-p %d", dockerPort))
		} else {
			dockerArguments = append(dockerArguments, fmt.Sprintf("-p %d:%d", localPort, dockerPort))
		}
	}

	if runOptions.RestartPolicy != nil && runOptions.RestartPolicy.Name != RestartPolicyNameNo {

		// sanity check
		// https://docs.docker.com/engine/reference/run/#restart-policies---restart
		// combining --restart (restart policy) with the --rm (clean up) flag results in an error.
		if runOptions.Remove {
			return "", errors.Errorf("Cannot combine restart policy with container removal")
		}
		restartMaxRetries := runOptions.RestartPolicy.MaximumRetryCount
		restartPolicy := fmt.Sprintf("--restart %s", runOptions.RestartPolicy.Name)
		if runOptions.RestartPolicy.Name == RestartPolicyNameOnFailure && restartMaxRetries >= 0 {
			restartPolicy += fmt.Sprintf(":%d", restartMaxRetries)
		}
		dockerArguments = append(dockerArguments, restartPolicy)
	}

	if runOptions.GPUs != "" {
		// TODO not supported?
	}


	if runOptions.Network != "" {
		dockerArguments = append(dockerArguments, fmt.Sprintf("--net %s", runOptions.Network))
	}

	if runOptions.Labels != nil {
		for labelName, labelValue := range runOptions.Labels {
			dockerArguments = append(dockerArguments,
				fmt.Sprintf("--label %s='%s'", labelName, c.replaceSingleQuotes(labelValue)))
		}
	}

	if runOptions.Env != nil {
		for envName, envValue := range runOptions.Env {
			dockerArguments = append(dockerArguments, fmt.Sprintf("--env %s='%s'", envName, envValue))
		}
	}

	if runOptions.Volumes != nil {
		for volumeHostPath, volumeContainerPath := range runOptions.Volumes {
			dockerArguments = append(dockerArguments,
				fmt.Sprintf("--volume %s:%s ", volumeHostPath, volumeContainerPath))
		}
	}

	if len(runOptions.MountPoints) > 0 {
		for _, mountPoint := range runOptions.MountPoints {
			readonly := ""
			if !mountPoint.RW {
				readonly = ",readonly"
			}
			dockerArguments = append(dockerArguments,
				fmt.Sprintf("--mount source=%s,destination=%s%s",
					mountPoint.Source,
					mountPoint.Destination,
					readonly))
		}
	}

	if runOptions.RunAsUser != nil || runOptions.RunAsGroup != nil {
		userStr := ""
		if runOptions.RunAsUser != nil {
			userStr += fmt.Sprintf("%d", *runOptions.RunAsUser)
		}
		if runOptions.RunAsGroup != nil {
			userStr += fmt.Sprintf(":%d", *runOptions.RunAsGroup)
		}

		dockerArguments = append(dockerArguments, fmt.Sprintf("--user %s", userStr))
	}

	if runOptions.FSGroup != nil {
		dockerArguments = append(dockerArguments, fmt.Sprintf("--group-add %d", *runOptions.FSGroup))
	}

	runResult, err := c.cmdRunner.Run(
		&cmdrunner.RunOptions{LogRedactions: c.redactedValues},
		"docker run %s %s %s",
		strings.Join(dockerArguments, " "),
		imageName,
		runOptions.Command)

	if err != nil {
		c.logger.WarnWith("Failed to run container",
			"err", err,
			"stdout", runResult.Output,
			"stderr", runResult.Stderr)

		return "", err
	}

	// if user requested, set stdout / stderr
	if runOptions.Stdout != nil {
		*runOptions.Stdout = runResult.Output
	}

	if runOptions.Stderr != nil {
		*runOptions.Stderr = runResult.Stderr
	}

	stdoutLines := strings.Split(runResult.Output, "\n")
	lastStdoutLine := c.getLastNonEmptyLine(stdoutLines, 0)

	// make sure there are no spaces in the ID, as normally we expect this command to only produce container ID
	if strings.Contains(lastStdoutLine, " ") {

		// if the image didn't exist prior to calling RunContainer, it will be pulled implicitly which will
		// cause additional information to be outputted. if runOptions.ImageMayNotExist is false,
		// this will result in an error.
		if !runOptions.ImageMayNotExist {
			return "", fmt.Errorf("Output from docker command includes more than just ID: %s", lastStdoutLine)
		}

		// if the implicit image pull was allowed and actually happened, the container ID will appear in the
		// second to last line ¯\_(ツ)_/¯
		lastStdoutLine = c.getLastNonEmptyLine(stdoutLines, 1)
	}

	return lastStdoutLine, err
}

func (ac *ApiClient) ExecInContainer(containerID string, execOptions *ExecOptions) error {
	envArgument := ""
	if execOptions.Env != nil {
		for envName, envValue := range execOptions.Env {
			envArgument += fmt.Sprintf("--env %s='%s' ", envName, envValue)
		}
	}

	runResult, err := c.cmdRunner.Run(
		&cmdrunner.RunOptions{LogRedactions: c.redactedValues},
		"docker exec %s %s %s",
		envArgument,
		containerID,
		execOptions.Command)

	if err != nil {
		c.logger.DebugWith("Failed to execute command in container",
			"err", err,
			"stdout", runResult.Output,
			"stderr", runResult.Stderr)

		return err
	}

	// if user requested, set stdout / stderr
	if execOptions.Stdout != nil {
		*execOptions.Stdout = runResult.Output
	}

	if execOptions.Stderr != nil {
		*execOptions.Stderr = runResult.Stderr
	}

	return nil
}

func (ac *ApiClient) RemoveContainer(containerID string) error {
	return ac.apiClient.ContainerRemove(context.TODO(), containerID, types.ContainerRemoveOptions{
		Force: true,
	})
}

func (ac *ApiClient) StopContainer(containerID string) error {

	// docker defaults
	containerStopTimeout := 10 * time.Second
	return ac.apiClient.ContainerStop(context.TODO(), containerID, &containerStopTimeout)
}

func (ac *ApiClient) StartContainer(containerID string) error {
	return ac.apiClient.ContainerStart(context.TODO(), containerID, types.ContainerStartOptions{})
}

func (ac *ApiClient) GetContainerLogs(containerID string) (string, error) {
	containerLogs, err := ac.apiClient.ContainerLogs(context.TODO(), containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return "", errors.Wrap(err, "Failed to get container logs")
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, containerLogs)
	return buf.String(), err
}

func (ac *ApiClient) GetContainers(options *GetContainerOptions) ([]Container, error) {
	c.logger.DebugWith("Getting containers", "options", options)

	stoppedContainersArgument := ""
	if options.Stopped {
		stoppedContainersArgument = "--all "
	}

	nameFilterArgument := ""
	if options.Name != "" {
		nameFilterArgument = fmt.Sprintf(`--filter "name=^/%s$" `, options.Name)
	}

	idFilterArgument := ""
	if options.ID != "" {
		idFilterArgument = fmt.Sprintf(`--filter "id=%s"`, options.ID)
	}

	labelFilterArgument := ""
	for labelName, labelValue := range options.Labels {
		labelFilterArgument += fmt.Sprintf(`--filter "label=%s=%s" `,
			labelName,
			labelValue)
	}

	runResult, err := c.runCommand(nil,
		"docker ps --quiet %s %s %s %s",
		stoppedContainersArgument,
		idFilterArgument,
		nameFilterArgument,
		labelFilterArgument)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get containers")
	}

	containerIDsAsString := runResult.Output
	if len(containerIDsAsString) == 0 {
		return []Container{}, nil
	}

	runResult, err = c.runCommand(nil,
		"docker inspect %s",
		strings.Replace(containerIDsAsString, "\n", " ", -1))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to inspect containers")
	}

	containersInfoString := runResult.Output

	var containersInfo []Container

	// parse the result
	if err := json.Unmarshal([]byte(containersInfoString), &containersInfo); err != nil {
		return nil, errors.Wrap(err, "Failed to parse inspect response")
	}

	return containersInfo, nil
}

func (ac *ApiClient) GetContainerEvents(containerName string, since string, until string) ([]string, error) {
	filtersArgs := filters.NewArgs()
	filtersArgs.Add("container", containerName)
	eventsChan, errChan := ac.apiClient.Events(context.TODO(), types.EventsOptions{
		Since:   since,
		Until:   until,
		Filters: filtersArgs,
	})

	var events []string
	select {
	case err := <-errChan:
		return nil, errors.Wrap(err, "Failed to get container events")
	case eventMessage := <-eventsChan:
		events = append(events, fmt.Sprintf("%s", eventMessage.Status))
	case <-time.After(5 * time.Minute):
		return events, errors.New("Timeout waiting for container events")
	}
	return events, nil
}

func (ac *ApiClient) AwaitContainerHealth(containerID string, timeout *time.Duration) error {
	timedOut := false

	containerHealthy := make(chan error, 1)
	var timeoutChan <-chan time.Time

	// if no timeout is given, create a channel that we'll never send on
	if timeout == nil {
		timeoutChan = make(<-chan time.Time, 1)
	} else {
		timeoutChan = time.After(*timeout)
	}

	go func() {

		// start with a small interval between health checks, increasing it gradually
		inspectInterval := 100 * time.Millisecond

		for !timedOut {
			containers, err := c.GetContainers(&GetContainerOptions{
				ID:      containerID,
				Stopped: true,
			})
			if err == nil && len(containers) > 0 {
				container := containers[0]

				// container is healthy
				if container.State.Health.Status == "healthy" {
					containerHealthy <- nil
					return
				}

				// container exited, bail out
				if container.State.Status == "exited" {
					containerHealthy <- errors.Errorf("Container exited with status: %d", container.State.ExitCode)
					return
				}

				// container is dead, bail out
				// https://docs.docker.com/engine/reference/commandline/ps/#filtering
				if container.State.Status == "dead" {
					containerHealthy <- errors.New("Container seems to be dead")
					return
				}

				// wait a bit before retrying
				c.logger.DebugWith("Container not healthy yet, retrying soon",
					"timeout", timeout,
					"containerID", containerID,
					"containerState", container.State,
					"nextCheckIn", inspectInterval)
			}

			time.Sleep(inspectInterval)

			// increase the interval up to a cap
			if inspectInterval < 800*time.Millisecond {
				inspectInterval *= 2
			}
		}
	}()

	// wait for either the container to be healthy or the timeout
	select {
	case err := <-containerHealthy:
		if err != nil {
			return errors.Wrapf(err, "Container %s is not healthy", containerID)
		}
		c.logger.DebugWith("Container is healthy", "containerID", containerID)
	case <-timeoutChan:
		timedOut = true

		containerLogs, err := c.GetContainerLogs(containerID)
		if err != nil {
			c.logger.ErrorWith("Container wasn't healthy within timeout (failed to get logs)",
				"containerID", containerID,
				"timeout", timeout,
				"err", err)
		} else {
			c.logger.WarnWith("Container wasn't healthy within timeout",
				"containerID", containerID,
				"timeout", timeout,
				"logs", containerLogs)
		}

		return errors.New("Container wasn't healthy in time")
	}

	return nil
}

func (ac *ApiClient) LogIn(logInOptions *LogInOptions) error {
	if _, err := ac.apiClient.RegistryLogin(context.TODO(), types.AuthConfig{
		Username:      logInOptions.Username,
		Password:      logInOptions.Password,
		ServerAddress: logInOptions.URL,
	}); err != nil {
		return errors.Wrapf(err,
			"Failed to login to registry '%s' using username '%s'",
			logInOptions.URL,
			logInOptions.Username)
	}
	return nil
}

func (ac *ApiClient) CreateNetwork(options *CreateNetworkOptions) error {
	ac.logger.InfoWith("Creating network", "networkName", options.Name)
	_, err := ac.apiClient.NetworkCreate(context.TODO(), options.Name, types.NetworkCreate{})
	if err != nil {
		return errors.Wrap(err, "Failed to create network")
	}
	return err
}

func (ac *ApiClient) DeleteNetwork(networkName string) error {
	ac.logger.InfoWith("Deleting network", "networkName", networkName)
	return ac.apiClient.NetworkRemove(context.TODO(), networkName)
}

func (ac *ApiClient) CreateVolume(options *CreateVolumeOptions) error {
	ac.logger.InfoWith("Creating volume", "volumeName", options.Name)
	if _, err := ac.apiClient.VolumeCreate(context.TODO(), volume.VolumesCreateBody{
		Name: options.Name,
	}); err != nil {
		return errors.Wrap(err, "Failed to create volume")
	}
	return nil
}

func (ac *ApiClient) DeleteVolume(volumeName string) error {
	ac.logger.InfoWith("Deleting volume", "volumeName", volumeName)
	return ac.apiClient.VolumeRemove(context.TODO(), volumeName, false)
}

func (ac *ApiClient) Save(imageName string, outPath string) error {
	image, err := ac.apiClient.ImageSave(context.TODO(), []string{imageName})
	if err != nil {
		return errors.Wrap(err, "Failed to save image")
	}
	outPathFile, err := os.Create(outPath)
	if err != nil {
		return errors.Wrap(err, "Failed to create file")
	}
	if _, err := io.Copy(outPathFile, image); err != nil {
		return errors.Wrap(err, "Failed to save image to file")
	}
	return nil
}

func (ac *ApiClient) Load(inPath string) error {
	in, err := os.Open(inPath)
	if err != nil {
		return errors.Wrap(err, "Failed to open input path")
	}
	if _, err := ac.apiClient.ImageLoad(context.TODO(), in, false); err != nil {
		return errors.Wrap(err, "Failed to load image")
	}
	return nil
}

func (ac *ApiClient) getLastNonEmptyLine(lines []string, offset int) string {

	numLines := len(lines)

	// protect ourselves from overflows
	if offset >= numLines {
		offset = numLines - 1
	} else if offset < 0 {
		offset = 0
	}

	// iterate backwards over the lines
	for idx := numLines - 1 - offset; idx >= 0; idx-- {
		if lines[idx] != "" {
			return lines[idx]
		}
	}

	return ""
}

func (ac *ApiClient) replaceSingleQuotes(input string) string {
	return strings.Replace(input, "'", "’", -1)
}

func (ac *ApiClient) resolveDockerBuildNetwork() string {

	// may contain none as a value
	networkInterface := os.Getenv("NUCLIO_DOCKER_BUILD_NETWORK")
	if networkInterface == "" {
		networkInterface = common.GetEnvOrDefaultString("NUCLIO_BUILD_USE_HOST_NET", "host")
	}
	switch networkInterface {
	case "host":
		fallthrough
	case "default":
		fallthrough
	case "none":
		return fmt.Sprintf("--network %s", networkInterface)
	default:
		return ""
	}
}

func (ac *ApiClient) build(buildOptions *BuildOptions) error {
	var lastBuildErr error
	retryOnErrorMessages := []string{

		// when one of the underlying image is gone (from cache)
		"^No such image: sha256:",

		// when overlay image is gone (from disk)
		"^failed to get digest sha256:",
	}

	buildArgs := map[string]*string{}
	for argName, argValue := range buildOptions.BuildArgs {
		buildArgs[argName] = &argValue
	}

	contextDir, relDockerFilePath, err := builder.GetContextFromLocalDir(buildOptions.ContextDir,
		buildOptions.DockerfilePath)
	if err != nil {
		return errors.Wrap(err, "Failed to get context from local dir")
	}

	tarStream, err := archive.Tar(contextDir, archive.Uncompressed)
	if err != nil {
		return errors.Wrap(err, "Failed to create archive tar")
	}
	defer tarStream.Close() // nolint: errcheck

	tarSumContext, err := builder.MakeTarSumContext(tarStream)
	if err != nil {
		return errors.Wrap(err, "Failed to make tar sum context")
	}
	defer tarSumContext.Close() // nolint: errcheck

	// retry build on predefined errors that occur during race condition and collisions between
	// shared onbuild layers
	common.RetryUntilSuccessfulOnErrorPatterns(ac.buildTimeout, // nolint: errcheck
		ac.buildRetryInterval,
		retryOnErrorMessages,
		func() string { // nolint: errcheck
			buildResponse, err := ac.apiClient.ImageBuild(context.TODO(), tarStream, types.ImageBuildOptions{
				ForceRemove: true,
				Tags: []string{
					buildOptions.Image,
				},
				NetworkMode: ac.resolveDockerBuildNetwork(),
				Dockerfile:  relDockerFilePath,
				NoCache:     buildOptions.NoCache,
				BuildArgs:   buildArgs,
			})

			// preserve error
			lastBuildErr = err

			if err != nil {
				errStack := errors.GetErrorStackString(err, 10)
				if buildResponse.Body == nil {
					return errStack
				}
				buildResponseBody, _ := ioutil.ReadAll(buildResponse.Body)
				return string(buildResponseBody) + "\n" + errStack
			}

			return ""
		})
	return lastBuildErr
}

func (ac *ApiClient) createContainer(imageName string) (string, error) {
	var lastCreateContainerError error
	var containerID string
	retryOnErrorMessages := []string{

		// sometimes, creating the container fails on not finding the image because
		// docker was on high load and did not get to update its cache
		fmt.Sprintf("^Unable to find image '%s.*' locally", imageName),
	}

	// retry in case docker daemon is under high load
	// e.g.: between build and create, docker would need to update its cached manifest of built images
	common.RetryUntilSuccessfulOnErrorPatterns(10*time.Second, // nolint: errcheck
		2*time.Second,
		retryOnErrorMessages,
		func() string {
			containerCreateCreatedBody, err := ac.apiClient.ContainerCreate(context.TODO(),
				&container.Config{
					Image: imageName,
					Cmd:   strslice.StrSlice{"/bin/sh"},
				},
				&container.HostConfig{

				},
				&network.NetworkingConfig{

				}, "")

			// preserve error
			lastCreateContainerError = err

			if err != nil {
				errStack := errors.GetErrorStackString(err, 10)
				if containerCreateCreatedBody.Warnings == nil {
					return errStack

				}
				return strings.Join(containerCreateCreatedBody.Warnings, "\n") + "\n" + errStack
			}
			containerID = containerCreateCreatedBody.ID
			return ""
		})

	return containerID, lastCreateContainerError
}
