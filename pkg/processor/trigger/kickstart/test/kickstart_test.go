package test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/trigger/cron"
	"github.com/nuclio/nuclio/pkg/processor/trigger/test"

	"github.com/stretchr/testify/suite"
)

const triggerName = "test_kickstart"

type TestSuite struct {
	processorsuite.TestSuite
	functionPath string
	events       []cron.Event
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()

	// use the python event recorder
	suite.functionPath = path.Join(suite.GetTestFunctionsDir(),
		"common",
		"event-recorder",
		"python",
		"event_recorder.py")

	suite.events = []cron.Event{
		{Body: "First", Headers: map[string]interface{}{"h1": "v1", "h2": "v2"}},
		{Body: "Second", Headers: map[string]interface{}{"h3": "v3", "h4": "v2"}},
	}
}

func (suite *TestSuite) TestPostEventPythonInterval() {
	createFunctionOptions := suite.getKickStartDeployOptions()
	suite.invokeEventRecorder(createFunctionOptions)
}

func (suite *TestSuite) getKickStartDeployOptions() *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("event_recorder",
		suite.GetFunctionPath(path.Join("event_recorder_python")))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Meta.Name = "kickstart-trigger-test"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = suite.functionPath
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{}
	createFunctionOptions.FunctionConfig.Spec.Triggers[triggerName] = functionconfig.Trigger{
		Kind: "kickstart",
		Class: "async",
		Attributes: map[string]interface{}{
			"events": []map[string]interface{}{
				{
					"body":    suite.events[0].Body,
					"headers": suite.events[0].Headers,
				},
				{
					"body":    suite.events[1].Body,
					"headers": suite.events[1].Headers,
				},
			},

		},
	}

	return createFunctionOptions
}

func (suite *TestSuite) invokeEventRecorder(createFunctionOptions *platform.CreateFunctionOptions) {
	suite.DeployFunction(createFunctionOptions, func(deployResult *platform.CreateFunctionResult) bool {

		// Wait 10 seconds to give time for the container to kickstart the events
		time.Sleep(10 * time.Second)

		// Set http request url
		url := fmt.Sprintf("http://%s:%d", suite.GetTestHost(), deployResult.Port)

		// read the events from the function
		httpResponse, err := http.Get(url)
		suite.Require().NoError(err, "Failed to read events from function: %s; err: %v", url, err)

		marshalledResponseBody, err := ioutil.ReadAll(httpResponse.Body)
		suite.Require().NoError(err, "Failed to read response body")

		// unmarshal the body into a list
		var receivedEvents []triggertest.Event

		err = json.Unmarshal(marshalledResponseBody, &receivedEvents)
		suite.Require().NoError(err, "Failed to unmarshal response. Response: %s", marshalledResponseBody)

		recievedEventsAmount := len(receivedEvents)

		suite.Logger.DebugWith("Received events from container",
			"expected", len(suite.events),
			"actual", recievedEventsAmount)
		suite.Require().Equal(recievedEventsAmount, len(suite.events), "Not all events were kick started")

		// compare bodies / headers
		for idx, receivedEvent := range receivedEvents {
			suite.Require().Equal(suite.events[idx].Body, receivedEvent.Body)
			suite.Require().Equal(suite.events[idx].Headers, receivedEvent.Headers)
		}

		return true
	})
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(TestSuite))
}
