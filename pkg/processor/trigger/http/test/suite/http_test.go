/*
Copyright 2018 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package httpsuite

import (
	"path"
	"strings"
	"testing"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/stretchr/testify/suite"
	"github.com/valyala/fasthttp"
)

type HTTPTestSuite struct {
	TestSuite
	triggerName string
}

func (suite *HTTPTestSuite) SetupTest() {
	suite.TestSuite.SetupTest()
	suite.triggerName = "testHTTP"
}

func (suite *HTTPTestSuite) TestCORS() {
	allowHeaders := "Accept, Content-Length, Content-Type, X-nuclio-log-level"
	allowMethods := "OPTIONS, GET, POST, HEAD, PUT"
	allowOrigin := "foo.bar"
	createFunctionOptions := suite.getHTTPDeployOptions()
	createFunctionOptions.FunctionConfig.Spec.Triggers[suite.triggerName].Attributes["cors"] = map[string]interface{}{
		"enabled":      true,
		"allowOrigin":  allowOrigin,
		"allowHeaders": strings.Split(allowHeaders, ", "),
		"allowMethods": strings.Split(allowMethods, ", "),
	}
	validPreflightResponseStatusCode := fasthttp.StatusOK
	invalidPreflightResponseStatusCode := fasthttp.StatusBadRequest
	suite.DeployFunctionAndRequests(createFunctionOptions,
		[]*Request{

			// Happy flow
			{
				RequestMethod: "OPTIONS",
				RequestHeaders: map[string]interface{}{
					"Origin":                         allowOrigin,
					"Access-Control-Request-Method":  "POST",
					"Access-Control-Request-Headers": "X-nuclio-log-level",
				},
				ExpectedResponseStatusCode: &validPreflightResponseStatusCode,
				ExpectedResponseHeadersValues: map[string][]string{
					"Access-Control-Allow-Methods": {allowMethods},
					"Access-Control-Allow-Headers": {allowHeaders},
					"Access-Control-Allow-Origin":  {allowOrigin},
				},
			},

			// Disallowed request method
			{
				RequestMethod: "OPTIONS",
				RequestHeaders: map[string]interface{}{
					"Origin":                        allowOrigin,
					"Access-Control-Request-Method": "ABC",
				},
				ExpectedResponseStatusCode: &invalidPreflightResponseStatusCode,
			},
		})
}

func (suite *HTTPTestSuite) getHTTPDeployOptions() *platform.CreateFunctionOptions {
	createFunctionOptions := suite.GetDeployOptions("event_recorder",
		suite.GetFunctionPath(path.Join("event_recorder_python")))

	createFunctionOptions.FunctionConfig.Spec.Runtime = "python"
	createFunctionOptions.FunctionConfig.Meta.Name = "http-trigger-test"
	createFunctionOptions.FunctionConfig.Spec.Build.Path = path.Join(suite.GetTestFunctionsDir(),
		"common",
		"event-recorder",
		"python",
		"event_recorder.py")
	createFunctionOptions.FunctionConfig.Spec.Triggers = map[string]functionconfig.Trigger{
		suite.triggerName: {
			Kind:       "http",
			Attributes: map[string]interface{}{},
		},
	}
	return createFunctionOptions
}

func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		return
	}

	suite.Run(t, new(HTTPTestSuite))
}
