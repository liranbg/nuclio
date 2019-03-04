package kickstart

import (
	"testing"
	"time"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/test/suite"
	"github.com/nuclio/nuclio/pkg/processor/trigger/cron"

	"github.com/stretchr/testify/suite"

)

type KickStart struct {
	kickstart
}

func (k *kickstart) AllocateWorkerAndSubmitEvent(event nuclio.Event,
	functionLogger logger.Logger,
	timeout time.Duration) (response interface{}, submitError error, processError error) {
	switch string(event.GetBody()) {
	case "submitError":
		return nil, errors.New("submitError"), nil
	case "processError":
		return nil, nil, errors.New("processError")
	default:
		return event, nil, nil
	}
}

type TestSuite struct {
	processorsuite.TestSuite
	trigger KickStart
}

func (suite *TestSuite) SetupSuite() {
	suite.TestSuite.SetupSuite()
}

func (suite *TestSuite) TearDownSuite() {
	suite.TestSuite.TearDownTest()
}

func (suite *TestSuite) SetupTest() {
	suite.trigger = KickStart{}
	suite.trigger.Logger = suite.Logger.GetChild("kickstart")
	suite.trigger.configuration = &Configuration{Events: []cron.Event{}}
}

func (suite *TestSuite) TestStartMultiEvents() {
	var checkpoint functionconfig.Checkpoint

	var tests = []struct {
		input    []cron.Event
		expected error
	}{
		{[]cron.Event{}, nil},
		{[]cron.Event{{Body: "first"}}, nil},
		{[]cron.Event{{Body: "first"}, {Body: "second"}}, nil},
		{[]cron.Event{{Body: "submitError"}}, nil},
		{[]cron.Event{{Body: "processError"}}, nil},
	}

	for _, test := range tests {
		suite.trigger.configuration.Events = test.input
		err := suite.trigger.Start(checkpoint)
		suite.Assert().Equal(test.expected, err)
	}
}

func TestKickStartSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
